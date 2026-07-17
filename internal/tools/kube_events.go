package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vipulvc08/kubecause/internal/kube"
	"github.com/vipulvc08/kubecause/internal/llm"
)

const defaultEventSinceSeconds = 900

type kubeEventsInput struct {
	Namespace          string `json:"namespace"`
	InvolvedObjectName string `json:"involved_object_name,omitempty"`
	SinceSeconds       int64  `json:"since_seconds,omitempty"`
}

type eventSummary struct {
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	InvolvedKind   string    `json:"involved_kind"`
	InvolvedName   string    `json:"involved_name"`
	Count          int32     `json:"count"`
	FirstTimestamp time.Time `json:"first_timestamp,omitempty"`
	LastTimestamp  time.Time `json:"last_timestamp,omitempty"`
}

// KubeEvents returns a Tool that lists Kubernetes Events in a namespace,
// optionally filtered to a specific involved object.
func KubeEvents(k *kube.Client) Tool {
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
		Handler: func(ctx context.Context, raw []byte) ([]byte, error) {
			var in kubeEventsInput
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("kube_events: bad input: %w", err)
			}
			if in.Namespace == "" {
				return nil, fmt.Errorf("kube_events: namespace is required")
			}
			if in.SinceSeconds == 0 {
				in.SinceSeconds = defaultEventSinceSeconds
			}

			opts := metav1.ListOptions{}
			if in.InvolvedObjectName != "" {
				opts.FieldSelector = "involvedObject.name=" + in.InvolvedObjectName
			}
			list, err := k.CS.CoreV1().Events(in.Namespace).List(ctx, opts)
			if err != nil {
				return nil, fmt.Errorf("kube_events: list failed: %w", err)
			}

			cutoff := time.Now().Add(-time.Duration(in.SinceSeconds) * time.Second)
			out := filterAndSort(list.Items, cutoff)
			return json.Marshal(map[string]any{
				"namespace": in.Namespace,
				"count":     len(out),
				"events":    out,
			})
		},
	}
}

func filterAndSort(items []corev1.Event, cutoff time.Time) []eventSummary {
	out := make([]eventSummary, 0, len(items))
	for _, e := range items {
		last := lastEventTime(e)
		if !last.IsZero() && last.Before(cutoff) {
			continue
		}
		out = append(out, eventSummary{
			Type:           e.Type,
			Reason:         e.Reason,
			Message:        e.Message,
			InvolvedKind:   e.InvolvedObject.Kind,
			InvolvedName:   e.InvolvedObject.Name,
			Count:          e.Count,
			FirstTimestamp: e.FirstTimestamp.Time,
			LastTimestamp:  last,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastTimestamp.After(out[j].LastTimestamp)
	})
	return out
}

func lastEventTime(e corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	if !e.EventTime.IsZero() {
		return e.EventTime.Time
	}
	return e.FirstTimestamp.Time
}
