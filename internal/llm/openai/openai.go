// Package openai implements the llm.Client interface for OpenAI's Chat
// Completions API and any OpenAI-compatible local model server.
package openai

import (
	"context"
	"errors"

	"github.com/vipulvc08/kubecause/internal/llm"
)

const defaultModel = "gpt-4.1"

type Client struct {
	apiKey  string
	model   string
	baseURL string
}

type Option func(*Client)

// WithBaseURL points the client at an OpenAI-compatible server (Ollama,
// vLLM, LM Studio, etc.). Leave unset for api.openai.com.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

func New(apiKey, model string, opts ...Option) *Client {
	if model == "" {
		model = defaultModel
	}
	c := &Client{apiKey: apiKey, model: model}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) Name() string { return "openai" }

func (c *Client) Chat(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, errors.New("openai: Chat not yet implemented")
}
