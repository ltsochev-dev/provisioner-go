package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrAlreadyExists = errors.New("tenant already exists")
	ErrNotFound      = errors.New("tenant not found")
)

type Store interface {
	All(ctx context.Context) ([]Tenant, error)
	CreateTenant(ctx context.Context, insert TenantInsert) (Tenant, error)
	FindByAPIKey(ctx context.Context, key string) (Tenant, error)
	FindBySlugAndAPIKey(ctx context.Context, slug string, key string) (Tenant, error)
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

	id, err := uuid.NewV7()
	if err != nil {
		return CreateResponse{}, fmt.Errorf("generate tenant id: %w", err)
	}

	secretKey, err := GenerateSecretKey("prod")
	if err != nil {
		return CreateResponse{}, fmt.Errorf("generate tenant api key: %w", err)
	}

	created, err := s.store.CreateTenant(ctx, TenantInsert{
		ID:        id.String(),
		Email:     req.Email,
		Name:      req.Name,
		Slug:      req.Slug,
		Domain:    req.Domain,
		Plan:      req.Plan,
		SecretKey: secretKey,
	})
	if err != nil {
		return CreateResponse{}, fmt.Errorf("create tenant: %w", err)
	}

	// @todo
	// Queue provisioning for this tenant once provisioning state is persisted.

	return CreateResponse{
		Status: "provisioning",
		Tenant: created.Slug,
		APIKey: secretKey,
	}, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string, key string) (Tenant, error) {
	if !IsSafeSlug(slug) {
		return Tenant{}, ValidationError{Field: "slug", Message: "slug may only contain lowercase letters, numbers, and hyphens"}
	}

	return s.store.FindBySlugAndAPIKey(ctx, slug, key)
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, key string) (Tenant, error) {
	return s.store.FindByAPIKey(ctx, key)
}

func (s *Service) All(ctx context.Context) ([]Tenant, error) {
	return s.store.All(ctx)
}

func GenerateSecretKey(env string) (string, error) {
	prefix := "dev"
	if env == "prod" {
		prefix = "lv"
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return fmt.Sprintf("erp_%s_%s", prefix, hex.EncodeToString(b)), nil
}
