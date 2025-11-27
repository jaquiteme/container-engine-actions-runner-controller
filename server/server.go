package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	docker "github.com/fsouza/go-dockerclient"
)

const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Reset  = "\033[0m"
)

var (
	infoLogger    = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(os.Stdout, "[WARN] ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger   = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
)

// Github Workflow Event Type
type WorkflowJobEvent struct {
	Action      string `json:"action"`
	WorkflowJob struct {
		ID int `json:"id"`
	} `json:"workflow_job"`
}

// Autodect container runtime execution
func whichContainerRuntime() string {
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "podman"
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "docker"
	}
	return "none"
}

// Get container runtime socket path
func getContainerSocket(runtime string) string {
	if runtime == "podman" {
		// For rootful podman
		podman_runtime := "/run/podman/podman.sock"
		// For rootless podman
		if val := os.Getenv("XDG_RUNTIME_DIR"); val != "" {
			podman_runtime = val + "/podman/podman.sock"
		}
		return podman_runtime
	}
	return "/var/run/docker.sock"
}

// ListenContainerEvents start listen on container events
// and fire a callback when the container is terminating
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

// Create a container througth container runtime socket
func createContainerUsingSocket(runtime string, imageName string, env []string) {
	socket := getContainerSocket(runtime)
	infoLogger.Println("Container runtime socket path used:", socket)
	client, err := docker.NewClient("unix://" + socket)

	if err != nil {
		log.Fatalf("Unable to init Docker client: %v", err)
	}

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
		log.Fatalf("Encouter an error when creating container: %v", err)
	}

	err = client.StartContainer(container.ID, nil)
	if err != nil {
		log.Fatalf("Encouter an error when starting container: %v", err)
	}
	infoLogger.Println("Container started with ID:", container.ID)

	err = ListenContainerEvents(client, func(containerID string, exitCode string) {
		if _exitCode, err := strconv.Atoi(exitCode); err == nil {
			if _exitCode != 0 {
				errorLogger.Printf("Container %s terminated with exit code %s\n", containerID, exitCode)
				errorLogger.Println("To find out what happened, please inspect the container logs")
			} else {
				infoLogger.Printf("Container %s terminated with exit code %s\n", containerID, exitCode)
				client.RemoveContainer(docker.RemoveContainerOptions{
					ID: containerID,
				})
			}
		} else {
			fmt.Printf("Cannot convert %s into integer\n", os.Getenv("PORT"))
		}
	})
	if err != nil {
		errorLogger.Fatal(err)
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
	Runtime string
	Image   string
	Env     []string
}

var (
	// Limit the number of concurrent container creations
	maxConcurrentContainers = 5
	containerJobQueue       = make(chan ContainerOpts, 100) // buffered channel for jobs
)

func containerWorker() {
	for val := range containerJobQueue {
		createContainerUsingSocket(val.Runtime, val.Image, val.Env)
	}
}

// Webhook endpoint handler
func (cfg *Config) webhookHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Hub-Signature-256")
	secret := cfg.WebhookToken

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
	runtime := cfg.RunnerContainerRuntime
	if runtime == "" {
		runtime = whichContainerRuntime()
	}
	infoLogger.Println("Container runtime:", runtime)
	select {
	case containerJobQueue <- ContainerOpts{Runtime: runtime, Image: cfg.RunnerContainerImage, Env: []string{
		"GH_RUNNER_REPO_PATH=" + cfg.RunnerRepoPath,
		"GH_RUNNER_TOKEN=" + cfg.RunnerRegistrationToken}}:
		infoLogger.Println("Job added to container creation queue")
	default:
		warningLogger.Println("Container creation queue is full, dropping job")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{}`))
}

func main() {
	port := 3000

	cfg, err := readConfig()
	if err != nil {
		errorLogger.Fatal(err)
	}
	_runnerConfig := &cfg

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

	http.HandleFunc("/webhook", _runnerConfig.webhookHandler)
	infoLogger.Printf("Github webhook server is listening on port %d\n", port)
	addr := fmt.Sprintf(":%d", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		errorLogger.Println("Server error:", err)
	}
}
