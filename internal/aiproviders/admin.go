package aiproviders

import "context"

// AdminProvisioner is an OPTIONAL capability (type-asserted, like the infra-side
// optional capabilities): a provider that can programmatically provision org /
// workspace ACCESS via its admin API, replacing the default governance-only
// ProvisionAccess "manual" stub. Anthropic implements this via its Admin API
// (/v1/organizations/*). The rules encoded below were established by live recon
// on 2026-06-12; see docs/adr/0022-anthropic-admin-provisioning.md.
//
// Four non-obvious constraints any implementation must honour:
//
//  1. TWO CREDS. The admin key (/v1/organizations/*) is SEPARATE from the
//     inference key (/v1/models, used by check/sync). A provider needs both.
//  2. INVITE IS TWO-PHASE + ASYNC. InviteUser returns a *pending* invite (no
//     user_id); the person must accept out-of-band before they get a user_id and
//     can be added to a workspace. So provisioning a NEW user is two-phase: invite,
//     then (after acceptance) GrantWorkspaceAccess. The OPORD instance stays
//     pending until accepted, then → active.
//  3. ROLE AXES ARE NOT INDEPENDENT. org role × workspace role has a compatibility
//     matrix (RoleComboAllowed) plus inheritance (org admin → workspace_admin;
//     org billing → workspace_billing, promotable only to workspace_admin). Org
//     admin is NOT settable via the API (Console only).
//  4. MEMBERS API = EXPLICIT ONLY. The workspace members list omits INHERITED
//     members (org admins, billing). Effective access must be COMPUTED, not read —
//     see EffectiveWorkspaceAccess.
//
// Also: GrantWorkspaceAccess is not idempotent upstream (re-add → 400 "already a
// member"); implementations should treat that as success.
type AdminProvisioner interface {
	// InviteUser creates a pending org invite with the given org role. Returns an
	// invite id; the user does NOT exist (no user_id) until they accept. The invite
	// has a TTL (~21d) and can expire unaccepted.
	InviteUser(ctx context.Context, req InviteRequest) (*InviteResult, error)

	// GrantWorkspaceAccess adds an EXISTING user (a real user_id, i.e. post-accept)
	// to a workspace with a workspace role. Must reject incompatible org/workspace
	// role combinations (RoleComboAllowed) and treat an "already a member" upstream
	// error as success.
	GrantWorkspaceAccess(ctx context.Context, req WorkspaceGrantRequest) error

	// EffectiveWorkspaceAccess computes the REAL access set for a workspace:
	// explicit members UNION inherited (org admins as workspace_admin, org billing
	// as workspace_billing). The raw members list is explicit-only and must not be
	// used as the source of truth.
	EffectiveWorkspaceAccess(ctx context.Context, workspaceID string) ([]WorkspaceAccess, error)
}

// OrgRole and WorkspaceRole mirror the Anthropic Admin API role enums.
type OrgRole string

type WorkspaceRole string

const (
	OrgRoleUser           OrgRole = "user"
	OrgRoleClaudeCodeUser OrgRole = "claude_code_user"
	OrgRoleDeveloper      OrgRole = "developer"
	OrgRoleBilling        OrgRole = "billing"
	OrgRoleAdmin          OrgRole = "admin" // NOT settable via API - Console only

	WSRoleUser                WorkspaceRole = "workspace_user"
	WSRoleDeveloper           WorkspaceRole = "workspace_developer"
	WSRoleRestrictedDeveloper WorkspaceRole = "workspace_restricted_developer"
	WSRoleAdmin               WorkspaceRole = "workspace_admin"
	WSRoleBilling             WorkspaceRole = "workspace_billing" // inherited-only; not assignable
)

// RoleComboAllowed reports whether a workspace role can be ASSIGNED (via the API)
// to a user holding the given org role. Empirically derived (live recon):
//   - org billing → only workspace_admin (its workspace_billing is inherited, not
//     assignable);
//   - org user / claude_code_user / developer → any assignable workspace role;
//   - workspace_billing is never directly assignable (inherited only).
//
// (org admin is not API-settable; admins inherit workspace_admin everywhere.)
func RoleComboAllowed(org OrgRole, ws WorkspaceRole) bool {
	if ws == WSRoleBilling {
		return false // inherited-only, never create/update-assignable
	}
	if org == OrgRoleBilling {
		return ws == WSRoleAdmin // a billing user is only promotable to workspace_admin
	}
	return true // user / claude_code_user / developer: any assignable workspace role
}

// InheritedWorkspaceRole returns the workspace role a user inherits from their org
// role on EVERY workspace (no explicit membership needed), or "" when none. Used to
// compute effective access alongside explicit members.
func InheritedWorkspaceRole(org OrgRole) WorkspaceRole {
	switch org {
	case OrgRoleAdmin:
		return WSRoleAdmin
	case OrgRoleBilling:
		return WSRoleBilling
	default:
		return "" // user / claude_code_user / developer inherit nothing
	}
}

// InviteRequest creates a pending org invite.
type InviteRequest struct {
	Email string
	Role  OrgRole
}

// InviteResult is the pending invite; Status is "pending" until accepted.
type InviteResult struct {
	InviteID  string
	Status    string // pending | accepted | expired | deleted
	ExpiresAt string // RFC3339; re-invite on expiry
}

// WorkspaceGrantRequest adds an existing user (post-accept) to a workspace.
type WorkspaceGrantRequest struct {
	WorkspaceID   string
	UserID        string // a real user_id, NOT an invite id
	WorkspaceRole WorkspaceRole
}

// WorkspaceAccess is one entry in a workspace's EFFECTIVE access set.
type WorkspaceAccess struct {
	UserID        string
	WorkspaceRole WorkspaceRole
	Inherited     bool // true = from org admin/billing; invisible to the members API
}
