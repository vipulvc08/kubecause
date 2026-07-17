package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vipulvc08/kubecause/internal/kube"
)

func TestRolloutHistory_deployment_orderedByRevision(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "web"},
		Spec:       appsv1.DeploymentSpec{Selector: sel},
	}
	rs1 := replicaSet("ns1", "web-1", "1", time.Now().Add(-2*time.Hour), "web:1.0")
	rs2 := replicaSet("ns1", "web-2", "2", time.Now().Add(-1*time.Hour), "web:1.1")
	rs3 := replicaSet("ns1", "web-3", "3", time.Now(), "web:1.2")

	cs := fake.NewSimpleClientset(dep, rs1, rs2, rs3)
	tool := RolloutHistory(kube.NewFromInterface(cs))

	raw, err := tool.Handler(context.Background(),
		[]byte(`{"kind":"deployment","namespace":"ns1","name":"web","limit":2}`))
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}

	var got struct {
		Revisions []revisionSummary `json:"revisions"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(got.Revisions) != 2 {
		t.Fatalf("want 2 revisions after limit, got %d", len(got.Revisions))
	}
	if got.Revisions[0].Revision != 3 || got.Revisions[1].Revision != 2 {
		t.Errorf("revisions not sorted desc: got %v, %v",
			got.Revisions[0].Revision, got.Revisions[1].Revision)
	}
	if got.Revisions[0].Image != "web:1.2" {
		t.Errorf("want image web:1.2 for latest, got %q", got.Revisions[0].Image)
	}
}

func replicaSet(ns, name, rev string, created time.Time, image string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         ns,
			Name:              name,
			Labels:            map[string]string{"app": "web"},
			Annotations:       map[string]string{revisionAnnotation: rev},
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: image}}},
			},
		},
	}
}
