# LLM providers

`kubecause` is provider-neutral. The agent loop only talks to `internal/llm.Client`. Each provider is one file under `internal/llm/<name>/`.

## Selecting a provider

Via chart:

```sh
helm install kubecause ./charts/kubecause \
  --set llm.provider=claude \
  --set llm.apiKey=... \
  --set llm.model=claude-opus-4-7
```

Or via environment on the pod:

- `LLM_PROVIDER` — `claude` | `openai`
- `LLM_API_KEY`
- `LLM_MODEL` — provider default when empty

## Supported providers

### Anthropic Claude (default)

- Package: `internal/llm/claude`
- Default model: `claude-opus-4-7`
- Recommended: `claude-opus-4-7` for full RCA fidelity, `claude-haiku-4-5` for cheap runs

### OpenAI

- Package: `internal/llm/openai`
- Default model: `gpt-4.1`
- OpenAI-compatible endpoints (Ollama, vLLM, LM Studio) work via `openai.WithBaseURL`.

### Coming next

- **AWS Bedrock (Claude via Bedrock)** — enterprise story, no separate API key required in-cluster if the pod's IRSA role has `bedrock:InvokeModel`.
- **Local models** — via OpenAI-compatible endpoint, no code change required, config only.

## Adding a new provider

1. Create `internal/llm/<name>/<name>.go`.
2. Implement `llm.Client` (`Chat` + `Name`).
3. Translate `llm.ToolSpec`/`llm.ToolCall` to the provider's native shape.
4. Wire it into the provider switch in `cmd/kubecause/main.go` (once that switch exists — v0.1 currently defaults to Claude).
5. Add tests that exercise a stubbed tool-use round-trip.

The interface is deliberately small so the surface for a new provider stays under ~150 lines.
