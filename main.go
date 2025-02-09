package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

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
	// Run the Docker command to extract the rootfs
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

	// Extract the rootfs from the provided Docker image
	outputDir := "./output"
	if err := extractRootFS(vmConfig.Config.Image, outputDir); err != nil {
		logrus.WithError(err).Error("Failed to extract rootfs")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logrus.Infof("Rootfs extracted for image: %s", vmConfig.Config.Image)
	fmt.Fprintf(w, "Rootfs extracted for image: %s\n", vmConfig.Config.Image)
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
