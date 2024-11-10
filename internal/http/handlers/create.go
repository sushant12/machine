package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	fmt.Fprintf(w, "Received: %+v\n", req)
}
