// Package auth provides API-key authentication and role-based authorization for
// the OPORD HTTP API. Identities carry a tenant + role; middleware authenticates
// requests and gates them by role. When auth is disabled (dev), a default admin
// identity is injected so existing flows keep working.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Role is an RBAC role, ordered viewer < operator < admin.
type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

func (r Role) rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleOperator:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether r is at least as privileged as min.
func (r Role) AtLeast(min Role) bool { return r.rank() >= min.rank() }

// ValidRole reports whether s is a known role.
func ValidRole(s string) bool {
	switch Role(s) {
	case RoleViewer, RoleOperator, RoleAdmin:
		return true
	}
	return false
}

// Identity is the authenticated caller.
type Identity struct {
	Email    string
	Tenant   string
	TenantID uuid.UUID
	Role     Role
}

// Resolver maps an API-key hash to an identity.
type Resolver func(ctx context.Context, apiKeyHash string) (Identity, bool)

// GenerateAPIKey returns a new plaintext key (shown once) and its sha256 hash.
func GenerateAPIKey() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plain = "opd_" + hex.EncodeToString(b)
	return plain, HashKey(plain), nil
}

// HashKey returns the sha256 hex of an API key (what we store/compare).
func HashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

type ctxKey int

const identityKey ctxKey = 0

func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFrom returns the caller's identity from the request context.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	for _, prefix := range []string{"Bearer ", "Token "} {
		if strings.HasPrefix(h, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(h, prefix))
		}
	}
	return strings.TrimSpace(h)
}

func unauthorized(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

// Middleware authenticates requests via a Bearer API key. When enabled is false
// it injects a dev admin identity (so the API works without auth configured).
func Middleware(resolve Resolver, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				dev := Identity{Email: "dev", Tenant: "default", Role: RoleAdmin}
				next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), dev)))
				return
			}
			token := bearer(r)
			if token == "" {
				unauthorized(w, http.StatusUnauthorized, "missing API key")
				return
			}
			id, ok := resolve(r.Context(), HashKey(token))
			if !ok {
				unauthorized(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
		})
	}
}

// RequireRole gates a route: the caller must be at least min.
func RequireRole(min Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFrom(r.Context())
			if !ok || !id.Role.AtLeast(min) {
				unauthorized(w, http.StatusForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
