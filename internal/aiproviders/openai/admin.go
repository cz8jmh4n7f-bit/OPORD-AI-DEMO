package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// adminAPIError carries the upstream HTTP status so callers can branch on it
// (e.g. 409 Conflict = the user is already a member) instead of matching the
// error text, which can change between API versions.
type adminAPIError struct {
	StatusCode int
	Body       string
}

// Error returns a STATUS-ONLY message - it deliberately omits the raw upstream
// body so a verbatim OpenAI error blob can't leak into HTTP responses or audit
// logs (callers propagate err.Error()). The raw body stays in .Body for
// server-side branching (e.g. isAlreadyExists), not for surfacing to clients.
func (e *adminAPIError) Error() string {
	if txt := http.StatusText(e.StatusCode); txt != "" {
		return fmt.Sprintf("openai admin api error: HTTP %d (%s)", e.StatusCode, txt)
	}
	return fmt.Sprintf("openai admin api error: HTTP %d", e.StatusCode)
}

// isAlreadyExists reports whether an admin call failed because the target already
// exists / the user is already a member: a 409 Conflict (preferred signal) or,
// as a fallback for backends that return 400 with an explanatory body, an
// "already" substring.
func isAlreadyExists(err error) bool {
	var apiErr *adminAPIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusConflict {
			return true
		}
		return strings.Contains(strings.ToLower(apiErr.Body), "already")
	}
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already")
}

// AdminProvisioner over the OpenAI organization API (/v1/organization/*), the
// OpenAI twin of the Anthropic admin client. OpenAI "projects" are the workspace
// concept; org roles are owner/reader, project roles owner/member (no
// inherited-billing quirk - effective access is just the explicit members).
// Driven by an OpenAI ADMIN key (sk-admin-..., stored as admin_api_key) with
// Bearer auth.

var _ aiproviders.AdminProvisioner = Provider{}
var _ aiproviders.ProjectControlsProvisioner = Provider{}

// adminKey reads the OpenAI org admin key (sk-admin-...).
func adminKey(creds map[string]string) string {
	if v := strings.TrimSpace(creds["admin_api_key"]); v != "" {
		return v
	}
	return strings.TrimSpace(creds["api_key"])
}

func (p Provider) adminDo(ctx context.Context, ac aiproviders.AdminContext, method, path string, in, out any) error {
	key := adminKey(ac.Credentials)
	if key == "" {
		return fmt.Errorf("openai admin key missing (store it as admin_api_key, sk-admin-...)")
	}
	base := strings.TrimRight(baseURL(ac.Config, "https://api.openai.com"), "/")
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
	req.Header.Set("Authorization", "Bearer "+key)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.http().Do(req)
	if err != nil {
		return fmt.Errorf("openai admin api call failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return &adminAPIError{StatusCode: resp.StatusCode, Body: msg}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding openai admin response: %w", err)
		}
	}
	return nil
}

// adminList pages an org list endpoint (?limit=100&after=...).
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
			u += "&after=" + url.QueryEscape(after)
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

func tsToString(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

type oaiUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	AddedAt int64  `json:"added_at"`
}

func (p Provider) ListOrgUsers(ctx context.Context, ac aiproviders.AdminContext) ([]aiproviders.OrgUser, error) {
	rows, err := adminList[oaiUser](ctx, p, ac, "/v1/organization/users")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.OrgUser, 0, len(rows))
	for _, u := range rows {
		out = append(out, aiproviders.OrgUser{ID: u.ID, Email: u.Email, Name: u.Name, Role: aiproviders.OrgRole(u.Role), AddedAt: tsToString(u.AddedAt)})
	}
	return out, nil
}

