package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
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

func generateShortID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func createConfigFile(vmConfig VMConfig, rootfsPath, vsockPath, configFilePath string) error {
	config := map[string]interface{}{
		"boot-source": map[string]interface{}{
			"kernel_image_path": "./bin/vmlinux",
			"boot_args":         "console=ttyS0 reboot=k panic=1 pci=off init=/fly/init",
			"initrd_path":       nil,
		},
		"drives": []map[string]interface{}{
			{
				"drive_id":       "init",
				"is_root_device": true,
				"is_read_only":   false,
				"path_on_host":   "./bin/tmpinit",
			},
			{
				"drive_id":       "rootfs",
				"is_root_device": false,
				"is_read_only":   false,
				"path_on_host":   rootfsPath,
			},
		},
		"machine-config": map[string]interface{}{
			"vcpu_count":        vmConfig.Config.Guest.CPUs,
			"mem_size_mib":      vmConfig.Config.Guest.MemoryMB,
			"smt":               false,
			"track_dirty_pages": false,
			"huge_pages":        "None",
		},
		"network-interfaces": []map[string]interface{}{
			{
				"iface_id":      "eth0",
				"guest_mac":     "06:00:AC:10:00:02",
				"host_dev_name": "tap0",
			},
		},
		"vsock": map[string]interface{}{
			"guest_cid": 3,
			"uds_path":  vsockPath,
		},
	}

	configFile, err := os.Create(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer configFile.Close()

	if err := json.NewEncoder(configFile).Encode(config); err != nil {
		return fmt.Errorf("failed to encode config file: %w", err)
	}

	return nil
}

func startFirecrackerInstance(vmConfig VMConfig, rootfsPath, socketPath, vsockPath, configFilePath string) error {
	if err := createConfigFile(vmConfig, rootfsPath, vsockPath, configFilePath); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	cmd := exec.Command("sudo", "./bin/firecracker", "--api-sock", socketPath, "--config-file", configFilePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start firecracker process: %w", err)
	}
	defer func() {
		cmd.Process.Kill()
		logrus.Infof("Firecracker stdout: %s", stdout.String())
		logrus.Infof("Firecracker stderr: %s", stderr.String())
	}()

	time.Sleep(2 * time.Second)

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

	socketPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-%s.socket", generateShortID(6)))
	vsockPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-vsock-%s.sock", generateShortID(6)))
	configFilePath := filepath.Join("/tmp", fmt.Sprintf("firecracker-config-%s.json", generateShortID(6)))

	rootfsPath := filepath.Join(outputDir, "image.ext4")
	if err := startFirecrackerInstance(vmConfig, rootfsPath, socketPath, vsockPath, configFilePath); err != nil {
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
