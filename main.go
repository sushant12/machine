package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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

func extractRootFS(imageName, outputFile string) error {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		logrus.WithError(err).Error("Error parsing image reference")
		return err
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		logrus.WithError(err).Error("Error pulling image")
		return err
	}

	digest, err := img.Digest()
	if err != nil {
		logrus.WithError(err).Error("Error getting image digest")
		return err
	}

	destDir := "./extracted_fs"
	if err := os.MkdirAll(destDir, 0755); err != nil {
		log.Fatalf("Error creating destination directory: %v", err)
	}

	// Extract the image layers in order (later layers overlay earlier ones).
	if err := extractImage(img, destDir); err != nil {
		log.Fatalf("Error extracting image: %v", err)
	}

	logrus.Infof("Successfully pulled image %s with digest: %s", imageName, digest)
	return nil
}

// extractImage iterates over the image layers and extracts each one into dest.
// Note: This example does not handle whiteout files (which remove files from previous layers).
func extractImage(img v1.Image, dest string) error {
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	for _, layer := range layers {
		if err := extractLayer(layer, dest); err != nil {
			return fmt.Errorf("failed to extract a layer: %w", err)
		}
	}
	return nil
}

// extractLayer extracts a single layer's tarball into the destination directory.
func extractLayer(layer v1.Layer, dest string) error {
	rc, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("failed to get compressed layer: %w", err)
	}
	defer rc.Close()

	gr, err := gzip.NewReader(rc)
	var tarReader *tar.Reader
	if err == nil {
		defer gr.Close()
		tarReader = tar.NewReader(gr)
	} else {
		tarReader = tar.NewReader(rc)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar entry: %w", err)
		}

		targetPath := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for file %s: %w", targetPath, err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}
		default:
		}
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

	outputFile := "/path/to/output/rootfs.img"
	if err := extractRootFS(vmConfig.Config.Image, outputFile); err != nil {
		logrus.WithError(err).Error("Failed to extract rootfs")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
