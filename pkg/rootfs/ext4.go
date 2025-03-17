package rootfs

import (
	"fmt"
	"os"
	"os/exec"
)

func CreateExt4Image(inputDir, outputImage string, sizeMB int) error {
	if err := createEmptyFile(outputImage, sizeMB); err != nil {
		return fmt.Errorf("creating empty file: %w", err)
	}

	if err := formatExt4(outputImage); err != nil {
		return fmt.Errorf("formatting ext4: %w", err)
	}

	mountPoint, err := os.MkdirTemp("", "ext4-mount-*")
	if err != nil {
		return fmt.Errorf("creating mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	if err := mountImage(outputImage, mountPoint); err != nil {
		return fmt.Errorf("mounting image: %w", err)
	}

	if err := copyContents(inputDir, mountPoint); err != nil {
		unmountImage(mountPoint)
		return fmt.Errorf("copying contents: %w", err)
	}

	if err := unmountImage(mountPoint); err != nil {
		return fmt.Errorf("unmounting image: %w", err)
	}

	return nil
}

func createEmptyFile(path string, sizeMB int) error {
	cmd := exec.Command("dd", "if=/dev/zero", "of="+path, fmt.Sprintf("bs=%dM", sizeMB), "count=1")
	return cmd.Run()
}

func formatExt4(imagePath string) error {
	cmd := exec.Command("mkfs.ext4", imagePath)
	return cmd.Run()
}

func mountImage(imagePath, mountPoint string) error {
	cmd := exec.Command("sudo", "mount", imagePath, mountPoint)
	return cmd.Run()
}

func unmountImage(mountPoint string) error {
	cmd := exec.Command("sudo", "umount", mountPoint)
	return cmd.Run()
}

func copyContents(src, dst string) error {
	cmd := exec.Command("sudo", "cp", "-a", src+"/.", dst+"/")
	return cmd.Run()
}
