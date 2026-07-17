// Package tools implements the evidence-gathering tools available to the
// agent. Each tool is provider-neutral: it takes a JSON input and returns
// a JSON output.
//
// Tools are the security surface of kubecause. Every tool is read-only.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vipulvc08/kubecause/internal/llm"
)

// Handler is the runtime function for a tool. Input is provider-neutral
// JSON; output is JSON that will be fed back into the LLM as a tool result.
type Handler func(ctx context.Context, input []byte) ([]byte, error)

// Tool bundles a spec (shown to the model) with its handler (executed
// when the model calls it).
type Tool struct {
	Spec    llm.ToolSpec
	Handler Handler
}

// Registry is a set of tools the agent can invoke by name.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Spec.Name] = t
}

// Specs returns tool specs in a stable order for the LLM prompt.
func (r *Registry) Specs() []llm.ToolSpec {
	out := make([]llm.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Spec)
	}
	return out
}

// Invoke runs the tool with the given name, if registered.
func (r *Registry) Invoke(ctx context.Context, name string, input []byte) ([]byte, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tools: unknown tool %q", name)
	}
	return t.Handler(ctx, input)
}

// mustSchema marshals a struct type description into JSON Schema bytes.
// Kept trivial in v0.1 — we hand-write schemas per tool.
func mustSchema(schema map[string]any) []byte {
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("tools: bad schema: %v", err))
	}
	return b
}
