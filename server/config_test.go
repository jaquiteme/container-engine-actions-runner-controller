package main

import (
	"os"
	"testing"
)

func TestReadConfig_Success(t *testing.T) {
	os.Setenv("GH_RUNNER_REPO_PATH", "/path/to/repo")
	os.Setenv("GH_RUNNER_REPO_ACCESS_TOKEN", "ghtoken123")
	os.Setenv("GH_RUNNER_CT_IMAGE", "image:latest")
	os.Setenv("CT_ENGINE", "podman")
	os.Setenv("GH_WEBHOOK_SECRET", "webhook<S3cr3t123")
	defer os.Clearenv()

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RunnerRepoPath != "/path/to/repo" {
		t.Errorf("expected /path/to/repo, got %s", cfg.RunnerRepoPath)
	}
	if cfg.RunnerRepoAccessToken != "ghtoken123" {
		t.Errorf("expected ghtoken123, got %s", cfg.RunnerRepoAccessToken)
	}
}

func TestReadConfig_MissingRepoPath(t *testing.T) {
	os.Clearenv()
	os.Setenv("GH_RUNNER_REPO_ACCESS_TOKEN", "ghtoken123")
	os.Setenv("GH_RUNNER_CT_IMAGE", "image:latest")

	_, err := ReadConfig()
	if err == nil {
		t.Fatal("expected error for missing GH_RUNNER_REPO_PATH")
	}
}

func TestReadConfig_MissingContainerImage(t *testing.T) {
	os.Clearenv()
	os.Setenv("GH_RUNNER_REPO_PATH", "/path/to/repo")
	os.Setenv("GH_RUNNER_REPO_ACCESS_TOKEN", "ghtoken12@3")

	_, err := ReadConfig()
	if err == nil {
		t.Fatal("expected error for missing GH_RUNNER_CT_IMAGE")
	}
}

func TestReadConfig_MissingAccessToken(t *testing.T) {
	os.Clearenv()
	os.Setenv("GH_RUNNER_REPO_PATH", "/path/to/repo")
	os.Setenv("GH_RUNNER_CT_IMAGE", "image:latest")

	_, err := ReadConfig()
	if err == nil {
		t.Fatal("expected error for missing GH_RUNNER_REPO_ACCESS_TOKEN")
	}
}

func TestReadConfig_OptionalFields(t *testing.T) {
	os.Clearenv()
	os.Setenv("GH_RUNNER_REPO_PATH", "/path/to/repo")
	os.Setenv("GH_RUNNER_REPO_ACCESS_TOKEN", "ghtoken123")
	os.Setenv("GH_RUNNER_CT_IMAGE", "image:latest")

	cfg, _ := ReadConfig()
	if cfg.RunnerContainerEngine != "" {
		t.Errorf("expected empty engine, got %s", cfg.RunnerContainerEngine)
	}
	if cfg.WebhookToken != "" {
		t.Errorf("expected empty webhook token, got %s", cfg.WebhookToken)
	}
}