func (p Provider) ListWorkspaces(ctx context.Context, ac aiproviders.AdminContext) ([]aiproviders.OrgWorkspace, error) {
	type oaiProject struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		CreatedAt  int64  `json:"created_at"`
		ArchivedAt int64  `json:"archived_at"`
	}
	rows, err := adminList[oaiProject](ctx, p, ac, "/v1/organization/projects?include_archived=true")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.OrgWorkspace, 0, len(rows))
	for _, w := range rows {
		out = append(out, aiproviders.OrgWorkspace{ID: w.ID, Name: w.Name, CreatedAt: tsToString(w.CreatedAt), ArchivedAt: tsToString(w.ArchivedAt)})
	}
	return out, nil
}

type oaiInvite struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	InvitedAt int64  `json:"invited_at"`
	ExpiresAt int64  `json:"expires_at"`
}

func (i oaiInvite) result() aiproviders.InviteResult {
	return aiproviders.InviteResult{InviteID: i.ID, Email: i.Email, Role: aiproviders.OrgRole(i.Role), Status: i.Status, InvitedAt: tsToString(i.InvitedAt), ExpiresAt: tsToString(i.ExpiresAt)}
}

func (p Provider) ListInvites(ctx context.Context, ac aiproviders.AdminContext) ([]aiproviders.InviteResult, error) {
	rows, err := adminList[oaiInvite](ctx, p, ac, "/v1/organization/invites")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.InviteResult, 0, len(rows))
	for _, i := range rows {
		out = append(out, i.result())
	}
	return out, nil
}

// oaiOrgRole normalizes an abstract org role to OpenAI's allowed set (owner|reader).
// OpenAI org roles are only owner|reader, so an elevated role (admin/owner) maps to
// owner and every other role floors to reader (least privilege). The abstract enum
// is Anthropic-shaped, so "admin" - not a literal "owner" - is the elevated value.
func oaiOrgRole(r aiproviders.OrgRole) string {
	if strings.EqualFold(string(r), string(aiproviders.OrgRoleAdmin)) || strings.EqualFold(string(r), "owner") {
		return "owner"
	}
	return "reader"
}

// oaiProjectRole normalizes a workspace role to OpenAI's project set (owner|member).
func oaiProjectRole(r aiproviders.WorkspaceRole) string {
	if strings.Contains(strings.ToLower(string(r)), "admin") || strings.EqualFold(string(r), "owner") {
		return "owner"
	}
	return "member"
}

func (p Provider) InviteUser(ctx context.Context, ac aiproviders.AdminContext, req aiproviders.InviteRequest) (*aiproviders.InviteResult, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	var out oaiInvite
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/invites",
		map[string]string{"email": email, "role": oaiOrgRole(req.Role)}, &out); err != nil {
		return nil, err
	}
	res := out.result()
	return &res, nil
}

func (p Provider) CreateWorkspace(ctx context.Context, ac aiproviders.AdminContext, name string) (*aiproviders.OrgWorkspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	var out struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt int64  `json:"created_at"`
	}
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/projects", map[string]string{"name": name}, &out); err != nil {
		return nil, err
	}
	return &aiproviders.OrgWorkspace{ID: out.ID, Name: out.Name, CreatedAt: tsToString(out.CreatedAt)}, nil
}

func (p Provider) ArchiveWorkspace(ctx context.Context, ac aiproviders.AdminContext, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("project id is required")
	}
	return p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(workspaceID)+"/archive", nil, nil)
}

func (p Provider) GrantWorkspaceAccess(ctx context.Context, ac aiproviders.AdminContext, req aiproviders.WorkspaceGrantRequest) error {
	if req.WorkspaceID == "" || req.UserID == "" {
		return fmt.Errorf("project id and user id are required")
	}
	memberPath := "/v1/organization/projects/" + url.PathEscape(req.WorkspaceID) + "/users"
	role := oaiProjectRole(req.WorkspaceRole)
	err := p.adminDo(ctx, ac, http.MethodPost, memberPath, map[string]string{"user_id": req.UserID, "role": role}, nil)
	if isAlreadyExists(err) {
		// Already a member -> update the role (upsert).
		return p.adminDo(ctx, ac, http.MethodPost, memberPath+"/"+url.PathEscape(req.UserID), map[string]string{"role": role}, nil)
	}
	return err
}

