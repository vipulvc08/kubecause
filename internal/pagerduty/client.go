package pagerduty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.pagerduty.com"
	apiAccept      = "application/vnd.pagerduty+json;version=2"
	defaultTimeout = 15 * time.Second
)

// Client is a thin wrapper over the PagerDuty REST v2 API for the two
// endpoints kubecause needs: fetch an incident, post an incident note.
//
// It intentionally does not attempt to be a general-purpose PD client.
// If you need one, use github.com/PagerDuty/go-pagerduty.
type Client struct {
	token   string
	baseURL string
	from    string // email of the user creating notes (some PD accounts require this)
	http    *http.Client
}

type ClientOption func(*Client)

func WithClientBaseURL(u string) ClientOption   { return func(c *Client) { c.baseURL = u } }
func WithHTTPClient(h *http.Client) ClientOption { return func(c *Client) { c.http = h } }
func WithFrom(email string) ClientOption         { return func(c *Client) { c.from = email } }

func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		token:   token,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Incident is a compact view of the fields kubecause reads.
type Incident struct {
	ID          string `json:"id"`
	IncidentNumber int `json:"incident_number"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Urgency     string `json:"urgency"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	Service     struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"service"`
	Body struct {
		Details map[string]any `json:"details"`
	} `json:"body"`
}

// GetIncident fetches a single incident by ID.
func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/incidents/"+id, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Incident Incident `json:"incident"`
	}
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out.Incident, nil
}

// PostNote appends a note to an incident.
//
// PagerDuty accounts vary on whether the "From" header is required.
// When it is, callers must construct the Client with WithFrom(...).
func (c *Client) PostNote(ctx context.Context, incidentID, content string) error {
	body := map[string]any{
		"note": map[string]any{
			"content": content,
		},
	}
	req, err := c.newRequest(ctx, http.MethodPost,
		"/incidents/"+incidentID+"/notes", body)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// --- internals ---

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token token="+c.token)
	req.Header.Set("Accept", apiAccept)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.from != "" {
		req.Header.Set("From", c.from)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pagerduty: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("pagerduty: %s: %s", resp.Status, snippet(raw))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("pagerduty: decode: %w", err)
	}
	return nil
}

func snippet(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
