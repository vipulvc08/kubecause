package pagerduty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetIncident(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/incidents/PABCDEF" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Token token=") {
			t.Errorf("missing Authorization: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != apiAccept {
			t.Errorf("bad Accept: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{
			"incident": {
				"id":"PABCDEF",
				"incident_number":42,
				"title":"payments-api high error rate",
				"status":"triggered",
				"urgency":"high",
				"service": {"id":"PSVC1","summary":"payments-api"}
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithClientBaseURL(srv.URL))
	inc, err := c.GetIncident(context.Background(), "PABCDEF")
	if err != nil {
		t.Fatalf("GetIncident err: %v", err)
	}
	if inc.ID != "PABCDEF" || inc.IncidentNumber != 42 {
		t.Errorf("unexpected incident: %+v", inc)
	}
	if inc.Service.Summary != "payments-api" {
		t.Errorf("service.summary = %q", inc.Service.Summary)
	}
}

func TestPostNote(t *testing.T) {
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/incidents/P1/notes" {
			t.Errorf("unexpected req: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("From") != "oncall@example.com" {
			t.Errorf("missing From header")
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &seenBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"note":{"id":"NX"}}`))
	}))
	defer srv.Close()

	c := NewClient("tok",
		WithClientBaseURL(srv.URL),
		WithFrom("oncall@example.com"),
	)
	if err := c.PostNote(context.Background(), "P1", "hello RCA"); err != nil {
		t.Fatalf("PostNote err: %v", err)
	}
	note, _ := seenBody["note"].(map[string]any)
	if note == nil || note["content"] != "hello RCA" {
		t.Errorf("body did not carry note content: %+v", seenBody)
	}
}

func TestGetIncident_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithClientBaseURL(srv.URL))
	_, err := c.GetIncident(context.Background(), "gone")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got %v", err)
	}
}
