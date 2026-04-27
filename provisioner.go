package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

type TenantRequest struct {
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	Plan   string `json:"plan"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	db, dbErr := openDB()
	if dbErr != nil {
		slog.Error("database connection failed", "err", dbErr)
		panic(dbErr)
	}

	defer db.Close()

	token := getEnv("PROVISIONER_TOKEN", "dev-token")

	mux := http.NewServeMux()

	mux.Handle("GET /health", http.HandlerFunc(healthHandler))
	mux.Handle("POST /tenants", authMiddleware(token, http.HandlerFunc(createTenantHandler)))
	mux.Handle("GET /tenants/{slug}", authMiddleware(token, http.HandlerFunc(getTenantHandler)))
	// mux.Handle("PATCH /tenants/{slug}", authMiddleware(token, http.HandlerFunc(updateTenantHandler)))
	// mux.Handle("DELETE /tenants/{slug}", authMiddleware(token, http.HandlerFunc(deleteTenantHandler)))

	port := getEnv("PROVISIONER_PORT", "8181")

	slog.Info("starting server", "port", port)

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
	if err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func createTenantHandler(w http.ResponseWriter, r *http.Request) {
	var req TenantRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if errMsg := validateTenantRequest(req); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	// TODO:
	// 1. create DB
	// 2. create DB user
	// 3. create k8s namespace
	// 4. create secret/deployment/service/ingress
	// 5. run migration job

	slog.Info("creating tenant",
		"slug", req.Slug,
		"domain", req.Domain,
		"plan", req.Plan,
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "provisioning",
		"tenant": req.Slug,
	})
}

func getTenantHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant": slug,
		"status": "active",
	})
}

func validateTenantRequest(req TenantRequest) string {
	if req.Slug == "" {
		return "slug is required"
	}

	if req.Domain == "" {
		return "domain is required"
	}

	if req.Plan == "" {
		return "plan is required"
	}

	if !isSafeSlug(req.Slug) {
		return "slug may only contain lowercase letters, numbers, and hyphens"
	}

	return ""
}
