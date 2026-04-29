package provisioning

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
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

type KubernetesService interface {
	NamespaceExists(ctx context.Context, name string) (bool, error)
	CreateNamespace(ctx context.Context, name string) error
}

type Config struct {
	ScanInterval     time.Duration
	StaleLockTimeout time.Duration
	BatchSize        int
	Logger           *slog.Logger
	Store            TenantStore
	DB               *sql.DB
	Kubernetes       KubernetesService
}

type Service struct {
	scanInterval     time.Duration
	staleLockTimeout time.Duration
	batchSize        int
	logger           *slog.Logger
	store            TenantStore
	db               *sql.DB
	kubernetes       KubernetesService
	triggerCh        chan struct{}
}

type provisionRun struct {
	tenant tenant.Tenant
	db     tenantDB
}

type tenantDB struct {
	name     string
	user     string
	password string
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
		db:               cfg.DB,
		kubernetes:       cfg.Kubernetes,
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
	if s.db == nil {
		s.logger.Error("provisioning worker has no database connection")
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

	run := &provisionRun{tenant: t}

	steps := []struct {
		name string
		run  func(context.Context, *provisionRun) error
	}{
		{name: "create k8s namespace", run: s.createK8sNamespace},
		{name: "create database", run: s.setDb},
		{name: "add secrets", run: s.addSecrets},
		{name: "create pods", run: s.createPods},
		{name: "create ingress", run: s.createIngress},
		{name: "update tenant state", run: s.finishProvisioning},
	}

	for _, step := range steps {
		if err := step.run(ctx, run); err != nil {
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

// # ensure namespace
// # ensure database
// # ensure db user
// # ensure db grants
// ensure secrets
// ensure workload
// ensure ingress

func (s *Service) createK8sNamespace(ctx context.Context, run *provisionRun) error {
	if s.kubernetes == nil {
		return errors.New("kubernetes services is required")
	}

	ns := tenantToNamespace(run.tenant)
	exists, err := s.kubernetes.NamespaceExists(ctx, ns)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return s.kubernetes.CreateNamespace(ctx, ns)
}

func (s *Service) setDb(ctx context.Context, run *provisionRun) error {
	dbName := tenantToDbName(run.tenant)
	dbUser := safeString(run.tenant.Slug, "erp_user_")
	dbPass, err := randomString(14, 18)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	query := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s`",
		strings.ReplaceAll(dbName, "`", "``"),
	)

	tx.ExecContext(ctx, query)

	query = fmt.Sprintf(
		"CREATE USER IF NOT EXISTS `%s`@'%%' IDENTIFIED BY 'password'",
		strings.ReplaceAll(dbUser, "`", "``"),
	)

	tx.ExecContext(ctx, query)

	query = fmt.Sprintf(
		"ALTER USER '%s'@'%%' IDENTIFIED BY '%s'",
		strings.ReplaceAll(dbUser, "`", "``"), strings.ReplaceAll(dbPass, "`", "``"),
	)

	tx.ExecContext(ctx, query)

	query = fmt.Sprintf(
		"GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, DROP ON `%s`.* TO '%s'@'%%'",
		strings.ReplaceAll(dbName, "`", "``"), strings.ReplaceAll(dbUser, "`", "``"),
	)

	tx.ExecContext(ctx, query)

	if err := tx.Commit(); err != nil {
		return err
	}

	run.db = tenantDB{
		name:     dbName,
		user:     dbUser,
		password: dbPass,
	}

	return nil
}

func (s *Service) addSecrets(ctx context.Context, run *provisionRun) error {
	return ErrUnimplemented
}

func (s *Service) createPods(ctx context.Context, run *provisionRun) error {
	return ErrUnimplemented
}

func (s *Service) createIngress(ctx context.Context, run *provisionRun) error {
	return ErrUnimplemented
}

func (s *Service) finishProvisioning(ctx context.Context, run *provisionRun) error {
	return ErrUnimplemented
}

func tenantToDbName(t tenant.Tenant) string {
	const prefix = "tenant_"

	return safeString(t.Slug, prefix)
}

func tenantToNamespace(t tenant.Tenant) string {
	const prefix = "erp-ns-"

	return safeString(t.Slug, prefix)
}

func safeString(str string, prefix string) string {
	var b strings.Builder

	b.Grow(len(prefix) + len(str))

	b.WriteString(prefix)

	for _, r := range strings.ToLower(str) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}

	return b.String()
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"

func randomString(minLen, maxLen int) (string, error) {
	lengthRange := maxLen - minLen + 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(lengthRange)))
	if err != nil {
		return "", err
	}
	length := minLen + int(nBig.Int64())

	result := make([]byte, length)

	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}

	return string(result), nil
}
