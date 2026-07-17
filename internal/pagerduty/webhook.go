// Package pagerduty implements the PagerDuty webhook receiver and API client.
package pagerduty

import (
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

// WebhookHandler receives incident events from PagerDuty.
//
// Signature verification follows PagerDuty's v3 webhook scheme:
// https://developer.pagerduty.com/docs/webhooks-overview#webhook-signatures
type WebhookHandler struct {
	cfg config.PagerDutyConfig
}

func NewWebhookHandler(cfg config.PagerDutyConfig) *WebhookHandler {
	return &WebhookHandler{cfg: cfg}
}

// Event is the minimal shape of a PagerDuty v3 webhook payload we care about.
// We keep this deliberately loose — PD adds fields over time.
type Event struct {
	Event struct {
		ID         string          `json:"id"`
		EventType  string          `json:"event_type"`
		OccurredAt string          `json:"occurred_at"`
		Data       json.RawMessage `json:"data"`
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

	slog.Info("pagerduty event received",
		"id", evt.Event.ID,
		"type", evt.Event.EventType,
		"occurred_at", evt.Event.OccurredAt)

	// TODO(v0.1): enqueue for the agent loop. For now, ack fast.
	w.WriteHeader(http.StatusAccepted)
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
