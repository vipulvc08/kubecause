package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/vipulvc08/kubecause/internal/kube"
	"github.com/vipulvc08/kubecause/internal/llm"
)

const (
	defaultTailLines = 500
	maxTailLines     = 2000
	maxBytes         = 128 * 1024 // hard cap on returned log payload
)

type podLogsInput struct {
	Namespace    string `json:"namespace"`
	Pod          string `json:"pod"`
	Container    string `json:"container,omitempty"`
	TailLines    int64  `json:"tail_lines,omitempty"`
	SinceSeconds int64  `json:"since_seconds,omitempty"`
	Previous     bool   `json:"previous,omitempty"`
}

// PodLogs returns a Tool that fetches logs from a pod, with a bounded
// tail size and time window, and optional access to the previous container
// instance (where crash-loop errors usually live).
func PodLogs(k *kube.Client) Tool {
	return Tool{
		Spec: llm.ToolSpec{
			Name:        "pod_logs",
			Description: "Fetch recent log lines from a pod. Supports selecting a container and reading the previous crashed container instance. Output is truncated to a bounded number of lines and bytes.",
			InputSchema: mustSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace":     map[string]any{"type": "string"},
					"pod":           map[string]any{"type": "string"},
					"container":     map[string]any{"type": "string", "description": "Optional. Container name for multi-container pods."},
					"tail_lines":    map[string]any{"type": "integer", "description": "Max lines to return. Default 500, max 2000.", "minimum": 1, "maximum": maxTailLines},
					"since_seconds": map[string]any{"type": "integer", "description": "Optional. Only lines from the last N seconds.", "minimum": 1},
					"previous":      map[string]any{"type": "boolean", "description": "If true, read the previous crashed container instance."},
				},
				"required": []string{"namespace", "pod"},
			}),
		},
		Handler: func(ctx context.Context, raw []byte) ([]byte, error) {
			var in podLogsInput
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("pod_logs: bad input: %w", err)
			}
			if in.Namespace == "" || in.Pod == "" {
				return nil, fmt.Errorf("pod_logs: namespace and pod are required")
			}
			if in.TailLines <= 0 {
				in.TailLines = defaultTailLines
			}
			if in.TailLines > maxTailLines {
				in.TailLines = maxTailLines
			}

			opts := &corev1.PodLogOptions{
				Container: in.Container,
				TailLines: &in.TailLines,
				Previous:  in.Previous,
			}
			if in.SinceSeconds > 0 {
				s := in.SinceSeconds
				opts.SinceSeconds = &s
			}

			req := k.CS.CoreV1().Pods(in.Namespace).GetLogs(in.Pod, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				return nil, fmt.Errorf("pod_logs: stream failed: %w", err)
			}
			defer stream.Close()

			lines, truncated, err := readCapped(stream, maxBytes)
			if err != nil {
				return nil, fmt.Errorf("pod_logs: read failed: %w", err)
			}

			return json.Marshal(map[string]any{
				"namespace":  in.Namespace,
				"pod":        in.Pod,
				"container":  in.Container,
				"previous":   in.Previous,
				"line_count": len(lines),
				"truncated":  truncated,
				"lines":      lines,
			})
		},
	}
}

func readCapped(r io.Reader, cap int) ([]string, bool, error) {
	var (
		lines     []string
		total     int
		truncated bool
	)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if total+len(line)+1 > cap {
			truncated = true
			break
		}
		total += len(line) + 1
		lines = append(lines, strings.TrimRight(line, "\r\n"))
	}
	if err := scanner.Err(); err != nil {
		return lines, truncated, err
	}
	return lines, truncated, nil
}
