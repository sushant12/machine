package http

import (
	"net/http"
)

func Start() error {
	mux := http.NewServeMux()
	RegisterRoutes(mux)
	return http.ListenAndServe(":8080", mux)
}
