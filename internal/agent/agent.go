// Package agent runs the tool-use loop that turns a PagerDuty incident
// into a written RCA.
//
// The loop is intentionally small: gather evidence via Tools, feed the
// conversation to the LLM, execute any tool calls the LLM asks for,
// repeat until the LLM stops asking or MaxIterations is reached.
package agent

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
	"github.com/vipulvc08/kubecause/internal/tools"
)

const defaultMaxIterations = 10

type Agent struct {
	llm           llm.Client
	toolset       *tools.Registry
	maxIterations int
	systemPrompt  string
}

type Options struct {
	MaxIterations int
	SystemPrompt  string
}

func New(client llm.Client, toolset *tools.Registry, opts Options) *Agent {
	if opts.MaxIterations == 0 {
		opts.MaxIterations = defaultMaxIterations
	}
	if opts.SystemPrompt == "" {
		opts.SystemPrompt = DefaultSystemPrompt
	}
	return &Agent{
		llm:           client,
		toolset:       toolset,
		maxIterations: opts.MaxIterations,
		systemPrompt:  opts.SystemPrompt,
	}
}

// Run executes the tool-use loop for a single incident and returns the
// final RCA text produced by the model. It stops early if the model does
// not request any further tool calls, or when maxIterations is exhausted.
func (a *Agent) Run(ctx context.Context, incidentSummary string) (string, error) {
	if a.llm == nil {
		return "", errors.New("agent: no llm client configured")
	}

	messages := []llm.Message{
		{Role: llm.RoleUser, Text: incidentSummary},
	}

	for i := 0; i < a.maxIterations; i++ {
		resp, err := a.llm.Chat(ctx, llm.ChatRequest{
			System:   a.systemPrompt,
			Messages: messages,
			Tools:    a.toolset.Specs(),
		})
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Text, nil
		}

		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Text:      resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			result, err := a.toolset.Invoke(ctx, call.Name, call.Input)
			if err != nil {
				result = []byte(`{"error":"` + err.Error() + `"}`)
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: call.ID,
				Text:       string(result),
			})
		}
	}

	return "", errors.New("agent: max iterations reached before final answer")
}

// DefaultSystemPrompt shapes the model into a disciplined RCA writer.
// Kept concise on purpose — the tool descriptions carry most of the load.
const DefaultSystemPrompt = `You are kubecause, an on-call assistant that writes root-cause analyses for Kubernetes incidents.

Rules:
1. Only make claims you can back with evidence from tool calls.
2. Prefer specific timestamps, event names, and log lines over generalities.
3. If evidence is inconclusive, say so — do not guess.
4. Write the final RCA in this structure:
   - Summary (1-2 sentences)
   - Likely root cause (with citations)
   - Timeline of relevant events
   - Suggested next steps for the on-call engineer
5. Never suggest changes that require write access to the cluster in the RCA text itself.
   You have read-only access; humans apply fixes.`
