package main

import (
	"database/sql"
	"net/http"
	"strings"
)

func authMiddleware(expectedToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		if header == "" {
			writeError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "invalid Authorization header")
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")

		if token != expectedToken {
			writeError(w, http.StatusForbidden, "invalid bearer token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

type Tenant struct {
	ID     int64
	Email  string
	Name   string
	Slug   string
	Domain string
	Plan   string
}

func tenantMiddleware(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		if header == "" {
			writeError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "invalid Authorization header")
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")

		if strings.Trim(token, " ") == "" {
			writeError(w, http.StatusUnauthorized, "missing Authorization bearer")
			return
		}

		row := db.QueryRow(
			`SELECT tenants.id, tenants.email, tenants.name, tenants.slug, tenants.domain, tenants.plan 
			FROM tenants 
			JOIN tenant_keys ON tenant_keys.tenant_id = tenants.id 
			WHERE tenant_keys.key = ?`,
			token,
		)

		var tenant Tenant
		err := row.Scan(&tenant.ID, &tenant.Email, &tenant.Name, &tenant.Slug, &tenant.Domain, &tenant.Plan)

		if err != nil || tenant.ID == 0 {
			writeError(w, http.StatusUnauthorized, "invalid Authorization bearer")
			return
		}

		next.ServeHTTP(w, r)
	})
}
