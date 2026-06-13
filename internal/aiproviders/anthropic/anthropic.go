// Package anthropic implements OPORD's Anthropic / Claude governance provider.
package anthropic

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// Provider grants governance records for Anthropic Claude access. MVP
// provisioning is manual: OPORD tracks approval, ownership, expiry, revoke, and
// audit; the real seat/API grant remains in Anthropic/admin tooling.
type Provider struct {
	client *http.Client
}

// Register wires AnthropicProvider into the AI provider registry.
func Register(r *aiproviders.Registry) {
	r.Register(aiproviders.ProviderAnthropic, func() aiproviders.Provider {
		return Provider{client: &http.Client{Timeout: 10 * time.Second}}
	})
}

func (Provider) Type() aiproviders.ProviderType { return aiproviders.ProviderAnthropic }

// ValidateCredentials understands Anthropic's two-credential model (ADR-0022):
// the ADMIN key (sk-ant-admin..., stored as admin_api_key) drives governance -
// org users, workspaces, roles, billing - and is validated against the Admin API;
// an inference key (api_key) only serves model/gateway calls and is OPTIONAL.
// Each key is probed against the surface it can actually reach (an admin key
// 401s on /v1/models and vice versa), so a governance-only setup checks green.
func (p Provider) ValidateCredentials(ctx context.Context, req aiproviders.CredentialRequest) error {
	admin := adminKey(req.Credentials)
	inference := apiKey(req.Credentials, req.Config)
	if admin == "" && inference == "" {
		return fmt.Errorf("anthropic key missing: store admin_api_key (sk-ant-admin..., governance/billing) and/or api_key (inference) in the secret_ref, or set ANTHROPIC_API_KEY")
	}
	base := strings.TrimRight(baseURL(req.Config, "https://api.anthropic.com"), "/")
	if admin != "" {
		if err := p.probe(ctx, base+"/v1/organizations/users?limit=1", admin, req.Config); err != nil {
			return fmt.Errorf("admin key (admin_api_key): %w", err)
		}
	}
	if inference != "" {
		if err := p.probe(ctx, base+"/v1/models", inference, req.Config); err != nil {
			return fmt.Errorf("inference key (api_key): %w", err)
		}
	}
	return nil
}

