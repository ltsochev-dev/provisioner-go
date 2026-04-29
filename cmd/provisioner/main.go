package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erp/provisioner/internal/config"
	"erp/provisioner/internal/database"
	"erp/provisioner/internal/httpapi"
	k8s "erp/provisioner/internal/kubernetes"
	"erp/provisioner/internal/provisioning"
	"erp/provisioner/internal/tenant"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(context.Background(), logger); err != nil {
		logger.Error("provisioner stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	tenantStore := tenant.NewMySQLStore(db)
	tenantService := tenant.NewService(tenantStore)

	kubeRESTConfig, err := k8s.RESTConfig(cfg.KubeconfigPath)
	if err != nil {
		return err
	}

	kubernetesService, err := k8s.NewService(k8s.Config{RESTConfig: kubeRESTConfig})
	if err != nil {
		return err
	}

	provisioningService := provisioning.NewService(provisioning.Config{
		Store:      tenantStore,
		DB:         db,
		Kubernetes: kubernetesService,
		Logger:     logger,
	})

	server := httpapi.NewServer(httpapi.ServerConfig{
		Addr:               cfg.HTTPAddr(),
		ProvisionToken:     cfg.ProvisionerToken,
		TenantService:      tenantService,
		ProvisioningWorker: provisioningService,
		Logger:             logger,
	})

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go provisioningService.Run(ctx)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		logger.Info("shutting down server")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	}
}
