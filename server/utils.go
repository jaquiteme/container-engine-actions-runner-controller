package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type GHRunnerRegistrationTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// GitHub Action Runners Registration Token Type
type RunnerRegistrationToken struct {
	Value     string
	ExpiresAt time.Time
}

// token manager type with concurrency management
type ServerConfigManager struct {
	Mu     sync.Mutex
	Token  RunnerRegistrationToken
	Config Config
}

// fetchNewRunnerRegistrationTokenForPrivateRepo make an API request to retrieve a runner action token
// to registrate a self-hosted runner in a private repository
//
// Parameters:
//   - repoPath: GitHub repository path (Ex: username/repo-name)
//   - accessToken: GitHub repo access token with repo scopes as mentionned in https://docs.github.com/en/actions/reference/runners/self-hosted-runners#authentication-requirements
func fetchNewRunnerRegistrationTokenForPrivateRepo(repoPath string, accessToken string) (RunnerRegistrationToken, error) {
	if repoPath == "" {
		return RunnerRegistrationToken{}, fmt.Errorf("Please provide value for repoPath")
	}
	if accessToken == "" {
		return RunnerRegistrationToken{}, fmt.Errorf("Please provide value for accessToken")
	}

	url := "https://api.github.com/repos/" + repoPath + "/actions/runners/registration-token"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	req.Header.Add("Authorization", "Bearer "+accessToken)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("Encounter error when preparing request %s failed with : %v", url, err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("Request %s failed with : %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)

	var tokenRes GHRunnerRegistrationTokenResponse
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("Encounter error when unmashalling %s response body with : %v", url, err)
	}

	timeLayout := "2006-01-02T15:04:05.000-07:00"
	expiresAt, err := time.Parse(timeLayout, tokenRes.ExpiresAt)
	if err != nil {
		return RunnerRegistrationToken{}, fmt.Errorf("Encounter error when parsing token expires date %s response body with : %v", tokenRes.ExpiresAt, err)
	}

	return RunnerRegistrationToken{
		Value:     tokenRes.Token,
		ExpiresAt: expiresAt,
	}, nil
}

// getRunnerRegistationToken get runner token from memory
func (tm *ServerConfigManager) getRunnerRegistationToken() (string, error) {
	tm.Mu.Lock()
	defer tm.Mu.Unlock()

	if tm.Token.Value == "" || time.Until(tm.Token.ExpiresAt) < 5*time.Minute {
		token, err := fetchNewRunnerRegistrationTokenForPrivateRepo(tm.Config.RunnerRepoPath, tm.Config.RunnerRepoAccessToken)
		if err != nil {
			return "", fmt.Errorf("Unable to fetch new runner registration token: %v", err)
		}
		tm.Token = token
		infoLogger.Println("Successfuly fetched runner registration token.")
		return token.Value, nil
	}

	return tm.Token.Value, nil
}
