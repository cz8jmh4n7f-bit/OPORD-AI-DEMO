package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/ansible"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// clusterSpecOf decodes a cluster's stored desired spec.
func clusterSpecOf(c db.Cluster) models.ClusterSpec {
	var s models.ClusterSpec
	_ = json.Unmarshal(c.DesiredSpec, &s)
	return s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// provisionClusterAsync runs the full cluster delivery on its own background
// context: Phase 1 (provider.Provision == tofu apply) records the nodes, then
// Phase 2 (Ansible kubeadm bootstrap) brings up Kubernetes and fetches the
// kubeconfig. Status flows provisioning -> bootstrapping -> ready/failed and the
// provision job is marked running -> succeeded/failed.
// startProvisionCluster hands cluster provisioning to the job queue when
// configured, otherwise runs it in-process. Both converge on ProvisionClusterByID.
func (s *Service) startProvisionCluster(clusterID, jobID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionCluster(context.Background(), clusterID, jobID); err != nil {
			s.log.Error("enqueue provision_cluster failed; running in-process", "id", clusterID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		_ = s.ProvisionClusterByID(ctx, clusterID, jobID)
	}()
}

// startScaleCluster runs the same re-apply as a provision but enqueues a
// scale_cluster job, so the queue shows the operation as a scale, not a create.
// Falls back to an in-process apply (CLI, no enqueuer) like startProvisionCluster.
func (s *Service) startScaleCluster(clusterID, jobID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueScaleCluster(context.Background(), clusterID, jobID); err != nil {
			s.log.Error("enqueue scale_cluster failed; running in-process", "id", clusterID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		_ = s.ProvisionClusterByID(ctx, clusterID, jobID)
	}()
}

// ProvisionClusterByID loads a cluster + its provider from the database and runs
// the full delivery: Phase 1 (provider.Provision == tofu apply) records nodes,
// then Phase 2 (Ansible kubeadm bootstrap) and fetches the kubeconfig. Loaded by
// id so it runs from either a goroutine or a River worker. Status flows
// provisioning -> bootstrapping -> ready/failed; the job is marked accordingly.
func (s *Service) ProvisionClusterByID(ctx context.Context, clusterID, jobID uuid.UUID) error {
	c, err := s.q.GetCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("loading cluster: %w", err)
	}
	p, err := s.q.GetProvider(ctx, c.ProviderID)
	if err != nil {
		s.failCluster(ctx, c.ID, jobID, fmt.Errorf("provider lookup: %w", err))
		return err
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		s.failCluster(ctx, c.ID, jobID, err)
		return err
	}
	cfg := s.providerCfg(ctx, p)
	spec := clusterSpecOf(c)
	// AWS clusters (EKS) ALWAYS use the assumed_role factory creds: EKS creates IAM
	// roles, and federation_token creds can't call IAM at all. resolveClusterCreds
	// forces the assumed_role path for AWS; no-op for GCP/Azure (config override).
	creds, _ := s.resolveClusterCreds(ctx, p, spec.TargetAccount)

	_, _ = s.q.MarkJobRunning(ctx, db.MarkJobRunningParams{ID: jobID})
	_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "provisioning"})
	s.log.Info("cluster provisioning started", "name", c.Name, "workspace", c.TofuWorkspace)

	req := providers.Request{Workspace: c.TofuWorkspace, Name: c.Name, Spec: spec, Credentials: creds, Config: cfg}
	pr, err := prov.Provision(ctx, req)
	if err != nil {
		s.failCluster(ctx, c.ID, jobID, fmt.Errorf("phase 1 (provision) failed: %w", err))
		s.emit("cluster", "failed", c.Name, c.Environment, p.Name, err.Error())
		return err
	}

	// Record the nodes the provider created.
	for _, n := range pr.Nodes {
		_, _ = s.q.UpsertNode(ctx, db.UpsertNodeParams{
			ClusterID: c.ID,
			Name:      n.Name,
			Role:      string(n.Role),
			IpAddress: strPtr(n.IPAddress),
			VmMoid:    strPtr(n.VMMoid),
			Status:    "provisioned",
		})
	}
	observed, _ := json.Marshal(pr)
	_, _ = s.q.UpdateClusterState(ctx, db.UpdateClusterStateParams{ID: c.ID, ObservedState: observed})

	kubeconfig := pr.Kubeconfig
	if pr.Managed {
		// Managed control plane (e.g. EKS): no SSH nodes to kubeadm-bootstrap.
		s.log.Info("managed control plane; skipping ansible bootstrap", "name", c.Name)
	} else {
		// Phase 2: Kubernetes bootstrap via Ansible.
		_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "bootstrapping"})
		s.log.Info("cluster bootstrap started", "name", c.Name, "nodes", len(pr.Nodes))

		kubeconfig, err = s.bootstrapCluster(ctx, c, pr)
		if err != nil {
			s.failCluster(ctx, c.ID, jobID, fmt.Errorf("phase 2 (bootstrap) failed: %w", err))
			s.emit("cluster", "failed", c.Name, c.Environment, p.Name, err.Error())
			return err
		}

		// Mark nodes ready.
		for _, n := range pr.Nodes {
			_, _ = s.q.UpsertNode(ctx, db.UpsertNodeParams{
				ClusterID: c.ID,
				Name:      n.Name,
				Role:      string(n.Role),
				IpAddress: strPtr(n.IPAddress),
				VmMoid:    strPtr(n.VMMoid),
				Status:    "ready",
			})
		}
	}

	_, _ = s.q.UpdateClusterState(ctx, db.UpdateClusterStateParams{ID: c.ID, ObservedState: observed, KubeconfigRef: strPtr(kubeconfig)})
	_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "ready"})
	_, _ = s.q.MarkJobFinished(ctx, db.MarkJobFinishedParams{ID: jobID, Status: "succeeded"})
	s.log.Info("cluster ready", "name", c.Name, "kubeconfig", kubeconfig, "managed", pr.Managed)
	s.emit("cluster", "ready", c.Name, c.Environment, p.Name, "endpoint "+pr.ControlPlaneEndpoint)
	return nil
}

