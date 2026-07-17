package tools

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
)

// RolloutHistory returns a Tool that lists recent rollouts of a workload,
// with timestamps and revision numbers. This is the single most valuable
// piece of context in a fresh incident: what changed, and when.
func RolloutHistory() Tool {
	return Tool{
		Spec: llm.ToolSpec{
			Name:        "rollout_history",
			Description: "List recent rollout revisions for a Deployment or StatefulSet, with timestamps. Use this to check whether a recent deploy correlates with the incident.",
			InputSchema: mustSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind":      map[string]any{"type": "string", "enum": []string{"deployment", "statefulset"}},
					"namespace": map[string]any{"type": "string"},
					"name":      map[string]any{"type": "string"},
					"limit":     map[string]any{"type": "integer", "description": "Max revisions to return. Default 10.", "minimum": 1, "maximum": 50},
				},
				"required": []string{"kind", "namespace", "name"},
			}),
		},
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, errors.New("rollout_history: not yet implemented")
		},
	}
}
