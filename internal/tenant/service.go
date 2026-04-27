package tenant

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("tenant not found")

type Store interface {
	FindByAPIKey(ctx context.Context, key string) (Tenant, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	if err := ValidateCreateRequest(req); err != nil {
		return CreateResponse{}, err
	}

	// Provisioning work belongs behind this service boundary:
	// database, database user, namespace, secrets, workload, ingress, migrations.
	return CreateResponse{
		Status: "provisioning",
		Tenant: req.Slug,
	}, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (Tenant, error) {
	if !IsSafeSlug(slug) {
		return Tenant{}, ValidationError{Field: "slug", Message: "slug may only contain lowercase letters, numbers, and hyphens"}
	}

	return Tenant{
		Slug:   slug,
		Status: "active",
	}, nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, key string) (Tenant, error) {
	return s.store.FindByAPIKey(ctx, key)
}
