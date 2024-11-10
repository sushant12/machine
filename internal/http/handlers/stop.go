package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type StopRequest struct {
	ID string `json:"id"`
}

func StopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Received: %+v\n", req)
}
