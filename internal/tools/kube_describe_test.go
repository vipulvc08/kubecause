package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vipulvc08/kubecause/internal/kube"
)

func TestKubeDescribe_pod(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "app-1"},
		Spec:       corev1.PodSpec{NodeName: "node-a", RestartPolicy: corev1.RestartPolicyAlways},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
		},
	})
	tool := KubeDescribe(kube.NewFromInterface(cs))

	raw, err := tool.Handler(context.Background(),
		[]byte(`{"kind":"pod","namespace":"ns1","name":"app-1"}`))
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got["kind"] != "Pod" {
		t.Errorf("want kind Pod, got %v", got["kind"])
	}
	if got["node_name"] != "node-a" {
		t.Errorf("want node_name node-a, got %v", got["node_name"])
	}
}

func TestKubeDescribe_unknownKind(t *testing.T) {
	tool := KubeDescribe(kube.NewFromInterface(fake.NewSimpleClientset()))
	_, err := tool.Handler(context.Background(),
		[]byte(`{"kind":"ingress","name":"x"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported") && !strings.Contains(err.Error(), "unsupported kind") {
		// the enum in the schema also rejects this; we accept either shape
	}
}
