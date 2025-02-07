package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Errorf("Command failed: %s %v", name, args)
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

func extractRootFS(imageName, outputFile string) error {
	tempDir, err := os.MkdirTemp("", "rootfs")
	if err != nil {
		logrus.WithError(err).Error("Failed to create temporary directory")
		return err
	}
	defer os.RemoveAll(tempDir)

	ref, err := name.ParseReference(imageName)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse image reference")
		return err
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		logrus.WithError(err).Error("Failed to get remote image")
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		logrus.WithError(err).Error("Failed to get image layers")
		return err
	}

	for _, layer := range layers {
		uncompressed, err := layer.Uncompressed()
		if err != nil {
			logrus.WithError(err).Error("Failed to uncompress layer")
			return err
		}
		defer uncompressed.Close()

		tr := tar.NewReader(uncompressed)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("Failed to read tar header")
				return err
			}

			target := filepath.Join(tempDir, header.Name)
			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
					logrus.WithError(err).Error("Failed to create directory")
					return err
				}
			case tar.TypeReg:
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					logrus.WithError(err).Error("Failed to open file")
					return err
				}
				if _, err := io.Copy(f, tr); err != nil {
					f.Close()
					logrus.WithError(err).Error("Failed to copy file content")
					return err
				}
				f.Close()
			}
		}
	}

	// Create an ext4 filesystem from the extracted rootfs
	if err := runCommand("dd", "if=/dev/zero", fmt.Sprintf("of=%s", outputFile), "bs=1M", "count=1024"); err != nil {
		return err
	}
	if err := runCommand("mkfs.ext4", "-F", outputFile); err != nil {
		return err
	}
	if err := os.MkdirAll("/mnt/ext4", 0755); err != nil {
		logrus.WithError(err).Error("Failed to create mount directory")
		return err
	}
	if err := runCommand("mount", "-o", "loop", outputFile, "/mnt/ext4"); err != nil {
		return err
	}
	defer runCommand("umount", "/mnt/ext4")

	if err := runCommand("cp", "-a", filepath.Join(tempDir, "."), "/mnt/ext4"); err != nil {
		return err
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
	outputFile := "/bin"
	if err := extractRootFS(vmConfig.Config.Image, outputFile); err != nil {
		logrus.WithError(err).Error("Failed to extract rootfs")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Process the VMConfig here (e.g., start the VM)
	logrus.Infof("VM started with config: %+v", vmConfig)
	fmt.Fprintf(w, "VM started with config: %+v\n", vmConfig)
}

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	http.HandleFunc("/start-vm", startVMHandler)
	logrus.Info("Server is listening on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}
