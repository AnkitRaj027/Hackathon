package health

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Response represents the JSON health status payload.
type Response struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	NodeID  string `json:"node_id,omitempty"`
}

// StartHealthServer starts an HTTP server serving /health endpoint.
func StartHealthServer(port int, serviceName string, nodeID string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response{
			Status:  "healthy",
			Service: serviceName,
			NodeID:  nodeID,
		})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	return server
}
