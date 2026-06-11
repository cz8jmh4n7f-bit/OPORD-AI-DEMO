package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// ProviderInput describes a provider instance to register.
type ProviderInput struct {
	Name      string
	Type      string
	Config    map[string]any // non-secret connection config (datacenter, datastore, ...)
	SecretRef string         // Vault path holding credentials (recorded for later use)
}

// AddProvider validates and persists a provider instance.
func (s *Service) AddProvider(ctx context.Context, in ProviderInput) (db.Provider, error) {
	if in.Name == "" {
		return db.Provider{}, fmt.Errorf("provider name is required")
	}
	switch models.ProviderType(in.Type) {
	case models.ProviderVSphere, models.ProviderProxmox, models.ProviderAWS, models.ProviderAzure, models.ProviderGCP:
	default:
		return db.Provider{}, fmt.Errorf("unsupported provider type %q (want vsphere, proxmox, aws, azure, or gcp)", in.Type)
	}

	cfg := in.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return db.Provider{}, fmt.Errorf("marshaling provider config: %w", err)
	}

	p, err := s.q.CreateProvider(ctx, db.CreateProviderParams{
		Name:      in.Name,
		Type:      in.Type,
		Config:    cfgJSON,
		SecretRef: in.SecretRef,
	})
	if err != nil {
		return db.Provider{}, fmt.Errorf("creating provider %q: %w", in.Name, err)
	}
	s.log.Info("provider registered", "name", p.Name, "type", p.Type)
	s.emit("provider", "registered", p.Name, "", p.Type, fmt.Sprintf("registered %s provider %q", p.Type, p.Name))
	return p, nil
}

// ListProviders returns all registered provider instances.
func (s *Service) ListProviders(ctx context.Context) ([]db.Provider, error) {
	return s.q.ListProviders(ctx)
}

// ProviderUpdate carries changes for an existing provider. Config keys are merged
// into the existing config (only the provided keys change); a nil SecretRef
// leaves the credential reference unchanged.
type ProviderUpdate struct {
	Name      *string
	Type      *string
	Config    map[string]any
	SecretRef *string
}

func validProviderType(t string) bool {
	switch models.ProviderType(t) {
	case models.ProviderVSphere, models.ProviderProxmox, models.ProviderAWS, models.ProviderAzure, models.ProviderGCP:
		return true
	default:
		return false
	}
}

// UpdateProvider merges metadata, config changes, and an optional secret ref
// into a provider. Type changes are refused once resources reference the
// provider; changing implementations under live state is too easy to make
// destructive.
func (s *Service) UpdateProvider(ctx context.Context, name string, in ProviderUpdate) (db.Provider, error) {
	p, err := s.q.GetProviderByName(ctx, name)
	if err != nil {
		return db.Provider{}, fmt.Errorf("provider %q not found: %w", name, err)
	}

	nextName := p.Name
	if in.Name != nil {
		nextName = *in.Name
	}
	if nextName == "" {
		return db.Provider{}, fmt.Errorf("provider name is required")
	}

	nextType := p.Type
	typeChanged := false
	if in.Type != nil {
		nextType = *in.Type
		typeChanged = nextType != p.Type
	}
	if !validProviderType(nextType) {
		return db.Provider{}, fmt.Errorf("unsupported provider type %q (want vsphere, proxmox, aws, azure, or gcp)", nextType)
	}
	if typeChanged {
		clusterRefs, err := s.q.CountClustersByProvider(ctx, p.ID)
		if err != nil {
			return db.Provider{}, fmt.Errorf("checking provider cluster references: %w", err)
		}
		resourceRefs, err := s.q.CountResourcesByProvider(ctx, p.ID)
		if err != nil {
			return db.Provider{}, fmt.Errorf("checking provider resource references: %w", err)
		}
		if clusterRefs+resourceRefs > 0 {
			return db.Provider{}, fmt.Errorf("provider %q has %d referenced resources; destroy or move them before changing provider type", name, clusterRefs+resourceRefs)
		}
	}

	cfg := map[string]any{}
	if len(p.Config) > 0 && !typeChanged {
		_ = json.Unmarshal(p.Config, &cfg)
	}
	for k, v := range in.Config {
		cfg[k] = v
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return db.Provider{}, fmt.Errorf("marshaling provider config: %w", err)
	}

	secretRef := p.SecretRef
	if in.SecretRef != nil {
		secretRef = *in.SecretRef
	}

	out, err := s.q.UpdateProvider(ctx, db.UpdateProviderParams{
		ID:        p.ID,
		Name:      nextName,
		Type:      nextType,
		Config:    cfgJSON,
		SecretRef: secretRef,
	})
	if err != nil {
		return db.Provider{}, fmt.Errorf("updating provider %q: %w", name, err)
	}
	s.log.Info("provider updated", "name", name, "next_name", out.Name, "type", out.Type)
	s.emit("provider", "updated", out.Name, "", out.Type, providerChangeSummary(p, in))
	return out, nil
}

// providerChangeSummary describes what an UpdateProvider call changed, for the
// audit-event message - the durable record of provider config edits (incl.
// renames, the gap a user hit when a rename left no old to new trace). Config
// VALUES and the secret-ref value are deliberately omitted (only keys / "changed")
// so nothing sensitive reaches the audit/SIEM sinks.
func providerChangeSummary(old db.Provider, in ProviderUpdate) string {
	var parts []string
	if in.Name != nil && *in.Name != old.Name {
		parts = append(parts, fmt.Sprintf("renamed %q to %q", old.Name, *in.Name))
	}
	if in.Type != nil && *in.Type != old.Type {
		parts = append(parts, fmt.Sprintf("type %s to %s", old.Type, *in.Type))
	}
	if len(in.Config) > 0 {
		keys := make([]string, 0, len(in.Config))
		for k := range in.Config {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts = append(parts, "config["+strings.Join(keys, ",")+"]")
	}
	if in.SecretRef != nil && *in.SecretRef != old.SecretRef {
		parts = append(parts, "secret_ref changed")
	}
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, "; ")
}

// DeleteProvider removes a provider. The schema uses `on delete restrict`, so a
// provider still referenced by clusters or resources is refused with a clear
// message rather than orphaning state.
func (s *Service) DeleteProvider(ctx context.Context, name string) error {
	p, err := s.q.GetProviderByName(ctx, name)
	if err != nil {
		return fmt.Errorf("provider %q not found: %w", name, err)
	}
	if err := s.q.DeleteProvider(ctx, p.ID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("provider %q still has clusters or resources - destroy them first", name)
		}
		return fmt.Errorf("deleting provider %q: %w", name, err)
	}
	s.log.Info("provider deleted", "name", name)
	s.emit("provider", "deleted", name, "", p.Type, fmt.Sprintf("deregistered %s provider %q", p.Type, name))
	return nil
}
