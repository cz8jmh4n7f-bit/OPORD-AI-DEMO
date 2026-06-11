package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// DatabaseProvisioner: managed PostgreSQL Flexible Server via modules/azure-postgres.

func (p *Provider) writeAzureDBVars(req providers.DBRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildAzureDBVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure db vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-pg-*.tfvars.json")
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

// PreflightDB validates the var mapping + the azure-postgres module offline.
func (p *Provider) PreflightDB(ctx context.Context, req providers.DBRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeAzureDBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.dbModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionDB creates the Postgres Flexible Server (tofu apply).
func (p *Provider) ProvisionDB(ctx context.Context, req providers.DBRequest) (*providers.DBResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.dbModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeAzureDBVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-pg-*.tfplan")
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
	// The admin password is a secret - pull it out and strip it from the raw
	// outputs so it never reaches resources.observed; the orchestrator stores it in
	// the secrets store (OpenBao).
	pw := azureOutString(outs, "admin_password")
	delete(outs, "admin_password")
	return &providers.DBResult{
		Endpoint:   azureOutString(outs, "endpoint"),
		Port:       azureOutInt(outs, "port"),
		Password:   pw,
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyDB tears down the Postgres server for the request's workspace.
func (p *Provider) DestroyDB(ctx context.Context, req providers.DBRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.dbModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeAzureDBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildAzureDBVars maps a DBRequest (provider-neutral DatabaseSpec + creds +
// config) onto the modules/azure-postgres OpenTofu inputs.
func buildAzureDBVars(req providers.DBRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	location := cfgString(cfg, "location")
	if location == "" {
		location = "westeurope"
	}

	namePrefix := req.Name
	if namePrefix == "" {
		namePrefix = "opord-" + safePrefix(req.Workspace, 12)
	} else {
		namePrefix = safePrefix(namePrefix, 30)
	}

	username := spec.Username
	if username == "" {
		username = "opordadmin"
	}

	version := spec.Version
	if version == "" {
		version = "16"
	}

	sku := spec.InstanceClass
	if sku == "" {
		sku = cfgStringDefault(cfg, "pg_sku_name", "B_Standard_B1ms")
	}

	storage := spec.StorageGB
	if storage <= 0 {
		storage = 32
	}
	storageMB := storage * 1024

	allowPublic := spec.PublicAccess
	if azureIsProd(cfg) {
		allowPublic = false
	}

	return map[string]any{
		"location":            location,
		"name_prefix":         namePrefix,
		"environment":         cfgStringDefault(cfg, "environment", "dev"),
		"postgres_version":    version,
		"sku_name":            sku,
		"storage_mb":          storageMB,
		"admin_username":      username,
		"database_name":       spec.DBName,
		"allow_public_access": allowPublic,
	}
}

// azureOutInt unmarshals a tofu number output as int. Returns 0 on miss/parse-fail.
func azureOutInt(outs map[string]json.RawMessage, key string) int {
	raw, ok := outs[key]
	if !ok {
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	return 0
}

// azureOutString unmarshals a tofu string output. Returns "" on miss/parse-fail.
func azureOutString(outs map[string]json.RawMessage, key string) string {
	raw, ok := outs[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}
