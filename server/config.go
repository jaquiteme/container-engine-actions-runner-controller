package main

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	RunnerRepoPath        string
	RunnerRepoAccessToken string
	RunnerContainerImage  string
	RunnerContainerEngine string
	WebhookToken          string
	RunnerLabels          []string
}

// ReadConfig reads configuration from environment variables and returns a validated Config.
// Required: GH_RUNNER_REPO_PATH, GH_RUNNER_REPO_ACCESS_TOKEN, GH_RUNNER_CT_IMAGE, GH_WEBHOOK_SECRET.
// Optional: CT_ENGINE, GH_RUNNER_LABELS (comma-separated, e.g. "self-hosted,linux").
func ReadConfig() (*Config, error) {
	cfg := Config{
		RunnerRepoPath:        os.Getenv("GH_RUNNER_REPO_PATH"),
		RunnerRepoAccessToken: strings.TrimSpace(os.Getenv("GH_RUNNER_REPO_ACCESS_TOKEN")),
		RunnerContainerImage:  os.Getenv("GH_RUNNER_CT_IMAGE"),
		RunnerContainerEngine: os.Getenv("CT_ENGINE"),
		WebhookToken:          os.Getenv("GH_WEBHOOK_SECRET"),
	}

	if cfg.RunnerRepoPath == "" {
		return nil, fmt.Errorf("env variable GH_RUNNER_REPO_PATH is required")
	}
	infoLogger.Println("Current server repo path:", cfg.RunnerRepoPath)

	if cfg.RunnerContainerImage == "" {
		return nil, fmt.Errorf("env variable GH_RUNNER_CT_IMAGE is required")
	}

	if cfg.RunnerRepoAccessToken == "" {
		return nil, fmt.Errorf("env variable GH_RUNNER_REPO_ACCESS_TOKEN is required")
	}

	if cfg.WebhookToken == "" {
		return nil, fmt.Errorf("env variable GH_WEBHOOK_SECRET is required")
	}

	if val := os.Getenv("GH_RUNNER_LABELS"); val != "" {
		for _, l := range strings.Split(val, ",") {
			if trimmed := strings.TrimSpace(l); trimmed != "" {
				cfg.RunnerLabels = append(cfg.RunnerLabels, trimmed)
			}
		}
	}

	return &cfg, nil
}
