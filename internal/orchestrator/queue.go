package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
)

// CreateQueueInput is the request to provision a message queue.
type CreateQueueInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.QueueSpec
	DryRun      bool
}

// CreateQueueResult reports the outcome (dry-run summary, or persisted resource).
type CreateQueueResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// QueueSummary is a queue resource enriched for list/detail views.
type QueueSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.QueueSpec
}

func queueSpecOf(r db.Resource) models.QueueSpec {
	var s models.QueueSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

// Queue names: SQS allows alphanumeric, hyphens, underscores, up to 80 chars
// (the module appends .fifo for FIFO). Azure Service Bus names are sanitised
// by the provider before apply.
var queueNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

func validateQueueSpec(spec models.QueueSpec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	var errs []string
	if !queueNameRe.MatchString(name) {
		errs = append(errs, "name must be 1-80 chars of letters, numbers, hyphens, or underscores")
	}
	if spec.DLQMaxReceiveCount < 0 {
		errs = append(errs, "dlq_max_receive_count must be >= 0")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid queue spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateQueue validates a queue spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing
// QueueProvisioner.
func (s *Service) CreateQueue(ctx context.Context, in CreateQueueInput) (*CreateQueueResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("queue name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateQueueSpec(in.Spec, in.Name); err != nil {
		return nil, err
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
	qp, ok := prov.(providers.QueueProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support message queues", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := qp.PreflightQueue(ctx, providers.QueueRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("queue preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; message queue %q on %s", in.Spec.Name, in.Provider)
		s.log.Info("queue preflight ok", "name", in.Spec.Name, "provider", in.Provider)
		return &CreateQueueResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling queue spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "queue",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating queue resource: %w", err)
	}
	s.log.Info("queue resource created", "name", r.Name, "queue", in.Spec.Name)
	s.emit("queue", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionQueue(r.ID)
	return &CreateQueueResult{Resource: &r}, nil
}

func (s *Service) startProvisionQueue(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionQueue(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_queue failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionQueueByID(ctx, resourceID)
	}()
}

// ProvisionQueueByID loads a queue resource + its provider and runs tofu apply.
func (s *Service) ProvisionQueueByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading queue resource: %w", err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		s.markVMFailed(ctx, r.ID)
		return err
	}
	qp, ok := prov.(providers.QueueProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support message queues", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("queue provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := qp.ProvisionQueue(ctx, providers.QueueRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: queueSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("queue provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("queue", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("queue provisioning complete", "name", r.Name, "queue", res.QueueARN)
	s.emit("queue", "ready", r.Name, r.Environment, p.Name, res.QueueARN)
	return nil
}

// DestroyQueue tears down a queue resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyQueue(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("queue %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	qp, ok := prov.(providers.QueueProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support message queues", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("queue destroy started", "name", r.Name)

	if err := qp.DestroyQueue(ctx, providers.QueueRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: queueSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("queue destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("queue destroy complete", "name", r.Name)
	s.emit("queue", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyQueueAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyQueueAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyQueue(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_queue failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyQueue(ctx, name, env); err != nil {
			s.log.Error("async queue destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteQueueRecord forgets a terminal queue resource's tracking row (no tofu).
func (s *Service) DeleteQueueRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("queue %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("queue %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing queue record %q: %w", name, err)
	}
	s.log.Info("queue record removed", "name", name)
	return nil
}

// ListQueues returns all queue resources with provider name + parsed spec.
func (s *Service) ListQueues(ctx context.Context) ([]QueueSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "queue")
	if err != nil {
		return nil, fmt.Errorf("listing queue resources: %w", err)
	}
	provs, err := s.q.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing providers: %w", err)
	}
	names := make(map[uuid.UUID]string, len(provs))
	for _, p := range provs {
		names[p.ID] = p.Name
	}
	tid, scoped := scopeTenant(ctx)
	out := make([]QueueSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, QueueSummary{Resource: r, Provider: names[r.ProviderID], Spec: queueSpecOf(r)})
	}
	return out, nil
}

// QueueStatus returns one queue resource by name + environment.
func (s *Service) QueueStatus(ctx context.Context, name, env string) (*QueueSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("queue %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("queue %q (env %q) not found", name, env)
	}
	summary := &QueueSummary{Resource: r, Spec: queueSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