func (p Provider) RemoveWorkspaceMember(ctx context.Context, ac aiproviders.AdminContext, workspaceID, userID string) error {
	if workspaceID == "" || userID == "" {
		return fmt.Errorf("project id and user id are required")
	}
	return p.adminDo(ctx, ac, http.MethodDelete, "/v1/organization/projects/"+url.PathEscape(workspaceID)+"/users/"+url.PathEscape(userID), nil, nil)
}

func (p Provider) SetOrgRole(ctx context.Context, ac aiproviders.AdminContext, userID string, role aiproviders.OrgRole) (*aiproviders.OrgUser, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	var out oaiUser
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/users/"+url.PathEscape(userID), map[string]string{"role": oaiOrgRole(role)}, &out); err != nil {
		return nil, err
	}
	return &aiproviders.OrgUser{ID: out.ID, Email: out.Email, Name: out.Name, Role: aiproviders.OrgRole(out.Role), AddedAt: tsToString(out.AddedAt)}, nil
}

func (p Provider) RemoveOrgUser(ctx context.Context, ac aiproviders.AdminContext, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user id is required")
	}
	return p.adminDo(ctx, ac, http.MethodDelete, "/v1/organization/users/"+url.PathEscape(userID), nil, nil)
}

// EffectiveWorkspaceAccess is the explicit project members (OpenAI projects have
// no inherited-billing concept, so there is nothing to union).
func (p Provider) EffectiveWorkspaceAccess(ctx context.Context, ac aiproviders.AdminContext, workspaceID string) ([]aiproviders.WorkspaceAccess, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	type oaiMember struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	members, err := adminList[oaiMember](ctx, p, ac, "/v1/organization/projects/"+url.PathEscape(workspaceID)+"/users")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.WorkspaceAccess, 0, len(members))
	for _, m := range members {
		out = append(out, aiproviders.WorkspaceAccess{
			UserID: m.ID, Email: m.Email, WorkspaceRole: aiproviders.WorkspaceRole(m.Role), Inherited: false,
		})
	}
	return out, nil
}

type oaiProjectAPIKey struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RedactedValue string `json:"redacted_value"`
	CreatedAt     int64  `json:"created_at"`
	LastUsedAt    int64  `json:"last_used_at"`
	Owner         struct {
		Type string `json:"type"`
		User *struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
		ServiceAccount *struct {
			Name string `json:"name"`
		} `json:"service_account"`
	} `json:"owner"`
}

func (k oaiProjectAPIKey) result() aiproviders.ProjectAPIKey {
	out := aiproviders.ProjectAPIKey{
		ID:            k.ID,
		Name:          k.Name,
		RedactedValue: k.RedactedValue,
		CreatedAt:     tsToString(k.CreatedAt),
		LastUsedAt:    tsToString(k.LastUsedAt),
		OwnerType:     k.Owner.Type,
	}
	if k.Owner.User != nil {
		out.OwnerName = k.Owner.User.Name
		out.OwnerEmail = k.Owner.User.Email
	}
	if k.Owner.ServiceAccount != nil {
		out.OwnerName = k.Owner.ServiceAccount.Name
	}
	return out
}

func (p Provider) ListProjectAPIKeys(ctx context.Context, ac aiproviders.AdminContext, projectID string) ([]aiproviders.ProjectAPIKey, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	rows, err := adminList[oaiProjectAPIKey](ctx, p, ac, "/v1/organization/projects/"+url.PathEscape(projectID)+"/api_keys")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.ProjectAPIKey, 0, len(rows))
	for _, k := range rows {
		out = append(out, k.result())
	}
	return out, nil
}

func (p Provider) DeleteProjectAPIKey(ctx context.Context, ac aiproviders.AdminContext, projectID, keyID string) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(keyID) == "" {
		return fmt.Errorf("project id and api key id are required")
	}
	return p.adminDo(ctx, ac, http.MethodDelete, "/v1/organization/projects/"+url.PathEscape(projectID)+"/api_keys/"+url.PathEscape(keyID), nil, nil)
}

