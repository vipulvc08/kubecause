# kubecause

**Root-cause analysis agent for PagerDuty incidents in Kubernetes clusters.**

When a PagerDuty incident fires, `kubecause` gathers evidence from your cluster — events, pod logs, rollout history, resource state — reasons over it with an LLM, and posts a structured RCA back to the incident note before the on-call human even opens their laptop.

> **Status:** early — v0.1 scaffolding. Not ready for production.

---

## Why

Most on-call minutes are spent on the same first 10 minutes: pull events, `kubectl describe`, tail logs, check the last deploy, correlate. That work is mechanical, cite-able, and now automatable.

`kubecause` is the agent that does it for you, and shows its work.

## What it is (and isn't)

**It is:**
- An in-cluster Go service triggered by PagerDuty webhooks
- A tool-use agent loop that gathers evidence, cites sources, and writes an RCA
- Read-only by design — enforced by RBAC, not by prompt

**It is not:**
- A chatbot
- A remediation tool (v0.1 does not touch your cluster state)
- A replacement for a human on-call

## Security posture

The agent's ClusterRole grants:
- `get`, `list`, `watch` on core workload resources
- `get` on `pods/log`
- Read access to metrics API (if available)

The agent explicitly **cannot**:
- Read Secrets
- `exec`, `attach`, or `port-forward` into pods
- `create`, `update`, `patch`, or `delete` any resource
- Read service account tokens

This is not a prompt-level guardrail — it's enforced by Kubernetes RBAC. See [`charts/kubecause/templates/rbac.yaml`](charts/kubecause/templates/rbac.yaml).

## Quickstart

```sh
# 1. Install via Helm
helm install kubecause ./charts/kubecause \
  --namespace kubecause --create-namespace \
  --set pagerduty.webhookSecret=... \
  --set llm.provider=claude \
  --set llm.apiKey=...

# 2. Point your PagerDuty service webhook at:
#    https://<your-ingress>/webhook/pagerduty

# 3. Fire a test incident. Watch the note appear on the incident.
```

Full setup: [`docs/quickstart.md`](docs/quickstart.md).

## Architecture

```
PagerDuty incident
      │
      ▼
[Webhook receiver] ── signature verify ──► [Agent loop]
                                                │
                                                ▼
                                       ┌────────────────┐
                                       │  LLM (Claude/  │
                                       │   OpenAI/etc.) │
                                       └────────┬───────┘
                                                │ tool calls
                       ┌────────────────────────┼────────────────────────┐
                       ▼                        ▼                        ▼
                  kube_events              pod_logs               rollout_history
                       │                        │                        │
                       └────────────────────────┼────────────────────────┘
                                                ▼
                                     [RCA formatter]
                                                │
                                                ▼
                                    PagerDuty incident note
```

More detail: [`docs/architecture.md`](docs/architecture.md).

## LLM providers

Pluggable from day one. See [`docs/llm-providers.md`](docs/llm-providers.md).

- ✅ Anthropic Claude
- ✅ OpenAI
- 🔜 AWS Bedrock (Claude via Bedrock)
- 🔜 Local models via OpenAI-compatible API

## Development

Docker-based — no local Go install required.

```sh
make build     # build binary in a container
make test      # run tests in a container
make image     # build the runtime image
make chart-lint  # lint the Helm chart
```

## Roadmap

- **v0.1** — PagerDuty webhook + 4 evidence tools + Claude adapter + Helm chart
- **v0.2** — Prometheus + Loki tools, Slack integration
- **v0.3** — Multi-cluster, alert correlation, timeline reconstruction
- **v0.4** — Suggested remediations (still read-only; humans apply)

## License

Apache 2.0
