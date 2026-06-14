package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/google/uuid"
)

func fakeOpenAIOrg(t *testing.T) (*httptest.Server, aiproviders.AdminContext, *map[string]any) {
	t.Helper()
	lastBody := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-admin-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
		}
		switch {
		case r.URL.Path == "/v1/organization/users" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "user-1", "email": "a@x.io", "role": "reader", "added_at": 1700000000}}, "has_more": false})
		case r.URL.Path == "/v1/organization/invites" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "invite-1", "email": lastBody["email"], "role": lastBody["role"], "status": "pending"})
		case r.URL.Path == "/v1/organization/projects" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "proj-1", "name": lastBody["name"], "created_at": 1700000001})
		case strings.HasSuffix(r.URL.Path, "/users") && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"user_id": lastBody["user_id"], "role": lastBody["role"]})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, aiproviders.AdminContext{Credentials: map[string]string{"admin_api_key": "sk-admin-good"}, Config: map[string]any{"base_url": srv.URL}}, &lastBody
}

func TestOpenAIListUsers(t *testing.T) {
	srv, ac, _ := fakeOpenAIOrg(t)
	p := Provider{client: srv.Client()}
	users, err := p.ListOrgUsers(context.Background(), ac)
	if err != nil || len(users) != 1 || users[0].Email != "a@x.io" || users[0].Role != "reader" {
		t.Fatalf("list users: %v / %+v", err, users)
	}
}

func TestOpenAIInviteRoleNormalized(t *testing.T) {
	srv, ac, body := fakeOpenAIOrg(t)
	p := Provider{client: srv.Client()}
	// A non-owner org role must normalize to "reader" for OpenAI.
	if _, err := p.InviteUser(context.Background(), ac, aiproviders.InviteRequest{Email: "b@x.io", Role: "developer"}); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if (*body)["role"] != "reader" {
		t.Fatalf("role not normalized to reader: %v", (*body)["role"])
	}
	// "owner" stays owner.
	_, _ = p.InviteUser(context.Background(), ac, aiproviders.InviteRequest{Email: "c@x.io", Role: "owner"})
	if (*body)["role"] != "owner" {
		t.Fatalf("owner role lost: %v", (*body)["role"])
	}
}

func TestOpenAIGrantProjectRoleNormalized(t *testing.T) {
	srv, ac, body := fakeOpenAIOrg(t)
	p := Provider{client: srv.Client()}
	// workspace_admin -> openai project "owner"; anything else -> "member".
	_ = p.GrantWorkspaceAccess(context.Background(), ac, aiproviders.WorkspaceGrantRequest{WorkspaceID: "proj-1", UserID: "user-1", WorkspaceRole: aiproviders.WSRoleAdmin})
	if (*body)["role"] != "owner" {
		t.Fatalf("workspace_admin should map to owner: %v", (*body)["role"])
	}
	_ = p.GrantWorkspaceAccess(context.Background(), ac, aiproviders.WorkspaceGrantRequest{WorkspaceID: "proj-1", UserID: "user-1", WorkspaceRole: aiproviders.WSRoleUser})
	if (*body)["role"] != "member" {
		t.Fatalf("workspace_user should map to member: %v", (*body)["role"])
	}
}

func TestOpenAIImplementsAdminProvisioner(t *testing.T) {
	var _ aiproviders.AdminProvisioner = Provider{}
}

// TestSetModelPermissionsRejectsEmptyList locks in the deny-all guard: an
// allow_list with no model ids would silently DENY every model, so it must be
// rejected BEFORE any API call (no server needed - it fails on validation).
func TestSetModelPermissionsRejectsEmptyList(t *testing.T) {
	p := Provider{client: http.DefaultClient}
	ac := aiproviders.AdminContext{Credentials: map[string]string{"admin_api_key": "sk-admin-x"}}
	if _, err := p.SetProjectModelPermissions(context.Background(), ac, "proj-1", aiproviders.ProjectModelPermissions{Mode: "allow_list", ModelIDs: nil}); err == nil {
		t.Fatal("empty allow_list must be rejected (would deny ALL models)")
	}
	if _, err := p.SetProjectModelPermissions(context.Background(), ac, "proj-1", aiproviders.ProjectModelPermissions{Mode: "deny_list", ModelIDs: []string{"  "}}); err == nil {
		t.Fatal("blank-only model_ids must be rejected")
	}
}

