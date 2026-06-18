// Package orchestrator holds OPORD's AI governance lifecycle logic. It is the
// single reusable home for provider, request/approval, policy, quota, budget,
// gateway, and audit behavior, so the HTTP API and the CLI both drive it through
// the same code path (no duplicated logic).
package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/events"
)

// CredentialResolver fetches credentials and non-secret config for a provider
// record. Credentials live in OpenBao (referenced by SecretRef) with an env
// fallback; the resolver also exposes raw secret reads used by the AI domain.
type CredentialResolver interface {
	Resolve(ctx context.Context, p db.Provider) (map[string]string, error)
	ResolveConfig(ctx context.Context, p db.Provider) (map[string]any, error)
}

// Service coordinates the AI governance lifecycle over the database, the AI
// provider registry, and credential resolution.
type Service struct {
	q        db.Querier
	creds    CredentialResolver
	log      *slog.Logger
	events   *events.Bus
	ticketer Ticketer
	ai       *aiproviders.Registry
}

// New constructs a Service. A nil logger defaults to slog.Default().
func New(q db.Querier, creds CredentialResolver, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{q: q, creds: creds, log: log}
}

// SetEvents wires the connector bus (audit / Slack / SIEM notifications). Optional.
func (s *Service) SetEvents(b *events.Bus) { s.events = b }

// Ticketer opens an ITSM ticket for a self-service request. Optional - requests
// work without it, just with no ticket.
type Ticketer interface {
	CreateTicket(ctx context.Context, title, content string) (string, error)
}

// SetTicketer wires the ITSM ticket backend used by the request workflow.
func (s *Service) SetTicketer(t Ticketer) { s.ticketer = t }

// SetAIProviders wires the AI provider registry.
func (s *Service) SetAIProviders(r *aiproviders.Registry) { s.ai = r }

// providerCfg returns a provider's effective config: the DB config merged with
// any non-credential keys stored at its OpenBao SecretRef (SecretRef keys win).
func (s *Service) providerCfg(ctx context.Context, p db.Provider) map[string]any {
	cfg := map[string]any{}
	_ = json.Unmarshal(p.Config, &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	if s.creds != nil {
		if vc, err := s.creds.ResolveConfig(ctx, p); err == nil {
			for k, v := range vc {
				cfg[k] = v
			}
		}
	}
	return cfg
}

// emit publishes a lifecycle event to the connector bus (no-op if unset).
func (s *Service) emit(kind, action, name, env, provider, message string) {
	if s.events == nil {
		return
	}
	s.events.Publish(events.Event{
		Kind: kind, Action: action, Name: name,
		Environment: env, Provider: provider, Message: message,
	})
}

// firstNonEmpty returns the first non-empty string in vals, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
