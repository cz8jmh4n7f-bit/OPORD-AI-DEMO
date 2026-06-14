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

func (s *Service) projectControlsFor(ctx context.Context, name string) (db.AiProvider, aiproviders.ProjectControlsProvisioner, aiproviders.AdminContext, error) {
	p, err := s.q.GetAIProviderByName(ctx, name)
	if err != nil {
		return db.AiProvider{}, nil, aiproviders.AdminContext{}, fmt.Errorf("ai provider %q not found: %w", name, err)
	}
	prov, err := s.aiProvider(p.Type)
	if err != nil {
		return db.AiProvider{}, nil, aiproviders.AdminContext{}, err
	}
	controls, ok := prov.(aiproviders.ProjectControlsProvisioner)
	if !ok {
		return db.AiProvider{}, nil, aiproviders.AdminContext{}, fmt.Errorf("ai provider %q (%s) does not support project controls", p.Name, p.Type)
	}
	ac := aiproviders.AdminContext{Credentials: s.aiCredentials(ctx, p), Config: aiProviderConfig(p)}
	return p, controls, ac, nil
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

func (s *Service) ListAIProjectAPIKeys(ctx context.Context, provider, projectID string) ([]aiproviders.ProjectAPIKey, error) {
	_, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return controls.ListProjectAPIKeys(ctx, ac, projectID)
}

func (s *Service) ListAIProjectRateLimits(ctx context.Context, provider, projectID string) ([]aiproviders.ProjectRateLimit, error) {
	_, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return controls.ListProjectRateLimits(ctx, ac, projectID)
}

func (s *Service) GetAIProjectModelPermissions(ctx context.Context, provider, projectID string) (*aiproviders.ProjectModelPermissions, error) {
	_, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return controls.GetProjectModelPermissions(ctx, ac, projectID)
}

func (s *Service) GetAIProjectHostedToolPermissions(ctx context.Context, provider, projectID string) (*aiproviders.ProjectHostedToolPermissions, error) {
	_, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return controls.GetProjectHostedToolPermissions(ctx, ac, projectID)
}

func (s *Service) GetAIProjectDataRetention(ctx context.Context, provider, projectID string) (*aiproviders.ProjectDataRetention, error) {
	_, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return controls.GetProjectDataRetention(ctx, ac, projectID)
}

func (s *Service) ListAIProjectSpendAlerts(ctx context.Context, provider, projectID string) ([]aiproviders.ProjectSpendAlert, error) {
	_, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	return controls.ListProjectSpendAlerts(ctx, ac, projectID)
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

func (s *Service) DeleteAIProjectAPIKey(ctx context.Context, provider, projectID, keyID string) error {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return err
	}
	if err := controls.DeleteProjectAPIKey(ctx, ac, projectID, keyID); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_api_key_delete_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID, "api_key_id": keyID}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_api_key_deleted", "AI project API key deleted", map[string]any{"provider": p.Name, "project_id": projectID, "api_key_id": keyID}, "")
	return nil
}

func (s *Service) UpdateAIProjectRateLimit(ctx context.Context, provider, projectID, rateLimitID string, req aiproviders.ProjectRateLimitUpdate) (*aiproviders.ProjectRateLimit, error) {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := controls.UpdateProjectRateLimit(ctx, ac, projectID, rateLimitID, req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_rate_limit_update_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID, "rate_limit_id": rateLimitID}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_rate_limit_updated", "AI project rate limit updated", map[string]any{"provider": p.Name, "project_id": projectID, "rate_limit_id": rateLimitID}, "")
	return res, nil
}

func (s *Service) SetAIProjectModelPermissions(ctx context.Context, provider, projectID string, req aiproviders.ProjectModelPermissions) (*aiproviders.ProjectModelPermissions, error) {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := controls.SetProjectModelPermissions(ctx, ac, projectID, req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_model_permissions_update_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID, "mode": req.Mode, "model_ids": req.ModelIDs}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_model_permissions_updated", "AI project model permissions updated", map[string]any{"provider": p.Name, "project_id": projectID, "mode": req.Mode, "model_ids": req.ModelIDs}, "")
	return res, nil
}

func (s *Service) DeleteAIProjectModelPermissions(ctx context.Context, provider, projectID string) error {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return err
	}
	if err := controls.DeleteProjectModelPermissions(ctx, ac, projectID); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_model_permissions_delete_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_model_permissions_deleted", "AI project model permissions deleted", map[string]any{"provider": p.Name, "project_id": projectID}, "")
	return nil
}

func (s *Service) SetAIProjectHostedToolPermissions(ctx context.Context, provider, projectID string, req aiproviders.ProjectHostedToolPermissions) (*aiproviders.ProjectHostedToolPermissions, error) {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := controls.SetProjectHostedToolPermissions(ctx, ac, projectID, req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_tool_permissions_update_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_tool_permissions_updated", "AI project hosted tool permissions updated", map[string]any{"provider": p.Name, "project_id": projectID}, "")
	return res, nil
}

func (s *Service) SetAIProjectDataRetention(ctx context.Context, provider, projectID string, req aiproviders.ProjectDataRetention) (*aiproviders.ProjectDataRetention, error) {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := controls.SetProjectDataRetention(ctx, ac, projectID, req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_data_retention_update_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID, "type": req.Type}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_data_retention_updated", "AI project data retention updated", map[string]any{"provider": p.Name, "project_id": projectID, "type": req.Type}, "")
	return res, nil
}

func (s *Service) CreateAIProjectSpendAlert(ctx context.Context, provider, projectID string, req aiproviders.ProjectSpendAlertInput) (*aiproviders.ProjectSpendAlert, error) {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := controls.CreateProjectSpendAlert(ctx, ac, projectID, req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_spend_alert_create_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_spend_alert_created", "AI project spend alert created", map[string]any{"provider": p.Name, "project_id": projectID, "alert_id": res.ID, "threshold_cents": req.ThresholdCents}, "")
	return res, nil
}

func (s *Service) UpdateAIProjectSpendAlert(ctx context.Context, provider, projectID, alertID string, req aiproviders.ProjectSpendAlertInput) (*aiproviders.ProjectSpendAlert, error) {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	res, err := controls.UpdateProjectSpendAlert(ctx, ac, projectID, alertID, req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_spend_alert_update_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID, "alert_id": alertID}, "")
		return nil, err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_spend_alert_updated", "AI project spend alert updated", map[string]any{"provider": p.Name, "project_id": projectID, "alert_id": res.ID, "threshold_cents": req.ThresholdCents}, "")
	return res, nil
}

func (s *Service) DeleteAIProjectSpendAlert(ctx context.Context, provider, projectID, alertID string) error {
	p, controls, ac, err := s.projectControlsFor(ctx, provider)
	if err != nil {
		return err
	}
	if err := controls.DeleteProjectSpendAlert(ctx, ac, projectID, alertID); err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "project_spend_alert_delete_failed", err.Error(), map[string]any{"provider": p.Name, "project_id": projectID, "alert_id": alertID}, "")
		return err
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "project_spend_alert_deleted", "AI project spend alert deleted", map[string]any{"provider": p.Name, "project_id": projectID, "alert_id": alertID}, "")
	return nil
}
