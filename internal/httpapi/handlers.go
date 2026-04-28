package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"erp/provisioner/internal/tenant"
)

type API struct {
	provisionToken string
	tenants        *tenant.Service
	logger         *slog.Logger
}

func (api *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (api *API) createTenant(w http.ResponseWriter, r *http.Request) {
	var req tenant.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := api.tenants.Create(r.Context(), req)
	if err != nil {
		var validationErr tenant.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}

		if errors.Is(err, tenant.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "tenant already exists")
			return
		}

		api.logger.Error("tenant creation failed", "err", err)
		writeError(w, http.StatusInternalServerError, "tenant creation failed")
		return
	}

	api.logger.Info("creating tenant", "slug", req.Slug, "domain", req.Domain, "plan", req.Plan)
	writeJSON(w, http.StatusCreated, resp)
}

func (api *API) getTenant(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
		return
	}

	resp, err := api.tenants.GetBySlug(r.Context(), r.PathValue("slug"), token)
	if err != nil {
		var validationErr tenant.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}

		if errors.Is(err, tenant.ErrNotFound) {
			writeError(w, http.StatusForbidden, "tenant key does not belong to requested tenant")
			return
		}

		api.logger.Error("tenant lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, "tenant lookup failed")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (api *API) getTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := api.tenants.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error: "+err.Error())
	}

	writeJSON(w, http.StatusOK, tenants)
}