// TestAdminAPIErrorDoesNotLeakBody locks in the leak fix: the error string is
// status-only - the raw upstream OpenAI body (which can carry org detail) must not
// reach callers that propagate err.Error() into HTTP responses / audit logs.
func TestAdminAPIErrorDoesNotLeakBody(t *testing.T) {
	e := &adminAPIError{StatusCode: 400, Body: `{"error":{"message":"SECRET-org-detail sk-leak"}}`}
	msg := e.Error()
	if strings.Contains(msg, "SECRET") || strings.Contains(msg, "sk-leak") {
		t.Fatalf("Error() must not leak the raw upstream body: %q", msg)
	}
	if !strings.Contains(msg, "400") {
		t.Fatalf("Error() should still convey the status: %q", msg)
	}
	// .Body is still available for internal branching (isAlreadyExists).
	if !isAlreadyExists(&adminAPIError{StatusCode: 400, Body: "user is already a member"}) {
		t.Fatal("isAlreadyExists must still read .Body even though Error() hides it")
	}
}

// TestOpenAIValidateRoutesByKeyType checks the admin-key-aware credential check:
// an admin key (sk-admin-) validates against /v1/organization/* (where it has
// access), while a project key (sk-proj-) routes to /v1/models. The fake server
// 403s /v1/models, so the admin key passes and the project key fails - proving the
// routing is by key prefix, not a misleading 403 for valid admin keys.
func TestOpenAIValidateRoutesByKeyType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/organization/") {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusForbidden) // /v1/models is forbidden for admin keys
	}))
	t.Cleanup(srv.Close)
	p := Provider{client: srv.Client()}

	admin := aiproviders.CredentialRequest{
		Credentials: map[string]string{"admin_api_key": "sk-admin-xyz", "api_key": "sk-admin-xyz"},
		Config:      map[string]any{"base_url": srv.URL},
	}
	if err := p.ValidateCredentials(context.Background(), admin); err != nil {
		t.Fatalf("admin key must validate against the admin endpoint (green), got %v", err)
	}

	project := aiproviders.CredentialRequest{
		Credentials: map[string]string{"api_key": "sk-proj-abc"},
		Config:      map[string]any{"base_url": srv.URL},
	}
	if err := p.ValidateCredentials(context.Background(), project); err == nil {
		t.Fatal("project key must route to /v1/models (403 here) and fail")
	}
}

func TestIsAlreadyExists(t *testing.T) {
	if !isAlreadyExists(&adminAPIError{StatusCode: http.StatusConflict, Body: "anything"}) {
		t.Error("409 Conflict must be treated as already-exists")
	}
	if !isAlreadyExists(&adminAPIError{StatusCode: http.StatusBadRequest, Body: "user is already a member"}) {
		t.Error("400 with an 'already' body must fall back to already-exists")
	}
	if isAlreadyExists(&adminAPIError{StatusCode: http.StatusBadRequest, Body: "invalid role"}) {
		t.Error("an unrelated 400 must NOT be already-exists")
	}
	if isAlreadyExists(nil) {
		t.Error("nil error must not be already-exists")
	}
}

// TestOpenAIOrgAdminMapsToOwner covers the role-floor fix: the abstract org role
// "admin" (Anthropic-shaped enum) must map to OpenAI's elevated "owner", not floor
// silently to "reader".
func TestOpenAIOrgAdminMapsToOwner(t *testing.T) {
	srv, ac, body := fakeOpenAIOrg(t)
	p := Provider{client: srv.Client()}
	if _, err := p.InviteUser(context.Background(), ac, aiproviders.InviteRequest{Email: "d@x.io", Role: aiproviders.OrgRoleAdmin}); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if (*body)["role"] != "owner" {
		t.Fatalf("admin org role must map to owner, got %v", (*body)["role"])
	}
}

// TestOpenAIGrantUpsertsOnConflict checks that a 409 on member-add triggers the
// role-update (upsert) call instead of returning an error.
func TestOpenAIGrantUpsertsOnConflict(t *testing.T) {
	var roleUpdated bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/users") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusConflict) // already a member
			_, _ = w.Write([]byte(`{"error":"conflict"}`))
		case strings.Contains(r.URL.Path, "/users/") && r.Method == http.MethodPost:
			roleUpdated = true
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	ac := aiproviders.AdminContext{Credentials: map[string]string{"admin_api_key": "sk-admin-good"}, Config: map[string]any{"base_url": srv.URL}}
	p := Provider{client: srv.Client()}
	if err := p.GrantWorkspaceAccess(context.Background(), ac, aiproviders.WorkspaceGrantRequest{WorkspaceID: "proj-1", UserID: "user-1", WorkspaceRole: aiproviders.WSRoleUser}); err != nil {
		t.Fatalf("grant should upsert on conflict, got %v", err)
	}
	if !roleUpdated {
		t.Fatal("a 409 on member-add must trigger the role-update upsert call")
	}
}

