package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
)

// Github Workflow Event Type
type WorkflowJobEvent struct {
	Action      string `json:"action"`
	WorkflowJob struct {
		ID int `json:"id"`
	} `json:"workflow_job"`
}

// Autodect container engine execution installed on a host
func WhichContainerEngine() (string, error) {
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "podman", nil
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "docker", nil
	}
	return "none", fmt.Errorf("No container engine found on this server.")
}

// GetContainerShortID return short ID of a container full ID
// by default we substract the first 12 char
func GetContainerShortID(fullID string) string {
	if len(fullID) < 12 {
		return fullID
	}
	return fullID[:12]
}

// GetContainerSocketPath return container engine socket path
// Parameters:
func GetContainerSocketPath(ce string) string {
	if ce == "podman" {
		// For rootful podman
		podmanSocketPath := "/run/podman/podman.sock"
		// For rootless podman
		if val := os.Getenv("XDG_RUNTIME_DIR"); val != "" {
			podmanSocketPath = val + "/podman/podman.sock"
		}
		return podmanSocketPath
	}
	return "/var/run/docker.sock"
}

// ListenContainerEvents start listen on container events
// and trigger a callback when the container is terminating
func ListenContainerEvents(client *docker.Client, onDie func(containerID string, exitCode string)) error {
	events := make(chan *docker.APIEvents)
	if err := client.AddEventListener(events); err != nil {
		return err
	}

	infoLogger.Println("Start listening on container events.")
	go func() {
		for ev := range events {
			infoLogger.Printf("Event received: %s on container %s", ev.Status, GetContainerShortID(ev.ID))
			if ev.Status == "die" && os.Getenv("LOG_LEVEL") != "debug" {
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

// ProvisionNewContainer creates and starts a container using the specified container engine socket,
// image name, and environment variables. It listens for container termination events and handles cleanup or error logging.
// Parameters:
//   - ce: the container engine type ("docker" or "podman").
//   - imageName: the name of the container image to use.
//   - env: a slice of environment variables to set in the container.
func ProvisionNewContainer(client *docker.Client, imageName string, env []string) error {
	container, err := CreateContainer(client, imageName, env)
	if err != nil {
		return err
	}

	err = client.StartContainer(container.ID, nil)
	if err != nil {
		return fmt.Errorf("Encounter an error when starting container: %v", err)
	}
	infoLogger.Println("Container started with ID:", GetContainerShortID(container.ID))
	return nil
}

// InitLocalContainerClient
func InitLocalContainerClient(ce string) (*docker.Client, error) {
	socket := GetContainerSocketPath(ce)
	infoLogger.Println("Container engine socket path found:", socket)
	client, err := docker.NewClient("unix://" + socket)
	if err != nil {
		return nil, fmt.Errorf("unable to init Docker client: %v", err)
	}
	return client, nil
}

// CreateContainer
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
			CapAdd: []string{"all"},
		},
	}
	container, err := client.CreateContainer(opts)
	if err != nil {
		return nil, fmt.Errorf("Encounter an error when creating container: %v", err)
	}
	return container, nil
}

// handleContainerExit remove a container when the exit code = 0
// and print a error when a container terminated with an error (exit code != 0)
func handleContainerExit(client *docker.Client) func(string, string) {
	return func(containerID string, exitCode string) {
		if _exitCode, err := strconv.Atoi(exitCode); err == nil {
			if _exitCode != 0 {
				errorLogger.Printf("Container %s terminated with exit code %s\n", containerID, exitCode)
				errorLogger.Println("To find out what happened, please inspect the container logs")
			} else {
				infoLogger.Printf("Container %s terminated with exit code %s\n", containerID, exitCode)
				client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID})
			}
		} else {
			errorLogger.Println(err)
		}
	}
}

// Check if crypto signature are equals
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
	// Limit the number of concurrent container creations
	maxConcurrentContainers = 5
	containerJobQueue       = make(chan ContainerOpts, 100) // buffered channel for jobs
)

