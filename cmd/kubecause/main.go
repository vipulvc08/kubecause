// Command kubecause is the entrypoint for the kubecause RCA agent.
//
// It exposes an HTTP server that receives PagerDuty webhooks, kicks off an
// LLM-driven agent loop that gathers evidence from the local Kubernetes
// cluster, and posts a structured RCA back to the incident.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vipulvc08/kubecause/internal/agent"
	"github.com/vipulvc08/kubecause/internal/config"
	"github.com/vipulvc08/kubecause/internal/incident"
	"github.com/vipulvc08/kubecause/internal/kube"
	"github.com/vipulvc08/kubecause/internal/llm"
	"github.com/vipulvc08/kubecause/internal/llm/claude"
	"github.com/vipulvc08/kubecause/internal/pagerduty"
	"github.com/vipulvc08/kubecause/internal/tools"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	llmClient, err := buildLLM(cfg.LLM)
	if err != nil {
		logger.Error("failed to build llm client", "err", err)
		os.Exit(1)
	}

	kc, err := kube.New()
	if err != nil {
		logger.Warn("kube client unavailable — tools will fail at call time", "err", err)
	}

	registry := tools.NewRegistry()
	if kc != nil {
		registry.Register(tools.KubeEvents(kc))
		registry.Register(tools.PodLogs(kc))
		registry.Register(tools.KubeDescribe(kc))
		registry.Register(tools.RolloutHistory(kc))
	}
	logger.Info("tools registered", "count", len(registry.Specs()))

	pdClient := pagerduty.NewClient(cfg.PagerDuty.APIToken)
	ag := agent.New(llmClient, registry, agent.Options{})
	orch := incident.New(pdClient, ag)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/webhook/pagerduty", pagerduty.NewWebhookHandler(cfg.PagerDuty, orch))

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("kubecause listening", "addr", cfg.Server.Addr, "llm", llmClient.Name())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}

func buildLLM(cfg config.LLMConfig) (llm.Client, error) {
	switch cfg.Provider {
	case "", "claude":
		return claude.New(cfg.APIKey, cfg.Model), nil
	default:
		return nil, errors.New("llm: unknown provider " + cfg.Provider)
	}
}
