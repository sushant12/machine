package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartVMHandler(t *testing.T) {
	vmConfig := VMConfig{
		Config: struct {
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
		}{
			Init: struct {
				Exec []string `json:"exec"`
			}{
				Exec: []string{"/bin/sleep", "inf"},
			},
			AutoDestroy: true,
			Image:       "alpine:latest",
			Files: []struct {
				GuestPath string `json:"guest_path"`
				RawValue  string `json:"raw_value"`
			}{
				{
					GuestPath: "/main.sh",
					RawValue:  "example-base64-encoded-value",
				},
			},
			Guest: struct {
				CPUKind  string `json:"cpu_kind"`
				CPUs     int    `json:"cpus"`
				MemoryMB int    `json:"memory_mb"`
			}{
				CPUKind:  "shared",
				CPUs:     2,
				MemoryMB: 2048,
			},
		},
	}

	body, err := json.Marshal(vmConfig)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/start-vm", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(startVMHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := "VM started with config"
	if !bytes.Contains(rr.Body.Bytes(), []byte(expected)) {
		t.Errorf("Handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}
