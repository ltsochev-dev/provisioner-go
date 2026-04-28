package provisioning

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"erp/provisioner/internal/tenant"
)

const (
	defaultScanInterval     = 15 * time.Second
	defaultStaleLockTimeout = 10 * time.Minute
	defaultBatchSize        = 25
)

var ErrUnimplemented = errors.New("unimplemented")

type TenantStore interface {
	Pending(ctx context.Context, limit int) ([]tenant.Tenant, error)
	BeginProvision(ctx context.Context, tenantID string) (tenant.Tenant, error)
	ReleaseStaleProvisioning(ctx context.Context, olderThan time.Duration) ([]tenant.Tenant, error)
}

type Config struct {
	ScanInterval     time.Duration
	StaleLockTimeout time.Duration
	BatchSize        int
	Logger           *slog.Logger
	Store            TenantStore
}

type Service struct {
	scanInterval     time.Duration
	staleLockTimeout time.Duration
	batchSize        int
	logger           *slog.Logger
	store            TenantStore
	triggerCh        chan struct{}
}

func NewService(cfg Config) *Service {
	scanInterval := cfg.ScanInterval
	if scanInterval == 0 {
		scanInterval = defaultScanInterval
	}

	staleLockTimeout := cfg.StaleLockTimeout
	if staleLockTimeout == 0 {
		staleLockTimeout = defaultStaleLockTimeout
	}

	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = defaultBatchSize
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		scanInterval:     scanInterval,
		staleLockTimeout: staleLockTimeout,
		batchSize:        batchSize,
		logger:           logger,
		store:            cfg.Store,
		triggerCh:        make(chan struct{}, 1),
	}
}

func (s *Service) Trigger() {
	select {
	case s.triggerCh <- struct{}{}:
	default:
	}
}

func (s *Service) Run(ctx context.Context) {
	s.scan(ctx)

	ticker := time.NewTicker(s.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("stopping provisioning worker")
			return
		case <-s.triggerCh:
			s.scan(ctx)
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Service) scan(ctx context.Context) {
	if s.store == nil {
		s.logger.Error("provisioning worker has no tenant store")
		return
	}

	released, err := s.store.ReleaseStaleProvisioning(ctx, s.staleLockTimeout)
	if err != nil {
		s.logger.Error("release stale provisioning locks failed", "err", err)
	} else {
		for _, t := range released {
			s.logger.Error(
				"released stale provisioning lock",
				"tenant_id", t.ID,
				"slug", t.Slug,
				"stale_after", s.staleLockTimeout.String(),
			)
		}
	}

	pending, err := s.store.Pending(ctx, s.batchSize)
	if err != nil {
		s.logger.Error("pending tenant scan failed", "err", err)
		return
	}

	for _, pendingTenant := range pending {
		go s.provision(ctx, pendingTenant.ID)
	}
}

func (s *Service) provision(ctx context.Context, tenantID string) {
	t, err := s.store.BeginProvision(ctx, tenantID)
	if err != nil {
		if errors.Is(err, tenant.ErrNotFound) {
			s.logger.Debug("tenant was already claimed for provisioning", "tenant_id", tenantID)
			return
		}

		s.logger.Error("begin tenant provisioning failed", "tenant_id", tenantID, "err", err)
		return
	}

	s.logger.Info("starting tenant provisioning", "tenant_id", t.ID, "slug", t.Slug)

	steps := []struct {
		name string
		run  func(context.Context, tenant.Tenant) error
	}{
		{name: "create database", run: s.createDatabase},
		{name: "add users", run: s.addUsers},
		{name: "create k8s namespace", run: s.createK8sNamespace},
		{name: "create pods", run: s.createPods},
		{name: "create ingress", run: s.createIngress},
	}

	for _, step := range steps {
		if err := step.run(ctx, t); err != nil {
			s.logger.Error(
				"tenant provisioning step failed",
				"tenant_id", t.ID,
				"slug", t.Slug,
				"step", step.name,
				"err", err,
			)
			return
		}
	}

	s.logger.Info("tenant provisioning completed", "tenant_id", t.ID, "slug", t.Slug)
}

func (s *Service) createDatabase(ctx context.Context, t tenant.Tenant) error {
	return ErrUnimplemented
}

func (s *Service) addUsers(ctx context.Context, t tenant.Tenant) error {
	return ErrUnimplemented
}

func (s *Service) createK8sNamespace(ctx context.Context, t tenant.Tenant) error {
	return ErrUnimplemented
}

func (s *Service) createPods(ctx context.Context, t tenant.Tenant) error {
	return ErrUnimplemented
}

func (s *Service) createIngress(ctx context.Context, t tenant.Tenant) error {
	return ErrUnimplemented
}
