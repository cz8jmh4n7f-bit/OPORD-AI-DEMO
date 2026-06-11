package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// DatabaseProvisioner: a Cloud SQL instance (+ database + user) via
// modules/gcp-cloudsql. The master password is generated + kept in state; OPORD
// returns only the endpoint.

var _ providers.DatabaseProvisioner = (*Provider)(nil)

func (p *Provider) writeDBVars(req providers.DBRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildDBVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp db vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-cloudsql-*.tfvars.json")
	if err != nil {
		return "", noop, err
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		remove()
		return "", noop, err
	}
	if err := f.Close(); err != nil {
		remove()
		return "", noop, err
	}
	return f.Name(), remove, nil
}

func (p *Provider) PreflightDB(ctx context.Context, req providers.DBRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeDBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.cloudsqlModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionDB(ctx context.Context, req providers.DBRequest) (*providers.DBResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.cloudsqlModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeDBVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-cloudsql-*.tfplan")
	if err != nil {
		return nil, err
	}
	planPath := planFile.Name()
	_ = planFile.Close()
	defer os.Remove(planPath)

	if _, _, err := r.Plan(ctx, varsFile, planPath); err != nil {
		return nil, err
	}
	if _, err := r.Apply(ctx, planPath); err != nil {
		return nil, err
	}
	outs, err := r.Output(ctx)
	if err != nil {
		return nil, err
	}
	// The DB user password is a secret - pull it out and strip it from the raw
	// outputs so it never reaches resources.observed; the orchestrator stores it in
	// the secrets store (OpenBao).
	pw := outString(outs, "password")
	delete(outs, "password")
	return &providers.DBResult{
		Endpoint:   outString(outs, "endpoint"),
		Port:       outInt(outs, "port"),
		Password:   pw,
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyDB(ctx context.Context, req providers.DBRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.cloudsqlModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeDBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildDBVars maps a DBRequest onto the modules/gcp-cloudsql inputs.
func buildDBVars(req providers.DBRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := req.Name
	if name == "" {
		name = "opord-" + req.Workspace
	}
	region := cfgStringDefault(cfg, "region", "europe-west1")

	dbName := spec.DBName
	if dbName == "" {
		dbName = "opord"
	}
	username := spec.Username
	if username == "" {
		username = "opord"
	}
	disk := spec.StorageGB
	if disk < 10 {
		disk = 10
	}
	tier := cloudSQLTier(spec.InstanceClass)
	publicAccess := spec.PublicAccess
	if gcpIsProd(cfg) {
		publicAccess = false
	}

	return map[string]any{
		"name":             safeName(name, 40),
		"region":           region,
		"database_version": cloudSQLVersion(spec.Engine, spec.Version),
		"tier":             tier,
		"disk_gb":          disk,
		"db_name":          dbName,
		"username":         username,
		"public_access":    publicAccess,
		"iam_auth":         strings.EqualFold(spec.AuthMode, "iam"),
		"iam_principal":    spec.AuthPrincipal,
		"labels": map[string]string{
			"opord_kind":      "database",
			"opord_workspace": safeName(req.Workspace, 60),
		},
	}
}

// cloudSQLVersion maps (engine, version) onto a Cloud SQL database_version.
func cloudSQLVersion(engine, version string) string {
	version = strings.TrimSpace(version)
	if strings.EqualFold(engine, "mysql") {
		if version == "" {
			return "MYSQL_8_0"
		}
		return "MYSQL_" + strings.ReplaceAll(version, ".", "_")
	}
	// default postgres
	if version == "" {
		return "POSTGRES_16"
	}
	major := version
	if i := strings.Index(version, "."); i > 0 {
		major = version[:i]
	}
	return "POSTGRES_" + major
}

// cloudSQLTier passes through a GCP-shaped tier (db-...) or falls back to the
// smallest shared-core tier (the AWS-style db.t3.micro class doesn't map).
func cloudSQLTier(instanceClass string) string {
	if strings.HasPrefix(instanceClass, "db-") {
		return instanceClass
	}
	return "db-f1-micro"
}
