package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vipulvc08/kubecause/internal/llm"
)

func TestChat_toolUseRoundTrip(t *testing.T) {
	// Anthropic mock server that scripts two turns:
	//   turn 1: user asks -> assistant emits tool_use for kube_events
	//   turn 2: user replies with tool_result -> assistant emits final text
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("missing/bad api key: %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Errorf("missing anthropic-version header")
		}

		body, _ := io.ReadAll(r.Body)
		var req wireRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("mock: bad request body: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		if turn == 0 {
			if len(req.Tools) == 0 {
				t.Errorf("turn 1: expected tools in request")
			}
			turn++
			_, _ = w.Write([]byte(`{
				"content": [
					{"type":"text","text":"looking at events"},
					{"type":"tool_use","id":"tu_1","name":"kube_events","input":{"namespace":"prod"}}
				],
				"stop_reason":"tool_use",
				"usage":{"input_tokens":10,"output_tokens":5}
			}`))
			return
		}
		// turn 2: verify prior assistant + tool result are in payload
		if len(req.Messages) < 3 {
			t.Fatalf("turn 2: expected >=3 messages, got %d", len(req.Messages))
		}
		last := req.Messages[len(req.Messages)-1]
		if last.Role != "user" || len(last.Content) == 0 || last.Content[0].Type != "tool_result" {
			t.Errorf("turn 2: expected trailing user/tool_result, got %+v", last)
		}
		if last.Content[0].ToolUseID != "tu_1" {
			t.Errorf("turn 2: expected tool_use_id=tu_1, got %q", last.Content[0].ToolUseID)
		}
		_, _ = w.Write([]byte(`{
			"content":[{"type":"text","text":"root cause: OOMKilled on payments-7f8"}],
			"stop_reason":"end_turn",
			"usage":{"input_tokens":30,"output_tokens":12}
		}`))
	}))
	defer srv.Close()

	client := New("test-key", "claude-test", WithBaseURL(srv.URL))

	// -- Turn 1
	resp1, err := client.Chat(context.Background(), llm.ChatRequest{
		System: "you are helpful",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "why is payments down?"},
		},
		Tools: []llm.ToolSpec{{
			Name:        "kube_events",
			Description: "list events",
			InputSchema: []byte(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatalf("turn 1 err: %v", err)
	}
	if resp1.StopReason != "tool_use" {
		t.Errorf("turn 1: want stop_reason tool_use, got %q", resp1.StopReason)
	}
	if len(resp1.ToolCalls) != 1 || resp1.ToolCalls[0].Name != "kube_events" {
		t.Fatalf("turn 1: unexpected tool calls: %+v", resp1.ToolCalls)
	}
	if !strings.Contains(resp1.Text, "looking at events") {
		t.Errorf("turn 1: missing text, got %q", resp1.Text)
	}

	// -- Turn 2: feed tool result back
	resp2, err := client.Chat(context.Background(), llm.ChatRequest{
		System: "you are helpful",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "why is payments down?"},
			{
				Role:      llm.RoleAssistant,
				Text:      resp1.Text,
				ToolCalls: resp1.ToolCalls,
			},
			{
				Role:       llm.RoleTool,
				ToolCallID: resp1.ToolCalls[0].ID,
				Text:       `{"events":[{"reason":"OOMKilled"}]}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("turn 2 err: %v", err)
	}
	if resp2.StopReason != "end_turn" {
		t.Errorf("turn 2: want stop_reason end_turn, got %q", resp2.StopReason)
	}
	if !strings.Contains(resp2.Text, "OOMKilled") {
		t.Errorf("turn 2: expected OOMKilled in final answer, got %q", resp2.Text)
	}
	if resp2.Usage.OutputTokens != 12 {
		t.Errorf("turn 2: want 12 output tokens, got %d", resp2.Usage.OutputTokens)
	}
}

func TestChat_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer srv.Close()

	client := New("bad", "", WithBaseURL(srv.URL))
	_, err := client.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Errorf("expected error message to include api-key detail, got %v", err)
	}
}

func TestName(t *testing.T) {
	if got := New("k", "").Name(); got != "claude" {
		t.Errorf("Name = %q, want claude", got)
	}
}
