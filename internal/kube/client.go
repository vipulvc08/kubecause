// Package kube wraps client-go with a tiny factory that works both
// in-cluster (via ServiceAccount) and out-of-cluster (via ~/.kube/config).
//
// All tool handlers depend on the *Client returned here rather than
// importing client-go directly, so that unit tests can inject fakes.
package kube

import (
	"errors"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is the narrow surface tool handlers use. Kept small so a fake
// can implement it in tests.
type Client struct {
	CS kubernetes.Interface
}

// New returns a Client configured for the current environment.
//
// It tries, in order:
//  1. In-cluster (rest.InClusterConfig) — the normal production path.
//  2. $KUBECONFIG env var.
//  3. $HOME/.kube/config — the developer path.
func New() (*Client, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{CS: cs}, nil
}

// NewFromInterface wraps a caller-supplied kubernetes.Interface.
// Used by tests to inject a fake client.
func NewFromInterface(cs kubernetes.Interface) *Client {
	return &Client{CS: cs}
}

func restConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return clientcmd.BuildConfigFromFlags("", p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(p); err == nil {
			return clientcmd.BuildConfigFromFlags("", p)
		}
	}
	return nil, errors.New("kube: no in-cluster config, KUBECONFIG, or ~/.kube/config found")
}

// ObjectMeta pulls fields we quote back to the model. Keeping this in one
// place stops each tool from re-inventing the same trimming.
type ObjectMeta struct {
	Namespace         string            `json:"namespace,omitempty"`
	Name              string            `json:"name"`
	CreationTimestamp metav1.Time       `json:"creation_timestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

func Meta(o metav1.Object) ObjectMeta {
	return ObjectMeta{
		Namespace:         o.GetNamespace(),
		Name:              o.GetName(),
		CreationTimestamp: metav1.NewTime(o.GetCreationTimestamp().Time),
		Labels:            o.GetLabels(),
	}
}

// PodPhase is a compact status view used by kube_describe.
type PodPhase struct {
	Phase             corev1.PodPhase             `json:"phase"`
	Conditions        []corev1.PodCondition       `json:"conditions,omitempty"`
	ContainerStatuses []corev1.ContainerStatus    `json:"container_statuses,omitempty"`
	InitContainerSts  []corev1.ContainerStatus    `json:"init_container_statuses,omitempty"`
	QOSClass          corev1.PodQOSClass          `json:"qos_class,omitempty"`
}
