package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vipulvc08/kubecause/internal/kube"
	"github.com/vipulvc08/kubecause/internal/llm"
)

const (
	defaultRolloutLimit = 10
	maxRolloutLimit     = 50
	// revisionAnnotation is set by the Deployment controller on each
	// ReplicaSet to identify the rollout number.
	revisionAnnotation = "deployment.kubernetes.io/revision"
)

type rolloutHistoryInput struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Limit     int    `json:"limit,omitempty"`
}

type revisionSummary struct {
	Revision  int64             `json:"revision"`
	Name      string            `json:"name"`
	Created   metav1.Time       `json:"created"`
	Replicas  int32             `json:"replicas"`
	Ready     int32             `json:"ready"`
	Labels    map[string]string `json:"labels,omitempty"`
	Image     string            `json:"image,omitempty"`
	CauseHint string            `json:"cause_hint,omitempty"`
}

// RolloutHistory returns a Tool that lists recent rollouts of a workload,
// with timestamps and revision numbers.
func RolloutHistory(k *kube.Client) Tool {
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
					"limit":     map[string]any{"type": "integer", "description": "Max revisions to return. Default 10.", "minimum": 1, "maximum": maxRolloutLimit},
				},
				"required": []string{"kind", "namespace", "name"},
			}),
		},
		Handler: func(ctx context.Context, raw []byte) ([]byte, error) {
			var in rolloutHistoryInput
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("rollout_history: bad input: %w", err)
			}
			if in.Namespace == "" || in.Name == "" || in.Kind == "" {
				return nil, fmt.Errorf("rollout_history: kind, namespace, name are required")
			}
			if in.Limit <= 0 {
				in.Limit = defaultRolloutLimit
			}
			if in.Limit > maxRolloutLimit {
				in.Limit = maxRolloutLimit
			}
			switch strings.ToLower(in.Kind) {
			case "deployment":
				return deploymentHistory(ctx, k, in.Namespace, in.Name, in.Limit)
			case "statefulset":
				return statefulSetHistory(ctx, k, in.Namespace, in.Name, in.Limit)
			}
			return nil, fmt.Errorf("rollout_history: unsupported kind %q", in.Kind)
		},
	}
}

func deploymentHistory(ctx context.Context, k *kube.Client, ns, name string, limit int) ([]byte, error) {
	dep, err := k.CS.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sel, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return nil, err
	}
	list, err := k.CS.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{
		LabelSelector: sel.String(),
	})
	if err != nil {
		return nil, err
	}
	revisions := make([]revisionSummary, 0, len(list.Items))
	for _, rs := range list.Items {
		revisions = append(revisions, replicaSetToRevision(rs))
	}
	sort.SliceStable(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}
	return json.Marshal(map[string]any{
		"kind":      "Deployment",
		"namespace": ns,
		"name":      name,
		"revisions": revisions,
	})
}

func statefulSetHistory(ctx context.Context, k *kube.Client, ns, name string, limit int) ([]byte, error) {
	ss, err := k.CS.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sel, err := metav1.LabelSelectorAsSelector(ss.Spec.Selector)
	if err != nil {
		return nil, err
	}
	list, err := k.CS.AppsV1().ControllerRevisions(ns).List(ctx, metav1.ListOptions{
		LabelSelector: sel.String(),
	})
	if err != nil {
		return nil, err
	}
	revisions := make([]revisionSummary, 0, len(list.Items))
	for _, cr := range list.Items {
		revisions = append(revisions, revisionSummary{
			Revision: cr.Revision,
			Name:     cr.Name,
			Created:  cr.CreationTimestamp,
			Labels:   cr.Labels,
		})
	}
	sort.SliceStable(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}
	return json.Marshal(map[string]any{
		"kind":      "StatefulSet",
		"namespace": ns,
		"name":      name,
		"revisions": revisions,
	})
}

func replicaSetToRevision(rs appsv1.ReplicaSet) revisionSummary {
	rev, _ := strconv.ParseInt(rs.Annotations[revisionAnnotation], 10, 64)
	var img string
	if len(rs.Spec.Template.Spec.Containers) > 0 {
		img = rs.Spec.Template.Spec.Containers[0].Image
	}
	return revisionSummary{
		Revision:  rev,
		Name:      rs.Name,
		Created:   rs.CreationTimestamp,
		Replicas:  ptrDeref(rs.Spec.Replicas),
		Ready:     rs.Status.ReadyReplicas,
		Labels:    rs.Labels,
		Image:     img,
		CauseHint: rs.Annotations["kubernetes.io/change-cause"],
	}
}

func ptrDeref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
