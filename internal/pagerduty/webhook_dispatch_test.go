package pagerduty

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vipulvc08/kubecause/internal/config"
)

type recordingOrchestrator struct {
	mu      sync.Mutex
	called  []string
	wg      sync.WaitGroup
	errOnce error
}

func (r *recordingOrchestrator) Handle(_ context.Context, id string) error {
	defer r.wg.Done()
	r.mu.Lock()
	r.called = append(r.called, id)
	r.mu.Unlock()
	return r.errOnce
}

func sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "v1=" + hex.EncodeToString(m.Sum(nil))
}

func TestWebhookDispatch_triggered(t *testing.T) {
	secret := "shhh"
	orch := &recordingOrchestrator{}
	orch.wg.Add(1)

	h := NewWebhookHandler(config.PagerDutyConfig{WebhookSecret: secret}, orch)
	srv := httptest.NewServer(h)
	defer srv.Close()

	body := []byte(`{"event":{"id":"E1","event_type":"incident.triggered","data":{"id":"PABCDEF","type":"incident"}}}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(body)))
	req.Header.Set("X-PagerDuty-Signature", sign(secret, body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post err: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}

	// dispatch is async — wait for it
	done := make(chan struct{})
	go func() { orch.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("orchestrator was not called")
	}
	if len(orch.called) != 1 || orch.called[0] != "PABCDEF" {
		t.Errorf("want called=[PABCDEF], got %v", orch.called)
	}
}

func TestWebhookDispatch_ignoredEventType(t *testing.T) {
	secret := "shhh"
	orch := &recordingOrchestrator{}

	h := NewWebhookHandler(config.PagerDutyConfig{WebhookSecret: secret}, orch)
	srv := httptest.NewServer(h)
	defer srv.Close()

	body := []byte(`{"event":{"id":"E2","event_type":"incident.resolved","data":{"id":"P1","type":"incident"}}}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(body)))
	req.Header.Set("X-PagerDuty-Signature", sign(secret, body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post err: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}

	time.Sleep(50 * time.Millisecond)
	if len(orch.called) != 0 {
		t.Errorf("orchestrator should not have been called, got %v", orch.called)
	}
}

func TestWebhookDispatch_badSignature(t *testing.T) {
	orch := &recordingOrchestrator{}
	h := NewWebhookHandler(config.PagerDutyConfig{WebhookSecret: "shhh"}, orch)
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL,
		strings.NewReader(`{"event":{"event_type":"incident.triggered"}}`))
	req.Header.Set("X-PagerDuty-Signature", "v1=nope")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post err: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
	if len(orch.called) != 0 {
		t.Errorf("orchestrator should not have been called")
	}
}
