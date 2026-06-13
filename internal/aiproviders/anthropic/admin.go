package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// AdminProvisioner implementation: real org/workspace/role provisioning over the
// Anthropic Admin API (/v1/organizations/*), driven by the ADMIN key
// (admin_api_key) - the build phase of ADR-0022. Empirically established rules
// encoded here: invite is two-phase (no user_id until accepted); add-member is
// NOT idempotent upstream (re-add -> 400 "already a member" - we upsert to a role
// update instead); the org x workspace role matrix (RoleComboAllowed); org admin
// is Console-only; the members list is EXPLICIT-only, so effective access is
// computed as explicit UNION inherited.

var _ aiproviders.AdminProvisioner = Provider{}

// adminAPIError carries the upstream status + body for precise handling.
type adminAPIError struct {
	Status int
	Body   string
}

func (e *adminAPIError) Error() string {
	msg := e.Body
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return fmt.Sprintf("anthropic admin api returned %d: %s", e.Status, msg)
}

// adminDo performs one Admin-API call with the admin key. `out` may be nil.
func (p Provider) adminDo(ctx context.Context, ac aiproviders.AdminContext, method, path string, in, out any) error {
	key := adminKey(ac.Credentials)
	if key == "" {
		return fmt.Errorf("anthropic admin key missing: store it as admin_api_key (sk-ant-admin...) in the provider secret_ref")
	}
	base := strings.TrimRight(baseURL(ac.Config, "https://api.anthropic.com"), "/")
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion(ac.Config))
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.http().Do(req)
	if err != nil {
		return fmt.Errorf("anthropic admin api call failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &adminAPIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding anthropic admin response: %w", err)
		}
	}
	return nil
}

