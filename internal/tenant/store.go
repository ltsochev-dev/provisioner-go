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

	defer tx.Rollback()

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

	return Tenant{
		ID:     insert.ID,
		Email:  insert.Email,
		Name:   insert.Name,
		Domain: insert.Domain,
		Slug:   insert.Slug,
		Plan:   insert.Plan,
		Status: "pending",
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
		`SELECT tenants.id, tenants.email, tenants.name, tenants.slug, tenants.domain, tenants.plan, tenants.status
		 FROM tenants
		 JOIN tenant_keys ON tenant_keys.tenant_id = tenants.id
		 WHERE tenant_keys.key = ?`,
		key,
	)

	var tenant Tenant
	if err := row.Scan(&tenant.ID, &tenant.Email, &tenant.Name, &tenant.Slug, &tenant.Domain, &tenant.Plan, &tenant.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tenant{}, ErrNotFound
		}

		return Tenant{}, err
	}

	return tenant, nil
}

func (s *MySQLStore) FindBySlugAndAPIKey(ctx context.Context, slug string, key string) (Tenant, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT tenants.id, tenants.email, tenants.name, tenants.slug, tenants.domain, tenants.plan, tenants.status
		 FROM tenants
		 JOIN tenant_keys ON tenant_keys.tenant_id = tenants.id
		 WHERE tenants.slug = ? AND tenant_keys.key = ?`,
		slug,
		key,
	)

	var tenant Tenant
	if err := row.Scan(&tenant.ID, &tenant.Email, &tenant.Name, &tenant.Slug, &tenant.Domain, &tenant.Plan, &tenant.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tenant{}, ErrNotFound
		}

		return Tenant{}, err
	}

	return tenant, nil
}

func (s *MySQLStore) All(ctx context.Context) ([]Tenant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, name, slug, domain, plan FROM tenants ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var tenants []Tenant

	for rows.Next() {
		var t Tenant

		err := rows.Scan(
			&t.ID,
			&t.Email,
			&t.Name,
			&t.Slug,
			&t.Domain,
			&t.Plan,
		)
		if err != nil {
			return nil, err
		}

		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tenants, nil
}

func (s *MySQLStore) BeginProvision(ctx context.Context, tenantId string) (Tenant, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tenant{}, err
	}

	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, name, email, slug, domain, plan, status FROM tenants
		WHERE id = ? AND status = ?
		FOR UPDATE SKIP LOCKED
	`,
		tenantId, "pending",
	)

	var tenant Tenant
	if err := row.Scan(&tenant.ID, &tenant.Name, &tenant.Email, &tenant.Slug, &tenant.Domain, &tenant.Plan, &tenant.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tenant{}, ErrNotFound
		}

		return Tenant{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE tenants
		SET status = ?, locked_at = NOW()
		WHERE id = ?
	`, "provisioning", tenantId); err != nil {
		return Tenant{}, err
	}

	if err := tx.Commit(); err != nil {
		return Tenant{}, err
	}

	tenant.Status = "provisioning"

	return tenant, nil
}
