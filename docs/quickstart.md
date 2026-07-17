# Quickstart

Get `kubecause` running against a local cluster in under 5 minutes.

## Prereqs

- Docker Desktop (or any local Docker daemon)
- A Kubernetes cluster you can reach with `kubectl` (kind, minikube, colima, or a real one)
- Helm 3
- A PagerDuty account with a service you can attach a webhook to
- An LLM API key (Anthropic Claude by default)

## 1. Build the image

```sh
make image
# → kubecause:dev, kubecause:latest
```

If you're pushing to a registry, tag and push manually for now. A CI-managed release flow ships with v0.2.

## 2. Install the chart

```sh
kubectl create namespace kubecause

helm install kubecause ./charts/kubecause \
  --namespace kubecause \
  --set image.repository=kubecause \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --set pagerduty.webhookSecret="$(openssl rand -hex 32)" \
  --set llm.provider=claude \
  --set llm.apiKey="$ANTHROPIC_API_KEY"
```

The `pullPolicy=Never` bit only matters if the image is a local build not in a registry.

## 3. Verify it's up

```sh
kubectl -n kubecause get pods
kubectl -n kubecause logs -l app.kubernetes.io/name=kubecause
```

You should see `kubecause listening addr=:8080`.

## 4. Point PagerDuty at it

For a real cluster, expose it via Ingress:

```sh
helm upgrade kubecause ./charts/kubecause \
  --reuse-values \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=kubecause.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix
```

For local testing, port-forward:

```sh
kubectl -n kubecause port-forward svc/kubecause 8080:80
```

Then in PagerDuty:

1. Go to **Integrations → Extensions → Add** for your service.
2. Choose **Generic Webhook v3**.
3. URL: `https://<your-host>/webhook/pagerduty` (or a tunnel like ngrok for local).
4. Secret: the value you passed as `pagerduty.webhookSecret`.
5. Subscribe to `incident.triggered` and `incident.escalated`.

## 5. Fire a test incident

Trigger any incident on that service. Within a few seconds you should see:

- Logs on the pod acknowledging the webhook
- (Once agent loop is wired in v0.1) a note appear on the incident with the RCA

## Uninstalling

```sh
helm uninstall kubecause -n kubecause
kubectl delete namespace kubecause
```
