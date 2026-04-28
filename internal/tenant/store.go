package tenant

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) CreateTenant(ctx context.Context, insert TenantInsert) (Tenant, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tenant{}, err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tenants 
			(id, email, name, domain, slug, plan)
		VALUES 
			(?, ?, ?, ?, ?, ?)
	`,
		insert.ID,
		insert.Email,
		insert.Name,
		insert.Domain,
		insert.Slug,
		insert.Plan,
	)
	if err != nil {
		return Tenant{}, mapMySQLError(err)
	}

	_, err = tx.ExecContext(ctx, "\n"+
		"INSERT INTO tenant_keys\n"+
		"	(tenant_id, `key`)\n"+
		"VALUES\n"+
		"	(?, ?)\n",
		insert.ID,
		insert.SecretKey,
	)
	if err != nil {
		return Tenant{}, mapMySQLError(err)
	}

	if err := tx.Commit(); err != nil {
		return Tenant{}, mapMySQLError(err)
	}
	committed = true

	return Tenant{
		ID:     insert.ID,
		Email:  insert.Email,
		Name:   insert.Name,
		Domain: insert.Domain,
		Slug:   insert.Slug,
		Plan:   insert.Plan,
		Status: "active",
	}, nil
}

func mapMySQLError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrAlreadyExists
	}

	return err
}

func (s *MySQLStore) FindByAPIKey(ctx context.Context, key string) (Tenant, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT tenants.id, tenants.email, tenants.name, tenants.slug, tenants.domain, tenants.plan
		 FROM tenants
		 JOIN tenant_keys ON tenant_keys.tenant_id = tenants.id
		 WHERE tenant_keys.key = ?`,
		key,
	)

	var tenant Tenant
	if err := row.Scan(&tenant.ID, &tenant.Email, &tenant.Name, &tenant.Slug, &tenant.Domain, &tenant.Plan); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tenant{}, ErrNotFound
		}

		return Tenant{}, err
	}

	tenant.Status = "active"
	return tenant, nil
}

func (s *MySQLStore) FindBySlugAndAPIKey(ctx context.Context, slug string, key string) (Tenant, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT tenants.id, tenants.email, tenants.name, tenants.slug, tenants.domain, tenants.plan
		 FROM tenants
		 JOIN tenant_keys ON tenant_keys.tenant_id = tenants.id
		 WHERE tenants.slug = ? AND tenant_keys.key = ?`,
		slug,
		key,
	)

	var tenant Tenant
	if err := row.Scan(&tenant.ID, &tenant.Email, &tenant.Name, &tenant.Slug, &tenant.Domain, &tenant.Plan); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tenant{}, ErrNotFound
		}

		return Tenant{}, err
	}

	tenant.Status = "active"
	return tenant, nil
}
