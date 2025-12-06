package main

import (
	"fmt"
	"os"
)

type Config struct {
	RunnerRepoPath        string
	RunnerRepoAccessToken string
	RunnerContainerImage  string
	RunnerContainerEngine string
	WebhookToken          string
}

// readConfig reads configuration values from environment variables and
// constructs a Config value.
//
// It populates the following Config fields from environment variables:
//   - RunnerRepoPath:          GH_RUNNER_REPO_PATH (required)
//   - RunnerRepoAccessToken: GH_RUNNER_REPO_ACCESS_TOKEN (required)
//   - RunnerContainerImage:    GH_RUNNER_CT_IMAGE (required)
//   - RunnerContainerEngine:  CT_ENGINE (optional)
//   - WebhookToken:            GH_WEBHOOK_SECRET (optional)
//
// Behavior:
//   - If RunnerRepoPath is set, its value is logged via infoLogger.
//   - If any of the required variables (GH_RUNNER_REPO_PATH,
//     GH_RUNNER_TOKEN, GH_RUNNER_CT_IMAGE) are missing, readConfig returns
//     a non-nil error explaining which environment variable is required.
//   - On success it returns the populated Config and a nil error.
func ReadConfig() (*Config, error) {
	cfg := Config{
		RunnerRepoPath:        os.Getenv("GH_RUNNER_REPO_PATH"),
		RunnerRepoAccessToken: os.Getenv("GH_RUNNER_REPO_ACCESS_TOKEN"),
		RunnerContainerImage:  os.Getenv("GH_RUNNER_CT_IMAGE"),
		RunnerContainerEngine: os.Getenv("CT_ENGINE"),
		WebhookToken:          os.Getenv("GH_WEBHOOK_SECRET"),
	}
	// Check if github repo path is set; the server will exit if this variable is not set
	if cfg.RunnerRepoPath != "" {
		infoLogger.Println("Current server repo path:", cfg.RunnerRepoPath)
	} else {
		return &Config{}, fmt.Errorf("The server cannot run without env variable 'GH_RUNNER_REPO_PATH'")
	}
	// Check if github runner is provided; the server will exit if this variable is not set
	if cfg.RunnerContainerImage == "" {
		return &Config{}, fmt.Errorf("The server cannot run without env variable 'GH_RUNNER_CT_IMAGE'")
	}
	// Check if github runner token is provided; the server will exit if this variable is not set
	if cfg.RunnerRepoAccessToken == "" {
		return &Config{}, fmt.Errorf("The server cannot run without env variable 'GH_RUNNER_REPO_ACCESS_TOKEN'")
	}
	return &cfg, nil
}
