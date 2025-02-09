package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type VMConfig struct {
	Config struct {
		Init struct {
			Exec []string `json:"exec"`
		} `json:"init"`
		AutoDestroy bool   `json:"auto_destroy"`
		Image       string `json:"image"`
		Files       []struct {
			GuestPath string `json:"guest_path"`
			RawValue  string `json:"raw_value"`
		} `json:"files"`
		Guest struct {
			CPUKind  string `json:"cpu_kind"`
			CPUs     int    `json:"cpus"`
			MemoryMB int    `json:"memory_mb"`
		} `json:"guest"`
	} `json:"config"`
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Errorf("Command failed: %s %v\nStdout: %s\nStderr: %s", name, args, stdout.String(), stderr.String())
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	logrus.Infof("Command succeeded: %s %v\nStdout: %s", name, args, stdout.String())
	return nil
}

func extractRootFS(imageName, outputDir string) error {
	dockerCmd := []string{
		"run", "--privileged",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", fmt.Sprintf("%s:/output", outputDir),
		"anyfiddle/firecracker-rootfs-builder", imageName,
	}
	if err := runCommand("docker", dockerCmd...); err != nil {
		return fmt.Errorf("failed to run Docker command: %w", err)
	}

	return nil
}

func makeUnixSocketRequest(socketPath, method, endpoint string, body interface{}) (*http.Response, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to unix socket: %w", err)
	}
	defer conn.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return conn, nil
			},
		},
	}

	url := "http://unix" + endpoint
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, url, &reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

func startFirecrackerInstance(vmConfig VMConfig, rootfsPath, socketPath string) error {
	cmd := exec.Command("./firecracker", "--api-sock", socketPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start firecracker process: %w", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(2 * time.Second)

	vmConfigBody := map[string]interface{}{
		"vcpu_count":   vmConfig.Config.Guest.CPUs,
		"mem_size_mib": vmConfig.Config.Guest.MemoryMB,
		"ht_enabled":   false,
	}
	if _, err := makeUnixSocketRequest(socketPath, "PUT", "/machine-config", vmConfigBody); err != nil {
		return fmt.Errorf("failed to set VM configuration: %w", err)
	}

	driveBody := map[string]interface{}{
		"drive_id":       "rootfs",
		"path_on_host":   rootfsPath,
		"is_root_device": true,
		"is_read_only":   false,
	}
	if _, err := makeUnixSocketRequest(socketPath, "PUT", "/drives/rootfs", driveBody); err != nil {
		return fmt.Errorf("failed to set root drive: %w", err)
	}

	actionBody := map[string]interface{}{
		"action_type": "InstanceStart",
	}
	if _, err := makeUnixSocketRequest(socketPath, "PUT", "/actions", actionBody); err != nil {
		return fmt.Errorf("failed to start firecracker instance: %w", err)
	}

	return nil
}

func startVMHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var vmConfig VMConfig
	if err := json.NewDecoder(r.Body).Decode(&vmConfig); err != nil {
		logrus.WithError(err).Error("Failed to decode JSON")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	outputDir := "./output"
	if err := extractRootFS(vmConfig.Config.Image, outputDir); err != nil {
		logrus.WithError(err).Error("Failed to extract rootfs")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	socketPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-%d.socket", time.Now().UnixNano()))

	rootfsPath := filepath.Join(outputDir, "rootfs.img")
	if err := startFirecrackerInstance(vmConfig, rootfsPath, socketPath); err != nil {
		logrus.WithError(err).Error("Failed to start Firecracker instance")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logrus.Infof("VM started with config: %+v", vmConfig)
	fmt.Fprintf(w, "VM started with config: %+v\n", vmConfig)
}

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	r := mux.NewRouter()
	r.HandleFunc("/start-vm", startVMHandler).Methods("POST")

	logrus.Info("Server is listening on port 8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}
