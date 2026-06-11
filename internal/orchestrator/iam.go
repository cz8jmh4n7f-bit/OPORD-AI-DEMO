package orchestrator

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/auth"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/google/uuid"
)

// CreateTenant adds an organization/team boundary.
func (s *Service) CreateTenant(ctx context.Context, name string) (db.Tenant, error) {
	if name == "" {
		return db.Tenant{}, fmt.Errorf("tenant name is required")
	}
	t, err := s.q.CreateTenant(ctx, name)
	if err != nil {
		return db.Tenant{}, fmt.Errorf("creating tenant: %w", err)
	}
	s.log.Info("tenant created", "name", t.Name)
	return t, nil
}

// ListTenants returns all tenants.
func (s *Service) ListTenants(ctx context.Context) ([]db.Tenant, error) {
	return s.q.ListTenants(ctx)
}

// CreateUser adds a user to a tenant with a role and a fresh API key. The
// plaintext key is returned once (only its hash is stored).
func (s *Service) CreateUser(ctx context.Context, email, tenantName, role string) (db.User, string, error) {
	if email == "" {
		return db.User{}, "", fmt.Errorf("email is required")
	}
	if !auth.ValidRole(role) {
		return db.User{}, "", fmt.Errorf("invalid role %q (want admin|operator|viewer)", role)
	}
	t, err := s.q.GetTenantByName(ctx, tenantName)
	if err != nil {
		return db.User{}, "", fmt.Errorf("tenant %q not found: %w", tenantName, err)
	}
	plain, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return db.User{}, "", err
	}
	u, err := s.q.CreateUser(ctx, db.CreateUserParams{
		Email:      email,
		TenantID:   t.ID,
		Role:       role,
		ApiKeyHash: hash,
	})
	if err != nil {
		return db.User{}, "", fmt.Errorf("creating user: %w", err)
	}
	s.log.Info("user created", "email", u.Email, "tenant", tenantName, "role", role)
	return u, plain, nil
}

// EnsureSeedUser idempotently provisions a tenant + a user with a FIXED API key.
// It powers the demo's RBAC-on-by-default mode: a no-op if the user already
// exists, so it is safe to run on every API start. The plaintext key is
// well-known and documented in the README - it is for LOCAL DEMO USE ONLY and
// must be rotated (or this path disabled via OPORD_SEED_DEMO_USERS=false) for any
// real deployment.
func (s *Service) EnsureSeedUser(ctx context.Context, email, tenantName, role, plainKey string) (created bool, err error) {
	if email == "" || plainKey == "" {
		return false, fmt.Errorf("seed user email and key are required")
	}
	if !auth.ValidRole(role) {
		return false, fmt.Errorf("invalid role %q (want admin|operator|viewer)", role)
	}
	// Tenant: reuse if present, else create.
	t, terr := s.q.GetTenantByName(ctx, tenantName)
	if terr != nil {
		t, terr = s.q.CreateTenant(ctx, tenantName)
		if terr != nil {
			return false, fmt.Errorf("ensuring tenant %q: %w", tenantName, terr)
		}
	}
	// User: skip if it already exists (idempotent across restarts).
	if _, gerr := s.q.GetUserByEmail(ctx, email); gerr == nil {
		return false, nil
	}
	if _, cerr := s.q.CreateUser(ctx, db.CreateUserParams{
		Email:      email,
		TenantID:   t.ID,
		Role:       role,
		ApiKeyHash: auth.HashKey(plainKey),
	}); cerr != nil {
		return false, fmt.Errorf("creating seed user %q: %w", email, cerr)
	}
	return true, nil
}

// ListUsers returns all users with their tenant name.
type UserSummary struct {
	User   db.User
	Tenant string
}

func (s *Service) ListUsers(ctx context.Context) ([]UserSummary, error) {
	users, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	tenants, err := s.q.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(tenants))
	for _, t := range tenants {
		names[t.ID] = t.Name
	}
	out := make([]UserSummary, 0, len(users))
	for _, u := range users {
		out = append(out, UserSummary{User: u, Tenant: names[u.TenantID]})
	}
	return out, nil
}

// ResolveAPIKey maps an API-key hash to an auth.Identity (the auth.Resolver).
func (s *Service) ResolveAPIKey(ctx context.Context, apiKeyHash string) (auth.Identity, bool) {
	if apiKeyHash == "" {
		return auth.Identity{}, false
	}
	u, err := s.q.GetUserByAPIKeyHash(ctx, apiKeyHash)
	if err != nil {
		return auth.Identity{}, false
	}
	id := auth.Identity{Email: u.Email, TenantID: u.TenantID, Role: auth.Role(u.Role)}
	if t, err := s.q.GetTenant(ctx, u.TenantID); err == nil {
		id.Tenant = t.Name
	}
	return id, true
}