// adminList pages through a list endpoint (?limit=100&after_id=...) decoding
// each page's `data` into out via the supplied append function.
func adminList[T any](ctx context.Context, p Provider, ac aiproviders.AdminContext, path string) ([]T, error) {
	var all []T
	after := ""
	for page := 0; page < 50; page++ {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		u := path + sep + "limit=100"
		if after != "" {
			u += "&after_id=" + url.QueryEscape(after)
		}
		var payload struct {
			Data    []T    `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := p.adminDo(ctx, ac, http.MethodGet, u, nil, &payload); err != nil {
			return nil, err
		}
		all = append(all, payload.Data...)
		if !payload.HasMore || strings.TrimSpace(payload.LastID) == "" {
			break
		}
		after = payload.LastID
	}
	return all, nil
}

type wireUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	AddedAt string `json:"added_at"`
}

func (p Provider) ListOrgUsers(ctx context.Context, ac aiproviders.AdminContext) ([]aiproviders.OrgUser, error) {
	rows, err := adminList[wireUser](ctx, p, ac, "/v1/organizations/users")
	if err != nil {
		return nil, err
	}
	users := make([]aiproviders.OrgUser, 0, len(rows))
	for _, u := range rows {
		users = append(users, aiproviders.OrgUser{
			ID: u.ID, Email: u.Email, Name: u.Name,
			Role: aiproviders.OrgRole(u.Role), AddedAt: u.AddedAt,
		})
	}
	return users, nil
}

func (p Provider) ListWorkspaces(ctx context.Context, ac aiproviders.AdminContext) ([]aiproviders.OrgWorkspace, error) {
	type wireWS struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		CreatedAt  string `json:"created_at"`
		ArchivedAt string `json:"archived_at"`
	}
	rows, err := adminList[wireWS](ctx, p, ac, "/v1/organizations/workspaces")
	if err != nil {
		return nil, err
	}
	wss := make([]aiproviders.OrgWorkspace, 0, len(rows))
	for _, w := range rows {
		wss = append(wss, aiproviders.OrgWorkspace{
			ID: w.ID, Name: w.Name, CreatedAt: w.CreatedAt, ArchivedAt: w.ArchivedAt,
		})
	}
	return wss, nil
}

type wireInvite struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	InvitedAt string `json:"invited_at"`
	ExpiresAt string `json:"expires_at"`
}

func (i wireInvite) result() aiproviders.InviteResult {
	return aiproviders.InviteResult{
		InviteID: i.ID, Email: i.Email, Role: aiproviders.OrgRole(i.Role),
		Status: i.Status, InvitedAt: i.InvitedAt, ExpiresAt: i.ExpiresAt,
	}
}

func (p Provider) ListInvites(ctx context.Context, ac aiproviders.AdminContext) ([]aiproviders.InviteResult, error) {
	rows, err := adminList[wireInvite](ctx, p, ac, "/v1/organizations/invites")
	if err != nil {
		return nil, err
	}
	invites := make([]aiproviders.InviteResult, 0, len(rows))
	for _, i := range rows {
		invites = append(invites, i.result())
	}
	return invites, nil
}

func (p Provider) InviteUser(ctx context.Context, ac aiproviders.AdminContext, req aiproviders.InviteRequest) (*aiproviders.InviteResult, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if req.Role == aiproviders.OrgRoleAdmin {
		return nil, fmt.Errorf("the admin org role cannot be granted via the API - assign it in the provider Console")
	}
	var out wireInvite
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organizations/invites",
		map[string]string{"email": email, "role": string(req.Role)}, &out); err != nil {
		return nil, err
	}
	res := out.result()
	return &res, nil
}

func (p Provider) CreateWorkspace(ctx context.Context, ac aiproviders.AdminContext, name string) (*aiproviders.OrgWorkspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("workspace name is required")
	}
	var out struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organizations/workspaces",
		map[string]string{"name": name}, &out); err != nil {
		return nil, err
	}
	return &aiproviders.OrgWorkspace{ID: out.ID, Name: out.Name, CreatedAt: out.CreatedAt}, nil
}

func (p Provider) ArchiveWorkspace(ctx context.Context, ac aiproviders.AdminContext, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace id is required")
	}
	return p.adminDo(ctx, ac, http.MethodPost,
		"/v1/organizations/workspaces/"+url.PathEscape(workspaceID)+"/archive", nil, nil)
}

// GrantWorkspaceAccess validates the org x workspace role combination, then adds
// the member - upserting to a role update when the user is already a member
// (add-member is not idempotent upstream).
func (p Provider) GrantWorkspaceAccess(ctx context.Context, ac aiproviders.AdminContext, req aiproviders.WorkspaceGrantRequest) error {
	if req.WorkspaceID == "" || req.UserID == "" {
		return fmt.Errorf("workspace id and user id are required")
	}
	var u wireUser
	if err := p.adminDo(ctx, ac, http.MethodGet,
		"/v1/organizations/users/"+url.PathEscape(req.UserID), nil, &u); err != nil {
		return fmt.Errorf("looking up user: %w", err)
	}
	if !aiproviders.RoleComboAllowed(aiproviders.OrgRole(u.Role), req.WorkspaceRole) {
		return fmt.Errorf("workspace role %q cannot be assigned to a user with org role %q (a billing user can only be promoted to workspace_admin; workspace_billing is inherited-only)", req.WorkspaceRole, u.Role)
	}
	memberPath := "/v1/organizations/workspaces/" + url.PathEscape(req.WorkspaceID) + "/members"
	err := p.adminDo(ctx, ac, http.MethodPost, memberPath,
		map[string]string{"user_id": req.UserID, "workspace_role": string(req.WorkspaceRole)}, nil)
	var apiErr *adminAPIError
	if err != nil {
		if errors.As(err, &apiErr) && strings.Contains(apiErr.Body, "already a member") {
			// Upsert: switch to a role update on the existing membership.
			return p.adminDo(ctx, ac, http.MethodPost, memberPath+"/"+url.PathEscape(req.UserID),
				map[string]string{"workspace_role": string(req.WorkspaceRole)}, nil)
		}
		return err
	}
	return nil
}

func (p Provider) RemoveWorkspaceMember(ctx context.Context, ac aiproviders.AdminContext, workspaceID, userID string) error {
	if workspaceID == "" || userID == "" {
		return fmt.Errorf("workspace id and user id are required")
	}
	return p.adminDo(ctx, ac, http.MethodDelete,
		"/v1/organizations/workspaces/"+url.PathEscape(workspaceID)+"/members/"+url.PathEscape(userID), nil, nil)
}

func (p Provider) SetOrgRole(ctx context.Context, ac aiproviders.AdminContext, userID string, role aiproviders.OrgRole) (*aiproviders.OrgUser, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if role == aiproviders.OrgRoleAdmin {
		return nil, fmt.Errorf("the admin org role cannot be granted via the API - assign it in the provider Console")
	}
	var out wireUser
	if err := p.adminDo(ctx, ac, http.MethodPost,
		"/v1/organizations/users/"+url.PathEscape(userID),
		map[string]string{"role": string(role)}, &out); err != nil {
		return nil, err
	}
	return &aiproviders.OrgUser{
		ID: out.ID, Email: out.Email, Name: out.Name,
		Role: aiproviders.OrgRole(out.Role), AddedAt: out.AddedAt,
	}, nil
}

func (p Provider) RemoveOrgUser(ctx context.Context, ac aiproviders.AdminContext, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user id is required")
	}
	return p.adminDo(ctx, ac, http.MethodDelete,
		"/v1/organizations/users/"+url.PathEscape(userID), nil, nil)
}

// EffectiveWorkspaceAccess = explicit members UNION inherited org roles. The
// members API returns ONLY explicit assignments (proven live: org admins and
// billing users are invisible there yet hold workspace_admin / workspace_billing
// in the Console), so the real access set must be computed.
func (p Provider) EffectiveWorkspaceAccess(ctx context.Context, ac aiproviders.AdminContext, workspaceID string) ([]aiproviders.WorkspaceAccess, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace id is required")
	}
	type wireMember struct {
		UserID        string `json:"user_id"`
		WorkspaceRole string `json:"workspace_role"`
	}
	members, err := adminList[wireMember](ctx, p, ac,
		"/v1/organizations/workspaces/"+url.PathEscape(workspaceID)+"/members")
	if err != nil {
		return nil, err
	}
	users, err := p.ListOrgUsers(ctx, ac)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]aiproviders.OrgUser, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	var access []aiproviders.WorkspaceAccess
	explicit := make(map[string]bool, len(members))
	for _, m := range members {
		u := byID[m.UserID]
		explicit[m.UserID] = true
		access = append(access, aiproviders.WorkspaceAccess{
			UserID: m.UserID, Email: u.Email, OrgRole: u.Role,
			WorkspaceRole: aiproviders.WorkspaceRole(m.WorkspaceRole), Inherited: false,
		})
	}
	for _, u := range users {
		if explicit[u.ID] {
			continue
		}
		if inherited := aiproviders.InheritedWorkspaceRole(u.Role); inherited != "" {
			access = append(access, aiproviders.WorkspaceAccess{
				UserID: u.ID, Email: u.Email, OrgRole: u.Role,
				WorkspaceRole: inherited, Inherited: true,
			})
		}
	}
	return access, nil
}
