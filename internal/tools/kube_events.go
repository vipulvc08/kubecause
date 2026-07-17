package tools

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
)

// KubeEvents returns a Tool that lists Kubernetes Events in a namespace,
// optionally filtered to a specific involved object.
//
// v0.1 scaffold: schema + wiring only. The client-go implementation lands
// alongside the first end-to-end incident test.
func KubeEvents() Tool {
	return Tool{
		Spec: llm.ToolSpec{
			Name:        "kube_events",
			Description: "List Kubernetes Events in a namespace. Prefer this over pod_logs when you need the cluster's view of what happened (scheduling failures, image pulls, probe failures).",
			InputSchema: mustSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace": map[string]any{
						"type":        "string",
						"description": "Namespace to search. Required.",
					},
					"involved_object_name": map[string]any{
						"type":        "string",
						"description": "Optional. Restrict to events for this object (e.g. a pod name).",
					},
					"since_seconds": map[string]any{
						"type":        "integer",
						"description": "Optional. Only return events from the last N seconds. Default 900.",
						"minimum":     10,
						"maximum":     86400,
					},
				},
				"required": []string{"namespace"},
			}),
		},
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, errors.New("kube_events: not yet implemented")
		},
	}
}
