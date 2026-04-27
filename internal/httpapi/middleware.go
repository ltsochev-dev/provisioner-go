package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"erp/provisioner/internal/tenant"
)

func (api *API) provisionerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		if token != api.provisionToken {
			writeError(w, http.StatusForbidden, "invalid bearer token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (api *API) tenantAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		if _, err := api.tenants.AuthenticateAPIKey(r.Context(), token); err != nil {
			if errors.Is(err, tenant.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid Authorization bearer")
				return
			}

			api.logger.Error("tenant authentication failed", "err", err)
			writeError(w, http.StatusInternalServerError, "authentication failed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (api *API) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		api.logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func (api *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				api.logger.Error("panic recovered", "panic", recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}
