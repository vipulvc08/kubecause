package tools

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
)

// PodLogs returns a Tool that fetches logs from a pod, with a bounded
// tail size and time window, and optional access to the previous container
// instance (where crash-loop errors usually live).
func PodLogs() Tool {
	return Tool{
		Spec: llm.ToolSpec{
			Name:        "pod_logs",
			Description: "Fetch recent log lines from a pod. Supports selecting a container and reading the previous crashed container instance. Output is truncated to a bounded number of lines.",
			InputSchema: mustSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace":     map[string]any{"type": "string"},
					"pod":           map[string]any{"type": "string"},
					"container":     map[string]any{"type": "string", "description": "Optional. Container name for multi-container pods."},
					"tail_lines":    map[string]any{"type": "integer", "description": "Max lines to return. Default 500, max 2000.", "minimum": 1, "maximum": 2000},
					"since_seconds": map[string]any{"type": "integer", "description": "Optional. Only lines from the last N seconds.", "minimum": 1},
					"previous":      map[string]any{"type": "boolean", "description": "If true, read the previous crashed container instance."},
				},
				"required": []string{"namespace", "pod"},
			}),
		},
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, errors.New("pod_logs: not yet implemented")
		},
	}
}