func (p Provider) ListProjectRateLimits(ctx context.Context, ac aiproviders.AdminContext, projectID string) ([]aiproviders.ProjectRateLimit, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	rows, err := adminList[map[string]any](ctx, p, ac, "/v1/organization/projects/"+url.PathEscape(projectID)+"/rate_limits")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.ProjectRateLimit, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectRateLimitFromMap(row))
	}
	return out, nil
}

func (p Provider) UpdateProjectRateLimit(ctx context.Context, ac aiproviders.AdminContext, projectID, rateLimitID string, req aiproviders.ProjectRateLimitUpdate) (*aiproviders.ProjectRateLimit, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(rateLimitID) == "" {
		return nil, fmt.Errorf("project id and rate limit id are required")
	}
	body := map[string]any{}
	setFloat(body, "max_requests_per_1_minute", req.MaxRequestsPer1Minute)
	setFloat(body, "max_tokens_per_1_minute", req.MaxTokensPer1Minute)
	setFloat(body, "max_requests_per_1_day", req.MaxRequestsPer1Day)
	setFloat(body, "max_images_per_1_minute", req.MaxImagesPer1Minute)
	setFloat(body, "max_audio_megabytes_per_1_minute", req.MaxAudioMegabytesPer1Minute)
	setFloat(body, "batch_1_day_max_input_tokens", req.Batch1DayMaxInputTokens)
	if len(body) == 0 {
		return nil, fmt.Errorf("at least one rate limit value is required")
	}
	var out map[string]any
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(projectID)+"/rate_limits/"+url.PathEscape(rateLimitID), body, &out); err != nil {
		return nil, err
	}
	res := projectRateLimitFromMap(out)
	return &res, nil
}

func (p Provider) GetProjectModelPermissions(ctx context.Context, ac aiproviders.AdminContext, projectID string) (*aiproviders.ProjectModelPermissions, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	var out struct {
		Mode     string   `json:"mode"`
		ModelIDs []string `json:"model_ids"`
	}
	if err := p.adminDo(ctx, ac, http.MethodGet, "/v1/organization/projects/"+url.PathEscape(projectID)+"/model_permissions", nil, &out); err != nil {
		var apiErr *adminAPIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return &aiproviders.ProjectModelPermissions{Mode: "allow_list"}, nil
		}
		return nil, err
	}
	return &aiproviders.ProjectModelPermissions{Mode: out.Mode, ModelIDs: out.ModelIDs}, nil
}

func (p Provider) SetProjectModelPermissions(ctx context.Context, ac aiproviders.AdminContext, projectID string, req aiproviders.ProjectModelPermissions) (*aiproviders.ProjectModelPermissions, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	mode := strings.TrimSpace(req.Mode)
	if mode != "allow_list" && mode != "deny_list" {
		return nil, fmt.Errorf("mode must be allow_list or deny_list")
	}
	// Reject an empty model list: an empty allow_list silently DENIES every model
	// (and an empty deny_list is a no-op) - almost always an unintended footgun, so
	// require the caller to name at least one model id.
	ids := make([]string, 0, len(req.ModelIDs))
	for _, m := range req.ModelIDs {
		if s := strings.TrimSpace(m); s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("model_ids must list at least one model for %s (an empty allow_list would deny ALL models)", mode)
	}
	var out struct {
		Mode     string   `json:"mode"`
		ModelIDs []string `json:"model_ids"`
	}
	body := map[string]any{"mode": mode, "model_ids": ids}
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(projectID)+"/model_permissions", body, &out); err != nil {
		return nil, err
	}
	return &aiproviders.ProjectModelPermissions{Mode: out.Mode, ModelIDs: out.ModelIDs}, nil
}

