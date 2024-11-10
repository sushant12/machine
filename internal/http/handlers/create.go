package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

type CreateRequest struct {
	ID string `json:"id"`
}

func CreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	opts = fc.newOptions()
	firecrackerBinary := "bin/firecracker"

	ctx := context.Background()

	socketFilePath := fmt.Sprintf("/tmp/handin-firecracker-%d.socket", time.Now().UnixNano())
	cmd := firecracker.VMCommandBuilder{}.
		WithBin(firecrackerBinary).
		WithStdin(os.Stdin).
		WithStdout(io.Discard).
		WithStderr(io.Discard).
		WithSocketPath(socketFilePath).
		Build(ctx)
	cfg := firecracker.Config{
		SocketPath:      socketFilePath,
		KernelImagePath: "bin/vmlinux",
		KernelArgs:      "console=ttyS0 noapic reboot=k panic=1 pci=off nomodules rw",
		LogLevel:        "Debug",
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(1),
			Smt:        firecracker.Bool(false),
			MemSizeMib: firecracker.Int64(256),
		},
	}

	m, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		log.Fatalf("failed to create machine: %v", err)
	}

	if err := m.Start(ctx); err != nil {
		log.Fatalf("failed to start machine: %v", err)
	}
	log.Printf("Created Firecracker VM with socket: %s", socketFilePath)
	fmt.Fprintf(w, "Received: %+v\n", req)
}