// bootstrapCluster runs the provider-agnostic Phase 2 (ansible-playbook
// site.yml) against the inventory the provider emitted, and fetches the admin
// kubeconfig back to ArtifactsDir. Returns the local kubeconfig path.
func (s *Service) bootstrapCluster(ctx context.Context, c db.Cluster, pr *providers.ProvisionResult) (string, error) {
	if s.bootstrap.AnsibleDir == "" {
		return "", fmt.Errorf("ansible dir not configured (set ANSIBLE_DIR)")
	}
	if pr.AnsibleInventory == "" {
		return "", fmt.Errorf("provider returned an empty ansible inventory")
	}

	invPath, cleanup, err := ansible.WriteInventory(pr.AnsibleInventory)
	if err != nil {
		return "", err
	}
	defer cleanup()

	artifacts := s.bootstrap.ArtifactsDir
	if artifacts == "" {
		artifacts = os.TempDir()
	}
	if err := os.MkdirAll(artifacts, 0o700); err != nil {
		return "", fmt.Errorf("creating artifacts dir: %w", err)
	}
	kubeconfigPath := filepath.Join(artifacts, fmt.Sprintf("%s-%s.kubeconfig", c.Name, c.Environment))

	runner := ansible.New(s.bootstrap.AnsibleBin, s.bootstrap.AnsibleDir, s.log)
	out, err := runner.Playbook(ctx, ansible.Options{
		Playbook:      "playbooks/site.yml",
		InventoryPath: invPath,
		PrivateKey:    s.bootstrap.SSHPrivateKey,
		ExtraVars:     map[string]string{"kubeconfig_local_path": kubeconfigPath},
	})
	if err != nil {
		return "", err
	}
	s.log.Debug("ansible bootstrap output", "name", c.Name, "tail", lastLines(out, 20))

	if _, statErr := os.Stat(kubeconfigPath); statErr != nil {
		// Bootstrap succeeded but the kubeconfig was not fetched; not fatal.
		s.log.Warn("kubeconfig not found after bootstrap", "name", c.Name, "path", kubeconfigPath)
		return "", nil
	}
	return kubeconfigPath, nil
}

// failCluster records a failed cluster outcome (status + job) with the error.
// Finding E: the cause is also persisted into observed_state (`{"error": …}`) so
// the web/API can surface WHY the cluster failed, not just status="failed".
func (s *Service) failCluster(ctx context.Context, clusterID, jobID uuid.UUID, err error) {
	s.log.Error("cluster provisioning failed", "id", clusterID, "err", err)
	if obs, mErr := json.Marshal(map[string]string{"error": err.Error()}); mErr == nil {
		_, _ = s.q.UpdateClusterState(ctx, db.UpdateClusterStateParams{ID: clusterID, ObservedState: obs})
	}
	_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: clusterID, Status: "failed"})
	msg := err.Error()
	_, _ = s.q.MarkJobFinished(ctx, db.MarkJobFinishedParams{ID: jobID, Status: "failed", Error: &msg})
}

