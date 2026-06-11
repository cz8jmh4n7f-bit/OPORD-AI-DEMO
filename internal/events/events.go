// Package events is OPORD's lightweight connector bus. The orchestrator
// publishes lifecycle events (a VM became ready, a cluster failed, ...) and the
// bus fans them out to registered sinks (audit log, Slack, SIEM, CMDB) without
// blocking or failing the operation that produced them.
package events

import (
	"context"
	"log/slog"
	"time"
)

// Event is a single lifecycle notification.
type Event struct {
	Time        time.Time      `json:"time"`
	Kind        string         `json:"kind"`        // vm | cluster | database | environment | stack
	Action      string         `json:"action"`      // created | ready | failed | destroyed | ...
	Name        string         `json:"name"`        // resource name
	Environment string         `json:"environment"` // dev | staging | ...
	Provider    string         `json:"provider"`    // provider name (when known)
	Message     string         `json:"message"`     // human-readable summary
	Fields      map[string]any `json:"fields,omitempty"`
}

// Sink consumes events for one destination (Slack, SIEM, ...).
type Sink interface {
	Name() string
	Emit(ctx context.Context, e Event) error
}

// Bus fans events out to its sinks. A nil *Bus is a valid no-op, so callers can
// hold an optional bus without nil checks at every call site.
type Bus struct {
	sinks []Sink
	log   *slog.Logger
}

// NewBus builds a bus. A nil logger defaults to slog.Default().
func NewBus(log *slog.Logger, sinks ...Sink) *Bus {
	if log == nil {
		log = slog.Default()
	}
	return &Bus{sinks: sinks, log: log}
}

// SinkConfig selects which connectors the bus fans out to. Empty values are
// skipped (the audit sink is always on).
type SinkConfig struct {
	SlackWebhookURL string
	SIEMURL         string
	GLPIURL         string
	GLPIAppToken    string
	GLPIUserToken   string
	GLPIItemType    string
}

// FromConfig builds a bus with the always-on audit sink plus any configured
// connectors (Slack webhook, SIEM HTTP, GLPI CMDB).
func FromConfig(c SinkConfig, log *slog.Logger) *Bus {
	sinks := []Sink{NewAuditSink(log)}
	if c.SlackWebhookURL != "" {
		sinks = append(sinks, NewSlackSink(c.SlackWebhookURL))
	}
	if c.SIEMURL != "" {
		sinks = append(sinks, NewSIEMSink(c.SIEMURL, "opord"))
	}
	if c.GLPIURL != "" && c.GLPIAppToken != "" && c.GLPIUserToken != "" {
		sinks = append(sinks, NewGLPISink(c.GLPIURL, c.GLPIAppToken, c.GLPIUserToken, c.GLPIItemType))
	}
	return NewBus(log, sinks...)
}

// Sinks returns the names of registered sinks (for startup logging).
func (b *Bus) Sinks() []string {
	if b == nil {
		return nil
	}
	out := make([]string, 0, len(b.sinks))
	for _, s := range b.sinks {
		out = append(out, s.Name())
	}
	return out
}

// Publish fans an event out to every sink asynchronously. It never blocks the
// caller and never returns an error: sink failures are logged, not propagated.
func (b *Bus) Publish(e Event) {
	if b == nil || len(b.sinks) == 0 {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	for _, s := range b.sinks {
		go func(s Sink) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.Emit(ctx, e); err != nil {
				b.log.Warn("event sink failed", "sink", s.Name(), "kind", e.Kind, "action", e.Action, "err", err)
			}
		}(s)
	}
}
