package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func parseImageRef(ref string) (string, string) {
	tag := "latest"
	if idx := strings.LastIndex(ref, ":"); idx != -1 && strings.LastIndex(ref, "/") < idx {
		return ref[:idx], ref[idx+1:]
	}
	return ref, tag
}

func TestGetContainerShortID(t *testing.T) {
	tests := []struct {
		fullID   string
		expected string
	}{
		{"abcdef1234567890", "abcdef123456"},
		{"abcdef123456", "abcdef123456"},
		{"abc", "abc"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.fullID, func(t *testing.T) {
			if got := GetContainerShortID(tc.fullID); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestGetContainerSocketPath_Docker(t *testing.T) {
	got := GetContainerSocketPath("docker")
	if got != "/var/run/docker.sock" {
		t.Errorf("got %q, want /var/run/docker.sock", got)
	}
}

func TestGetContainerSocketPath_Podman_Default(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := GetContainerSocketPath("podman")
	if got != "/run/podman/podman.sock" {
		t.Errorf("got %q, want /run/podman/podman.sock", got)
	}
}

func TestGetContainerSocketPath_Podman_XDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	got := GetContainerSocketPath("podman")
	if got != "/run/user/1000/podman/podman.sock" {
		t.Errorf("got %q, want /run/user/1000/podman/podman.sock", got)
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		ref  string
		repo string
		tag  string
	}{
		{"nginx:1.25", "nginx", "1.25"},
		{"nginx", "nginx", "latest"},
		{"registry.io/myorg/app:v2", "registry.io/myorg/app", "v2"},
		{"localhost/myimage", "localhost/myimage", "latest"},
		{"registry:5000/image:tag", "registry:5000/image", "tag"},
		{"registry:5000/image", "registry:5000/image", "latest"},
		{"image:latest", "image", "latest"},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			repo, tag := parseImageRef(tc.ref)
			if repo != tc.repo || tag != tc.tag {
				t.Errorf("parseImageRef(%q) = (%q, %q), want (%q, %q)", tc.ref, repo, tag, tc.repo, tc.tag)
			}
		})
	}
}

// --- getRunnerRegistrationToken tests ---

func makeTestManager(fetcher TokenFetcher) *ServerConfigManager {
	return &ServerConfigManager{
		Config: &Config{
			RunnerRepoPath:        "owner/repo",
			RunnerRepoAccessToken: "ghtoken",
			RunnerContainerImage:  "image:latest",
			WebhookToken:          "secret",
		},
		fetcher: fetcher,
	}
}

func TestGetRunnerToken_FetchesWhenEmpty(t *testing.T) {
	called := 0
	sm := makeTestManager(func(_, _ string) (RunnerRegistrationToken, error) {
		called++
		return RunnerRegistrationToken{Value: "tok1", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
	})

	tok, err := sm.getRunnerRegistationToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "tok1" {
		t.Errorf("got %q, want tok1", tok)
	}
	if called != 1 {
		t.Errorf("fetcher called %d times, want 1", called)
	}
}

func TestGetRunnerToken_ReturnsCached(t *testing.T) {
	called := 0
	sm := makeTestManager(func(_, _ string) (RunnerRegistrationToken, error) {
		called++
		return RunnerRegistrationToken{Value: "tok2", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
	})
	sm.Token = RunnerRegistrationToken{Value: "cached", ExpiresAt: time.Now().Add(30 * time.Minute)}

	tok, err := sm.getRunnerRegistationToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "cached" {
		t.Errorf("got %q, want cached", tok)
	}
	if called != 0 {
		t.Errorf("fetcher called %d times, want 0", called)
	}
}

func TestGetRunnerToken_RefreshesNearExpiry(t *testing.T) {
	called := 0
	sm := makeTestManager(func(_, _ string) (RunnerRegistrationToken, error) {
		called++
		return RunnerRegistrationToken{Value: "refreshed", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
	})
	sm.Token = RunnerRegistrationToken{Value: "expiring", ExpiresAt: time.Now().Add(4 * time.Minute)}

	tok, err := sm.getRunnerRegistationToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "refreshed" {
		t.Errorf("got %q, want refreshed", tok)
	}
	if called != 1 {
		t.Errorf("fetcher called %d times, want 1", called)
	}
}

func TestGetRunnerToken_RefreshesExpired(t *testing.T) {
	called := 0
	sm := makeTestManager(func(_, _ string) (RunnerRegistrationToken, error) {
		called++
		return RunnerRegistrationToken{Value: "new", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
	})
	sm.Token = RunnerRegistrationToken{Value: "expired", ExpiresAt: time.Now().Add(-1 * time.Minute)}

	tok, err := sm.getRunnerRegistationToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "new" {
		t.Errorf("got %q, want new", tok)
	}
	if called != 1 {
		t.Errorf("fetcher called %d times, want 1", called)
	}
}

func TestGetRunnerToken_PropagatesError(t *testing.T) {
	sm := makeTestManager(func(_, _ string) (RunnerRegistrationToken, error) {
		return RunnerRegistrationToken{}, fmt.Errorf("github api down")
	})

	_, err := sm.getRunnerRegistationToken()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
