package orchestrator

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/auth"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// tenantForCreate returns the tenant to stamp on a newly-created resource. An
// invalid (NULL) value means no tenant - CLI/dev/admin-without-tenant create
// global resources.
func tenantForCreate(ctx context.Context) pgtype.UUID {
	id, ok := auth.IdentityFrom(ctx)
	if !ok || id.TenantID == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id.TenantID, Valid: true}
}

// scopeTenant reports the tenant a list/get should be filtered to, and whether
// filtering applies at all. Admins and unauthenticated callers (CLI, worker,
// dev mode) are NOT scoped - they see everything.
func scopeTenant(ctx context.Context) (uuid.UUID, bool) {
	id, ok := auth.IdentityFrom(ctx)
	if !ok || id.Role == auth.RoleAdmin || id.TenantID == uuid.Nil {
		return uuid.Nil, false
	}
	return id.TenantID, true
}

// tenantVisible reports whether a nullable tenant column belongs to tid.
func tenantVisible(t pgtype.UUID, tid uuid.UUID) bool {
	return t.Valid && uuid.UUID(t.Bytes) == tid
}

// resourceVisible reports whether r belongs to tenant tid.
func resourceVisible(r db.Resource, tid uuid.UUID) bool {
	return tenantVisible(r.TenantID, tid)
}
