// Package pagerduty implements the PagerDuty webhook receiver and REST client.
package pagerduty

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vipulvc08/kubecause/internal/config"
)

// Orchestrator handles a single PagerDuty incident: fetch details, kick
// off the RCA agent loop, post the result back. Injected into the webhook
// handler so the receiver package doesn't take a dependency on the LLM
// or agent packages.
type Orchestrator interface {
	Handle(ctx context.Context, incidentID string) error
}

// WebhookHandler receives incident events from PagerDuty.
//
// Signature verification follows PagerDuty's v3 webhook scheme:
// https://developer.pagerduty.com/docs/webhooks-overview#webhook-signatures
type WebhookHandler struct {
	cfg          config.PagerDutyConfig
	orchestrator Orchestrator
}

func NewWebhookHandler(cfg config.PagerDutyConfig, o Orchestrator) *WebhookHandler {
	return &WebhookHandler{cfg: cfg, orchestrator: o}
}

// Event is the minimal shape of a PagerDuty v3 webhook payload we care about.
type Event struct {
	Event struct {
		ID         string `json:"id"`
		EventType  string `json:"event_type"`
		OccurredAt string `json:"occurred_at"`
		Data       struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"data"`
	} `json:"event"`
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read failed", http.StatusBadRequest)
		return
	}

	if !verifySignature(r.Header.Get("X-PagerDuty-Signature"), h.cfg.WebhookSecret, body) {
		slog.Warn("pagerduty webhook signature mismatch")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var evt Event
	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	log := slog.With(
		"event_id", evt.Event.ID,
		"event_type", evt.Event.EventType,
		"incident_id", evt.Event.Data.ID,
	)
	log.Info("pagerduty event received")

	if !shouldHandle(evt.Event.EventType) {
		log.Debug("ignoring event type")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if h.orchestrator == nil {
		log.Warn("no orchestrator configured — accepting event but not processing")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Dispatch asynchronously — PagerDuty expects a fast 2xx and will
	// retry with backoff on non-2xx. The agent loop can take 10-30s.
	go func(id string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := h.orchestrator.Handle(ctx, id); err != nil {
			log.Error("orchestrator failed", "err", err)
		}
	}(evt.Event.Data.ID)

	w.WriteHeader(http.StatusAccepted)
}

// shouldHandle picks the PagerDuty v3 event types kubecause reacts to.
// We intentionally scope to triggering events only. Resolves and
// acknowledgements do not need an RCA.
func shouldHandle(eventType string) bool {
	switch eventType {
	case "incident.triggered", "incident.escalated", "incident.reopened":
		return true
	}
	return false
}

// verifySignature checks any of the comma-separated HMAC-SHA256 signatures
// in the X-PagerDuty-Signature header against the configured secret.
//
// PagerDuty may send multiple signatures during secret rotation; matching any
// is sufficient. If no secret is configured we skip verification (dev only).
func verifySignature(header, secret string, body []byte) bool {
	if secret == "" {
		return true
	}
	if header == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range strings.Split(header, ",") {
		sig = strings.TrimSpace(sig)
		sig = strings.TrimPrefix(sig, "v1=")
		if hmac.Equal([]byte(sig), []byte(want)) {
			return true
		}
	}
	return false
}
