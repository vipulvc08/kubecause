package tools

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
)

// KubeDescribe returns a Tool that provides a compact, `kubectl describe`-
// style summary of a workload resource: its spec, status, conditions,
// and recent events.
func KubeDescribe() Tool {
	return Tool{
		Spec: llm.ToolSpec{
			Name:        "kube_describe",
			Description: "Return a compact describe-style summary of a Kubernetes resource (pod, deployment, statefulset, replicaset, daemonset, node, job). Includes status, conditions, container statuses, and recent events.",
			InputSchema: mustSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind":      map[string]any{"type": "string", "enum": []string{"pod", "deployment", "statefulset", "replicaset", "daemonset", "node", "job"}},
					"namespace": map[string]any{"type": "string", "description": "Required for namespaced kinds. Omit for cluster-scoped (node)."},
					"name":      map[string]any{"type": "string"},
				},
				"required": []string{"kind", "name"},
			}),
		},
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, errors.New("kube_describe: not yet implemented")
		},
	}
}
