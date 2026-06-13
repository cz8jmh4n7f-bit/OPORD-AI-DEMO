package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	credspkg "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// checkTimeout bounds a single connection probe so a hung backend can't wedge
// the request (and, later, a monitoring loop).
const checkTimeout = 15 * time.Second

// ProviderCheck is the result of a provider connectivity probe.
type ProviderCheck struct {
	Provider  string
	Type      string
	OK        bool
	Status    string // "ok" | "failed" | "unsupported"
	Message   string
	LatencyMs int
	CheckedAt time.Time
}

// CheckProviderConnection runs a live reachability + credential probe against a
// provider's backend and persists the result (so health is monitorable via
// GET /providers without re-probing on every read). Providers that don't
// implement the Connectivity capability return status "unsupported" - not an
// error - so the caller can still record and display that state.
func (s *Service) CheckProviderConnection(ctx context.Context, name string) (*ProviderCheck, error) {
	p, err := s.q.GetProviderByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found: %w", name, err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}

	res := &ProviderCheck{Provider: name, Type: p.Type}

	checker, ok := prov.(providers.Connectivity)
	if !ok {
		res.Status = "unsupported"
		res.Message = fmt.Sprintf("connection check not supported for %s providers", p.Type)
		s.persistHealth(ctx, p.ID, res)
		return res, nil
	}

	var cfg map[string]any
	_ = json.Unmarshal(p.Config, &cfg)
	// WithSecretWait: for Azure dynamic creds, a cold check settles the freshly
	// minted SP secret + warms the resolver cache, so this (and every later) check
	// is reliable. AWS/GCP/static resolve instantly (the flag is a no-op there).
	creds, _ := s.creds.Resolve(credspkg.WithSecretWait(ctx), p)

	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	start := time.Now()
	checkErr := checker.CheckConnection(cctx, creds, cfg)
	res.LatencyMs = int(time.Since(start).Milliseconds())
	if checkErr != nil {
		res.Status = "failed"
		res.Message = checkErr.Error()
	} else {
		res.OK = true
		res.Status = "ok"
		res.Message = "reachable; credentials valid"
	}

	s.persistHealth(ctx, p.ID, res)
	return res, nil
}

// CheckAllProviders probes every registered provider and persists each result.
// Returns counts for the periodic loop: checked (probe ran), unhealthy (status
// "failed"), and errored (couldn't run the probe at all). "unsupported"
// providers count as checked. A list error aborts the whole pass.
func (s *Service) CheckAllProviders(ctx context.Context) (checked, unhealthy, errored int, err error) {
	provs, err := s.q.ListProviders(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("listing providers: %w", err)
	}
	for _, p := range provs {
		res, cerr := s.CheckProviderConnection(ctx, p.Name)
		if cerr != nil {
			errored++
			continue
		}
		checked++
		if res.Status == "failed" {
			unhealthy++
		}
	}
	return checked, unhealthy, errored, nil
}

// persistHealth records the probe result on the provider row. A persistence
// failure is logged but doesn't fail the probe - the caller still gets the live
// result.
func (s *Service) persistHealth(ctx context.Context, id uuid.UUID, r *ProviderCheck) {
	out, err := s.q.UpdateProviderHealth(ctx, db.UpdateProviderHealthParams{
		ID:                 id,
		LastCheckStatus:    r.Status,
		LastCheckMessage:   r.Message,
		LastCheckLatencyMs: int32(r.LatencyMs),
	})
	if err != nil {
		s.log.Warn("persisting provider health failed", "provider", r.Provider, "err", err)
		return
	}
	if out.LastCheckAt.Valid {
		r.CheckedAt = out.LastCheckAt.Time
	}
}
