package orchestrator

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// ReconcileReport summarizes one drift-detection pass.
type ReconcileReport struct {
	Checked int
	Drifted int
	Errored int
}

// ReconcileClusters runs a `tofu plan` (provider.Plan) against every ready or
// degraded cluster to detect drift between desired and live state. Drift flips
// a cluster to "degraded"; a clean plan flips a previously-degraded cluster back
// to "ready". It never mutates infrastructure - detection only.
func (s *Service) ReconcileClusters(ctx context.Context) (ReconcileReport, error) {
	var rep ReconcileReport
	clusters, err := s.q.ListClusters(ctx)
	if err != nil {
		return rep, err
	}
	for _, c := range clusters {
		if c.Status != "ready" && c.Status != "degraded" {
			continue
		}
		p, err := s.q.GetProvider(ctx, c.ProviderID)
		if err != nil {
			rep.Errored++
			continue
		}
		// Managed control planes (EKS/AKS/GKE) auto-upgrade their version and own
		// their node pools, so `tofu plan` almost always shows a benign diff that is
		// NOT real drift. Flagging them "degraded" is a false alarm (the cloud owns
		// reconciliation), so skip the plan entirely - and clear any stale degraded
		// flag a previous run may have set.
		if models.IsManagedK8s(p.Type) {
			if c.Status == "degraded" {
				_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "ready"})
				s.log.Info("managed cluster: clearing false drift flag - back to ready", "cluster", c.Name)
			}
			continue
		}
		prov, err := s.registry.Get(models.ProviderType(p.Type))
		if err != nil {
			rep.Errored++
			continue
		}
		cfg := s.providerCfg(ctx, p)
		creds, _ := s.creds.Resolve(ctx, p)

		plan, err := prov.Plan(ctx, providers.Request{
			Workspace:   c.TofuWorkspace,
			Name:        c.Name,
			Spec:        clusterSpecOf(c),
			Credentials: creds,
			Config:      cfg,
		})
		if err != nil {
			s.log.Warn("reconcile plan failed", "cluster", c.Name, "err", err)
			rep.Errored++
			continue
		}
		rep.Checked++

		switch {
		case plan.HasChanges:
			rep.Drifted++
			if c.Status != "degraded" {
				_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "degraded"})
				s.log.Info("cluster drift detected - marking degraded", "cluster", c.Name)
			}
		case c.Status == "degraded":
			_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "ready"})
			s.log.Info("cluster drift resolved - back to ready", "cluster", c.Name)
		}
	}
	return rep, nil
}
