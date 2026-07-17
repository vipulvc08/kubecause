// Package claude implements the llm.Client interface for Anthropic Claude.
//
// The full Anthropic tool-use loop is documented at:
// https://docs.anthropic.com/en/docs/build-with-claude/tool-use
//
// This client uses net/http directly rather than the Anthropic Go SDK
// to keep the dependency footprint small — the Messages API surface we
// need is a single POST and a handful of stable field names.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vipulvc08/kubecause/internal/llm"
)

const (
	defaultBaseURL          = "https://api.anthropic.com"
	defaultAnthropicVersion = "2023-06-01"
	defaultModel            = "claude-opus-4-7"
	defaultMaxTokens        = 4096
	defaultTimeout          = 60 * time.Second
)

// Client talks to Anthropic's Messages API.
type Client struct {
	apiKey    string
	model     string
	baseURL   string
	version   string
	http      *http.Client
	maxTokens int
}

type Option func(*Client)

func WithBaseURL(u string) Option        { return func(c *Client) { c.baseURL = u } }
func WithAnthropicVersion(v string) Option { return func(c *Client) { c.version = v } }
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }
func WithMaxTokens(n int) Option           { return func(c *Client) { c.maxTokens = n } }

func New(apiKey, model string, opts ...Option) *Client {
	if model == "" {
		model = defaultModel
	}
	c := &Client{
		apiKey:    apiKey,
		model:     model,
		baseURL:   defaultBaseURL,
		version:   defaultAnthropicVersion,
		http:      &http.Client{Timeout: defaultTimeout},
		maxTokens: defaultMaxTokens,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) Name() string { return "claude" }

// --- wire types ---
//
// We keep these local and minimal. They cover text, tool_use, and
// tool_result blocks — the entire surface the agent loop needs.

type contentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// tool_use block (assistant → us)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result block (us → assistant)
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type wireMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type wireTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type wireRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []wireMessage `json:"messages"`
	Tools     []wireTool    `json:"tools,omitempty"`
}

type wireResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat runs one turn of the Messages API tool-use loop.
func (c *Client) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	body, err := c.buildRequest(req)
	if err != nil {
		return llm.ChatResponse{}, err
	}

	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return llm.ChatResponse{}, err
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", c.version)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return llm.ChatResponse{}, fmt.Errorf("claude: http error: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return llm.ChatResponse{}, fmt.Errorf("claude: read body: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return llm.ChatResponse{}, fmt.Errorf("claude: %s: %s", resp.Status, snippet(rawResp))
	}

	var wr wireResponse
	if err := json.Unmarshal(rawResp, &wr); err != nil {
		return llm.ChatResponse{}, fmt.Errorf("claude: decode response: %w", err)
	}

	return toChatResponse(wr), nil
}

func (c *Client) buildRequest(req llm.ChatRequest) (wireRequest, error) {
	max := req.MaxTokens
	if max == 0 {
		max = c.maxTokens
	}
	wr := wireRequest{
		Model:     c.model,
		MaxTokens: max,
		System:    req.System,
		Messages:  make([]wireMessage, 0, len(req.Messages)),
	}
	for _, t := range req.Tools {
		wr.Tools = append(wr.Tools, wireTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: json.RawMessage(t.InputSchema),
		})
	}
	for _, m := range req.Messages {
		wm, err := toWireMessage(m)
		if err != nil {
			return wr, err
		}
		wr.Messages = append(wr.Messages, wm)
	}
	return wr, nil
}

func toWireMessage(m llm.Message) (wireMessage, error) {
	switch m.Role {
	case llm.RoleUser:
		return wireMessage{
			Role: "user",
			Content: []contentBlock{
				{Type: "text", Text: m.Text},
			},
		}, nil

	case llm.RoleAssistant:
		blocks := make([]contentBlock, 0, 1+len(m.ToolCalls))
		if m.Text != "" {
			blocks = append(blocks, contentBlock{Type: "text", Text: m.Text})
		}
		for _, tc := range m.ToolCalls {
			blocks = append(blocks, contentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Name,
				Input: json.RawMessage(tc.Input),
			})
		}
		return wireMessage{Role: "assistant", Content: blocks}, nil

	case llm.RoleTool:
		// Anthropic collapses tool results into the *user* role, using
		// tool_result blocks referenced by tool_use_id.
		return wireMessage{
			Role: "user",
			Content: []contentBlock{{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Text,
			}},
		}, nil

	default:
		return wireMessage{}, fmt.Errorf("claude: unsupported role %q", m.Role)
	}
}

func toChatResponse(wr wireResponse) llm.ChatResponse {
	out := llm.ChatResponse{
		StopReason: wr.StopReason,
		Usage: llm.Usage{
			InputTokens:  wr.Usage.InputTokens,
			OutputTokens: wr.Usage.OutputTokens,
		},
	}
	var text bytes.Buffer
	for _, b := range wr.Content {
		switch b.Type {
		case "text":
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(b.Text)
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{
				ID:    b.ID,
				Name:  b.Name,
				Input: []byte(b.Input),
			})
		}
	}
	out.Text = text.String()
	return out
}

func snippet(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
