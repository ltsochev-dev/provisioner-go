package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"erp/provisioner/internal/tenant"
)

type ServerConfig struct {
	Addr               string
	ProvisionToken     string
	TenantService      *tenant.Service
	ProvisioningWorker ProvisioningWorker
	Logger             *slog.Logger
}

func NewServer(cfg ServerConfig) *http.Server {
	api := &API{
		provisionToken: cfg.ProvisionToken,
		tenants:        cfg.TenantService,
		provisioning:   cfg.ProvisioningWorker,
		logger:         cfg.Logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.health)
	mux.Handle("POST /tenants", api.provisionerAuth(http.HandlerFunc(api.createTenant)))
	mux.Handle("GET /tenants", api.provisionerAuth(http.HandlerFunc(api.getTenants)))
	mux.Handle("GET /tenant/{slug}", api.tenantAuth(http.HandlerFunc(api.getTenant)))
	mux.Handle("GET /tenants/{slug}", api.tenantAuth(http.HandlerFunc(api.getTenant)))

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.recoverPanic(api.requestLogger(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
