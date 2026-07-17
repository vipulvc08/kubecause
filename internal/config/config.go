// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Server     ServerConfig
	PagerDuty  PagerDutyConfig
	LLM        LLMConfig
	Kubernetes KubernetesConfig
}

type ServerConfig struct {
	Addr string
}

type PagerDutyConfig struct {
	WebhookSecret string
	APIToken      string
}

type LLMConfig struct {
	Provider string
	APIKey   string
	Model    string
}

type KubernetesConfig struct {
	// LogTailLines caps the number of log lines returned per pod_logs call.
	LogTailLines int64
	// LogWindowSeconds is the default lookback window for log queries.
	LogWindowSeconds int64
}

func Load() (Config, error) {
	cfg := Config{
		Server: ServerConfig{
			Addr: getenv("KUBECAUSE_ADDR", ":8080"),
		},
		PagerDuty: PagerDutyConfig{
			WebhookSecret: os.Getenv("PAGERDUTY_WEBHOOK_SECRET"),
			APIToken:      os.Getenv("PAGERDUTY_API_TOKEN"),
		},
		LLM: LLMConfig{
			Provider: getenv("LLM_PROVIDER", "claude"),
			APIKey:   os.Getenv("LLM_API_KEY"),
			Model:    getenv("LLM_MODEL", ""),
		},
		Kubernetes: KubernetesConfig{
			LogTailLines:     500,
			LogWindowSeconds: 300,
		},
	}

	if cfg.PagerDuty.WebhookSecret == "" {
		return cfg, fmt.Errorf("PAGERDUTY_WEBHOOK_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
