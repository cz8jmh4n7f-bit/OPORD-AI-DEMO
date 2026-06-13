package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// fakeOrg emulates the Anthropic Admin API surface we drive: users, workspaces,
// members (NOT idempotent), and the org-role of a looked-up user (for the
// matrix check). Minimal but faithful to the live behavior in ADR-0022.
type fakeOrg struct {
	users      map[string]string // user_id -> org role
	members    map[string]string // "ws|user" -> workspace role
	workspaces map[string]string // ws_id -> name
	invited    string            // last invited email
	createdWS  int
}

func newFakeOrgServer(t *testing.T, o *fakeOrg) (*httptest.Server, aiproviders.AdminContext) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-admin-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/v1/organizations/users" && r.Method == http.MethodGet:
			var data []map[string]any
			for id, role := range o.users {
				data = append(data, map[string]any{"id": id, "email": id + "@x.io", "role": role})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "has_more": false})
		case strings.HasPrefix(path, "/v1/organizations/users/") && r.Method == http.MethodGet:
			id := strings.TrimPrefix(path, "/v1/organizations/users/")
			role, ok := o.users[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "email": id + "@x.io", "role": role})
		case strings.HasPrefix(path, "/v1/organizations/workspaces/") && strings.HasSuffix(path, "/members") && r.Method == http.MethodGet:
			ws := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/organizations/workspaces/"), "/members")
			var data []map[string]any
			for k, role := range o.members {
				if strings.HasPrefix(k, ws+"|") {
					data = append(data, map[string]any{"user_id": strings.TrimPrefix(k, ws+"|"), "workspace_role": role})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "has_more": false})
		case strings.HasPrefix(path, "/v1/organizations/workspaces/") && strings.HasSuffix(path, "/members") && r.Method == http.MethodPost:
			ws := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/organizations/workspaces/"), "/members")
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			key := ws + "|" + body["user_id"]
			if _, exists := o.members[key]; exists {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "The specified user is already a member of the Workspace."}})
				return
			}
			o.members[key] = body["workspace_role"]
			_ = json.NewEncoder(w).Encode(map[string]any{"user_id": body["user_id"], "workspace_id": ws, "workspace_role": body["workspace_role"]})
		case strings.Contains(path, "/members/") && r.Method == http.MethodPost: // update role
			parts := strings.Split(strings.TrimPrefix(path, "/v1/organizations/workspaces/"), "/members/")
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			o.members[parts[0]+"|"+parts[1]] = body["workspace_role"]
			_ = json.NewEncoder(w).Encode(map[string]any{"user_id": parts[1], "workspace_role": body["workspace_role"]})
		case path == "/v1/organizations/workspaces" && r.Method == http.MethodGet:
			var data []map[string]any
			for id, name := range o.workspaces {
				data = append(data, map[string]any{"id": id, "name": name, "created_at": "now"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "has_more": false})
		case path == "/v1/organizations/workspaces" && r.Method == http.MethodPost:
			o.createdWS++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wrkspc_new", "name": "x", "created_at": "now"})
		case path == "/v1/organizations/invites" && r.Method == http.MethodPost:
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			o.invited = body["email"]
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "invite_new", "email": body["email"], "role": body["role"], "status": "pending"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, aiproviders.AdminContext{
		Credentials: map[string]string{"admin_api_key": "sk-ant-admin-good"},
		Config:      map[string]any{"base_url": srv.URL},
	}
}

func TestGrantWorkspaceAccessRejectsBillingNonAdmin(t *testing.T) {
	o := &fakeOrg{users: map[string]string{"u1": "billing"}, members: map[string]string{}}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	err := p.GrantWorkspaceAccess(context.Background(), ac, aiproviders.WorkspaceGrantRequest{
		WorkspaceID: "ws1", UserID: "u1", WorkspaceRole: aiproviders.WSRoleDeveloper,
	})
	if err == nil || !strings.Contains(err.Error(), "billing") {
		t.Fatalf("billing user + workspace_developer must be rejected by the matrix, got: %v", err)
	}
	// billing -> workspace_admin is allowed
	if err := p.GrantWorkspaceAccess(context.Background(), ac, aiproviders.WorkspaceGrantRequest{
		WorkspaceID: "ws1", UserID: "u1", WorkspaceRole: aiproviders.WSRoleAdmin,
	}); err != nil {
		t.Fatalf("billing -> workspace_admin must be allowed, got: %v", err)
	}
}

func TestGrantWorkspaceAccessUpsertsOnAlreadyMember(t *testing.T) {
	o := &fakeOrg{users: map[string]string{"u2": "developer"}, members: map[string]string{}}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	add := func(role aiproviders.WorkspaceRole) error {
		return p.GrantWorkspaceAccess(context.Background(), ac, aiproviders.WorkspaceGrantRequest{
			WorkspaceID: "ws1", UserID: "u2", WorkspaceRole: role,
		})
	}
	if err := add(aiproviders.WSRoleUser); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	// Re-grant a different role: upstream 400 "already a member" -> we upsert.
	if err := add(aiproviders.WSRoleAdmin); err != nil {
		t.Fatalf("re-grant must upsert, not fail: %v", err)
	}
	if o.members["ws1|u2"] != "workspace_admin" {
		t.Fatalf("role not upserted, got %q", o.members["ws1|u2"])
	}
}

