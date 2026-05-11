package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func signBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestIsValidSignature_Valid(t *testing.T) {
	body := []byte(`{"action":"queued"}`)
	secret := "mysecret"
	sig := signBody(body, secret)
	if !isValidSignature(body, sig, secret) {
		t.Error("expected valid signature to pass")
	}
}

func TestIsValidSignature_WrongSignatureValue(t *testing.T) {
	body := []byte(`{"action":"queued"}`)
	if isValidSignature(body, "sha256=deadbeef", "mysecret") {
		t.Error("expected wrong signature value to fail")
	}
}

func TestIsValidSignature_WrongSecret(t *testing.T) {
	body := []byte(`{"action":"queued"}`)
	sig := signBody(body, "mysecret")
	if isValidSignature(body, sig, "wrongsecret") {
		t.Error("expected wrong secret to fail")
	}
}

func TestIsValidSignature_EmptySignature(t *testing.T) {
	body := []byte(`{"action":"queued"}`)
	if isValidSignature(body, "", "mysecret") {
		t.Error("expected empty signature to fail")
	}
}

func TestIsValidSignature_TamperedBody(t *testing.T) {
	original := []byte(`{"action":"queued"}`)
	sig := signBody(original, "mysecret")
	tampered := []byte(`{"action":"completed"}`)
	if isValidSignature(tampered, sig, "mysecret") {
		t.Error("expected tampered body to fail")
	}
}

// --- webhookHandler tests ---

func makeWebhookTestManager() *ServerConfigManager {
	return &ServerConfigManager{
		Config: &Config{
			RunnerRepoPath:        "owner/repo",
			RunnerRepoAccessToken: "token",
			RunnerContainerImage:  "image:latest",
			WebhookToken:          "secret",
		},
		Token: RunnerRegistrationToken{
			Value:     "regtoken",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}
}

func sendWebhook(t *testing.T, sm *ServerConfigManager, event, body, sig string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	if sig != "" {
		req.Header.Set("X-Hub-Signature-256", sig)
	}
	rr := httptest.NewRecorder()
	sm.webhookHandler(rr, req)
	return rr
}

func TestWebhookHandler_NonWorkflowJobEvent(t *testing.T) {
	sm := makeWebhookTestManager()
	rr := sendWebhook(t, sm, "push", `{}`, "")
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWebhookHandler_MissingSignature(t *testing.T) {
	sm := makeWebhookTestManager()
	body := `{"action":"queued","workflow_job":{"id":1}}`
	rr := sendWebhook(t, sm, "workflow_job", body, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	sm := makeWebhookTestManager()
	body := `{"action":"queued","workflow_job":{"id":1}}`
	rr := sendWebhook(t, sm, "workflow_job", body, "sha256=invalid")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestWebhookHandler_MalformedJSON(t *testing.T) {
	sm := makeWebhookTestManager()
	body := `not json`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_ActionCompleted(t *testing.T) {
	sm := makeWebhookTestManager()
	body := `{"action":"completed","workflow_job":{"id":1}}`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWebhookHandler_ActionQueued(t *testing.T) {
	sm := makeWebhookTestManager()
	body := `{"action":"queued","workflow_job":{"id":42}}`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWebhookHandler_LabelFilter_Match(t *testing.T) {
	sm := makeWebhookTestManager()
	sm.Config.RunnerLabels = []string{"self-hosted"}
	body := `{"action":"queued","workflow_job":{"id":1,"labels":["self-hosted","linux"]}}`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWebhookHandler_LabelFilter_NoMatch(t *testing.T) {
	sm := makeWebhookTestManager()
	sm.Config.RunnerLabels = []string{"self-hosted"}
	body := `{"action":"queued","workflow_job":{"id":2,"labels":["ubuntu-latest"]}}`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWebhookHandler_LabelFilter_NoLabelsConfigured(t *testing.T) {
	sm := makeWebhookTestManager()
	body := `{"action":"queued","workflow_job":{"id":3,"labels":["ubuntu-latest"]}}`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestWebhookHandler_LabelFilter_MultipleRequiredLabels(t *testing.T) {
	sm := makeWebhookTestManager()
	sm.Config.RunnerLabels = []string{"self-hosted", "linux"}
	body := `{"action":"queued","workflow_job":{"id":4,"labels":["self-hosted","linux","x64"]}}`
	sig := signBody([]byte(body), "secret")
	rr := sendWebhook(t, sm, "workflow_job", body, sig)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestJobHasRequiredLabels(t *testing.T) {
	tests := []struct {
		jobLabels []string
		required  []string
		want      bool
	}{
		{[]string{"self-hosted", "linux"}, []string{"self-hosted"}, true},
		{[]string{"ubuntu-latest"}, []string{"self-hosted"}, false},
		{[]string{"self-hosted", "linux"}, []string{"self-hosted", "linux"}, true},
		{[]string{"self-hosted"}, []string{"self-hosted", "linux"}, false},
		{[]string{"ubuntu-latest"}, []string{}, true},
		{[]string{}, []string{}, true},
	}
	for _, tc := range tests {
		got := jobHasRequiredLabels(tc.jobLabels, tc.required)
		if got != tc.want {
			t.Errorf("jobHasRequiredLabels(%v, %v) = %v, want %v", tc.jobLabels, tc.required, got, tc.want)
		}
	}
}
