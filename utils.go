package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to write JSON response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: message,
	})
}

func isSafeSlug(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}

		if r >= '0' && r <= '9' {
			continue
		}

		if r == '-' {
			continue
		}

		return false
	}

	return true
}
