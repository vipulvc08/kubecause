// Package claude implements the llm.Client interface for Anthropic Claude.
//
// The full Anthropic tool-use loop is documented at:
// https://docs.anthropic.com/en/docs/build-with-claude/tool-use
package claude

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
)

const defaultModel = "claude-opus-4-7"

type Client struct {
	apiKey string
	model  string
}

func New(apiKey, model string) *Client {
	if model == "" {
		model = defaultModel
	}
	return &Client{apiKey: apiKey, model: model}
}

func (c *Client) Name() string { return "claude" }

// Chat is unimplemented in v0.1 scaffolding. The wire-up happens in v0.1's
// agent-loop PR once the tool set is finalized.
func (c *Client) Chat(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, errors.New("claude: Chat not yet implemented")
}
