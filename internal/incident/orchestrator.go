// Package incident wires the PagerDuty client, agent loop, and RCA
// formatter into a single Orchestrator that handles a triggered incident
// end-to-end.
package incident

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vipulvc08/kubecause/internal/agent"
	"github.com/vipulvc08/kubecause/internal/pagerduty"
	"github.com/vipulvc08/kubecause/internal/rca"
)

// Orchestrator runs the full flow for one incident.
type Orchestrator struct {
	pd      *pagerduty.Client
	agent   *agent.Agent
	timeout time.Duration
}

type Option func(*Orchestrator)

// WithTimeout caps the total wall time of a single incident handling.
// A slow LLM or a slow cluster should not tie up a goroutine forever.
func WithTimeout(d time.Duration) Option { return func(o *Orchestrator) { o.timeout = d } }

func New(pd *pagerduty.Client, ag *agent.Agent, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		pd:      pd,
		agent:   ag,
		timeout: 3 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Handle implements pagerduty.Orchestrator.
func (o *Orchestrator) Handle(ctx context.Context, incidentID string) error {
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	log := slog.With("incident_id", incidentID)
	start := time.Now()

	inc, err := o.pd.GetIncident(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("fetch incident: %w", err)
	}
	log = log.With("service", inc.Service.Summary, "urgency", inc.Urgency)
	log.Info("fetched incident", "title", inc.Title)

	summary := buildAgentPrompt(inc)
	rcaText, err := o.agent.Run(ctx, summary)
	if err != nil {
		return fmt.Errorf("agent run: %w", err)
	}

	note := rca.PagerDutyNote(rcaText)
	if err := o.pd.PostNote(ctx, incidentID, note); err != nil {
		return fmt.Errorf("post note: %w", err)
	}

	log.Info("rca posted", "elapsed_ms", time.Since(start).Milliseconds())
	return nil
}

// buildAgentPrompt turns an incident into the user-role prompt the agent
// receives on turn 1. Kept small on purpose — the system prompt in
// internal/agent carries the shape guidance.
func buildAgentPrompt(inc *pagerduty.Incident) string {
	return fmt.Sprintf(
		"A PagerDuty incident has been triggered.\n\n"+
			"Title: %s\n"+
			"Service: %s\n"+
			"Urgency: %s\n"+
			"Status: %s\n"+
			"Created: %s\n\n"+
			"Investigate the cluster for likely root causes and produce an RCA "+
			"following the structure in your system prompt.",
		inc.Title, inc.Service.Summary, inc.Urgency, inc.Status, inc.CreatedAt,
	)
}
