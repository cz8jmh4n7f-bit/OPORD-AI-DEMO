package orchestrator

import (
	"context"
	"fmt"
)

// EntraUser is one user to grant SAML access to via the enterprise app.
type EntraUser struct {
	Email  string // UPN or mail address
	Role   string // role key; must match a key in GrantEntraAccessInput.Roles
	Invite bool   // invite as a B2B guest first (for external emails)
}

// GrantEntraAccessInput drives the Entra side of AWS SAML federation: ensure the
// enterprise app has an app role per AWS role (whose value IS the Role-claim
// "<role_arn>,<provider_arn>"), then assign each user to their role. This is the
// automation of the formerly-manual Azure Portal steps (see runbook 08).
type GrantEntraAccessInput struct {
	AppID string            // the enterprise app's client/app id
	Roles map[string]string // role key -> Role-claim value ("<role_arn>,<provider_arn>")
	Users []EntraUser
}

// EntraGrantResult summarises what changed.
type EntraGrantResult struct {
	AppRoleIDs map[string]string // role key -> app role id
	Assigned   []string          // emails assigned
	Invited    []string          // emails invited as guests
}

// GrantEntraAccess automates the Entra side of SAML federation: ensures app
// roles for the AWS roles, optionally invites guests, then assigns each user.
// Requires the Graph client (SetEntra) - returns a clear error otherwise, so the
// Entra side can still be done manually per the runbook.
func (s *Service) GrantEntraAccess(ctx context.Context, in GrantEntraAccessInput) (*EntraGrantResult, error) {
	if s.entra == nil || !s.entra.Configured() {
		return nil, fmt.Errorf("entra/graph not configured (set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET)")
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("app_id is required")
	}
	if len(in.Roles) == 0 {
		return nil, fmt.Errorf(`at least one role is required (role -> "<role_arn>,<provider_arn>")`)
	}

	sp, err := s.entra.ServicePrincipalByAppID(ctx, in.AppID)
	if err != nil {
		return nil, fmt.Errorf("locate enterprise app %s: %w", in.AppID, err)
	}

	// 1) Ensure an app role per AWS role. The gallery "AWS Single-Account Access"
	// app maps the SAML Role claim to user.assignedroles, so the app role's value
	// (the ARN pair) becomes the Role claim automatically - no claims policy.
	roleIDs := make(map[string]string, len(in.Roles))
	for name, claim := range in.Roles {
		id, err := s.entra.EnsureAppRole(ctx, in.AppID, "AWS "+name, claim)
		if err != nil {
			return nil, fmt.Errorf("ensure app role %q: %w", name, err)
		}
		roleIDs[name] = id
	}
	res := &EntraGrantResult{AppRoleIDs: roleIDs}

	// 2) Invite (optional) + assign each user.
	for _, u := range in.Users {
		roleID, ok := roleIDs[u.Role]
		if !ok {
			return res, fmt.Errorf("user %s: role %q is not in the roles map", u.Email, u.Role)
		}
		var userID string
		if u.Invite {
			invited, err := s.entra.InviteGuest(ctx, u.Email, "")
			if err != nil {
				return res, fmt.Errorf("invite guest %s: %w", u.Email, err)
			}
			userID = invited.ID
			res.Invited = append(res.Invited, u.Email)
		} else {
			usr, err := s.entra.UserByEmail(ctx, u.Email)
			if err != nil {
				return res, fmt.Errorf("resolve user %s: %w", u.Email, err)
			}
			userID = usr.ID
		}
		if err := s.entra.AssignAppRole(ctx, sp.ID, userID, roleID); err != nil {
			return res, fmt.Errorf("assign %s: %w", u.Email, err)
		}
		res.Assigned = append(res.Assigned, u.Email)
		s.log.Info("entra access granted", "user", u.Email, "role", u.Role, "app", in.AppID)
	}
	return res, nil
}

// GrantEntraGroupToApp assigns a Microsoft Entra group to the workforce/enterprise
// app so the group's members can authenticate through it - e.g. sign in to GCP via
// Workforce Identity Federation (ADR-0012), where the same group object id is then
// used in the project IAM principalSet. Reuses appRoleAssignedTo with the
// default-access app role; idempotent. Requires the Graph client (SetEntra).
func (s *Service) GrantEntraGroupToApp(ctx context.Context, appID, groupID string) error {
	if s.entra == nil || !s.entra.Configured() {
		return fmt.Errorf("entra/graph not configured (set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET)")
	}
	if appID == "" || groupID == "" {
		return fmt.Errorf("app_id and group_id are required")
	}
	sp, err := s.entra.ServicePrincipalByAppID(ctx, appID)
	if err != nil {
		return fmt.Errorf("locate enterprise app %s: %w", appID, err)
	}
	// The all-zero GUID is Entra's "default access" app role - assigning it adds the
	// principal (here a group) to the app without requiring a specific app role.
	const defaultAccessRole = "00000000-0000-0000-0000-000000000000"
	if err := s.entra.AssignAppRole(ctx, sp.ID, groupID, defaultAccessRole); err != nil {
		return fmt.Errorf("assign group %s to app %s: %w", groupID, appID, err)
	}
	s.log.Info("entra group assigned to app", "group", groupID, "app", appID)
	return nil
}
