package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
)

// These are end-to-end orchestrator lifecycle tests: they drive the real
// CreateVM -> ProvisionVMByID -> DestroyVM code paths (status state machine,
// provider dispatch, observed/error recording) against a fake Querier (in-memory
// resource store), a fake VM provider (controllable success/failure), and a fake
// enqueuer (so CreateVM enqueues instead of spawning a background goroutine,
// keeping the test deterministic). No database, no Tofu, no cloud.

// --- fakes ---------------------------------------------------------------

type fakeVMProvider struct {
	provisionErr error
	provisioned  int
	destroyed    int
}

func (f *fakeVMProvider) Type() models.ProviderType { return models.ProviderAWS }
func (f *fakeVMProvider) Validate(context.Context, models.ClusterSpec) error { return nil }
func (f *fakeVMProvider) Preflight(context.Context, providers.Request) (*providers.PreflightResult, error) {
	return &providers.PreflightResult{ModuleValid: true}, nil
}
func (f *fakeVMProvider) Plan(context.Context, providers.Request) (*providers.PlanResult, error) {
	return &providers.PlanResult{}, nil
}
func (f *fakeVMProvider) Provision(context.Context, providers.Request) (*providers.ProvisionResult, error) {
	return &providers.ProvisionResult{}, nil
}
func (f *fakeVMProvider) Destroy(context.Context, providers.Request) error { return nil }

func (f *fakeVMProvider) PreflightVM(context.Context, providers.VMRequest) error { return nil }
func (f *fakeVMProvider) ProvisionVM(_ context.Context, _ providers.VMRequest) (*providers.VMResult, error) {
	f.provisioned++
	if f.provisionErr != nil {
		return nil, f.provisionErr
	}
	return &providers.VMResult{IDs: []string{"i-test123"}, PublicIPs: []string{"203.0.113.7"}}, nil
}
func (f *fakeVMProvider) DestroyVM(context.Context, providers.VMRequest) error {
	f.destroyed++
	return nil
}

type fakeCreds struct{}

func (fakeCreds) Resolve(context.Context, db.Provider) (map[string]string, error) {
	return map[string]string{}, nil
}
func (fakeCreds) ResolveConfig(context.Context, db.Provider) (map[string]any, error) {
	return map[string]any{}, nil
}

// fakeEnqueuer embeds the Enqueuer interface (nil) so only the two methods the
// VM path touches need real bodies; any other Enqueue* call would panic, which
// would correctly flag an untested code path.
type fakeEnqueuer struct {
	Enqueuer
	provisionVM []uuid.UUID
	destroyVM   []string
}

func (f *fakeEnqueuer) EnqueueProvisionVM(_ context.Context, id uuid.UUID) error {
	f.provisionVM = append(f.provisionVM, id)
	return nil
}
func (f *fakeEnqueuer) EnqueueDestroyVM(_ context.Context, name, _ string) error {
	f.destroyVM = append(f.destroyVM, name)
	return nil
}

// fakeVMQuerier is an in-memory resource store implementing only the Querier
// methods the VM lifecycle calls (the rest panic via the embedded interface).
type fakeVMQuerier struct {
	db.Querier
	provider db.Provider
	mu       sync.Mutex
	res      map[uuid.UUID]db.Resource
	byName   map[string]uuid.UUID
}

func newFakeVMQuerier(p db.Provider) *fakeVMQuerier {
	return &fakeVMQuerier{provider: p, res: map[uuid.UUID]db.Resource{}, byName: map[string]uuid.UUID{}}
}

func nameKey(name, env string) string { return name + "\x00" + env }

func (q *fakeVMQuerier) GetProviderByName(context.Context, string) (db.Provider, error) {
	return q.provider, nil
}
func (q *fakeVMQuerier) GetProvider(context.Context, uuid.UUID) (db.Provider, error) {
	return q.provider, nil
}
func (q *fakeVMQuerier) CreateResource(_ context.Context, arg db.CreateResourceParams) (db.Resource, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	r := db.Resource{
		ID:            uuid.New(),
		Name:          arg.Name,
		Environment:   arg.Environment,
		ProviderID:    arg.ProviderID,
		Kind:          arg.Kind,
		Status:        "pending",
		Spec:          arg.Spec,
		TofuWorkspace: arg.TofuWorkspace,
		TenantID:      arg.TenantID,
	}
	q.res[r.ID] = r
	q.byName[nameKey(r.Name, r.Environment)] = r.ID
	return r, nil
}
func (q *fakeVMQuerier) GetResource(_ context.Context, id uuid.UUID) (db.Resource, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	r, ok := q.res[id]
	if !ok {
		return db.Resource{}, errors.New("resource not found")
	}
	return r, nil
}
func (q *fakeVMQuerier) GetResourceByName(_ context.Context, arg db.GetResourceByNameParams) (db.Resource, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	id, ok := q.byName[nameKey(arg.Name, arg.Environment)]
	if !ok {
		return db.Resource{}, errors.New("resource not found")
	}
	return q.res[id], nil
}
func (q *fakeVMQuerier) UpdateResourceStatus(_ context.Context, arg db.UpdateResourceStatusParams) (db.Resource, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	r := q.res[arg.ID]
	r.Status = arg.Status
	q.res[arg.ID] = r
	return r, nil
}
func (q *fakeVMQuerier) UpdateResourceObserved(_ context.Context, arg db.UpdateResourceObservedParams) (db.Resource, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	r := q.res[arg.ID]
	r.Status = arg.Status
	r.Observed = arg.Observed
	q.res[arg.ID] = r
	return r, nil
}
func (q *fakeVMQuerier) DeleteResource(_ context.Context, id uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	r, ok := q.res[id]
	if !ok {
		return errors.New("resource not found")
	}
	delete(q.byName, nameKey(r.Name, r.Environment))
	delete(q.res, id)
	return nil
}

