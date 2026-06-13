package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// AI org administration: drive a provider's real org/workspace/role management
// (invite, create/archive workspace, grant/remove member, set/remove org role)
// through OPORD, with every write recorded in the AI audit trail. Backed by the
// AdminProvisioner capability (ADR-0022) - only providers that implement it (today
// Anthropic, via the admin_api_key) are governable; others return a clean error.

// adminFor resolves the provider row + its AdminProvisioner capability + an
// AdminContext carrying the resolved admin credentials and config.
func (s *Service) adminFor(ctx context.Context, name string) (db.AiProvider, aiproviders.AdminProvisioner, aiproviders.AdminContext, error) {
	p, err := s.q.GetAIProviderByName(ctx, name)
	if err != nil {
		return db.AiProvider{}, nil, aiproviders.AdminContext{}, fmt.Errorf("ai provider %q not found: %w", name, err)
	}
	prov, err := s.aiProvider(p.Type)
	if err != nil {
		return db.AiProvider{}, nil, aiproviders.AdminContext{}, err
	}
	admin, ok := prov.(aiproviders.AdminProvisioner)
	if !ok {
		return db.AiProvider{}, nil, aiproviders.AdminContext{}, fmt.Errorf("ai provider %q (%s) does not support org administration", p.Name, p.Type)
	}
	ac := aiproviders.AdminContext{Credentials: s.aiCredentials(ctx, p), Config: aiProviderConfig(p)}
	return p, admin, ac, nil
}

// resolveUserRef accepts EITHER a provider user id (e.g. user_…) or an email, and
// returns the canonical user id. Convenience so callers (and older UIs) can pass a
// human-readable email; an empty/already-id value is returned unchanged.
func resolveUserRef(ctx context.Context, admin aiproviders.AdminProvisioner, ac aiproviders.AdminContext, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !strings.Contains(ref, "@") {
		return ref, nil // already an id (or empty - let the provider validate)
	}
	users, err := admin.ListOrgUsers(ctx, ac)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", ref, err)
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, ref) {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("no organization user with email %q (invite them first, or use their user id)", ref)
}

// --- Reads (no audit) ---

func (s *Service) ListAIOrgUsers(ctx context.Context, provider string) ([]aiproviders.OrgUser, error) {
	_, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return admin.ListOrgUsers(ctx, ac)
}

func (s *Service) ListAIWorkspaces(ctx context.Context, provider string) ([]aiproviders.OrgWorkspace, error) {
	_, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return admin.ListWorkspaces(ctx, ac)
}

func (s *Service) ListAIInvites(ctx context.Context, provider string) ([]aiproviders.InviteResult, error) {
	_, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return admin.ListInvites(ctx, ac)
}

func (s *Service) AIWorkspaceAccess(ctx context.Context, provider, workspaceID string) ([]aiproviders.WorkspaceAccess, error) {
	_, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return admin.EffectiveWorkspaceAccess(ctx, ac, workspaceID)
}

// --- Writes (each audited) ---

func (s *Service) InviteAIOrgUser(ctx context.Context, provider, email, role string) (*aiproviders.InviteResult, error) {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := admin.InviteUser(ctx, ac, aiproviders.InviteRequest{Email: email, Role: aiproviders.OrgRole(role)})
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "org_invite_failed", err.Error(), map[string]any{"provider": p.Name, "email": email, "role": role}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "org_user_invited", "AI org user invited", map[string]any{"provider": p.Name, "email": email, "role": role, "invite_id": res.InviteID}, "")
	return res, nil
}

func (s *Service) SetAIOrgRole(ctx context.Context, provider, userID, role string) (*aiproviders.OrgUser, error) {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	u, err := admin.SetOrgRole(ctx, ac, userID, aiproviders.OrgRole(role))
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "org_role_change_failed", err.Error(), map[string]any{"provider": p.Name, "user_id": userID, "role": role}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "org_role_changed", "AI org role changed", map[string]any{"provider": p.Name, "user_id": userID, "role": role}, "")
	return u, nil
}

func (s *Service) RemoveAIOrgUser(ctx context.Context, provider, userID string) error {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return err
	}
	if err := admin.RemoveOrgUser(ctx, ac, userID); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "org_user_remove_failed", err.Error(), map[string]any{"provider": p.Name, "user_id": userID}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "org_user_removed", "AI org user removed", map[string]any{"provider": p.Name, "user_id": userID}, "")
	return nil
}

func (s *Service) CreateAIWorkspace(ctx context.Context, provider, name string) (*aiproviders.OrgWorkspace, error) {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	ws, err := admin.CreateWorkspace(ctx, ac, name)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_create_failed", err.Error(), map[string]any{"provider": p.Name, "name": name}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_created", "AI workspace created", map[string]any{"provider": p.Name, "name": name, "workspace_id": ws.ID}, "")
	return ws, nil
}

func (s *Service) ArchiveAIWorkspace(ctx context.Context, provider, workspaceID string) error {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return err
	}
	if err := admin.ArchiveWorkspace(ctx, ac, workspaceID); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_archive_failed", err.Error(), map[string]any{"provider": p.Name, "workspace_id": workspaceID}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_archived", "AI workspace archived", map[string]any{"provider": p.Name, "workspace_id": workspaceID}, "")
	return nil
}

func (s *Service) GrantAIWorkspaceAccess(ctx context.Context, provider, workspaceID, userID, role string) error {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return err
	}
	userID, err = resolveUserRef(ctx, admin, ac, userID)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_grant_failed", err.Error(), map[string]any{"provider": p.Name, "workspace_id": workspaceID, "user_ref": userID, "role": role}, "")
		return err
	}
	req := aiproviders.WorkspaceGrantRequest{WorkspaceID: workspaceID, UserID: userID, WorkspaceRole: aiproviders.WorkspaceRole(role)}
	if err := admin.GrantWorkspaceAccess(ctx, ac, req); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_grant_failed", err.Error(), map[string]any{"provider": p.Name, "workspace_id": workspaceID, "user_id": userID, "role": role}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_access_granted", "AI workspace access granted", map[string]any{"provider": p.Name, "workspace_id": workspaceID, "user_id": userID, "role": role}, "")
	return nil
}

func (s *Service) RemoveAIWorkspaceMember(ctx context.Context, provider, workspaceID, userID string) error {
	p, admin, ac, err := s.adminFor(ctx, provider)
	if err != nil {
		return err
	}
	if userID, err = resolveUserRef(ctx, admin, ac, userID); err != nil {
		return err
	}
	if err := admin.RemoveWorkspaceMember(ctx, ac, workspaceID, userID); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_member_remove_failed", err.Error(), map[string]any{"provider": p.Name, "workspace_id": workspaceID, "user_id": userID}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "workspace_member_removed", "AI workspace member removed", map[string]any{"provider": p.Name, "workspace_id": workspaceID, "user_id": userID}, "")
	return nil
}
