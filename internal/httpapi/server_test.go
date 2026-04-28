package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"erp/provisioner/internal/tenant"
)

type fakeTenantStore struct {
	tenantByKey map[string]tenant.Tenant
}

func (s fakeTenantStore) All(context.Context) ([]tenant.Tenant, error) {
	tenants := make([]tenant.Tenant, 0, len(s.tenantByKey))
	for _, tenant := range s.tenantByKey {
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}

func (s fakeTenantStore) CreateTenant(_ context.Context, insert tenant.TenantInsert) (tenant.Tenant, error) {
	return tenant.Tenant{
		ID:     insert.ID,
		Email:  insert.Email,
		Name:   insert.Name,
		Slug:   insert.Slug,
		Domain: insert.Domain,
		Plan:   insert.Plan,
		Status: "active",
	}, nil
}

func (s fakeTenantStore) FindByAPIKey(_ context.Context, key string) (tenant.Tenant, error) {
	found, ok := s.tenantByKey[key]
	if !ok {
		return tenant.Tenant{}, tenant.ErrNotFound
	}

	return found, nil
}

func (s fakeTenantStore) FindBySlugAndAPIKey(_ context.Context, slug string, key string) (tenant.Tenant, error) {
	found, ok := s.tenantByKey[key]
	if !ok || found.Slug != slug {
		return tenant.Tenant{}, tenant.ErrNotFound
	}

	return found, nil
}

type notFoundTenantStore struct{}

func (notFoundTenantStore) All(context.Context) ([]tenant.Tenant, error) {
	return nil, tenant.ErrNotFound
}

func (notFoundTenantStore) CreateTenant(context.Context, tenant.TenantInsert) (tenant.Tenant, error) {
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (notFoundTenantStore) FindByAPIKey(context.Context, string) (tenant.Tenant, error) {
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (notFoundTenantStore) FindBySlugAndAPIKey(context.Context, string, string) (tenant.Tenant, error) {
	return tenant.Tenant{}, tenant.ErrNotFound
}

type fakeProvisioningWorker struct {
	triggered bool
}

func (w *fakeProvisioningWorker) Trigger() {
	w.triggered = true
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
	req := httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(`{"email":"admin@acme.example","name":"Acme Ltd","slug":"acme","domain":"acme.example.com","plan":"starter"}`))
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateTenant(t *testing.T) {
	t.Parallel()

	server := testServer()
	req := httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(`{"email":"admin@acme.example","name":"Acme Ltd","slug":"acme","domain":"acme.example.com","plan":"starter"}`))
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

func TestCreateTenantTriggersProvisioningWorker(t *testing.T) {
	t.Parallel()

	worker := &fakeProvisioningWorker{}
	server := NewServer(ServerConfig{
		Addr:               ":0",
		ProvisionToken:     "test-token",
		TenantService:      tenant.NewService(fakeTenantStore{tenantByKey: map[string]tenant.Tenant{}}),
		ProvisioningWorker: worker,
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	req := httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(`{"email":"admin@acme.example","name":"Acme Ltd","slug":"acme","domain":"acme.example.com","plan":"starter"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if !worker.triggered {
		t.Fatal("provisioning worker was not triggered")
	}
}

func TestGetTenantsRequiresProvisionerToken(t *testing.T) {
	t.Parallel()

	server := testServer()
	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetTenants(t *testing.T) {
	t.Parallel()

	server := NewServer(ServerConfig{
		Addr:           ":0",
		ProvisionToken: "test-token",
		TenantService: tenant.NewService(fakeTenantStore{
			tenantByKey: map[string]tenant.Tenant{
				"acme-key": {
					ID:     "11111111-1111-4111-8111-111111111111",
					Email:  "admin@acme.example",
					Name:   "Acme Ltd",
					Slug:   "acme",
					Domain: "acme.example",
					Plan:   "starter",
					Status: "active",
				},
			},
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got []tenant.Tenant
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got) != 1 || got[0].Slug != "acme" {
		t.Fatalf("tenants = %+v, want acme tenant", got)
	}
}

func TestGetTenantReturnsTenantForOwnerKey(t *testing.T) {
	t.Parallel()

	server := NewServer(ServerConfig{
		Addr:           ":0",
		ProvisionToken: "test-token",
		TenantService: tenant.NewService(fakeTenantStore{
			tenantByKey: map[string]tenant.Tenant{
				"acme-key": {
					ID:     "11111111-1111-4111-8111-111111111111",
					Email:  "admin@acme.example",
					Name:   "Acme Ltd",
					Slug:   "acme",
					Domain: "acme.example",
					Plan:   "starter",
					Status: "active",
				},
			},
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	req := httptest.NewRequest(http.MethodGet, "/tenants/acme", nil)
	req.Header.Set("Authorization", "Bearer acme-key")
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got tenant.Tenant
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != "11111111-1111-4111-8111-111111111111" || got.Email != "admin@acme.example" || got.Name != "Acme Ltd" || got.Slug != "acme" || got.Domain != "acme.example" || got.Plan != "starter" || got.Status != "active" {
		t.Fatalf("tenant = %+v, want populated acme tenant", got)
	}
}

func TestGetTenantSupportsSingularRoute(t *testing.T) {
	t.Parallel()

	server := NewServer(ServerConfig{
		Addr:           ":0",
		ProvisionToken: "test-token",
		TenantService: tenant.NewService(fakeTenantStore{
			tenantByKey: map[string]tenant.Tenant{
				"acme-key": {
					ID:     "11111111-1111-4111-8111-111111111111",
					Slug:   "acme",
					Domain: "acme.example",
					Plan:   "starter",
					Status: "active",
				},
			},
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	req := httptest.NewRequest(http.MethodGet, "/tenant/acme", nil)
	req.Header.Set("Authorization", "Bearer acme-key")
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGetTenantRejectsKeyForDifferentTenant(t *testing.T) {
	t.Parallel()

	server := NewServer(ServerConfig{
		Addr:           ":0",
		ProvisionToken: "test-token",
		TenantService: tenant.NewService(fakeTenantStore{
			tenantByKey: map[string]tenant.Tenant{
				"globex-key": {
					ID:     "22222222-2222-4222-8222-222222222222",
					Slug:   "globex",
					Status: "active",
				},
			},
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	req := httptest.NewRequest(http.MethodGet, "/tenants/acme", nil)
	req.Header.Set("Authorization", "Bearer globex-key")
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func testServer() *http.Server {
	return NewServer(ServerConfig{
		Addr:           ":0",
		ProvisionToken: "test-token",
		TenantService: tenant.NewService(fakeTenantStore{
			tenantByKey: map[string]tenant.Tenant{},
		}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}