func (q *fakeVMQuerier) status(t *testing.T, name, env string) string {
	t.Helper()
	r, err := q.GetResourceByName(context.Background(), db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		t.Fatalf("resource %q not found: %v", name, err)
	}
	return r.Status
}

// --- harness -------------------------------------------------------------

func newVMTestService(prov *fakeVMProvider) (*Service, *fakeVMQuerier, *fakeEnqueuer) {
	p := db.Provider{ID: uuid.New(), Name: "aws-test", Type: "aws", Config: []byte("{}")}
	q := newFakeVMQuerier(p)
	reg := providers.NewRegistry()
	reg.Register(models.ProviderAWS, func() providers.Provider { return prov })
	svc := New(q, reg, fakeCreds{}, slog.New(slog.NewTextHandler(io.Discard, nil)), BootstrapConfig{})
	enq := &fakeEnqueuer{}
	svc.SetEnqueuer(enq)
	return svc, q, enq
}

func validVMInput(name string) CreateVMInput {
	return CreateVMInput{
		Name:     name,
		Provider: "aws-test",
		Spec:     models.VMSpec{Template: "ami-0123", Count: 1, DiskGB: 20, InstanceType: "t3.micro"},
	}
}

// --- tests ---------------------------------------------------------------

func TestVMLifecycle_ProvisionAndDestroy(t *testing.T) {
	prov := &fakeVMProvider{}
	svc, q, enq := newVMTestService(prov)
	ctx := context.Background()

	// Create: persists a pending resource and enqueues provisioning (no goroutine).
	res, err := svc.CreateVM(ctx, validVMInput("web1"))
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}
	if res.Resource == nil {
		t.Fatal("CreateVM returned no resource")
	}
	if got := q.status(t, "web1", "dev"); got != "pending" {
		t.Fatalf("status after create = %q, want pending", got)
	}
	if len(enq.provisionVM) != 1 {
		t.Fatalf("expected 1 enqueued provision, got %d", len(enq.provisionVM))
	}
	if prov.provisioned != 0 {
		t.Fatalf("provider should not have provisioned yet, got %d", prov.provisioned)
	}

	// Provision (what the worker would run): pending -> ready, observed recorded.
	if err := svc.ProvisionVMByID(ctx, res.Resource.ID); err != nil {
		t.Fatalf("ProvisionVMByID: %v", err)
	}
	if got := q.status(t, "web1", "dev"); got != "ready" {
		t.Fatalf("status after provision = %q, want ready", got)
	}
	if prov.provisioned != 1 {
		t.Fatalf("provider provisioned = %d, want 1", prov.provisioned)
	}
	r, _ := q.GetResource(ctx, res.Resource.ID)
	if !strings.Contains(string(r.Observed), "i-test123") {
		t.Fatalf("observed should record the VM id, got %s", string(r.Observed))
	}

	// Destroy: ready -> destroyed, provider destroy invoked once.
	if err := svc.DestroyVM(ctx, "web1", "dev"); err != nil {
		t.Fatalf("DestroyVM: %v", err)
	}
	if got := q.status(t, "web1", "dev"); got != "destroyed" {
		t.Fatalf("status after destroy = %q, want destroyed", got)
	}
	if prov.destroyed != 1 {
		t.Fatalf("provider destroyed = %d, want 1", prov.destroyed)
	}
}

func TestVMLifecycle_ProvisionFailureMarksFailed(t *testing.T) {
	prov := &fakeVMProvider{provisionErr: errors.New("boom: tofu apply failed")}
	svc, q, _ := newVMTestService(prov)
	ctx := context.Background()

	res, err := svc.CreateVM(ctx, validVMInput("web2"))
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if err := svc.ProvisionVMByID(ctx, res.Resource.ID); err == nil {
		t.Fatal("expected ProvisionVMByID to return the provider error")
	}
	if got := q.status(t, "web2", "dev"); got != "failed" {
		t.Fatalf("status after failed provision = %q, want failed", got)
	}
	// The failure reason is persisted into observed so the UI can show WHY.
	r, _ := q.GetResource(ctx, res.Resource.ID)
	if !strings.Contains(string(r.Observed), "boom") {
		t.Fatalf("observed should record the failure reason, got %s", string(r.Observed))
	}
}

func TestDeleteVMRecord_RefusesNonTerminal(t *testing.T) {
	prov := &fakeVMProvider{}
	svc, q, _ := newVMTestService(prov)
	ctx := context.Background()

	res, err := svc.CreateVM(ctx, validVMInput("web3"))
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	// A pending (non-terminal) record must not be purged - it would orphan infra.
	if err := svc.DeleteVMRecord(ctx, "web3", "dev"); err == nil {
		t.Fatal("expected DeleteVMRecord to refuse a non-terminal resource")
	}

	// Once terminal, the record can be forgotten.
	_, _ = q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: res.Resource.ID, Status: "destroyed"})
	if err := svc.DeleteVMRecord(ctx, "web3", "dev"); err != nil {
		t.Fatalf("DeleteVMRecord on terminal resource: %v", err)
	}
	if _, err := q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: "web3", Environment: "dev"}); err == nil {
		t.Fatal("record should be gone after purge")
	}
}
