// Package llm defines a provider-neutral interface for LLM chat with tool use.
//
// The agent loop lives above this package and never talks to a specific
// provider SDK. Each provider (Claude, OpenAI, Bedrock, ...) implements
// Client by translating to and from its native tool-use format.
package llm

import "context"

// Role identifies who authored a message in the conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn in the conversation. A single message may contain
// text, tool calls (assistant-authored), or a tool result (tool-authored).
type Message struct {
	Role      Role
	Text      string
	ToolCalls []ToolCall
	// ToolCallID is set on RoleTool messages to correlate with the
	// assistant's ToolCall.ID.
	ToolCallID string
}

// ToolSpec describes a tool the model is allowed to call.
//
// InputSchema is a JSON Schema document (as raw bytes) describing the
// tool's expected input. Providers translate this to their native shape.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema []byte
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID    string
	Name  string
	Input []byte // JSON
}

// ChatRequest is a provider-neutral chat request.
type ChatRequest struct {
	System   string
	Messages []Message
	Tools    []ToolSpec
	// MaxTokens caps the response length. Zero means provider default.
	MaxTokens int
}

// ChatResponse is a provider-neutral chat response.
type ChatResponse struct {
	Text      string
	ToolCalls []ToolCall
	Usage     Usage
	// StopReason is a provider-neutral stop signal: "end_turn", "tool_use",
	// "max_tokens", or the raw provider value if not mapped.
	StopReason string
}

// Usage reports token accounting for a single call.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Client is implemented by each LLM provider.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// Name returns the provider identifier ("claude", "openai", ...).
	Name() string
}
