package provisioning

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
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

type TenantStore interface {
	Pending(ctx context.Context, limit int) ([]tenant.Tenant, error)
	BeginProvision(ctx context.Context, tenantID string) (tenant.Tenant, error)
	ReleaseStaleProvisioning(ctx context.Context, olderThan time.Duration) ([]tenant.Tenant, error)
}

type KubernetesService interface {
	NamespaceExists(ctx context.Context, name string) (bool, error)
	CreateNamespace(ctx context.Context, name string) error
	CreateOrUpdateSecret(ctx context.Context, ns string, name string, values map[string]string) error
	CreateOrUpdateLaravelWorkload(ctx context.Context, ns string, name string, image string, secretName string) error
	ScaleDeployment(ctx context.Context, ns string, name string, replicas int32) error
	CreateOrUpdateIngress(ctx context.Context, ns string, name string, host string, serviceName string) error
}

type Config struct {
	ScanInterval     time.Duration
	StaleLockTimeout time.Duration
	BatchSize        int
	Logger           *slog.Logger
	Store            TenantStore
	DB               *sql.DB
	Kubernetes       KubernetesService
	TenantAppImage   string
}

type Service struct {
	scanInterval     time.Duration
	staleLockTimeout time.Duration
	batchSize        int
	logger           *slog.Logger
	store            TenantStore
	db               *sql.DB
	kubernetes       KubernetesService
	tenantAppImage   string
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
		tenantAppImage:   cfg.TenantAppImage,
		triggerCh:        make(chan struct{}, 1),
	}
}

func (s *Service) Trigger() {
	select {
	case s.triggerCh <- struct{}{}:
	default:
	}
}

func (s *Service) SuspendTenant(ctx context.Context, slug string) error {
	return s.setTenantRunningState(ctx, slug, "suspended", 0)
}

func (s *Service) ResumeTenant(ctx context.Context, slug string) error {
	return s.setTenantRunningState(ctx, slug, "active", 1)
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
		{name: "create database", run: s.setDb},
		{name: "create k8s namespace", run: s.createK8sNamespace},
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

	s.logger.Info("Provisioning db for tenant", "tenant_id", run.tenant.ID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	query := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS %s",
		mysqlIdentifier(dbName),
	)

	_, err = tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("create database %q: %w", dbName, err)
	}

	query = fmt.Sprintf(
		"CREATE USER IF NOT EXISTS %s@'%%' IDENTIFIED BY %s",
		mysqlString(dbUser),
		mysqlString(dbPass),
	)

	_, err = tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("create database user %q: %w", dbUser, err)
	}

	query = fmt.Sprintf(
		"ALTER USER %s@'%%' IDENTIFIED BY %s",
		mysqlString(dbUser),
		mysqlString(dbPass),
	)

	_, err = tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("set database user password for %q: %w", dbUser, err)
	}

	query = fmt.Sprintf(
		"GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, DROP ON %s.* TO %s@'%%'",
		mysqlIdentifier(dbName),
		mysqlString(dbUser),
	)

	_, err = tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("grant database privileges on %q to %q: %w", dbName, dbUser, err)
	}

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
	ns := tenantToNamespace(run.tenant)

	s.logger.Info("Provisioning secrets for", "tenant_id", run.tenant.ID, "ns", ns)

	appKey, err := randomLaravelAppKey()
	if err != nil {
		return err
	}

	values := map[string]string{
		"APP_NAME":         run.tenant.Name,
		"APP_KEY":          appKey,
		"APP_ENV":          "production",
		"APP_DEBUG":        "false",
		"APP_URL":          "https://" + run.tenant.Domain,
		"TENANT_API_KEY":   run.tenant.SecretKey,
		"DB_CONNECTION":    "mysql",
		"DB_HOST":          "localhost",
		"DB_PORT":          "3306",
		"DB_DATABASE":      run.db.name,
		"DB_USERNAME":      run.db.user,
		"DB_PASSWORD":      run.db.password,
		"SESSION_DRIVER":   "database",
		"QUEUE_CONNECTION": "database",
		"CACHE_STORE":      "database",
	}

	err = s.kubernetes.CreateOrUpdateSecret(ctx, ns, "laravel-env", values)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) createPods(ctx context.Context, run *provisionRun) error {
	ns := tenantToNamespace(run.tenant)
	name := tenantToWorkloadName(run.tenant)

	return s.kubernetes.CreateOrUpdateLaravelWorkload(ctx, ns, name, s.tenantAppImage, "laravel-env")
}

func (s *Service) createIngress(ctx context.Context, run *provisionRun) error {
	ns := tenantToNamespace(run.tenant)
	name := tenantToWorkloadName(run.tenant)

	return s.kubernetes.CreateOrUpdateIngress(ctx, ns, name, run.tenant.Domain, name)
}

func (s *Service) finishProvisioning(ctx context.Context, run *provisionRun) error {
	result, err := s.db.ExecContext(
		ctx,
		"UPDATE tenants SET status = ?, locked_at = NULL WHERE id = ?",
		"active", run.tenant.ID,
	)
	if err != nil {
		return fmt.Errorf("activate tenant %q: %w", run.tenant.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check activated tenant %q: %w", run.tenant.ID, err)
	}

	if rows == 0 {
		return tenant.ErrNotFound
	}

	return nil
}

func (s *Service) setTenantRunningState(ctx context.Context, slug string, status string, replicas int32) error {
	if !tenant.IsSafeSlug(slug) {
		return tenant.ValidationError{Field: "slug", Message: "slug may only contain lowercase letters, numbers, and hyphens"}
	}
	if s.kubernetes == nil {
		return errors.New("kubernetes services is required")
	}

	t := tenant.Tenant{Slug: slug}
	if err := s.kubernetes.ScaleDeployment(ctx, tenantToNamespace(t), tenantToWorkloadName(t), replicas); err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx, "UPDATE tenants SET status = ?, locked_at = NULL WHERE slug = ?", status, slug)
	if err != nil {
		return fmt.Errorf("set tenant %q status to %q: %w", slug, status, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check tenant %q status update to %q: %w", slug, status, err)
	}
	if rows == 0 {
		return tenant.ErrNotFound
	}

	return nil
}

func tenantToDbName(t tenant.Tenant) string {
	const prefix = "tenant_"

	return safeString(t.Slug, prefix)
}

func tenantToNamespace(t tenant.Tenant) string {
	const prefix = "erp-ns-"

	return safeString(t.Slug, prefix)
}

func tenantToWorkloadName(t tenant.Tenant) string {
	const prefix = "erp-app-"

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

func mysqlIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func mysqlString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
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

func randomLaravelAppKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}

	return "base64:" + base64.StdEncoding.EncodeToString(key), nil
}
