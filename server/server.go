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
func whichContainerEngine() (string, error) {
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "podman", nil
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "docker", nil
	}
	return "none", fmt.Errorf("No container engine found on this server.")
}

// Get container engine socket path
func getContainerSocket(ce string) string {
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

	go func() {
		for ev := range events {
			if ev.Status == "die" {
				// exitCode => docker, containerExitCode => podman
				exitCode := ev.Actor.Attributes["containerExitCode"]
				onDie(ev.ID, exitCode)
			}
		}
	}()

	return nil
}

// provisionNewContainer creates and starts a container using the specified container engine socket,
// image name, and environment variables. It listens for container termination events and handles cleanup or error logging.
// Parameters:
//   - ce: the container engine type ("docker" or "podman").
//   - imageName: the name of the container image to use.
//   - env: a slice of environment variables to set in the container.
func provisionNewContainer(ce string, imageName string, env []string) error {
	client, err := initLocalContainerClient(ce)
	if err != nil {
		return err
	}

	container, err := createContainer(client, imageName, env)
	if err != nil {
		return err
	}

	err = client.StartContainer(container.ID, nil)
	if err != nil {
		return fmt.Errorf("Encounter an error when starting container: %v", err)
	}
	infoLogger.Println("Container started with ID:", container.ID)

	return ListenContainerEvents(client, handleContainerExit(client))
}

func initLocalContainerClient(ce string) (*docker.Client, error) {
	socket := getContainerSocket(ce)
	infoLogger.Println("Container engine socket path used:", socket)
	client, err := docker.NewClient("unix://" + socket)
	if err != nil {
		return nil, fmt.Errorf("unable to init Docker client: %v", err)
	}
	return client, nil
}

func createContainer(client *docker.Client, imageName string, env []string) (*docker.Container, error) {
	opts := docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: imageName,
			Env:   env,
			Labels: map[string]string{
				"kind":     "runner",
				"platform": "github",
			},
		},
	}
	container, err := client.CreateContainer(opts)
	if err != nil {
		return nil, fmt.Errorf("Encounter an error when creating container: %v", err)
	}
	return container, nil
}

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
	Engine string
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
		err := provisionNewContainer(val.Engine, val.Image, val.Env)
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
	ce := sm.Config.RunnerContainerEngine
	if ce == "" {
		ce, err = whichContainerEngine()
	}
	infoLogger.Println("Container Engine:", ce)

	runnerRegistrationToken, err := sm.getRunnerRegistationToken()
	if err != nil {
		errorLogger.Fatalln(err)
		return
	}

	select {
	case containerJobQueue <- ContainerOpts{Engine: ce, Image: sm.Config.RunnerContainerImage, Env: []string{
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

func main() {
	port := 3000

	cfg, err := ReadConfig()
	if err != nil {
		errorLogger.Fatal(err)
	}

	// TODO: check if image exists

	manager := &ServerConfigManager{
		Config: cfg,
	}

	_, err = manager.getRunnerRegistationToken()
	if err != nil {
		errorLogger.Fatal(err)
	}

	// Start worker pool
	for range maxConcurrentContainers {
		go containerWorker()
	}

	if os.Getenv("PORT") != "" {
		if _port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
			port = _port
		} else {
			fmt.Printf("Cannot convert %s into integer\n", os.Getenv("PORT"))
		}
	}

	http.HandleFunc("/webhook", manager.webhookHandler)
	infoLogger.Printf("Github webhook server is listening on port %d\n", port)
	addr := fmt.Sprintf(":%d", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		errorLogger.Println("Server error:", err)
	}
}
