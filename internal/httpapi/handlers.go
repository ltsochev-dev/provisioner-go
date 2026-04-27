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

		api.logger.Error("tenant creation failed", "err", err)
		writeError(w, http.StatusInternalServerError, "tenant creation failed")
		return
	}

	api.logger.Info("creating tenant", "slug", req.Slug, "domain", req.Domain, "plan", req.Plan)
	writeJSON(w, http.StatusCreated, resp)
}

func (api *API) getTenant(w http.ResponseWriter, r *http.Request) {
	resp, err := api.tenants.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		var validationErr tenant.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}

		api.logger.Error("tenant lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, "tenant lookup failed")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
