package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// JobInfo is a queue row flattened for display (API/CLI), independent of the
// River types.
type JobInfo struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	Queue       string     `json:"queue"`
	State       string     `json:"state"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"maxAttempts"`
	CreatedAt   time.Time  `json:"createdAt"`
	FinalizedAt *time.Time `json:"finalizedAt"`
	Error       string     `json:"error,omitempty"`
}

// Migrate applies River's own schema (river_job, river_leader, ...). Idempotent,
// so it is safe to call on every startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate up: %w", err)
	}
	return nil
}

// Enqueuer is an insert-only River client. It implements orchestrator.Enqueuer
// so the API/CLI can hand work to the queue without running a worker pool.
type Enqueuer struct {
	client *river.Client[pgx.Tx]
}

// NewEnqueuer builds an insert-only client (no queues/workers configured).
func NewEnqueuer(pool *pgxpool.Pool) (*Enqueuer, error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return nil, fmt.Errorf("river insert-only client: %w", err)
	}
	return &Enqueuer{client: client}, nil
}

func (e *Enqueuer) EnqueueProvisionVM(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionVMArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyVM(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyVMArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionCluster(ctx context.Context, clusterID, jobID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionClusterArgs{ClusterID: clusterID, JobID: jobID}, nil)
	return err
}

func (e *Enqueuer) EnqueueScaleCluster(ctx context.Context, clusterID, jobID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ScaleClusterArgs{ClusterID: clusterID, JobID: jobID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyCluster(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyClusterArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionDatabase(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionDatabaseArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyDatabase(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyDatabaseArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionStack(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionStackArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyStack(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyStackArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionTable(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionTableArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyTable(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyTableArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionFunction(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionFunctionArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyFunction(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyFunctionArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionS3(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionS3Args{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyS3(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyS3Args{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionSecret(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionSecretArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroySecret(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroySecretArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionQueue(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionQueueArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyQueue(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyQueueArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionCache(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionCacheArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyCache(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyCacheArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionProject(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionProjectArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyProject(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyProjectArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionAccount(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionAccountArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyAccount(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyAccountArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionDNS(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionDNSArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyDNS(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyDNSArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionCert(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionCertArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyCert(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyCertArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionLoadBalancer(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionLoadBalancerArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyLoadBalancer(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyLoadBalancerArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionAPIGateway(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionAPIGatewayArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyAPIGateway(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyAPIGatewayArgs{Name: name, Environment: env}, nil)
	return err
}

func (e *Enqueuer) EnqueueProvisionCDN(ctx context.Context, resourceID uuid.UUID) error {
	_, err := e.client.Insert(ctx, ProvisionCDNArgs{ResourceID: resourceID}, nil)
	return err
}

func (e *Enqueuer) EnqueueDestroyCDN(ctx context.Context, name, env string) error {
	_, err := e.client.Insert(ctx, DestroyCDNArgs{Name: name, Environment: env}, nil)
	return err
}

// ListJobs returns the most recent queue rows (all states), newest first.
func (e *Enqueuer) ListJobs(ctx context.Context, limit int) ([]JobInfo, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	params := river.NewJobListParams().First(limit).OrderBy(river.JobListOrderByID, river.SortOrderDesc)
	res, err := e.client.JobList(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("river job list: %w", err)
	}
	out := make([]JobInfo, 0, len(res.Jobs))
	for _, j := range res.Jobs {
		info := JobInfo{
			ID:          j.ID,
			Kind:        j.Kind,
			Queue:       j.Queue,
			State:       string(j.State),
			Attempt:     j.Attempt,
			MaxAttempts: j.MaxAttempts,
			CreatedAt:   j.CreatedAt,
			FinalizedAt: j.FinalizedAt,
		}
		if n := len(j.Errors); n > 0 {
			info.Error = j.Errors[n-1].Error
		}
		out = append(out, info)
	}
	return out, nil
}

// NewWorkerClient builds a River client with the OPORD workers registered and a
// default queue. Call Start to begin processing and Stop for graceful shutdown.
func NewWorkerClient(pool *pgxpool.Pool, o Orchestrator, maxWorkers int) (*river.Client[pgx.Tx], error) {
	if maxWorkers < 1 {
		maxWorkers = 10
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers: registerWorkers(o),
		// tofu apply/destroy routinely runs for several minutes (multi-instance,
		// replacements, slow clouds). River's default JobTimeout is 1 minute, which
		// would cancel the job context mid-apply - killing the tofu subprocess
		// (exit -1) to River retries to instance churn and orphaned resources. Match
		// the in-process provision budget (30m) so long applies finish cleanly.
		JobTimeout: 30 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("river worker client: %w", err)
	}
	return client, nil
}