// DestroyCluster tears down a cluster (provider.Destroy == tofu destroy),
// removes its node rows, and marks it destroyed. Synchronous: callers that need
// to return immediately should use DestroyClusterAsync.
func (s *Service) DestroyCluster(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	c, err := s.q.GetClusterByName(ctx, db.GetClusterByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cluster %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(c.TenantID, tid) {
		return fmt.Errorf("cluster %q (env %q) not found", name, env)
	}
	p, err := s.q.GetProvider(ctx, c.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}

	cfg := s.providerCfg(ctx, p)
	spec := clusterSpecOf(c)
	// AWS clusters (EKS) ALWAYS use the assumed_role factory creds (federation_token
	// can't call IAM, which EKS needs); no-op for GCP/Azure or non-AWS.
	creds, err := s.resolveClusterCreds(ctx, p, spec.TargetAccount)
	if err != nil {
		return fmt.Errorf("resolving credentials: %w", err)
	}

	job, err := s.q.CreateJob(ctx, db.CreateJobParams{
		ClusterID: pgtype.UUID{Bytes: [16]byte(c.ID), Valid: true},
		Operation: "destroy",
	})
	if err != nil {
		return fmt.Errorf("creating destroy job: %w", err)
	}
	_, _ = s.q.MarkJobRunning(ctx, db.MarkJobRunningParams{ID: job.ID})
	_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "destroying"})
	s.log.Info("cluster destroy started", "name", c.Name)

	req := providers.Request{Workspace: c.TofuWorkspace, Name: c.Name, Spec: clusterSpecOf(c), Credentials: creds, Config: cfg}
	if err := prov.Destroy(ctx, req); err != nil {
		s.failCluster(ctx, c.ID, job.ID, fmt.Errorf("destroy failed: %w", err))
		return fmt.Errorf("cluster destroy failed: %w", err)
	}

	// Durable destroy-guard (ClusterReaper): a provision killed mid-apply can create
	// the managed cluster WITHOUT writing tofu state, so the destroy above was a
	// silent no-op against an empty state - verify the cloud is actually clean and
	// force-delete an orphan. On a (retryable) error keep the status "destroying" so
	// the durable job retries until the cloud confirms it gone - do NOT mark destroyed.
	if reaper, ok := prov.(providers.ClusterReaper); ok {
		if err := reaper.ReapCluster(ctx, req); err != nil {
			s.log.Warn("cluster destroy-guard: orphan check failed, retrying", "cluster", c.Name, "err", err)
			return fmt.Errorf("cluster destroy verify: %w", err)
		}
	}

	_ = s.q.DeleteNodesByCluster(ctx, c.ID)
	_, _ = s.q.UpdateClusterStatus(ctx, db.UpdateClusterStatusParams{ID: c.ID, Status: "destroyed"})
	_, _ = s.q.MarkJobFinished(ctx, db.MarkJobFinishedParams{ID: job.ID, Status: "succeeded"})
	s.log.Info("cluster destroy complete", "name", c.Name)
	s.emit("cluster", "destroyed", c.Name, c.Environment, p.Name, "")
	return nil
}

// DestroyClusterAsync enqueues a destroy job when a queue is configured,
// otherwise runs DestroyCluster in-process. Used by the HTTP API so the request
// returns immediately; progress shows via the cluster status (destroying ->
// destroyed/failed).
// DeleteClusterRecord forgets a terminal cluster's tracking row (no tofu/Ansible).
// Allowed only for destroyed/failed - destroy a live cluster first.
func (s *Service) DeleteClusterRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	c, err := s.q.GetClusterByName(ctx, db.GetClusterByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cluster %q (env %q) not found: %w", name, env, err)
	}
	switch c.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("cluster %q is %s - destroy it before removing the record", name, c.Status)
	}
	if err := s.q.DeleteCluster(ctx, c.ID); err != nil {
		return fmt.Errorf("removing cluster record %q: %w", name, err)
	}
	s.log.Info("cluster record removed", "name", name)
	return nil
}

func (s *Service) DestroyClusterAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyCluster(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_cluster failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		if err := s.DestroyCluster(ctx, name, env); err != nil {
			s.log.Error("async cluster destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// lastLines returns the trailing n lines of s (for compact log tails).
func lastLines(s string, n int) string {
	lines := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			lines++
			if lines > n {
				return s[i+1:]
			}
		}
	}
	return s
}
