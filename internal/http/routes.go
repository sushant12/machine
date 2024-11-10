package http

import (
	"machine/internal/http/handlers"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/create", handlers.CreateHandler)
	mux.HandleFunc("/api/v1/stop", handlers.StopHandler)
	mux.HandleFunc("/api/v1/destroy", handlers.DestroyHandler)
	mux.HandleFunc("/api/v1/status", handlers.StatusHandler)
}
