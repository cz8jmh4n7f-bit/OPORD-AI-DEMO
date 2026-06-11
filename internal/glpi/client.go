// Package glpi is a minimal GLPI REST API client (apirest.php): authenticate
// with an app token + user token, then add items (CMDB CIs, tickets). Used by
// the orchestrator (open a ticket for a request) and the events connector
// (create a CMDB CI when a resource is ready).
package glpi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Client talks to one GLPI instance.
type Client struct {
	url       string
	appToken  string
	userToken string
	http      *http.Client
}

// New builds a client. baseURL is the GLPI root (e.g. http://localhost:8081).
func New(baseURL, appToken, userToken string) *Client {
	return &Client{url: baseURL, appToken: appToken, userToken: userToken, http: &http.Client{}}
}

func (c *Client) api(path string) string { return c.url + "/apirest.php" + path }

// session opens a GLPI session and returns the session token.
func (c *Client) session(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api("/initSession"), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Authorization", "user_token "+c.userToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("glpi initSession returned %d", resp.StatusCode)
	}
	var out struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.SessionToken, nil
}

func (c *Client) kill(ctx context.Context, session string) {
	if session == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api("/killSession"), nil)
	if err != nil {
		return
	}
	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Session-Token", session)
	if resp, err := c.http.Do(req); err == nil {
		_ = resp.Body.Close()
	}
}

// AddItem creates a GLPI item of the given itemtype (e.g. "Computer", "Ticket")
// and returns its id.
func (c *Client) AddItem(ctx context.Context, itemType string, input map[string]any) (int, error) {
	session, err := c.session(ctx)
	if err != nil {
		return 0, err
	}
	defer c.kill(ctx, session)

	body, _ := json.Marshal(map[string]any{"input": input})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api("/"+itemType), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Session-Token", session)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("glpi add %s returned %d", itemType, resp.StatusCode)
	}
	var out struct {
		ID int `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.ID, nil
}

// CreateTicket opens a GLPI ticket and returns its id as a string.
func (c *Client) CreateTicket(ctx context.Context, title, content string) (string, error) {
	id, err := c.AddItem(ctx, "Ticket", map[string]any{"name": title, "content": content})
	if err != nil {
		return "", err
	}
	return strconv.Itoa(id), nil
}
