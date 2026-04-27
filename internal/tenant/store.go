package tenant

import (
	"context"
	"database/sql"
	"errors"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
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
