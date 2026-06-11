package jobs

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
)

// Orchestrator is the subset of orchestrator.Service the workers drive. Defined
// here (rather than importing orchestrator) so the dependency points one way:
// jobs <- cmd/worker -> orchestrator, with no import cycle.
type Orchestrator interface {
	ProvisionVMByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyVM(ctx context.Context, name, env string) error
	ProvisionClusterByID(ctx context.Context, clusterID, jobID uuid.UUID) error
	DestroyCluster(ctx context.Context, name, env string) error
	ProvisionDatabaseByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyDatabase(ctx context.Context, name, env string) error
	ProvisionStackByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyStack(ctx context.Context, name, env string) error
	ProvisionTableByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyTable(ctx context.Context, name, env string) error
	ProvisionFunctionByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyFunction(ctx context.Context, name, env string) error
	ProvisionS3ByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyS3(ctx context.Context, name, env string) error
	ProvisionSecretByID(ctx context.Context, resourceID uuid.UUID) error
	DestroySecret(ctx context.Context, name, env string) error
	ProvisionQueueByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyQueue(ctx context.Context, name, env string) error
	ProvisionCacheByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyCache(ctx context.Context, name, env string) error
	ProvisionProjectByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyProject(ctx context.Context, name, env string) error
	ProvisionAccountByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyAccount(ctx context.Context, name, env string) error
	ProvisionDNSByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyDNS(ctx context.Context, name, env string) error
	ProvisionCertByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyCert(ctx context.Context, name, env string) error
	ProvisionLoadBalancerByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyLoadBalancer(ctx context.Context, name, env string) error
	ProvisionAPIGatewayByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyAPIGateway(ctx context.Context, name, env string) error
	ProvisionCDNByID(ctx context.Context, resourceID uuid.UUID) error
	DestroyCDN(ctx context.Context, name, env string) error
}

// Each Work wraps the orchestrator call in cancelIfPermanent so permanent
// failures (bad config / precondition / auth / quota) stop instead of retrying
// River's default 25× - transient (state-lock / throttle) errors still retry.

type provisionVMWorker struct {
	river.WorkerDefaults[ProvisionVMArgs]
	o Orchestrator
}

