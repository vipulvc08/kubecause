package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vipulvc08/kubecause/internal/kube"
	"github.com/vipulvc08/kubecause/internal/llm"
)

type kubeDescribeInput struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// KubeDescribe returns a Tool that provides a compact, describe-style
// summary of a workload resource.
func KubeDescribe(k *kube.Client) Tool {
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
		Handler: func(ctx context.Context, raw []byte) ([]byte, error) {
			var in kubeDescribeInput
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("kube_describe: bad input: %w", err)
			}
			if in.Name == "" || in.Kind == "" {
				return nil, fmt.Errorf("kube_describe: kind and name are required")
			}
			return describe(ctx, k, strings.ToLower(in.Kind), in.Namespace, in.Name)
		},
	}
}

func describe(ctx context.Context, k *kube.Client, kind, ns, name string) ([]byte, error) {
	getOpts := metav1.GetOptions{}
	switch kind {
	case "pod":
		p, err := k.CS.CoreV1().Pods(ns).Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(describePod(p))
	case "deployment":
		d, err := k.CS.AppsV1().Deployments(ns).Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(describeDeployment(d))
	case "statefulset":
		s, err := k.CS.AppsV1().StatefulSets(ns).Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"kind": "StatefulSet", "meta": kube.Meta(s),
			"replicas":       s.Spec.Replicas,
			"status_ready":   s.Status.ReadyReplicas,
			"status_current": s.Status.CurrentReplicas,
			"status_updated": s.Status.UpdatedReplicas,
		})
	case "replicaset":
		r, err := k.CS.AppsV1().ReplicaSets(ns).Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"kind": "ReplicaSet", "meta": kube.Meta(r),
			"replicas":     r.Spec.Replicas,
			"status_ready": r.Status.ReadyReplicas,
		})
	case "daemonset":
		d, err := k.CS.AppsV1().DaemonSets(ns).Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"kind": "DaemonSet", "meta": kube.Meta(d),
			"desired":   d.Status.DesiredNumberScheduled,
			"ready":     d.Status.NumberReady,
			"available": d.Status.NumberAvailable,
		})
	case "node":
		n, err := k.CS.CoreV1().Nodes().Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"kind": "Node", "meta": kube.Meta(n),
			"conditions":    n.Status.Conditions,
			"capacity":      n.Status.Capacity,
			"allocatable":   n.Status.Allocatable,
			"kubelet":       n.Status.NodeInfo.KubeletVersion,
			"os":            n.Status.NodeInfo.OperatingSystem,
			"kernel":        n.Status.NodeInfo.KernelVersion,
			"container_run": n.Status.NodeInfo.ContainerRuntimeVersion,
		})
	case "job":
		j, err := k.CS.BatchV1().Jobs(ns).Get(ctx, name, getOpts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(describeJob(j))
	default:
		return nil, fmt.Errorf("kube_describe: unsupported kind %q", kind)
	}
}

func describePod(p *corev1.Pod) map[string]any {
	return map[string]any{
		"kind":                     "Pod",
		"meta":                     kube.Meta(p),
		"phase":                    p.Status.Phase,
		"reason":                   p.Status.Reason,
		"message":                  p.Status.Message,
		"conditions":               p.Status.Conditions,
		"container_statuses":       p.Status.ContainerStatuses,
		"init_container_statuses":  p.Status.InitContainerStatuses,
		"qos_class":                p.Status.QOSClass,
		"node_name":                p.Spec.NodeName,
		"restart_policy":           p.Spec.RestartPolicy,
	}
}

func describeDeployment(d *appsv1.Deployment) map[string]any {
	return map[string]any{
		"kind":              "Deployment",
		"meta":              kube.Meta(d),
		"replicas":          d.Spec.Replicas,
		"status_ready":      d.Status.ReadyReplicas,
		"status_available":  d.Status.AvailableReplicas,
		"status_updated":    d.Status.UpdatedReplicas,
		"status_conditions": d.Status.Conditions,
		"strategy":          d.Spec.Strategy.Type,
	}
}

func describeJob(j *batchv1.Job) map[string]any {
	return map[string]any{
		"kind":       "Job",
		"meta":       kube.Meta(j),
		"active":     j.Status.Active,
		"succeeded":  j.Status.Succeeded,
		"failed":     j.Status.Failed,
		"start_time": j.Status.StartTime,
		"conditions": j.Status.Conditions,
	}
}
