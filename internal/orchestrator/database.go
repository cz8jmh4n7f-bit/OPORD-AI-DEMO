package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// CreateDatabaseInput is the request to provision a managed database.
type CreateDatabaseInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.DatabaseSpec
	DryRun      bool
}

// CreateDatabaseResult reports the outcome (dry-run summary, or persisted resource).
type CreateDatabaseResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// DatabaseSummary is a database resource enriched for list/detail views.
type DatabaseSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.DatabaseSpec
}

func dbSpecOf(r db.Resource) models.DatabaseSpec {
	var s models.DatabaseSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateDatabaseSpec(spec models.DatabaseSpec) error {
	var errs []string
	switch spec.Engine {
	case "", "postgres", "mysql":
	default:
		errs = append(errs, fmt.Sprintf("engine %q unsupported (want postgres or mysql)", spec.Engine))
	}
	if strings.TrimSpace(spec.DBName) == "" {
		errs = append(errs, "db_name is required")
	}
	if strings.TrimSpace(spec.Username) == "" {
		errs = append(errs, "username is required")
	}
	if spec.StorageGB < 5 {
		errs = append(errs, "storage_gb must be >= 5")
	}
	switch strings.ToLower(spec.AuthMode) {
	case "", "password", "iam":
	default:
		errs = append(errs, fmt.Sprintf("auth_mode %q unsupported (want password or iam)", spec.AuthMode))
	}
	if strings.EqualFold(spec.AuthMode, "iam") && strings.TrimSpace(spec.AuthPrincipal) == "" {
		errs = append(errs, "auth_principal (IAM user/service-account email) is required when auth_mode=iam")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid database spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateDatabase validates a database spec and (unless DryRun) persists it and
// provisions it in the background (RDS apply). Requires a provider that
// implements DatabaseProvisioner (AWS today).
func (s *Service) CreateDatabase(ctx context.Context, in CreateDatabaseInput) (*CreateDatabaseResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("database name and provider are required")
	}
	if err := validateDatabaseSpec(in.Spec); err != nil {
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
	dbp, ok := prov.(providers.DatabaseProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support managed databases", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := dbp.PreflightDB(ctx, providers.DBRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("database preflight failed: %w", err)
	}

	if in.DryRun {
		engine := in.Spec.Engine
		if engine == "" {
			engine = "postgres"
		}
		summary := fmt.Sprintf("spec valid; %s db %q (%d GB) on %s", engine, in.Spec.DBName, in.Spec.StorageGB, in.Provider)
		s.log.Info("database preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateDatabaseResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling database spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "database",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating database resource: %w", err)
	}
	s.log.Info("database resource created", "name", r.Name, "engine", in.Spec.Engine)
	s.emit("database", "created", r.Name, env, in.Provider, fmt.Sprintf("%s %q (%d GB)", in.Spec.Engine, in.Spec.DBName, in.Spec.StorageGB))
	s.startProvisionDB(r.ID)
	return &CreateDatabaseResult{Resource: &r}, nil
}

func (s *Service) startProvisionDB(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionDatabase(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_database failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		_ = s.ProvisionDatabaseByID(ctx, resourceID)
	}()
}

// ProvisionDatabaseByID loads a database resource + its provider and runs the
// real RDS apply, recording the outcome. Status flows provisioning -> ready/failed.
func (s *Service) ProvisionDatabaseByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading database resource: %w", err)
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
	dbp, ok := prov.(providers.DatabaseProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support managed databases", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	// Deploy into an AWS member account (ADR-0013): the cross-account AssumeRole
	// needs creds that can CHAIN AssumeRole (federation_token can't) - resolveDeployCreds
	// routes through the factory's assumed_role path when target_account is set.
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("database provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := dbp.ProvisionDB(ctx, providers.DBRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: dbSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("database provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("database", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	// Store the generated master password in the secrets store (OpenBao) when one is
	// configured, rather than leaving it only in tofu state. Cloud SQL / Azure
	// Flexible Server require a password at create time (no RDS-style managed
	// password), so it also stays in the workspace state - treat state as sensitive
	// (or enable state encryption). The password never reaches resources.observed:
	// DBResult.Password is json:"-" and the provider strips it from the raw outputs.
	if res.Password != "" {
		if w, ok := s.creds.(interface {
			WriteSecret(ctx context.Context, path string, data map[string]string) error
		}); ok {
			path := "opord/databases/" + r.Name
			if err := w.WriteSecret(ctx, path, map[string]string{
				"password": res.Password,
				"username": dbSpecOf(r).Username,
				"endpoint": res.Endpoint,
			}); err != nil {
				s.log.Warn("could not store db password in the secrets store; it stays only in tofu state", "name", r.Name, "err", err)
			} else {
				s.log.Info("db master password stored in secrets store", "name", r.Name, "path", path)
				if res.RawOutputs == nil {
					res.RawOutputs = map[string]any{}
				}
				res.RawOutputs["password_secret"] = path
			}
		} else {
			s.log.Warn("no secret store configured (set VAULT_ADDR/VAULT_TOKEN); db password stays only in tofu state", "name", r.Name)
		}
		res.Password = ""
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("database provisioning complete", "name", r.Name, "endpoint", res.Endpoint)
	s.emit("database", "ready", r.Name, r.Environment, p.Name, "endpoint "+res.Endpoint)
	return nil
}

// ScaleDatabase changes a database's instance class and/or storage and
// re-provisions (RDS modify via tofu apply). A day-2 operation. Note: RDS
// storage can only grow.
func (s *Service) ScaleDatabase(ctx context.Context, name, env, instanceClass string, storageGB int) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("database %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return fmt.Errorf("database %q (env %q) not found", name, env)
	}
	spec := dbSpecOf(r)
	changed := false
	if instanceClass != "" && instanceClass != spec.InstanceClass {
		spec.InstanceClass = instanceClass
		changed = true
	}
	if storageGB > 0 && storageGB != spec.StorageGB {
		spec.StorageGB = storageGB
		changed = true
	}
	if !changed {
		return nil
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling database spec: %w", err)
	}
	if _, err := s.q.UpdateResourceSpec(ctx, db.UpdateResourceSpecParams{ID: r.ID, Spec: specJSON, Status: "provisioning"}); err != nil {
		return fmt.Errorf("updating database spec: %w", err)
	}
	s.log.Info("database scaling", "name", r.Name, "class", spec.InstanceClass, "storage", spec.StorageGB)
	s.emit("database", "scaling", r.Name, r.Environment, "", fmt.Sprintf("%s / %d GB", spec.InstanceClass, spec.StorageGB))
	s.startProvisionDB(r.ID)
	return nil
}

// DestroyDatabase tears down a database resource (RDS destroy) and marks it destroyed.
func (s *Service) DestroyDatabase(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("database %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	dbp, ok := prov.(providers.DatabaseProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support managed databases", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	// Deploy-into-member RDS destroy needs the same assumed_role path (ADR-0013).
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("database destroy started", "name", r.Name)

	if err := dbp.DestroyDB(ctx, providers.DBRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: dbSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("database destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("database destroy complete", "name", r.Name)
	s.emit("database", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyDatabaseAsync runs DestroyDatabase on a background context.
// DeleteDatabaseRecord forgets a terminal database's tracking row (no tofu).
// Allowed only for destroyed/failed - destroy a live DB first. Mirrors DeleteVMRecord.
func (s *Service) DeleteDatabaseRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("database %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("database %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing database record %q: %w", name, err)
	}
	s.log.Info("database record removed", "name", name)
	return nil
}

func (s *Service) DestroyDatabaseAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyDatabase(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_database failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		if err := s.DestroyDatabase(ctx, name, env); err != nil {
			s.log.Error("async database destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// ListDatabases returns all database resources with provider name + parsed spec.
func (s *Service) ListDatabases(ctx context.Context) ([]DatabaseSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "database")
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
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
	out := make([]DatabaseSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, DatabaseSummary{Resource: r, Provider: names[r.ProviderID], Spec: dbSpecOf(r)})
	}
	return out, nil
}

// BackupDatabase records a backup and runs an RDS snapshot in the background.
// Returns the pending backup record.
func (s *Service) BackupDatabase(ctx context.Context, name, env string) (*db.Backup, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("database %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("database %q (env %q) not found", name, env)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	snapper, ok := prov.(providers.DatabaseSnapshotter)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support database snapshots", p.Type)
	}

	ws := uuid.NewString()
	b, err := s.q.CreateBackup(ctx, db.CreateBackupParams{
		ResourceKind:  "database",
		ResourceName:  r.Name,
		Environment:   env,
		Provider:      p.Name,
		TofuWorkspace: ws,
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating backup record: %w", err)
	}
	s.log.Info("database backup started", "name", r.Name, "backup", b.ID)
	s.emit("database", "backup", r.Name, env, p.Name, "snapshot started")

	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)
	snapName := fmt.Sprintf("%s-%d", r.Name, b.CreatedAt.Unix())

	go func() {
		bctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_, _ = s.q.SetBackupResult(bctx, db.SetBackupResultParams{ID: b.ID, Status: "running"})
		res, serr := snapper.SnapshotDB(bctx, providers.SnapshotRequest{
			Workspace: ws, DBIdentifier: r.Name, SnapshotName: snapName, Credentials: creds, Config: cfg,
		})
		if serr != nil {
			s.log.Error("database backup failed", "name", r.Name, "err", serr)
			_, _ = s.q.SetBackupResult(bctx, db.SetBackupResultParams{ID: b.ID, Status: "failed"})
			return
		}
		_, _ = s.q.SetBackupResult(bctx, db.SetBackupResultParams{ID: b.ID, SnapshotID: res.SnapshotID, Status: "completed"})
		s.log.Info("database backup complete", "name", r.Name, "snapshot", res.SnapshotID)
	}()
	return &b, nil
}

// ListBackups returns all backup records, scoped to the caller's tenant.
func (s *Service) ListBackups(ctx context.Context) ([]db.Backup, error) {
	bs, err := s.q.ListBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing backups: %w", err)
	}
	tid, scoped := scopeTenant(ctx)
	if !scoped {
		return bs, nil
	}
	out := make([]db.Backup, 0, len(bs))
	for _, b := range bs {
		if tenantVisible(b.TenantID, tid) {
			out = append(out, b)
		}
	}
	return out, nil
}

// DatabaseStatus returns one database resource by name + environment.
func (s *Service) DatabaseStatus(ctx context.Context, name, env string) (*DatabaseSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("database %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("database %q (env %q) not found", name, env)
	}
	summary := &DatabaseSummary{Resource: r, Spec: dbSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
