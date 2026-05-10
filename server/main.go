package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

type WorkflowJobEvent struct {
	Action      string `json:"action"`
	WorkflowJob struct {
		ID     int      `json:"id"`
		Labels []string `json:"labels"`
	} `json:"workflow_job"`
}

// jobHasRequiredLabels returns true if jobLabels contains all entries in required.
// An empty required slice matches any job.
func jobHasRequiredLabels(jobLabels []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	labelSet := make(map[string]struct{}, len(jobLabels))
	for _, l := range jobLabels {
		labelSet[l] = struct{}{}
	}
	for _, r := range required {
		if _, ok := labelSet[r]; !ok {
			return false
		}
	}
	return true
}

// WhichContainerEngine auto-detects the container engine available on the host.
func WhichContainerEngine() (string, error) {
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "podman", nil
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "docker", nil
	}
	return "none", fmt.Errorf("no container engine found on this server")
}

// GetContainerShortID returns the first 12 characters of a container ID.
func GetContainerShortID(fullID string) string {
	if len(fullID) < 12 {
		return fullID
	}
	return fullID[:12]
}

// GetContainerSocketPath returns the socket path for the given container engine.
func GetContainerSocketPath(ce string) string {
	if ce == "podman" {
		podmanSocketPath := "/run/podman/podman.sock"
		if val := os.Getenv("XDG_RUNTIME_DIR"); val != "" {
			podmanSocketPath = val + "/podman/podman.sock"
		}
		return podmanSocketPath
	}
	return "/var/run/docker.sock"
}

// ListenContainerEvents subscribes to container events and calls onDie when a container exits.
func ListenContainerEvents(client *docker.Client, onDie func(containerID string, exitCode string)) error {
	events := make(chan *docker.APIEvents)
	if err := client.AddEventListener(events); err != nil {
		return err
	}

	infoLogger.Println("Start listening on container events.")
	go func() {
		for ev := range events {
			infoLogger.Printf("Event received: %s on container %s", ev.Status, GetContainerShortID(ev.ID))
			if ev.Status == "die" {
				// exitCode => docker, containerExitCode => podman
				exitCode := ev.Actor.Attributes["containerExitCode"]
				if exitCode == "" {
					exitCode = ev.Actor.Attributes["exitCode"]
				}
				onDie(ev.ID, exitCode)
			}
		}
	}()

	return nil
}

// ProvisionNewContainer creates and starts a container from the given image with the given env vars.
func ProvisionNewContainer(client *docker.Client, imageName string, env []string) error {
	container, err := CreateContainer(client, imageName, env)
	if err != nil {
		return err
	}

	if err = client.StartContainer(container.ID, nil); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}
	infoLogger.Println("Container started with ID:", GetContainerShortID(container.ID))
	return nil
}

// InitLocalContainerClient creates a Docker-compatible client connected to the local socket.
func InitLocalContainerClient(ce string) (*docker.Client, error) {
	socket := GetContainerSocketPath(ce)
	infoLogger.Println("Container engine socket path found:", socket)
	client, err := docker.NewClient("unix://" + socket)
	if err != nil {
		return nil, fmt.Errorf("unable to init container client: %v", err)
	}
	return client, nil
}

// CreateContainer creates (but does not start) a labelled runner container.
// Drops all Linux capabilities and adds back only what a CI runner needs.
func CreateContainer(client *docker.Client, imageName string, env []string) (*docker.Container, error) {
	opts := docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: imageName,
			Env:   env,
			Labels: map[string]string{
				"kind":     "runner",
				"platform": "github",
			},
		},
		HostConfig: &docker.HostConfig{
			CapDrop: []string{"ALL"},
			CapAdd:  []string{"CHOWN", "SETUID", "SETGID", "FOWNER", "NET_BIND_SERVICE", "SYS_PTRACE"},
		},
	}
	container, err := client.CreateContainer(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %v", err)
	}
	return container, nil
}

// handleContainerExit returns a callback that removes successfully exited containers
// and logs failures for non-zero exit codes.
func handleContainerExit(client *docker.Client) func(string, string) {
	return func(containerID string, exitCode string) {
		_exitCode, err := strconv.Atoi(exitCode)
		if err != nil {
			errorLogger.Printf("Failed to parse exit code %q for container %s: %v", exitCode, GetContainerShortID(containerID), err)
			return
		}
		if _exitCode != 0 {
			errorLogger.Printf("Container %s terminated with exit code %d — inspect logs for details", GetContainerShortID(containerID), _exitCode)
			return
		}
		infoLogger.Printf("Container %s terminated successfully, removing", GetContainerShortID(containerID))
		if err := client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID}); err != nil {
			errorLogger.Printf("Failed to remove container %s: %v", GetContainerShortID(containerID), err)
		}
	}
}

