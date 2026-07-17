# Docker-based dev workflow — no local Go install required.

IMAGE       ?= kubecause
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
GO_IMAGE    ?= golang:1.23-alpine
HELM_IMAGE  ?= alpine/helm:3.14.0
WORKDIR     := /src

DOCKER_RUN = docker run --rm \
	-v $(PWD):$(WORKDIR) \
	-w $(WORKDIR) \
	-e CGO_ENABLED=0 \
	-e GOCACHE=/tmp/.cache \
	-e GOMODCACHE=/tmp/.modcache \
	$(GO_IMAGE)

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## Run go mod tidy inside a container
	$(DOCKER_RUN) go mod tidy

.PHONY: build
build: ## Build the kubecause binary inside a container
	$(DOCKER_RUN) go build -trimpath -o bin/kubecause ./cmd/kubecause

.PHONY: test
test: ## Run tests inside a container
	$(DOCKER_RUN) go test ./...

.PHONY: vet
vet: ## Run go vet inside a container
	$(DOCKER_RUN) go vet ./...

.PHONY: image
image: ## Build the runtime container image
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

.PHONY: chart-lint
chart-lint: ## Lint the Helm chart
	docker run --rm -v $(PWD):$(WORKDIR) -w $(WORKDIR) $(HELM_IMAGE) \
		lint charts/kubecause

.PHONY: e2e
e2e: ## Run the end-to-end test against a local kind cluster (see e2e/README.md)
	./e2e/run.sh

.PHONY: e2e-clean
e2e-clean: ## Run the end-to-end test and tear down the cluster at the end
	E2E_CLEAN=1 ./e2e/run.sh

.PHONY: chart-template
chart-template: ## Render Helm chart templates for inspection
	docker run --rm -v $(PWD):$(WORKDIR) -w $(WORKDIR) $(HELM_IMAGE) \
		template kubecause charts/kubecause

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/ dist/