func (w *provisionVMWorker) Work(ctx context.Context, job *river.Job[ProvisionVMArgs]) error {
	return cancelIfPermanent(w.o.ProvisionVMByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyVMWorker struct {
	river.WorkerDefaults[DestroyVMArgs]
	o Orchestrator
}

func (w *destroyVMWorker) Work(ctx context.Context, job *river.Job[DestroyVMArgs]) error {
	return cancelIfPermanent(w.o.DestroyVM(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionClusterWorker struct {
	river.WorkerDefaults[ProvisionClusterArgs]
	o Orchestrator
}

func (w *provisionClusterWorker) Work(ctx context.Context, job *river.Job[ProvisionClusterArgs]) error {
	return cancelIfPermanent(w.o.ProvisionClusterByID(creds.WithSecretWait(ctx), job.Args.ClusterID, job.Args.JobID))
}

type destroyClusterWorker struct {
	river.WorkerDefaults[DestroyClusterArgs]
	o Orchestrator
}

func (w *destroyClusterWorker) Work(ctx context.Context, job *river.Job[DestroyClusterArgs]) error {
	return cancelIfPermanent(w.o.DestroyCluster(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

// scaleClusterWorker re-applies a cluster after a worker-count change. Same call as
// provision (idempotent), but a distinct kind so the queue shows "scale_cluster".
type scaleClusterWorker struct {
	river.WorkerDefaults[ScaleClusterArgs]
	o Orchestrator
}

func (w *scaleClusterWorker) Work(ctx context.Context, job *river.Job[ScaleClusterArgs]) error {
	return cancelIfPermanent(w.o.ProvisionClusterByID(creds.WithSecretWait(ctx), job.Args.ClusterID, job.Args.JobID))
}

type provisionDatabaseWorker struct {
	river.WorkerDefaults[ProvisionDatabaseArgs]
	o Orchestrator
}

func (w *provisionDatabaseWorker) Work(ctx context.Context, job *river.Job[ProvisionDatabaseArgs]) error {
	return cancelIfPermanent(w.o.ProvisionDatabaseByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyDatabaseWorker struct {
	river.WorkerDefaults[DestroyDatabaseArgs]
	o Orchestrator
}

func (w *destroyDatabaseWorker) Work(ctx context.Context, job *river.Job[DestroyDatabaseArgs]) error {
	return cancelIfPermanent(w.o.DestroyDatabase(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionStackWorker struct {
	river.WorkerDefaults[ProvisionStackArgs]
	o Orchestrator
}

func (w *provisionStackWorker) Work(ctx context.Context, job *river.Job[ProvisionStackArgs]) error {
	return cancelIfPermanent(w.o.ProvisionStackByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyStackWorker struct {
	river.WorkerDefaults[DestroyStackArgs]
	o Orchestrator
}

func (w *destroyStackWorker) Work(ctx context.Context, job *river.Job[DestroyStackArgs]) error {
	return cancelIfPermanent(w.o.DestroyStack(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionTableWorker struct {
	river.WorkerDefaults[ProvisionTableArgs]
	o Orchestrator
}

func (w *provisionTableWorker) Work(ctx context.Context, job *river.Job[ProvisionTableArgs]) error {
	return cancelIfPermanent(w.o.ProvisionTableByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyTableWorker struct {
	river.WorkerDefaults[DestroyTableArgs]
	o Orchestrator
}

func (w *destroyTableWorker) Work(ctx context.Context, job *river.Job[DestroyTableArgs]) error {
	return cancelIfPermanent(w.o.DestroyTable(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionFunctionWorker struct {
	river.WorkerDefaults[ProvisionFunctionArgs]
	o Orchestrator
}

func (w *provisionFunctionWorker) Work(ctx context.Context, job *river.Job[ProvisionFunctionArgs]) error {
	return cancelIfPermanent(w.o.ProvisionFunctionByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyFunctionWorker struct {
	river.WorkerDefaults[DestroyFunctionArgs]
	o Orchestrator
}

func (w *destroyFunctionWorker) Work(ctx context.Context, job *river.Job[DestroyFunctionArgs]) error {
	return cancelIfPermanent(w.o.DestroyFunction(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionS3Worker struct {
	river.WorkerDefaults[ProvisionS3Args]
	o Orchestrator
}

func (w *provisionS3Worker) Work(ctx context.Context, job *river.Job[ProvisionS3Args]) error {
	return cancelIfPermanent(w.o.ProvisionS3ByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyS3Worker struct {
	river.WorkerDefaults[DestroyS3Args]
	o Orchestrator
}

func (w *destroyS3Worker) Work(ctx context.Context, job *river.Job[DestroyS3Args]) error {
	return cancelIfPermanent(w.o.DestroyS3(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionSecretWorker struct {
	river.WorkerDefaults[ProvisionSecretArgs]
	o Orchestrator
}

func (w *provisionSecretWorker) Work(ctx context.Context, job *river.Job[ProvisionSecretArgs]) error {
	return cancelIfPermanent(w.o.ProvisionSecretByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroySecretWorker struct {
	river.WorkerDefaults[DestroySecretArgs]
	o Orchestrator
}

func (w *destroySecretWorker) Work(ctx context.Context, job *river.Job[DestroySecretArgs]) error {
	return cancelIfPermanent(w.o.DestroySecret(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionQueueWorker struct {
	river.WorkerDefaults[ProvisionQueueArgs]
	o Orchestrator
}

func (w *provisionQueueWorker) Work(ctx context.Context, job *river.Job[ProvisionQueueArgs]) error {
	return cancelIfPermanent(w.o.ProvisionQueueByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyQueueWorker struct {
	river.WorkerDefaults[DestroyQueueArgs]
	o Orchestrator
}

func (w *destroyQueueWorker) Work(ctx context.Context, job *river.Job[DestroyQueueArgs]) error {
	return cancelIfPermanent(w.o.DestroyQueue(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionCacheWorker struct {
	river.WorkerDefaults[ProvisionCacheArgs]
	o Orchestrator
}

func (w *provisionCacheWorker) Work(ctx context.Context, job *river.Job[ProvisionCacheArgs]) error {
	return cancelIfPermanent(w.o.ProvisionCacheByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyCacheWorker struct {
	river.WorkerDefaults[DestroyCacheArgs]
	o Orchestrator
}

func (w *destroyCacheWorker) Work(ctx context.Context, job *river.Job[DestroyCacheArgs]) error {
	return cancelIfPermanent(w.o.DestroyCache(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionProjectWorker struct {
	river.WorkerDefaults[ProvisionProjectArgs]
	o Orchestrator
}

func (w *provisionProjectWorker) Work(ctx context.Context, job *river.Job[ProvisionProjectArgs]) error {
	return cancelIfPermanent(w.o.ProvisionProjectByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyProjectWorker struct {
	river.WorkerDefaults[DestroyProjectArgs]
	o Orchestrator
}

func (w *destroyProjectWorker) Work(ctx context.Context, job *river.Job[DestroyProjectArgs]) error {
	return cancelIfPermanent(w.o.DestroyProject(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionAccountWorker struct {
	river.WorkerDefaults[ProvisionAccountArgs]
	o Orchestrator
}

func (w *provisionAccountWorker) Work(ctx context.Context, job *river.Job[ProvisionAccountArgs]) error {
	return cancelIfPermanent(w.o.ProvisionAccountByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyAccountWorker struct {
	river.WorkerDefaults[DestroyAccountArgs]
	o Orchestrator
}

func (w *destroyAccountWorker) Work(ctx context.Context, job *river.Job[DestroyAccountArgs]) error {
	return cancelIfPermanent(w.o.DestroyAccount(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

// ── Expose-layer workers (ADR-0016) ──

type provisionDNSWorker struct {
	river.WorkerDefaults[ProvisionDNSArgs]
	o Orchestrator
}

func (w *provisionDNSWorker) Work(ctx context.Context, job *river.Job[ProvisionDNSArgs]) error {
	return cancelIfPermanent(w.o.ProvisionDNSByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyDNSWorker struct {
	river.WorkerDefaults[DestroyDNSArgs]
	o Orchestrator
}

func (w *destroyDNSWorker) Work(ctx context.Context, job *river.Job[DestroyDNSArgs]) error {
	return cancelIfPermanent(w.o.DestroyDNS(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionCertWorker struct {
	river.WorkerDefaults[ProvisionCertArgs]
	o Orchestrator
}

func (w *provisionCertWorker) Work(ctx context.Context, job *river.Job[ProvisionCertArgs]) error {
	return cancelIfPermanent(w.o.ProvisionCertByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyCertWorker struct {
	river.WorkerDefaults[DestroyCertArgs]
	o Orchestrator
}

func (w *destroyCertWorker) Work(ctx context.Context, job *river.Job[DestroyCertArgs]) error {
	return cancelIfPermanent(w.o.DestroyCert(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionLoadBalancerWorker struct {
	river.WorkerDefaults[ProvisionLoadBalancerArgs]
	o Orchestrator
}

func (w *provisionLoadBalancerWorker) Work(ctx context.Context, job *river.Job[ProvisionLoadBalancerArgs]) error {
	return cancelIfPermanent(w.o.ProvisionLoadBalancerByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyLoadBalancerWorker struct {
	river.WorkerDefaults[DestroyLoadBalancerArgs]
	o Orchestrator
}

func (w *destroyLoadBalancerWorker) Work(ctx context.Context, job *river.Job[DestroyLoadBalancerArgs]) error {
	return cancelIfPermanent(w.o.DestroyLoadBalancer(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionAPIGatewayWorker struct {
	river.WorkerDefaults[ProvisionAPIGatewayArgs]
	o Orchestrator
}

func (w *provisionAPIGatewayWorker) Work(ctx context.Context, job *river.Job[ProvisionAPIGatewayArgs]) error {
	return cancelIfPermanent(w.o.ProvisionAPIGatewayByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyAPIGatewayWorker struct {
	river.WorkerDefaults[DestroyAPIGatewayArgs]
	o Orchestrator
}

func (w *destroyAPIGatewayWorker) Work(ctx context.Context, job *river.Job[DestroyAPIGatewayArgs]) error {
	return cancelIfPermanent(w.o.DestroyAPIGateway(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

type provisionCDNWorker struct {
	river.WorkerDefaults[ProvisionCDNArgs]
	o Orchestrator
}

func (w *provisionCDNWorker) Work(ctx context.Context, job *river.Job[ProvisionCDNArgs]) error {
	return cancelIfPermanent(w.o.ProvisionCDNByID(creds.WithSecretWait(ctx), job.Args.ResourceID))
}

type destroyCDNWorker struct {
	river.WorkerDefaults[DestroyCDNArgs]
	o Orchestrator
}

func (w *destroyCDNWorker) Work(ctx context.Context, job *river.Job[DestroyCDNArgs]) error {
	return cancelIfPermanent(w.o.DestroyCDN(creds.WithSecretWait(ctx), job.Args.Name, job.Args.Environment))
}

// registerWorkers wires every job kind to its worker.
func registerWorkers(o Orchestrator) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &provisionVMWorker{o: o})
	river.AddWorker(workers, &destroyVMWorker{o: o})
	river.AddWorker(workers, &provisionClusterWorker{o: o})
	river.AddWorker(workers, &destroyClusterWorker{o: o})
	river.AddWorker(workers, &scaleClusterWorker{o: o})
	river.AddWorker(workers, &provisionDatabaseWorker{o: o})
	river.AddWorker(workers, &destroyDatabaseWorker{o: o})
	river.AddWorker(workers, &provisionStackWorker{o: o})
	river.AddWorker(workers, &destroyStackWorker{o: o})
	river.AddWorker(workers, &provisionTableWorker{o: o})
	river.AddWorker(workers, &destroyTableWorker{o: o})
	river.AddWorker(workers, &provisionFunctionWorker{o: o})
	river.AddWorker(workers, &destroyFunctionWorker{o: o})
	river.AddWorker(workers, &provisionS3Worker{o: o})
	river.AddWorker(workers, &destroyS3Worker{o: o})
	river.AddWorker(workers, &provisionSecretWorker{o: o})
	river.AddWorker(workers, &destroySecretWorker{o: o})
	river.AddWorker(workers, &provisionQueueWorker{o: o})
	river.AddWorker(workers, &destroyQueueWorker{o: o})
	river.AddWorker(workers, &provisionCacheWorker{o: o})
	river.AddWorker(workers, &destroyCacheWorker{o: o})
	river.AddWorker(workers, &provisionProjectWorker{o: o})
	river.AddWorker(workers, &destroyProjectWorker{o: o})
	river.AddWorker(workers, &provisionAccountWorker{o: o})
	river.AddWorker(workers, &destroyAccountWorker{o: o})
	river.AddWorker(workers, &provisionDNSWorker{o: o})
	river.AddWorker(workers, &destroyDNSWorker{o: o})
	river.AddWorker(workers, &provisionCertWorker{o: o})
	river.AddWorker(workers, &destroyCertWorker{o: o})
	river.AddWorker(workers, &provisionLoadBalancerWorker{o: o})
	river.AddWorker(workers, &destroyLoadBalancerWorker{o: o})
	river.AddWorker(workers, &provisionAPIGatewayWorker{o: o})
	river.AddWorker(workers, &destroyAPIGatewayWorker{o: o})
	river.AddWorker(workers, &provisionCDNWorker{o: o})
	river.AddWorker(workers, &destroyCDNWorker{o: o})
	return workers
}
