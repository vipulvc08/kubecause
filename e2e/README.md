# E2E integration test

Deterministic end-to-end test for `kubecause` against a local `kind` cluster. Runs in ~2 minutes on a warm laptop.

## What it verifies

- The Helm chart installs cleanly under the intended cluster-wide read-only RBAC.
- The `kubecause` pod becomes ready inside a real cluster (not just unit tests).
- HMAC-SHA256 webhook signature verification behaves correctly:
  - **Valid signature** → `202 Accepted`, event logged
  - **Invalid signature** → `401 Unauthorized`, no dispatch
- A broken workload (`e2e-broken/crashy`) can be deployed and enters CrashLoopBackOff — this is the target the agent investigates in a real run.

## What it does NOT verify

- The agent's tool-use loop against a real LLM (that requires `LLM_API_KEY`).
- `PostNote` actually reaching PagerDuty (that requires `PAGERDUTY_API_TOKEN` + a real account).

These are covered by unit tests already; the E2E complements them by proving the plumbing works inside a Kubernetes runtime.

## Prereqs

- Docker Desktop (or any Docker daemon)
- `kind` — `brew install kind`
- `kubectl` — `brew install kubectl`
- `helm` — `brew install helm`
- `jq` — `brew install jq`
- `openssl` and `curl` (usually already present)

## Run

```sh
# From the repo root
make e2e

# Or directly
./e2e/run.sh

# Run and tear down the cluster at the end
E2E_CLEAN=1 ./e2e/run.sh
```

## Files

| Path                              | Purpose                                              |
|-----------------------------------|------------------------------------------------------|
| `e2e/run.sh`                      | Driver script                                        |
| `e2e/webhook.json`                | Synthetic PagerDuty v3 `incident.triggered` payload  |
| `e2e/manifests/broken-pod.yaml`   | Namespace + Deployment that CrashLoopBackOffs        |

## Troubleshooting

**"pod not ready"** — inspect logs directly:
```sh
kubectl -n kubecause logs -l app.kubernetes.io/name=kubecause --tail=200
```

**"expected 202 from webhook, got 401"** — the shared secret in the chart install and the one signing the payload must match. The script uses `$WEBHOOK_SECRET` for both (default `e2e-shared-secret`); check the value hasn't been overridden.

**"crashy pod did not reach a failing state"** — non-fatal warning. Some node scheduling on slow hosts can delay CrashLoopBackOff past the sleep window; the assertion continues without it.
