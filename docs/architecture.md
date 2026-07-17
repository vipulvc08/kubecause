# Architecture

`kubecause` is a small Go service that runs as a single Deployment inside a Kubernetes cluster and does one thing: turn a PagerDuty incident into a written RCA.

## The loop

```
1. PagerDuty webhook   →  /webhook/pagerduty
2. Signature verify    →  drop if invalid
3. Fetch incident      →  PagerDuty REST API (details, urgency, custom fields)
4. Kick agent loop     →  llm.Chat + tool-use
5. Tools gather        →  kube_events, pod_logs, kube_describe, rollout_history
6. RCA formatter       →  compact, cited markdown
7. Post note           →  PagerDuty incident note
```

The loop is intentionally small. There is no queue, no database, no persistent state — the agent runs the loop synchronously for a single incident, posts, and exits. Multiple incidents are handled by multiple concurrent goroutines.

## Packages

| Package                       | Responsibility                                                             |
|-------------------------------|----------------------------------------------------------------------------|
| `cmd/kubecause`               | Process lifecycle. HTTP server, config, graceful shutdown.                 |
| `internal/config`             | Load environment-based configuration.                                      |
| `internal/pagerduty`          | Webhook receiver + signature verification (v3 HMAC-SHA256).                |
| `internal/llm`                | Provider-neutral chat + tool-use interface.                                |
| `internal/llm/claude`         | Anthropic Claude implementation of `llm.Client`.                           |
| `internal/llm/openai`         | OpenAI (and OpenAI-compatible) implementation of `llm.Client`.             |
| `internal/agent`              | Tool-use loop. Provider-agnostic. Bounded iterations.                      |
| `internal/tools`              | Evidence-gathering tools. All read-only.                                   |
| `internal/rca`                | Format the final RCA for downstream channels.                              |

## Why these boundaries

- **`llm` is provider-neutral.** The agent loop never imports a provider SDK. Adding Bedrock or a local model is a new file under `internal/llm/*`, not a rewrite.
- **Tools are the security surface.** Every evidence source is behind a `Tool`. If a source doesn't have a tool, the model cannot access it. There is no free-form `kubectl` execution.
- **The agent is stateless.** No persistent memory of past incidents. This is a deliberate v0.1 choice: correlation across incidents is a v0.3 feature, not v0.1.

## Security posture

`kubecause` runs with a cluster-wide **read-only** ClusterRole. See [`charts/kubecause/templates/rbac.yaml`](../charts/kubecause/templates/rbac.yaml) for the exact rules. The chart also enforces:

- `runAsNonRoot: true`, non-root UID
- `readOnlyRootFilesystem: true`
- `allowPrivilegeEscalation: false`
- All Linux capabilities dropped

## What's next

- `internal/tools/pod_logs.go` and friends currently return "not yet implemented". The client-go wiring lands with the first end-to-end incident test.
- `internal/llm/claude/claude.go` `Chat` is stubbed until the tool-use schema is finalized against Anthropic's Messages API.
- No PagerDuty REST client yet; only the webhook receiver.

Follow [`docs/quickstart.md`](quickstart.md) to run the current scaffold.