func TestOpenAIProjectControls(t *testing.T) {
	var modelPermissionBody map[string]any
	var toolPermissionBody map[string]any
	var rateLimitBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/model_permissions") && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&modelPermissionBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"mode": modelPermissionBody["mode"], "model_ids": modelPermissionBody["model_ids"]})
		case strings.HasSuffix(r.URL.Path, "/hosted_tool_permissions") && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&toolPermissionBody)
			_ = json.NewEncoder(w).Encode(toolPermissionBody)
		case strings.HasSuffix(r.URL.Path, "/rate_limits") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "rl-1", "model": "gpt-test", "max_requests_per_1_minute": 10, "max_tokens_per_1_minute": 1000}}, "has_more": false})
		case strings.HasSuffix(r.URL.Path, "/rate_limits/rl-1") && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&rateLimitBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "rl-1", "model": "gpt-test", "max_requests_per_1_minute": rateLimitBody["max_requests_per_1_minute"]})
		case strings.HasSuffix(r.URL.Path, "/data_retention") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "organization_default"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	ac := aiproviders.AdminContext{Credentials: map[string]string{"admin_api_key": "sk-admin-good"}, Config: map[string]any{"base_url": srv.URL}}
	p := Provider{client: srv.Client()}

	models, err := p.SetProjectModelPermissions(context.Background(), ac, "proj-1", aiproviders.ProjectModelPermissions{Mode: "allow_list", ModelIDs: []string{"gpt-test"}})
	if err != nil || models.Mode != "allow_list" || len(models.ModelIDs) != 1 {
		t.Fatalf("model permissions: %v / %+v", err, models)
	}
	if modelPermissionBody["mode"] != "allow_list" {
		t.Fatalf("bad model permission body: %+v", modelPermissionBody)
	}

	tools, err := p.SetProjectHostedToolPermissions(context.Background(), ac, "proj-1", aiproviders.ProjectHostedToolPermissions{WebSearch: true, FileSearch: true})
	if err != nil || !tools.WebSearch || !tools.FileSearch {
		t.Fatalf("tool permissions: %v / %+v", err, tools)
	}

	limits, err := p.ListProjectRateLimits(context.Background(), ac, "proj-1")
	if err != nil || len(limits) != 1 || limits[0].Model != "gpt-test" {
		t.Fatalf("rate limits: %v / %+v", err, limits)
	}
	n := 42.0
	updated, err := p.UpdateProjectRateLimit(context.Background(), ac, "proj-1", "rl-1", aiproviders.ProjectRateLimitUpdate{MaxRequestsPer1Minute: &n})
	if err != nil || updated.MaxRequestsPer1Minute != 42 {
		t.Fatalf("rate limit update: %v / %+v", err, updated)
	}

	retention, err := p.GetProjectDataRetention(context.Background(), ac, "proj-1")
	if err != nil || retention.Type != "organization_default" {
		t.Fatalf("data retention: %v / %+v", err, retention)
	}
}

func TestOpenAIProvisionAppliesModelPermissions(t *testing.T) {
	var createdProject bool
	var appliedModels bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/organization/projects" && r.Method == http.MethodPost:
			createdProject = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "proj-new", "name": "team-ai", "created_at": 1700000001})
		case r.URL.Path == "/v1/organization/projects/proj-new/model_permissions" && r.Method == http.MethodPost:
			appliedModels = true
			_ = json.NewEncoder(w).Encode(map[string]any{"mode": "allow_list", "model_ids": []string{"gpt-test"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	p := Provider{client: srv.Client()}
	spec := json.RawMessage(`{"metadata":{"create_project":true,"project_name":"team-ai","mode":"allow_list","model_ids":["gpt-test"]}}`)
	res, err := p.ProvisionAccess(context.Background(), aiproviders.ProvisionRequest{
		RequestID:   mustUUID(t, "00000000-0000-0000-0000-000000000001"),
		Service:     aiproviders.Service{Slug: "openai-model-permissions"},
		Owner:       "team",
		Workspace:   "team-ai",
		Spec:        spec,
		Credentials: map[string]string{"admin_api_key": "sk-admin-good"},
		Config:      map[string]any{"base_url": srv.URL},
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if !createdProject || !appliedModels {
		t.Fatalf("expected project creation and model permissions application")
	}
	if res.Observed["external_provisioning"] != "openai_project_model_permissions" || res.Observed["project_id"] != "proj-new" {
		t.Fatalf("unexpected observed: %+v", res.Observed)
	}
}

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
