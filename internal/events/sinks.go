package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// --- audit sink: structured slog (always-on local audit trail) ---

type auditSink struct{ log *slog.Logger }

// NewAuditSink logs every event via slog. Cheap, dependency-free audit trail.
func NewAuditSink(log *slog.Logger) Sink {
	if log == nil {
		log = slog.Default()
	}
	return &auditSink{log: log}
}

func (a *auditSink) Name() string { return "audit" }

func (a *auditSink) Emit(_ context.Context, e Event) error {
	a.log.Info("event",
		"kind", e.Kind, "action", e.Action, "name", e.Name,
		"environment", e.Environment, "provider", e.Provider, "message", e.Message)
	return nil
}

// --- Slack sink: incoming webhook ---

type slackSink struct {
	webhook string
	client  *http.Client
}

// NewSlackSink posts events to a Slack incoming webhook.
func NewSlackSink(webhookURL string) Sink {
	return &slackSink{webhook: webhookURL, client: &http.Client{}}
}

func (s *slackSink) Name() string { return "slack" }

func (s *slackSink) Emit(ctx context.Context, e Event) error {
	emoji := actionEmoji(e.Action)
	text := fmt.Sprintf("%s *%s* `%s`", emoji, e.Kind, e.Name)
	if e.Environment != "" {
		text += fmt.Sprintf(" (%s)", e.Environment)
	}
	if e.Provider != "" {
		text += fmt.Sprintf(" on %s", e.Provider)
	}
	text += fmt.Sprintf(" - *%s*", e.Action)
	if e.Message != "" {
		text += "\n" + e.Message
	}
	body, _ := json.Marshal(map[string]string{"text": text})
	return postJSON(ctx, s.client, s.webhook, body)
}

func actionEmoji(action string) string {
	switch action {
	case "ready":
		return ":white_check_mark:"
	case "failed":
		return ":x:"
	case "destroyed":
		return ":wastebasket:"
	case "created", "provisioning":
		return ":rocket:"
	default:
		return ":information_source:"
	}
}

// --- SIEM sink: GELF-over-HTTP (Graylog) - also valid JSON for generic collectors ---

type siemSink struct {
	url    string
	host   string
	client *http.Client
}

// NewSIEMSink posts events as GELF JSON to an HTTP endpoint (e.g. Graylog GELF
// HTTP input). The payload is plain JSON, so generic HTTP log collectors work too.
func NewSIEMSink(url, host string) Sink {
	if host == "" {
		host = "opord"
	}
	return &siemSink{url: url, host: host, client: &http.Client{}}
}

func (s *siemSink) Name() string { return "siem" }

func (s *siemSink) Emit(ctx context.Context, e Event) error {
	short := fmt.Sprintf("%s %s: %s", e.Kind, e.Action, e.Name)
	gelf := map[string]any{
		"version":       "1.1",
		"host":          s.host,
		"short_message": short,
		"level":         gelfLevel(e.Action),
		"timestamp":     e.Time.Unix(),
		"_kind":         e.Kind,
		"_action":       e.Action,
		"_name":         e.Name,
		"_environment":  e.Environment,
		"_provider":     e.Provider,
	}
	if e.Message != "" {
		gelf["_message"] = e.Message
	}
	body, _ := json.Marshal(gelf)
	return postJSON(ctx, s.client, s.url, body)
}

// gelfLevel maps an action to a syslog severity (6=info, 3=error).
func gelfLevel(action string) int {
	if action == "failed" {
		return 3
	}
	return 6
}

func postJSON(ctx context.Context, client *http.Client, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s returned %d", url, resp.StatusCode)
	}
	return nil
}
