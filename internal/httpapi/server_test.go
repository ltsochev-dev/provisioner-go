package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"erp/provisioner/internal/tenant"
)

type fakeTenantStore struct{}

func (fakeTenantStore) FindByAPIKey(context.Context, string) (tenant.Tenant, error) {
	return tenant.Tenant{}, tenant.ErrNotFound
}

func TestHealthDoesNotRequireAuth(t *testing.T) {
	t.Parallel()

	server := testServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCreateTenantRequiresProvisionerToken(t *testing.T) {
	t.Parallel()

	server := testServer()
	req := httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(`{"slug":"acme","domain":"acme.example.com","plan":"starter"}`))
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateTenant(t *testing.T) {
	t.Parallel()

	server := testServer()
	req := httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(`{"slug":"acme","domain":"acme.example.com","plan":"starter"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"tenant":"acme"`) {
		t.Fatalf("response body = %s, want tenant slug", rec.Body.String())
	}
}

func testServer() *http.Server {
	return NewServer(ServerConfig{
		Addr:           ":0",
		ProvisionToken: "test-token",
		TenantService:  tenant.NewService(fakeTenantStore{}),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}
