package rest

import (
	"encoding/json"
	"net/http"
)

// corsMiddleware wraps a handler with permissive CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sendJSON writes a JSON body with the given HTTP status code.
func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

// sendError wraps errors in a machine-readable JSON envelope.
func sendError(w http.ResponseWriter, status int, code, message string) {
	sendJSON(w, status, map[string]string{"error": message, "code": code})
}

// Mount registers all REST handlers on the given mux.
func Mount(mux *http.ServeMux) {
	mux.Handle("/v1/sessions", corsMiddleware(http.HandlerFunc(handleSessions)))
	mux.Handle("/v1/sessions/", corsMiddleware(http.HandlerFunc(handleSessionDetail)))

	mux.Handle("/v1/models", corsMiddleware(http.HandlerFunc(handleModels)))
	mux.Handle("/v1/agents", corsMiddleware(http.HandlerFunc(handleAgents)))

	mux.Handle("/v1/tools", corsMiddleware(http.HandlerFunc(handleTools)))
	mux.Handle("/v1/capabilities", corsMiddleware(http.HandlerFunc(handleCapabilities)))

	mux.Handle("/v1/interact", corsMiddleware(http.HandlerFunc(handleInteract)))
}