// probe GETs one endpoint with one key and reports non-2xx as an error.
func (p Provider) probe(ctx context.Context, url, key string, cfg map[string]any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", anthropicVersion(cfg))
	resp, err := p.http().Do(httpReq)
	if err != nil {
		return fmt.Errorf("anthropic credential check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("anthropic credential check returned %s", resp.Status)
	}
	return nil
}

func (Provider) ListAvailableServices(context.Context, aiproviders.ServiceListRequest) ([]aiproviders.Service, error) {
	return []aiproviders.Service{
		{
			Name:                  "Claude API Access",
			Slug:                  "claude-api-access",
			Category:              "api_access",
			Description:           "Governed access to Anthropic Claude API usage.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "Claude Code Access",
			Slug:                  "claude-code-access",
			Category:              "developer_tool",
			Description:           "Governed Claude Code entitlement/license request with owner, expiry, and audit trail.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "repo_scope", "expires_at"),
			DefaultExpirationDays: 90,
			RequiresApproval:      true,
		},
	}, nil
}

// ListModels returns the LIVE model catalog from GET /v1/models (paginated) when
// an inference key is available. /v1/models is NOT reachable with the admin key
// (the two-credential split, ADR-0022), so a governance-only setup falls back to
// a curated list of the current real model IDs instead of failing.
func (p Provider) ListModels(ctx context.Context, req aiproviders.ModelListRequest) ([]aiproviders.Model, error) {
	key := apiKey(req.Credentials, req.Config)
	if key == "" {
		return curatedModels(), nil
	}
	base := strings.TrimRight(baseURL(req.Config, "https://api.anthropic.com"), "/")
	version := anthropicVersion(req.Config)
	var models []aiproviders.Model
	after := ""
	for page := 0; page < 20; page++ { // generous bound: 20 pages x 100 models
		u := base + "/v1/models?limit=100"
		if after != "" {
			u += "&after_id=" + url.QueryEscape(after)
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("x-api-key", key)
		httpReq.Header.Set("anthropic-version", version)
		resp, err := p.http().Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("anthropic model sync failed: %w", err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			msg := strings.TrimSpace(string(body))
			if len(msg) > 256 {
				msg = msg[:256]
			}
			return nil, fmt.Errorf("anthropic model sync returned %s: %s", resp.Status, msg)
		}
		var payload struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				CreatedAt   string `json:"created_at"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("decoding anthropic model list: %w", err)
		}
		for _, m := range payload.Data {
			if strings.TrimSpace(m.ID) == "" {
				continue
			}
			name := m.DisplayName
			if name == "" {
				name = m.ID
			}
			models = append(models, aiproviders.Model{
				Model:       m.ID,
				DisplayName: name,
				Modality:    "text",
				Status:      "active",
				Metadata: map[string]any{
					"source":     "anthropic_models_api",
					"created_at": m.CreatedAt,
				},
			})
		}
		if !payload.HasMore || strings.TrimSpace(payload.LastID) == "" {
			break
		}
		after = payload.LastID
	}
	return models, nil
}

// curatedModels is the governance-only fallback: the CURRENT real Anthropic
// model ids/aliases (not invented "-latest" placeholders), refreshed manually.
func curatedModels() []aiproviders.Model {
	curated := []struct{ id, name string }{
		{"claude-fable-5", "Claude Fable 5"},
		{"claude-opus-4-8", "Claude Opus 4.8"},
		{"claude-opus-4-7", "Claude Opus 4.7"},
		{"claude-opus-4-6", "Claude Opus 4.6"},
		{"claude-sonnet-4-6", "Claude Sonnet 4.6"},
		{"claude-haiku-4-5", "Claude Haiku 4.5"},
		{"claude-opus-4-5", "Claude Opus 4.5 (legacy)"},
		{"claude-opus-4-1", "Claude Opus 4.1 (legacy)"},
		{"claude-sonnet-4-5", "Claude Sonnet 4.5 (legacy)"},
	}
	models := make([]aiproviders.Model, 0, len(curated))
	for _, c := range curated {
		models = append(models, aiproviders.Model{
			Model:       c.id,
			DisplayName: c.name,
			Modality:    "text",
			Status:      "active",
			Metadata:    map[string]any{"source": "curated", "note": "live sync needs an inference api_key in the secret_ref"},
		})
	}
	return models
}

// ProvisionAccess does a REAL grant when the inputs allow it: an admin key, an
// email owner, and a concrete target workspace (not the placeholder "default")
// drive an actual Admin-API membership grant (inviting the user first if they are
// not in the org yet). Without those (e.g. no admin key, or a generic "default"
// workspace) it records the governance-only entitlement as before - the request,
// approval, owner, expiry, and audit trail still hold, the seat is just granted
// out-of-band. A real grant that is ATTEMPTED but fails returns the error so the
// approval surfaces it, rather than silently looking provisioned.
func (p Provider) ProvisionAccess(ctx context.Context, req aiproviders.ProvisionRequest) (*aiproviders.ProvisionResult, error) {
	admin := adminKey(req.Credentials)
	grantee := strings.TrimSpace(req.Owner)
	ws := strings.TrimSpace(req.Workspace)
	realGrant := admin != "" && strings.Contains(grantee, "@") && ws != "" && !strings.EqualFold(ws, "default")
	if realGrant {
		return p.grantRealAccess(ctx, req, grantee, ws)
	}

	accessID := deterministicID("anthropic", req.RequestID.String(), req.Service.Slug, req.Owner)
	reason := "Claude/Claude Code access approved in OPORD; grant/revoke in Anthropic admin or existing IdP workflow."
	if admin != "" {
		reason = "Governance entitlement recorded. For a real workspace grant, set the request owner to a user email and the workspace to a real workspace name."
	}
	return &aiproviders.ProvisionResult{
		ProviderAccessID: accessID,
		Observed: map[string]any{
			"provider":              "anthropic",
			"provider_access_id":    accessID,
			"service":               req.Service.Slug,
			"owner":                 req.Owner,
			"workspace":             req.Workspace,
			"external_provisioning": "manual",
			"message":               reason,
		},
	}, nil
}

// grantRealAccess resolves the owner email to an org user (inviting them if
// absent), resolves the workspace by name or id, picks a matrix-valid role, and
// adds the membership through the Admin API. The ProviderAccessID encodes the real
// workspace+user so RevokeAccess can undo it.
func (p Provider) grantRealAccess(ctx context.Context, req aiproviders.ProvisionRequest, email, wsRef string) (*aiproviders.ProvisionResult, error) {
	ac := aiproviders.AdminContext{Credentials: req.Credentials, Config: req.Config}

	users, err := p.ListOrgUsers(ctx, ac)
	if err != nil {
		return nil, err
	}
	var user *aiproviders.OrgUser
	for i := range users {
		if strings.EqualFold(users[i].Email, email) {
			user = &users[i]
			break
		}
	}
	if user == nil {
		// Not in the org yet - invite them (two-phase; they must accept before a
		// workspace membership is possible).
		inv, err := p.InviteUser(ctx, ac, aiproviders.InviteRequest{Email: email, Role: aiproviders.OrgRoleDeveloper})
		if err != nil {
			return nil, fmt.Errorf("inviting %s: %w", email, err)
		}
		return &aiproviders.ProvisionResult{
			ProviderAccessID: "anthropic-invite:" + inv.InviteID,
			Observed: map[string]any{
				"provider": "anthropic", "owner": email, "workspace": wsRef,
				"external_provisioning": "invited",
				"invite_id":             inv.InviteID, "invite_status": inv.Status,
				"message": fmt.Sprintf("Invited %s to the organization; workspace access is granted automatically once they accept.", email),
			},
		}, nil
	}

	workspaces, err := p.ListWorkspaces(ctx, ac)
	if err != nil {
		return nil, err
	}
	var wsID, wsName string
	for _, w := range workspaces {
		if w.ArchivedAt == "" && (strings.EqualFold(w.Name, wsRef) || w.ID == wsRef) {
			wsID, wsName = w.ID, w.Name
			break
		}
	}
	if wsID == "" {
		return nil, fmt.Errorf("no active workspace named %q in the organization", wsRef)
	}

	role := requestedWorkspaceRole(req.Spec)
	if !aiproviders.RoleComboAllowed(user.Role, role) {
		// A billing user can only be a workspace_admin - upgrade rather than fail.
		if user.Role == aiproviders.OrgRoleBilling {
			role = aiproviders.WSRoleAdmin
		} else {
			return nil, fmt.Errorf("workspace role %q is not allowed for org role %q", role, user.Role)
		}
	}
	if err := p.GrantWorkspaceAccess(ctx, ac, aiproviders.WorkspaceGrantRequest{
		WorkspaceID: wsID, UserID: user.ID, WorkspaceRole: role,
	}); err != nil {
		return nil, err
	}
	return &aiproviders.ProvisionResult{
		ProviderAccessID: fmt.Sprintf("anthropic-ws:%s:user:%s", wsID, user.ID),
		Observed: map[string]any{
			"provider": "anthropic", "owner": email,
			"workspace": wsName, "workspace_id": wsID,
			"user_id": user.ID, "workspace_role": string(role),
			"external_provisioning": "granted",
			"message":               fmt.Sprintf("Granted %s the %s role in workspace %q.", email, role, wsName),
		},
	}, nil
}

// requestedWorkspaceRole reads an optional workspace_role from the request spec;
// defaults to workspace_developer.
func requestedWorkspaceRole(spec []byte) aiproviders.WorkspaceRole {
	if len(spec) > 0 {
		var m map[string]any
		if json.Unmarshal(spec, &m) == nil {
			if v, ok := m["workspace_role"].(string); ok && strings.TrimSpace(v) != "" {
				return aiproviders.WorkspaceRole(strings.TrimSpace(v))
			}
		}
	}
	return aiproviders.WSRoleDeveloper
}

// RevokeAccess undoes a real grant encoded in the ProviderAccessID. A workspace
// membership is removed (a billing user, who can't be removed, is reset to their
// inherited workspace_billing); a pending invite is deleted; a governance-only
// record is a no-op.
func (p Provider) RevokeAccess(ctx context.Context, req aiproviders.RevokeRequest) error {
	if req.ProviderAccessID == "" {
		return fmt.Errorf("provider access id is required")
	}
	ac := aiproviders.AdminContext{Credentials: req.Credentials, Config: req.Config}
	id := req.ProviderAccessID
	switch {
	case strings.HasPrefix(id, "anthropic-ws:"):
		// anthropic-ws:<wsID>:user:<userID>
		rest := strings.TrimPrefix(id, "anthropic-ws:")
		parts := strings.SplitN(rest, ":user:", 2)
		if len(parts) != 2 {
			return fmt.Errorf("malformed access id %q", id)
		}
		wsID, userID := parts[0], parts[1]
		err := p.RemoveWorkspaceMember(ctx, ac, wsID, userID)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "billing") {
			// Billing users can't be removed - reset to the inherited billing role.
			return p.adminDo(ctx, ac, http.MethodPost,
				"/v1/organizations/workspaces/"+url.PathEscape(wsID)+"/members/"+url.PathEscape(userID),
				map[string]string{"workspace_role": string(aiproviders.WSRoleBilling)}, nil)
		}
		return err
	case strings.HasPrefix(id, "anthropic-invite:"):
		inviteID := strings.TrimPrefix(id, "anthropic-invite:")
		return p.adminDo(ctx, ac, http.MethodDelete, "/v1/organizations/invites/"+url.PathEscape(inviteID), nil, nil)
	default:
		return nil // governance-only record; nothing to undo upstream
	}
}

func (Provider) GetUsage(_ context.Context, req aiproviders.UsageRequest) ([]aiproviders.UsageRecord, error) {
	return []aiproviders.UsageRecord{
		{
			Metric:   "tokens",
			Quantity: 0,
			Unit:     "tokens",
			CostUSD:  0,
			Raw: map[string]any{
				"provider_access_id": req.ProviderAccessID,
				"source":             "not_imported",
				"message":            "Anthropic usage import is not implemented in this phase.",
			},
		},
	}, nil
}

func (Provider) GetStatus(_ context.Context, req aiproviders.StatusRequest) (*aiproviders.StatusResult, error) {
	return &aiproviders.StatusResult{
		Status: "active",
		Observed: map[string]any{
			"provider_access_id":    req.ProviderAccessID,
			"external_provisioning": "manual",
		},
	}, nil
}

func (p Provider) http() *http.Client {
	if p.client != nil {
		return p.client
	}
	return http.DefaultClient
}

func apiKey(creds map[string]string, _ map[string]any) string {
	for _, key := range []string{"api_key", "anthropic_api_key", "token"} {
		if v := strings.TrimSpace(creds[key]); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
}

// adminKey reads the org ADMIN key (governance/billing: /v1/organizations/*).
func adminKey(creds map[string]string) string {
	if v := strings.TrimSpace(creds["admin_api_key"]); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("ANTHROPIC_ADMIN_KEY"))
}

func baseURL(cfg map[string]any, fallback string) string {
	if v, ok := cfg["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func anthropicVersion(cfg map[string]any) string {
	if v, ok := cfg["anthropic_version"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "2023-06-01"
}

func deterministicID(parts ...string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, ":")))
	return parts[0] + "-" + hex.EncodeToString(sum[:])[:16]
}

func requestSchema(fields ...string) map[string]any {
	return map[string]any{"fields": fields}
}
