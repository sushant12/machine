package main

import (
	"log"
	"machine/internal/http"
)

func main() {
	log.Println("Starting server on :8080")
	if err := http.Start(); err != nil {
		log.Fatalf("could not start server: %s", err)
	}
}