func (p Provider) DeleteProjectModelPermissions(ctx context.Context, ac aiproviders.AdminContext, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("project id is required")
	}
	if err := p.adminDo(ctx, ac, http.MethodDelete, "/v1/organization/projects/"+url.PathEscape(projectID)+"/model_permissions", nil, nil); err != nil {
		var apiErr *adminAPIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (p Provider) GetProjectHostedToolPermissions(ctx context.Context, ac aiproviders.AdminContext, projectID string) (*aiproviders.ProjectHostedToolPermissions, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	var out map[string]map[string]bool
	if err := p.adminDo(ctx, ac, http.MethodGet, "/v1/organization/projects/"+url.PathEscape(projectID)+"/hosted_tool_permissions", nil, &out); err != nil {
		return nil, err
	}
	res := hostedToolPermissionsFromMap(out)
	return &res, nil
}

func (p Provider) SetProjectHostedToolPermissions(ctx context.Context, ac aiproviders.AdminContext, projectID string, req aiproviders.ProjectHostedToolPermissions) (*aiproviders.ProjectHostedToolPermissions, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	body := map[string]map[string]bool{
		"code_interpreter": {"enabled": req.CodeInterpreter},
		"file_search":      {"enabled": req.FileSearch},
		"image_generation": {"enabled": req.ImageGeneration},
		"mcp":              {"enabled": req.MCP},
		"web_search":       {"enabled": req.WebSearch},
	}
	var out map[string]map[string]bool
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(projectID)+"/hosted_tool_permissions", body, &out); err != nil {
		return nil, err
	}
	res := hostedToolPermissionsFromMap(out)
	return &res, nil
}

func (p Provider) GetProjectDataRetention(ctx context.Context, ac aiproviders.AdminContext, projectID string) (*aiproviders.ProjectDataRetention, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	var out struct {
		Type string `json:"type"`
	}
	if err := p.adminDo(ctx, ac, http.MethodGet, "/v1/organization/projects/"+url.PathEscape(projectID)+"/data_retention", nil, &out); err != nil {
		return nil, err
	}
	return &aiproviders.ProjectDataRetention{Type: out.Type}, nil
}

func (p Provider) SetProjectDataRetention(ctx context.Context, ac aiproviders.AdminContext, projectID string, req aiproviders.ProjectDataRetention) (*aiproviders.ProjectDataRetention, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(req.Type) == "" {
		return nil, fmt.Errorf("data retention type is required")
	}
	var out struct {
		Type string `json:"type"`
	}
	if err := p.adminDo(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(projectID)+"/data_retention", map[string]string{"type": req.Type}, &out); err != nil {
		return nil, err
	}
	return &aiproviders.ProjectDataRetention{Type: out.Type}, nil
}

func (p Provider) ListProjectSpendAlerts(ctx context.Context, ac aiproviders.AdminContext, projectID string) ([]aiproviders.ProjectSpendAlert, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	rows, err := adminList[map[string]any](ctx, p, ac, "/v1/organization/projects/"+url.PathEscape(projectID)+"/spend_alerts")
	if err != nil {
		return nil, err
	}
	out := make([]aiproviders.ProjectSpendAlert, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectSpendAlertFromMap(row))
	}
	return out, nil
}

func (p Provider) CreateProjectSpendAlert(ctx context.Context, ac aiproviders.AdminContext, projectID string, req aiproviders.ProjectSpendAlertInput) (*aiproviders.ProjectSpendAlert, error) {
	return p.writeProjectSpendAlert(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(projectID)+"/spend_alerts", req)
}

func (p Provider) UpdateProjectSpendAlert(ctx context.Context, ac aiproviders.AdminContext, projectID, alertID string, req aiproviders.ProjectSpendAlertInput) (*aiproviders.ProjectSpendAlert, error) {
	if strings.TrimSpace(alertID) == "" {
		return nil, fmt.Errorf("spend alert id is required")
	}
	return p.writeProjectSpendAlert(ctx, ac, http.MethodPost, "/v1/organization/projects/"+url.PathEscape(projectID)+"/spend_alerts/"+url.PathEscape(alertID), req)
}

