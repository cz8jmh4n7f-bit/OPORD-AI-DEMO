package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateClusterInput is the structured request for creating a cluster (built by
// the CLI from a YAML spec, by the API from a request body).
type CreateClusterInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.ClusterSpec
	DryRun      bool
}

// CreateClusterResult reports the outcome. For a dry run, Preflight is set and
// nothing was persisted. Otherwise Cluster and JobID identify the new records.
type CreateClusterResult struct {
	DryRun    bool
	Preflight *providers.PreflightResult
	Cluster   *db.Cluster
	JobID     uuid.UUID
}

// CreateCluster validates a spec against its provider and either (dry run)
// runs an offline preflight without persisting, or persists the desired state
// plus a provision job. Live provisioning (Tofu apply + Ansible) is a later
// step and requires a reachable provider API.
func (s *Service) CreateCluster(ctx context.Context, in CreateClusterInput) (*CreateClusterResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("cluster name and provider are required")
	}
	env := in.Environment
	if env == "" {
		env = "dev"
	}

	p, err := s.q.GetProviderByName(ctx, in.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found (register it with `opord provider add`): %w", in.Provider, err)
	}

	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	if err := prov.Validate(ctx, in.Spec); err != nil {
		return nil, err
	}

	providerConfig := s.providerCfg(ctx, p)
	creds, err := s.creds.Resolve(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("resolving credentials: %w", err)
	}

	if in.DryRun {
		req := providers.Request{
			Workspace:   fmt.Sprintf("%s-%s", env, in.Name),
			Name:        in.Name,
			Spec:        in.Spec,
			Credentials: creds,
			Config:      providerConfig,
		}
		pf, err := prov.Preflight(ctx, req)
		if err != nil {
			return nil, err
		}
		s.log.Info("preflight ok", "cluster", in.Name, "provider", in.Provider)
		return &CreateClusterResult{DryRun: true, Preflight: pf}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling spec: %w", err)
	}
	cluster, err := s.q.CreateCluster(ctx, db.CreateClusterParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		DesiredSpec:   specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating cluster: %w", err)
	}
	job, err := s.q.CreateJob(ctx, db.CreateJobParams{
		ClusterID: pgtype.UUID{Bytes: [16]byte(cluster.ID), Valid: true},
		Operation: "provision",
	})
	if err != nil {
		return nil, fmt.Errorf("creating provision job: %w", err)
	}
	s.log.Info("cluster registered", "name", cluster.Name, "id", cluster.ID, "status", cluster.Status)
	s.emit("cluster", "created", cluster.Name, env, in.Provider, fmt.Sprintf("k8s v%s, %d cp / %d workers", in.Spec.KubernetesVersion, in.Spec.ControlPlane.Count, in.Spec.Workers.Count))

	// Provision in the background (tofu apply + Ansible can take many minutes):
	// status flows pending -> provisioning -> bootstrapping -> ready/failed; the
	// caller returns immediately with the job id to poll.
	s.startProvisionCluster(cluster.ID, job.ID)

	return &CreateClusterResult{Cluster: &cluster, JobID: job.ID}, nil
}

// ScaleCluster changes a cluster's worker count and re-provisions (tofu apply
// adds/removes worker VMs; Ansible joins new ones - idempotent). Day-2 op.
// Note: scale-down removes the VM but leaves the k8s node object (needs a manual
// `kubectl delete node`); scale-up joins cleanly.
func (s *Service) ScaleCluster(ctx context.Context, name, env string, workers int) error {
	if env == "" {
		env = "dev"
	}
	if workers < 1 {
		return fmt.Errorf("worker count must be >= 1")
	}
	c, err := s.q.GetClusterByName(ctx, db.GetClusterByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cluster %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(c.TenantID, tid) {
		return fmt.Errorf("cluster %q (env %q) not found", name, env)
	}
	spec := clusterSpecOf(c)
	if spec.Workers.Count == workers {
		return nil
	}
	spec.Workers.Count = workers
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling cluster spec: %w", err)
	}
	if _, err := s.q.UpdateClusterSpec(ctx, db.UpdateClusterSpecParams{ID: c.ID, DesiredSpec: specJSON}); err != nil {
		return fmt.Errorf("updating cluster spec: %w", err)
	}
	job, err := s.q.CreateJob(ctx, db.CreateJobParams{
		ClusterID: pgtype.UUID{Bytes: [16]byte(c.ID), Valid: true},
		Operation: "reconcile",
	})
	if err != nil {
		return fmt.Errorf("creating scale job: %w", err)
	}
	_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "provisioning"})
	s.log.Info("cluster scaling", "name", c.Name, "workers", workers)
	s.emit("cluster", "scaling", c.Name, c.Environment, "", fmt.Sprintf("workers to %d", workers))
	s.startScaleCluster(c.ID, job.ID)
	return nil
}