func TestEffectiveAccessUnionsInherited(t *testing.T) {
	// adminUser is an org admin (inherits workspace_admin everywhere, invisible to
	// the members list); explicitUser is an explicit workspace_user.
	o := &fakeOrg{
		users:   map[string]string{"adminUser": "admin", "explicitUser": "developer", "billingUser": "billing"},
		members: map[string]string{"ws1|explicitUser": "workspace_user"},
	}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	access, err := p.EffectiveWorkspaceAccess(context.Background(), ac, "ws1")
	if err != nil {
		t.Fatalf("effective access: %v", err)
	}
	got := map[string]aiproviders.WorkspaceAccess{}
	for _, a := range access {
		got[a.UserID] = a
	}
	if got["explicitUser"].Inherited || got["explicitUser"].WorkspaceRole != aiproviders.WSRoleUser {
		t.Fatalf("explicitUser wrong: %+v", got["explicitUser"])
	}
	if !got["adminUser"].Inherited || got["adminUser"].WorkspaceRole != aiproviders.WSRoleAdmin {
		t.Fatalf("org admin must appear as inherited workspace_admin: %+v", got["adminUser"])
	}
	if !got["billingUser"].Inherited || got["billingUser"].WorkspaceRole != aiproviders.WSRoleBilling {
		t.Fatalf("org billing must appear as inherited workspace_billing: %+v", got["billingUser"])
	}
}

func TestInviteAndSetOrgRoleRejectAdmin(t *testing.T) {
	o := &fakeOrg{users: map[string]string{"u9": "user"}, members: map[string]string{}}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	if _, err := p.InviteUser(context.Background(), ac, aiproviders.InviteRequest{Email: "a@b.io", Role: aiproviders.OrgRoleAdmin}); err == nil {
		t.Fatal("inviting as admin must be rejected (Console-only)")
	}
	if _, err := p.SetOrgRole(context.Background(), ac, "u9", aiproviders.OrgRoleAdmin); err == nil {
		t.Fatal("setting org role to admin must be rejected (Console-only)")
	}
}

func TestProvisionAccessRealGrantExistingUser(t *testing.T) {
	o := &fakeOrg{
		users:      map[string]string{"alice": "developer"},
		members:    map[string]string{},
		workspaces: map[string]string{"ws1": "team-a"},
	}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	// alice@x.io is an existing dev; granting into the real workspace "team-a"
	// must ADD a real membership (not invite).
	res, err := p.ProvisionAccess(context.Background(), aiproviders.ProvisionRequest{
		Owner: "alice@x.io", Workspace: "team-a",
		Credentials: ac.Credentials, Config: ac.Config,
	})
	if err != nil {
		t.Fatalf("ProvisionAccess: %v", err)
	}
	if o.members["ws1|alice"] != "workspace_developer" {
		t.Fatalf("expected a real workspace_developer membership, got %q", o.members["ws1|alice"])
	}
	if res.Observed["external_provisioning"] != "granted" {
		t.Fatalf("expected granted, got %v", res.Observed["external_provisioning"])
	}
	if !strings.HasPrefix(res.ProviderAccessID, "anthropic-ws:ws1:user:alice") {
		t.Fatalf("access id should encode ws+user, got %q", res.ProviderAccessID)
	}
}

func TestProvisionAccessInvitesUnknownUser(t *testing.T) {
	o := &fakeOrg{users: map[string]string{}, members: map[string]string{}, workspaces: map[string]string{"ws1": "team-a"}}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	res, err := p.ProvisionAccess(context.Background(), aiproviders.ProvisionRequest{
		Owner: "newhire@x.io", Workspace: "team-a",
		Credentials: ac.Credentials, Config: ac.Config,
	})
	if err != nil {
		t.Fatalf("ProvisionAccess: %v", err)
	}
	if o.invited != "newhire@x.io" {
		t.Fatalf("unknown user must be invited, invited=%q", o.invited)
	}
	if res.Observed["external_provisioning"] != "invited" {
		t.Fatalf("expected invited, got %v", res.Observed["external_provisioning"])
	}
}

func TestProvisionAccessGovernanceOnlyWithoutRealTarget(t *testing.T) {
	o := &fakeOrg{users: map[string]string{}, members: map[string]string{}, workspaces: map[string]string{}}
	srv, ac := newFakeOrgServer(t, o)
	p := Provider{client: srv.Client()}
	// Workspace "default" (the placeholder) must NOT trigger a real grant.
	res, err := p.ProvisionAccess(context.Background(), aiproviders.ProvisionRequest{
		Owner: "someone@x.io", Workspace: "default",
		Credentials: ac.Credentials, Config: ac.Config,
	})
	if err != nil {
		t.Fatalf("ProvisionAccess: %v", err)
	}
	if res.Observed["external_provisioning"] != "manual" {
		t.Fatalf("placeholder workspace must stay governance-only, got %v", res.Observed["external_provisioning"])
	}
	if o.invited != "" {
		t.Fatalf("must not invite for a governance-only record, invited=%q", o.invited)
	}
}