func (p Provider) DeleteProjectSpendAlert(ctx context.Context, ac aiproviders.AdminContext, projectID, alertID string) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(alertID) == "" {
		return fmt.Errorf("project id and spend alert id are required")
	}
	return p.adminDo(ctx, ac, http.MethodDelete, "/v1/organization/projects/"+url.PathEscape(projectID)+"/spend_alerts/"+url.PathEscape(alertID), nil, nil)
}

func (p Provider) writeProjectSpendAlert(ctx context.Context, ac aiproviders.AdminContext, method, path string, req aiproviders.ProjectSpendAlertInput) (*aiproviders.ProjectSpendAlert, error) {
	if req.ThresholdCents <= 0 {
		return nil, fmt.Errorf("threshold cents must be greater than zero")
	}
	body := map[string]any{
		"threshold_amount": req.ThresholdCents,
		"notification_channel": map[string]any{
			"type":       "email",
			"recipients": req.Recipients,
		},
	}
	if strings.TrimSpace(req.SubjectPrefix) != "" {
		body["notification_channel"].(map[string]any)["subject_prefix"] = strings.TrimSpace(req.SubjectPrefix)
	}
	var out map[string]any
	if err := p.adminDo(ctx, ac, method, path, body, &out); err != nil {
		return nil, err
	}
	res := projectSpendAlertFromMap(out)
	// A 2xx with no alert id means the write didn't take effect as expected - surface
	// it rather than returning a hollow "success".
	if strings.TrimSpace(res.ID) == "" {
		return nil, fmt.Errorf("spend alert write returned no alert id (the change may not have been applied)")
	}
	return &res, nil
}

func setFloat(body map[string]any, key string, v *float64) {
	if v != nil {
		body[key] = *v
	}
}

func projectRateLimitFromMap(row map[string]any) aiproviders.ProjectRateLimit {
	return aiproviders.ProjectRateLimit{
		ID:                          mapString(row, "id"),
		Model:                       mapString(row, "model"),
		MaxRequestsPer1Minute:       mapFloat(row, "max_requests_per_1_minute"),
		MaxTokensPer1Minute:         mapFloat(row, "max_tokens_per_1_minute"),
		MaxRequestsPer1Day:          mapFloat(row, "max_requests_per_1_day"),
		MaxImagesPer1Minute:         mapFloat(row, "max_images_per_1_minute"),
		MaxAudioMegabytesPer1Minute: mapFloat(row, "max_audio_megabytes_per_1_minute"),
		Batch1DayMaxInputTokens:     mapFloat(row, "batch_1_day_max_input_tokens"),
		Raw:                         row,
	}
}

func hostedToolPermissionsFromMap(row map[string]map[string]bool) aiproviders.ProjectHostedToolPermissions {
	return aiproviders.ProjectHostedToolPermissions{
		CodeInterpreter: toolEnabled(row, "code_interpreter"),
		FileSearch:      toolEnabled(row, "file_search"),
		ImageGeneration: toolEnabled(row, "image_generation"),
		MCP:             toolEnabled(row, "mcp"),
		WebSearch:       toolEnabled(row, "web_search"),
	}
}

func toolEnabled(row map[string]map[string]bool, key string) bool {
	if v, ok := row[key]; ok {
		return v["enabled"]
	}
	return false
}

func projectSpendAlertFromMap(row map[string]any) aiproviders.ProjectSpendAlert {
	notification, _ := row["notification_channel"].(map[string]any)
	return aiproviders.ProjectSpendAlert{
		ID:             mapString(row, "id"),
		Currency:       mapString(row, "currency"),
		Interval:       mapString(row, "interval"),
		ThresholdCents: mapFloat(row, "threshold_amount"),
		Recipients:     mapStringSlice(notification, "recipients"),
		SubjectPrefix:  mapString(notification, "subject_prefix"),
		CreatedAt:      tsToString(int64(mapFloat(row, "created_at"))),
	}
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func mapFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	default:
		return 0
	}
}

func mapStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
