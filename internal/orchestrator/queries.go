package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// ClusterSummary is a cluster enriched for list views.
type ClusterSummary struct {
	Cluster       db.Cluster
	Provider      string
	ProviderType  string
	ControlPlanes int
	Workers       int
}

// ClusterDetail is a cluster with its provider, spec, nodes, and jobs.
type ClusterDetail struct {
	Cluster      db.Cluster
	Provider     string
	ProviderType string
	Spec         models.ClusterSpec
	Nodes        []db.Node
	Jobs         []db.Job
	LiveVMs      []providers.LiveNode
	LiveError    string
}

func specOf(c db.Cluster) models.ClusterSpec {
	var s models.ClusterSpec
	_ = json.Unmarshal(c.DesiredSpec, &s)
	return s
}

// ListClusters returns all clusters with provider name and node counts.
func (s *Service) ListClusters(ctx context.Context) ([]ClusterSummary, error) {
	clusters, err := s.q.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing clusters: %w", err)
	}
	provs, err := s.q.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing providers: %w", err)
	}
	names := make(map[uuid.UUID]string, len(provs))
	ptypes := make(map[uuid.UUID]string, len(provs))
	for _, p := range provs {
		names[p.ID] = p.Name
		ptypes[p.ID] = p.Type
	}

	tid, scoped := scopeTenant(ctx)
	out := make([]ClusterSummary, 0, len(clusters))
	for _, c := range clusters {
		if scoped && !tenantVisible(c.TenantID, tid) {
			continue
		}
		spec := specOf(c)
		out = append(out, ClusterSummary{
			Cluster:       c,
			Provider:      names[c.ProviderID],
			ProviderType:  ptypes[c.ProviderID],
			ControlPlanes: spec.ControlPlane.Count,
			Workers:       spec.Workers.Count,
		})
	}
	return out, nil
}

// ClusterStatus returns the full detail of one cluster (by name + environment).
func (s *Service) ClusterStatus(ctx context.Context, name, env string, live bool) (*ClusterDetail, error) {
	if env == "" {
		env = "dev"
	}
	c, err := s.q.GetClusterByName(ctx, db.GetClusterByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("cluster %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(c.TenantID, tid) {
		return nil, fmt.Errorf("cluster %q (env %q) not found", name, env)
	}

	detail := &ClusterDetail{Cluster: c, Spec: specOf(c)}

	provRec, provErr := s.q.GetProvider(ctx, c.ProviderID)
	if provErr == nil {
		detail.Provider = provRec.Name
		detail.ProviderType = provRec.Type
	}

	nodes, err := s.q.ListNodesByCluster(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	detail.Nodes = nodes

	jobs, err := s.q.ListJobsByCluster(ctx, pgtype.UUID{Bytes: [16]byte(c.ID), Valid: true})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	detail.Jobs = jobs

	// Best-effort live VM state from the provider's API (opt-in). Time-boxed so
	// an unreachable vCenter never hangs a status read.
	if live && provErr == nil {
		if prov, gerr := s.registry.Get(models.ProviderType(provRec.Type)); gerr == nil {
			if insp, ok := prov.(providers.Inspector); ok {
				var cfg map[string]any
				_ = json.Unmarshal(provRec.Config, &cfg)
				creds, _ := s.creds.Resolve(ctx, provRec)
				ictx, cancel := context.WithTimeout(ctx, 6*time.Second)
				vms, ierr := insp.InspectVMs(ictx, providers.Request{Spec: detail.Spec, Credentials: creds, Config: cfg})
				cancel()
				if ierr != nil {
					detail.LiveError = ierr.Error()
				} else {
					detail.LiveVMs = vms
				}
			}
		}
	}

	return detail, nil
}
