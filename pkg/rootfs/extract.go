package rootfs

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func extractLayerToRootFS(layer io.ReadCloser, outputDir string) error {
	tr := tar.NewReader(layer)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		path := filepath.Join(outputDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
		case tar.TypeReg:
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			file, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("creating file: %w", err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return fmt.Errorf("writing file: %w", err)
			}
			file.Close()
			
			if err := os.Chmod(path, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("setting file permissions: %w", err)
			}
		case tar.TypeSymlink:
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating parent directory for symlink: %w", err)
			}
			
			if _, err := os.Lstat(path); err == nil {
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("removing existing file before creating symlink: %w", err)
				}
			}
			
			if err := os.Symlink(header.Linkname, path); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", path, header.Linkname, err)
			}
		}
	}
	return nil
}

func ExtractFromImage(imageName, outputDir string) error {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return fmt.Errorf("parsing reference: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("getting image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}

	for _, layer := range layers {
		rc, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("getting layer: %w", err)
		}
		if err := extractLayerToRootFS(rc, outputDir); err != nil {
			rc.Close()
			return fmt.Errorf("extracting layer: %w", err)
		}
		rc.Close()
	}

	return nil
}