func containerWorker() {
	for val := range containerJobQueue {
		err := ProvisionNewContainer(val.Client, val.Image, val.Env)
		if err != nil {
			errorLogger.Println(err)
		}
	}
}

// Webhook endpoint handler
func (sm *ServerConfigManager) webhookHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Hub-Signature-256")
	secret := sm.Config.WebhookToken

	// If a secret is configured, require and validate signature.
	if secret != "" {
		if signature == "" {
			http.Error(w, "missing signature", http.StatusUnauthorized)
			return
		}
		if !isValidSignature(body, signature, secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	} else if signature != "" {
		warningLogger.Println("Webhook secret is not set; skipping signature validation")
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

	infoLogger.Printf("New job queued: ID=%d", event.WorkflowJob.ID)

	runnerRegistrationToken, err := sm.getRunnerRegistationToken()
	if err != nil {
		errorLogger.Fatalln(err)
		return
	}

	select {
	case containerJobQueue <- ContainerOpts{
		Client: sm.ContainerClient,
		Image:  sm.Config.RunnerContainerImage,
		Env: []string{
			"GH_RUNNER_REPO_PATH=" + sm.Config.RunnerRepoPath,
			"GH_RUNNER_TOKEN=" + runnerRegistrationToken,
		}}:
		infoLogger.Println("Job added to container creation queue")
	default:
		warningLogger.Println("Container creation queue is full, dropping job")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

// Pull a container image from a registry
func PullContainerImage(client *docker.Client, imageName string) (bool, error) {
	infoLogger.Printf("Trying to pull container image %s.", imageName)
	// Split image name parts
	parts := strings.Split(imageName, ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid image name format: %s (expected repo:tag)", imageName)
	}

	if err := client.PullImage(docker.PullImageOptions{
		Repository: parts[0],
		Tag:        parts[1],
	}, docker.AuthConfiguration{}); err != nil {
		return false, fmt.Errorf("failed to pull container image %s: %w", imageName, err)
	}

	infoLogger.Printf("Container image %s pulled successfully.", imageName)
	return true, nil
}

// Main
func main() {
	port := 3000

	cfg, err := ReadConfig()
	if err != nil {
		errorLogger.Fatal(err)
	}
	// Detect container engine
	ce := cfg.RunnerContainerEngine
	if ce == "" {
		ce, _ = WhichContainerEngine()
	}
	infoLogger.Println("Container Engine:", ce)
	// Init container client
	containerClient, err := InitLocalContainerClient(ce)
	if err != nil {
		errorLogger.Fatal(err)
	}
	// Check if container image exists so the server fail fast
	// when image not found
	imageExists, err := PullContainerImage(containerClient, cfg.RunnerContainerImage)
	if !imageExists {
		errorLogger.Fatal(err)
	}
	// Init the server config manager
	manager := &ServerConfigManager{
		Config:          cfg,
		ContainerClient: containerClient,
	}

	// Get runner registration token, so the server fail fast,
	// when auth server (GitHub) return an error
	_, err = manager.getRunnerRegistationToken()
	if err != nil {
		errorLogger.Fatal(err)
	}
	// Start worker pool
	for range maxConcurrentContainers {
		go containerWorker()
	}
	// Listening to container events
	ListenContainerEvents(containerClient, handleContainerExit(containerClient))

	if os.Getenv("PORT") != "" {
		if _port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
			port = _port
		} else {
			fmt.Printf("Cannot convert %s into integer\n", os.Getenv("PORT"))
		}
	}

	// Http handlers
	http.HandleFunc("/webhook", manager.webhookHandler)

	infoLogger.Printf("Github webhook server is listening on port %d\n", port)
	addr := fmt.Sprintf(":%d", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		errorLogger.Printf("Server error: %v", err)
	}
}
