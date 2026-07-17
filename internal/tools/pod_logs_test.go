package tools

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vipulvc08/kubecause/internal/kube"
)

func TestPodLogs_rejectsMissingRequired(t *testing.T) {
	tool := PodLogs(kube.NewFromInterface(fake.NewSimpleClientset()))

	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing namespace", `{"pod":"p"}`},
		{"missing pod", `{"namespace":"ns"}`},
		{"empty", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tool.Handler(context.Background(), []byte(tc.body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestPodLogs_streamsFromFake(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "app-1"}}
	cs := fake.NewSimpleClientset(pod)
	tool := PodLogs(kube.NewFromInterface(cs))

	raw, err := tool.Handler(context.Background(),
		[]byte(`{"namespace":"ns1","pod":"app-1","tail_lines":10}`))
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	// fake client returns a canned "fake logs" payload; we only assert
	// the tool wraps it in valid JSON with expected keys.
	body := string(raw)
	for _, key := range []string{`"namespace":"ns1"`, `"pod":"app-1"`, `"lines"`, `"line_count"`} {
		if !strings.Contains(body, key) {
			t.Errorf("missing key %s in output: %s", key, body)
		}
	}
}

func TestReadCapped_truncates(t *testing.T) {
	long := strings.Repeat("x", 100) + "\n" + strings.Repeat("y", 100) + "\n"
	lines, truncated, err := readCapped(strings.NewReader(long), 50)
	if err != nil {
		t.Fatalf("read err: %v", err)
	}
	if !truncated {
		t.Error("expected truncation")
	}
	if len(lines) > 1 {
		t.Errorf("expected <=1 line, got %d", len(lines))
	}
}
