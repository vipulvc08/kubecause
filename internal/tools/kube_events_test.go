package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vipulvc08/kubecause/internal/kube"
)

func TestKubeEvents_filtersAndSorts(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * time.Hour)

	cs := fake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "ns1", Name: "e-fresh"},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "app-1"},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "Back-off restarting failed container",
			Count:          3,
			LastTimestamp:  metav1.NewTime(now),
			FirstTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
		},
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "ns1", Name: "e-old"},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "app-2"},
			Type:           "Normal",
			Reason:         "Scheduled",
			Message:        "Successfully assigned",
			Count:          1,
			LastTimestamp:  metav1.NewTime(old),
			FirstTimestamp: metav1.NewTime(old),
		},
	)
	k := kube.NewFromInterface(cs)
	tool := KubeEvents(k)

	raw, err := tool.Handler(context.Background(), []byte(`{"namespace":"ns1","since_seconds":900}`))
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}

	var got struct {
		Count  int            `json:"count"`
		Events []eventSummary `json:"events"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got.Count != 1 {
		t.Fatalf("want 1 event after filter, got %d", got.Count)
	}
	if got.Events[0].Reason != "BackOff" {
		t.Errorf("want BackOff first, got %q", got.Events[0].Reason)
	}
}

func TestKubeEvents_rejectsMissingNamespace(t *testing.T) {
	k := kube.NewFromInterface(fake.NewSimpleClientset())
	tool := KubeEvents(k)
	if _, err := tool.Handler(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected error for missing namespace")
	}
}