// isValidSignature verifies an HMAC-SHA256 webhook signature against a secret.
func isValidSignature(body []byte, signature string, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMac := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedMac), []byte(signature))
}

type ContainerOpts struct {
	Client *docker.Client
	Image  string
	Env    []string
}

var (
	maxConcurrentContainers = 5
	containerJobQueue       = make(chan ContainerOpts, 100)
)

func containerWorker() {
	for val := range containerJobQueue {
		if err := ProvisionNewContainer(val.Client, val.Image, val.Env); err != nil {
			errorLogger.Println(err)
		}
	}
}

// ProvisionNewRunner requests a registration token and enqueues a new container runner.
func ProvisionNewRunner(sm *ServerConfigManager) {
	runnerRegistrationToken, err := sm.getRunnerRegistationToken()
	if err != nil {
		errorLogger.Println(err)
		return
	}

	select {
	case containerJobQueue <- ContainerOpts{
		Client: sm.ContainerClient,
		Image:  sm.Config.RunnerContainerImage,
		Env: []string{
			"REPO_URL=https://github.com/" + sm.Config.RunnerRepoPath,
			"RUNNER_TOKEN=" + runnerRegistrationToken,
			"DISABLE_AUTO_UPDATE=true",
			"EPHEMERAL=true",
		}}:
		infoLogger.Println("Job added to container creation queue")
	default:
		warningLogger.Println("Container creation queue is full, dropping job")
	}
}

// webhookHandler processes GitHub webhook payloads and provisions runners on queued workflow_job events.
func (sm *ServerConfigManager) webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-GitHub-Event") != "workflow_job" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Hub-Signature-256")
	secret := sm.Config.WebhookToken

	if signature == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}
	if !isValidSignature(body, signature, secret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var event WorkflowJobEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if event.Action != "queued" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
		return
	}

	if !jobHasRequiredLabels(event.WorkflowJob.Labels, sm.Config.RunnerLabels) {
		infoLogger.Printf("Job ID=%d skipped: labels %v do not match required %v", event.WorkflowJob.ID, event.WorkflowJob.Labels, sm.Config.RunnerLabels)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
		return
	}

	infoLogger.Printf("New job queued: ID=%d", event.WorkflowJob.ID)
	ProvisionNewRunner(sm)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// parseImageRef splits an image reference into repository and tag.
// Handles registries with ports (e.g. registry:5000/image:tag) and defaults tag to "latest".
func parseImageRef(ref string) (repository, tag string) {
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		return ref[:lastColon], ref[lastColon+1:]
	}
	return ref, "latest"
}

// PullContainerImage pulls an image from a remote registry.
// Localhost images are skipped with a warning.
func PullContainerImage(client *docker.Client, imageName string) error {
	infoLogger.Printf("Trying to pull container image %s.", imageName)

	repo, tag := parseImageRef(imageName)

	if strings.HasPrefix(repo, "localhost") {
		warningLogger.Printf("Skipping pull for localhost image %s.", imageName)
		return nil
	}

	if err := client.PullImage(docker.PullImageOptions{
		Repository: repo,
		Tag:        tag,
	}, docker.AuthConfiguration{}); err != nil {
		return fmt.Errorf("failed to pull container image %s: %w", imageName, err)
	}

	infoLogger.Printf("Container image %s pulled successfully.", imageName)
	return nil
}

func main() {
	port := 3000

	cfg, err := ReadConfig()
	if err != nil {
		errorLogger.Fatal(err)
	}

	ce := cfg.RunnerContainerEngine
	if ce == "" {
		ce, _ = WhichContainerEngine()
	}
	infoLogger.Println("Container Engine:", ce)

	containerClient, err := InitLocalContainerClient(ce)
	if err != nil {
		errorLogger.Fatal(err)
	}

	if err := PullContainerImage(containerClient, cfg.RunnerContainerImage); err != nil {
		errorLogger.Fatal(err)
	}

	manager := &ServerConfigManager{
		Config:          cfg,
		ContainerClient: containerClient,
	}

	if _, err = manager.getRunnerRegistationToken(); err != nil {
		errorLogger.Fatal(err)
	}

	for range maxConcurrentContainers {
		go containerWorker()
	}

	if err := ListenContainerEvents(containerClient, handleContainerExit(containerClient)); err != nil {
		errorLogger.Fatal(err)
	}

	if os.Getenv("PORT") != "" {
		if _port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
			port = _port
		} else {
			warningLogger.Printf("Cannot parse PORT=%s as integer, using default %d", os.Getenv("PORT"), port)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", manager.webhookHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		infoLogger.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			errorLogger.Printf("Server shutdown error: %v", err)
		}
	}()

	infoLogger.Printf("Github webhook server is listening on port %d\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errorLogger.Printf("Server error: %v", err)
	}
}
