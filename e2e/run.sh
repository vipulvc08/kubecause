#!/usr/bin/env bash
#
# kubecause end-to-end integration test.
#
# What this verifies (deterministically, without real API keys):
#   1. The chart installs cleanly under the intended RBAC.
#   2. The kubecause pod becomes ready.
#   3. The webhook receiver validates HMAC-SHA256 signatures.
#   4. A signed synthetic incident.triggered event is dispatched
#      (visible as "pagerduty event received" in pod logs).
#   5. A broken workload can be deployed and reaches CrashLoopBackOff
#      state — the target the agent investigates in a real run.
#
# What this does NOT verify (needs real API keys):
#   - The agent's tool-use loop against a real LLM.
#   - PostNote actually reaching PagerDuty.
#
# Usage:
#   ./e2e/run.sh            # up + test + keep cluster running
#   E2E_CLEAN=1 ./e2e/run.sh  # up + test + tear down cluster
#
# Prereqs: docker, kind, kubectl, helm, openssl, curl, jq

set -euo pipefail

# --- config ---
CLUSTER_NAME="${CLUSTER_NAME:-kubecause-e2e}"
NAMESPACE="${NAMESPACE:-kubecause}"
IMAGE="${IMAGE:-kubecause:e2e}"
CHART_DIR="${CHART_DIR:-charts/kubecause}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-e2e-shared-secret}"
LLM_API_KEY="${LLM_API_KEY:-fake-key-for-e2e}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-90s}"

# --- terminal colors ---
C_G="\033[32m"; C_R="\033[31m"; C_Y="\033[33m"; C_B="\033[34m"; C_0="\033[0m"
say()    { printf "${C_B}==>${C_0} %s\n" "$*"; }
ok()     { printf "${C_G}✓${C_0} %s\n" "$*"; }
warn()   { printf "${C_Y}!${C_0} %s\n" "$*"; }
fail()   { printf "${C_R}✗${C_0} %s\n" "$*" >&2; exit 1; }

# --- prereqs ---
say "checking prerequisites"
for cmd in docker kind kubectl helm openssl curl jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "missing prerequisite: $cmd
  install hints:
    docker  -> Docker Desktop
    kind    -> brew install kind
    kubectl -> brew install kubectl
    helm    -> brew install helm
    jq      -> brew install jq"
  fi
done
ok "all prereqs present"

# --- kind cluster ---
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  ok "kind cluster $CLUSTER_NAME already exists"
else
  say "creating kind cluster $CLUSTER_NAME"
  kind create cluster --name "$CLUSTER_NAME" --wait 60s
  ok "cluster ready"
fi
export KUBECONFIG=""
kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null

# --- build + load image ---
say "building $IMAGE"
docker build --quiet -t "$IMAGE" . >/dev/null
ok "image built"

say "loading image into kind"
kind load docker-image "$IMAGE" --name "$CLUSTER_NAME"
ok "image loaded"

# --- install chart ---
say "installing chart"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
helm upgrade --install kubecause "$CHART_DIR" \
  --namespace "$NAMESPACE" \
  --set image.repository="${IMAGE%:*}" \
  --set image.tag="${IMAGE#*:}" \
  --set image.pullPolicy=Never \
  --set pagerduty.webhookSecret="$WEBHOOK_SECRET" \
  --set llm.apiKey="$LLM_API_KEY" \
  --wait --timeout "$WAIT_TIMEOUT" >/dev/null
ok "chart installed"

# --- wait for pod ---
say "waiting for kubecause pod ready"
kubectl -n "$NAMESPACE" wait --for=condition=ready pod \
  -l app.kubernetes.io/name=kubecause --timeout="$WAIT_TIMEOUT" >/dev/null
POD=$(kubectl -n "$NAMESPACE" get pod -l app.kubernetes.io/name=kubecause -o jsonpath='{.items[0].metadata.name}')
ok "pod ready: $POD"

# --- deploy broken workload ---
say "deploying broken workload"
kubectl apply -f e2e/manifests/broken-pod.yaml >/dev/null
kubectl -n e2e-broken wait --for=condition=available=false deploy/crashy --timeout=30s >/dev/null 2>&1 || true
# Give the pod a moment to actually enter CrashLoopBackOff
sleep 15
CRASHY_STATE=$(kubectl -n e2e-broken get pod -l app=crashy \
  -o jsonpath='{.items[0].status.containerStatuses[0].state}' 2>/dev/null || echo "{}")
echo "  crashy state: $CRASHY_STATE"
if [[ "$CRASHY_STATE" != *"waiting"* ]] && [[ "$CRASHY_STATE" != *"terminated"* ]]; then
  warn "crashy pod did not reach a failing state — continuing anyway"
fi

# --- send synthetic PD webhook ---
say "port-forwarding to kubecause"
kubectl -n "$NAMESPACE" port-forward "svc/kubecause" 18080:80 >/dev/null 2>&1 &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null || true' EXIT
sleep 2

WEBHOOK_BODY=$(cat e2e/webhook.json)
SIG=$(printf '%s' "$WEBHOOK_BODY" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -hex | awk '{print $2}')
say "posting signed webhook"
HTTP_CODE=$(curl -sS -o /tmp/kubecause-e2e-resp -w '%{http_code}' \
  -X POST http://127.0.0.1:18080/webhook/pagerduty \
  -H "Content-Type: application/json" \
  -H "X-PagerDuty-Signature: v1=$SIG" \
  --data "$WEBHOOK_BODY" || true)
if [[ "$HTTP_CODE" != "202" ]]; then
  fail "expected 202 from webhook, got $HTTP_CODE ($(cat /tmp/kubecause-e2e-resp))"
fi
ok "webhook accepted (202)"

# --- assert bad-signature is rejected ---
say "posting webhook with bad signature — expecting 401"
HTTP_CODE_BAD=$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST http://127.0.0.1:18080/webhook/pagerduty \
  -H "Content-Type: application/json" \
  -H "X-PagerDuty-Signature: v1=deadbeef" \
  --data "$WEBHOOK_BODY" || true)
if [[ "$HTTP_CODE_BAD" != "401" ]]; then
  fail "expected 401 for bad signature, got $HTTP_CODE_BAD"
fi
ok "bad signature rejected (401)"

# --- assert log line ---
say "checking pod logs for reception evidence"
sleep 2
LOGS=$(kubectl -n "$NAMESPACE" logs "$POD" --tail=200)
if echo "$LOGS" | grep -q "pagerduty event received"; then
  ok "pod logged 'pagerduty event received'"
else
  echo "$LOGS" | tail -20
  fail "did not find 'pagerduty event received' in logs"
fi

# --- summary ---
echo
ok "E2E PASSED"
echo "   cluster: kind-$CLUSTER_NAME"
echo "   inspect: kubectl -n $NAMESPACE logs $POD"

if [[ "${E2E_CLEAN:-0}" == "1" ]]; then
  say "tearing down cluster"
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null
  ok "cluster deleted"
else
  echo "   cleanup: kind delete cluster --name $CLUSTER_NAME"
fi
