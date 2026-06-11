// Package azure is a minimal Microsoft Graph client for the slice of Entra
// (Azure AD) automation OPORD needs to manage the *Entra side* of AWS SAML
// federation: managing an enterprise app's app roles (which carry the AWS Role
// claim via user.assignedroles), assigning users to those roles, and inviting
// B2B guests. Raw net/http + client-credentials - same style as internal/glpi,
// no heavyweight SDK dependency.
//
// Auth: the OAuth2 client-credentials flow against OPORD's own Azure app
// registration (see docs/runbooks/aws-opord-graph-setup.md). The app needs the
// application permissions Application.ReadWrite.All (or OwnedBy),
// AppRoleAssignment.ReadWrite.All, User.Invite.All and User.Read.All, all
// admin-consented.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultLoginBase = "https://login.microsoftonline.com"
	defaultGraphBase = "https://graph.microsoft.com/v1.0"
	// The client-credentials scope for app-only Graph access.
	graphScope = "https://graph.microsoft.com/.default"
)

// Config holds OPORD's Azure app-registration credentials. LoginBase/GraphBase
// default to the public endpoints and are overridable for tests.
type Config struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	LoginBase    string
	GraphBase    string
}

// Client is a tenant-scoped Microsoft Graph client with a cached app token.
type Client struct {
	cfg  Config
	http *http.Client
	log  *slog.Logger

	mu      sync.Mutex
	token   string
	expires time.Time
}

// New builds a Graph client. It does not perform any network I/O.
func New(cfg Config, log *slog.Logger) *Client {
	if cfg.LoginBase == "" {
		cfg.LoginBase = defaultLoginBase
	}
	if cfg.GraphBase == "" {
		cfg.GraphBase = defaultGraphBase
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}, log: log}
}

// Configured reports whether the credentials needed to call Graph are present.
func (c *Client) Configured() bool {
	return c.cfg.TenantID != "" && c.cfg.ClientID != "" && c.cfg.ClientSecret != ""
}

// GraphError carries a non-2xx Graph response for callers that want to inspect it.
type GraphError struct {
	Status int
	Method string
	Path   string
	Body   string
}

func (e *GraphError) Error() string {
	return fmt.Sprintf("graph %s %s -> %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// accessToken returns a cached app token, refreshing it ~60s before expiry.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires.Add(-60*time.Second)) {
		return c.token, nil
	}
	form := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"grant_type":    {"client_credentials"},
		"scope":         {graphScope},
	}
	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.cfg.LoginBase, c.cfg.TenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("graph token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("graph token endpoint %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("graph token decode: %w", err)
	}
	c.token = tr.AccessToken
	c.expires = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return c.token, nil
}

// do performs an authenticated Graph request, JSON in/out.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.GraphBase+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("graph %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &GraphError{Status: resp.StatusCode, Method: method, Path: path, Body: strings.TrimSpace(string(data))}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// ServicePrincipal is the directory object that backs an enterprise app.
type ServicePrincipal struct {
	ID          string `json:"id"`
	AppID       string `json:"appId"`
	DisplayName string `json:"displayName"`
}

// ServicePrincipalByAppID looks up the service principal (enterprise app) for a
// client/app id. The SP object id is the resourceId for app-role assignments.
func (c *Client) ServicePrincipalByAppID(ctx context.Context, appID string) (*ServicePrincipal, error) {
	var resp struct {
		Value []ServicePrincipal `json:"value"`
	}
	path := "/servicePrincipals?$filter=" + url.QueryEscape("appId eq '"+appID+"'")
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Value) == 0 {
		return nil, fmt.Errorf("no service principal for appId %s", appID)
	}
	return &resp.Value[0], nil
}

type appRole struct {
	ID                 string   `json:"id"`
	AllowedMemberTypes []string `json:"allowedMemberTypes"`
	Description        string   `json:"description"`
	DisplayName        string   `json:"displayName"`
	IsEnabled          bool     `json:"isEnabled"`
	Value              string   `json:"value"`
}

type application struct {
	ID       string    `json:"id"`
	AppID    string    `json:"appId"`
	AppRoles []appRole `json:"appRoles"`
}

