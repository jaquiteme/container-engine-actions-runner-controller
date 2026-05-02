package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

type GHRunnerRegistrationTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type RunnerRegistrationToken struct {
	Value     string
	ExpiresAt time.Time
}

// TokenFetcher is a function that retrieves a new runner registration token.
// It is injectable to allow testing without real GitHub API calls.
type TokenFetcher func(repoPath, accessToken string) (RunnerRegistrationToken, error)

type ServerConfigManager struct {
	Mu              sync.Mutex
	Token           RunnerRegistrationToken
	Config          *Config
	ContainerClient *docker.Client
	fetcher         TokenFetcher
}

// fetchNewRunnerRegistrationTokenForPrivateRepo makes an API request to retrieve a runner
// registration token for a private repository.
func fetchNewRunnerRegistrationTokenForPrivateRepo(repoPath string, accessToken string) (RunnerRegistrationToken, error) {
	if repoPath == "" {
		return RunnerRegistrationToken{}, fmt.Errorf("repoPath is required")
	}
	if accessToken == "" {
		return RunnerRegistrationToken{}, fmt.Errorf("accessToken is required")
	}

	url := "https://api.github.com/repos/" + repoPath + "/actions/runners/registration-token"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("failed to prepare request %s: %v", url, err)
	}
	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("request %s failed: %v", url, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("failed to read response body from %s: %v", url, err)
	}

	if res.StatusCode != http.StatusCreated {
		return RunnerRegistrationToken{}, fmt.Errorf("unexpected status %d from %s: %s", res.StatusCode, url, string(body))
	}

	var tokenRes GHRunnerRegistrationTokenResponse
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("failed to unmarshal response from %s: %v", url, err)
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, tokenRes.ExpiresAt)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("failed to parse token expiry %q: %v", tokenRes.ExpiresAt, err)
	}

	return RunnerRegistrationToken{
		Value:     tokenRes.Token,
		ExpiresAt: expiresAt,
	}, nil
}

// getRunnerRegistationToken returns a valid runner token, refreshing it when expired or near expiry.
func (tm *ServerConfigManager) getRunnerRegistationToken() (string, error) {
	tm.Mu.Lock()
	defer tm.Mu.Unlock()

	fetcher := tm.fetcher
	if fetcher == nil {
		fetcher = fetchNewRunnerRegistrationTokenForPrivateRepo
	}

	if tm.Token.Value == "" || time.Until(tm.Token.ExpiresAt) < 5*time.Minute {
		token, err := fetcher(tm.Config.RunnerRepoPath, tm.Config.RunnerRepoAccessToken)
		if err != nil {
			return "", fmt.Errorf("unable to fetch new runner registration token: %v", err)
		}
		tm.Token = token
		infoLogger.Println("Successfully fetched runner registration token.")
		return token.Value, nil
	}

	return tm.Token.Value, nil
}