func (c *Client) applicationByAppID(ctx context.Context, appID string) (*application, error) {
	var resp struct {
		Value []application `json:"value"`
	}
	path := "/applications?$filter=" + url.QueryEscape("appId eq '"+appID+"'")
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Value) == 0 {
		return nil, fmt.Errorf("no application for appId %s", appID)
	}
	return &resp.Value[0], nil
}

// EnsureAppRole makes sure the application has an enabled app role whose value
// equals the AWS Role-claim string ("<role_arn>,<provider_arn>"), returning the
// app role id. Idempotent: an existing role with the same value is reused.
func (c *Client) EnsureAppRole(ctx context.Context, appID, displayName, value string) (string, error) {
	app, err := c.applicationByAppID(ctx, appID)
	if err != nil {
		return "", err
	}
	for _, r := range app.AppRoles {
		if r.Value == value {
			return r.ID, nil
		}
	}
	role := appRole{
		ID:                 uuid.NewString(),
		AllowedMemberTypes: []string{"User"},
		Description:        displayName,
		DisplayName:        displayName,
		IsEnabled:          true,
		Value:              value,
	}
	patch := map[string]any{"appRoles": append(app.AppRoles, role)}
	if err := c.do(ctx, http.MethodPatch, "/applications/"+app.ID, patch, nil); err != nil {
		return "", fmt.Errorf("patch appRoles: %w", err)
	}
	return role.ID, nil
}

// User is a directory user (member or guest).
type User struct {
	ID   string `json:"id"`
	UPN  string `json:"userPrincipalName"`
	Mail string `json:"mail"`
}

// UserByEmail resolves a user by UPN (direct lookup) and falls back to a mail
// filter for guests whose UPN differs from their email.
func (c *Client) UserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := c.do(ctx, http.MethodGet, "/users/"+url.PathEscape(email), nil, &u); err == nil && u.ID != "" {
		return &u, nil
	}
	var resp struct {
		Value []User `json:"value"`
	}
	path := "/users?$filter=" + url.QueryEscape("mail eq '"+email+"'")
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Value) == 0 {
		return nil, fmt.Errorf("no user for %s", email)
	}
	return &resp.Value[0], nil
}

// InviteGuest sends a B2B invitation and returns the created guest user.
func (c *Client) InviteGuest(ctx context.Context, email, redirectURL string) (*User, error) {
	if redirectURL == "" {
		redirectURL = "https://myapplications.microsoft.com"
	}
	in := map[string]any{
		"invitedUserEmailAddress": email,
		"inviteRedirectUrl":       redirectURL,
		"sendInvitationMessage":   true,
	}
	var resp struct {
		InvitedUser User `json:"invitedUser"`
	}
	if err := c.do(ctx, http.MethodPost, "/invitations", in, &resp); err != nil {
		return nil, err
	}
	resp.InvitedUser.Mail = email
	return &resp.InvitedUser, nil
}

// AssignAppRole grants a user the given app role on the enterprise app.
// Idempotent: an existing identical assignment is treated as success.
//
// An app role added to the *application* propagates to its *service principal*
// (where assignments are validated) with a short delay, so a freshly-created
// role can yield "Permission being assigned was not found on application". We
// retry with exponential backoff (no fixed sleep) until it propagates.
func (c *Client) AssignAppRole(ctx context.Context, spID, userID, appRoleID string) error {
	in := map[string]any{
		"principalId": userID,
		"resourceId":  spID,
		"appRoleId":   appRoleID,
	}
	delay := time.Second
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		err = c.do(ctx, http.MethodPost, "/servicePrincipals/"+spID+"/appRoleAssignedTo", in, nil)
		if err == nil {
			return nil
		}
		ge, ok := err.(*GraphError)
		if !ok || ge.Status != http.StatusBadRequest {
			return err
		}
		body := strings.ToLower(ge.Body)
		if strings.Contains(body, "already exists") {
			return nil // idempotent
		}
		if !strings.Contains(body, "not found on application") {
			return err // a real bad request, not propagation lag
		}
		// app role not yet synced to the SP - wait and retry
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		delay *= 2
	}
	return err
}
