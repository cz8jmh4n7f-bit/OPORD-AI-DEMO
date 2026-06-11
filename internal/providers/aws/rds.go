package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// DatabaseProvisioner: managed RDS via modules/aws-rds.

func (p *Provider) writeDBVars(req providers.DBRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildDBVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling db vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-rds-*.tfvars.json")
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

// PreflightDB validates the var mapping + the aws-rds module offline.
func (p *Provider) PreflightDB(ctx context.Context, req providers.DBRequest) error {
	_, cleanup, err := p.writeDBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.rdsModuleDir, p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionDB creates the RDS instance (tofu apply) for the request's workspace.
func (p *Provider) ProvisionDB(ctx context.Context, req providers.DBRequest) (*providers.DBResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.rdsModuleDir, p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	// Deploy into a member account: RDS needs subnets that exist THERE, not the
	// provider's own-account subnets (ADR-0013). No-op for the provider's account.
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return nil, err
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeDBVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-rds-*.tfplan")
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
	raw := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			raw[k] = val
		}
	}
	return &providers.DBResult{
		Endpoint:   dbOutString(outs, "db_endpoint"),
		Port:       dbOutInt(outs, "db_port"),
		RawOutputs: raw,
	}, nil
}

// DestroyDB tears down the RDS instance for the request's workspace.
func (p *Provider) DestroyDB(ctx context.Context, req providers.DBRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.rdsModuleDir, p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return err
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeDBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildDBVars maps a DBRequest onto the modules/aws-rds OpenTofu inputs.
func buildDBVars(req providers.DBRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	engine := spec.Engine
	if engine == "" {
		engine = "postgres"
	}
	name := req.Name
	if name == "" {
		name = "opord-" + req.Workspace
	}
	return map[string]any{
		"region":             cfgString(cfg, "region"),
		"name":               name,
		"engine":             engine,
		"engine_version":     spec.Version,
		"instance_class":     cfgStringDefault(cfg, "db_instance_class", firstNonEmpty(spec.InstanceClass, "db.t3.micro")),
		"storage_gb":         spec.StorageGB,
		"db_name":            spec.DBName,
		"username":           spec.Username,
		"subnet_ids":         cfgStringSlice(cfg, "subnet_ids"),
		"security_group_ids": cfgStringSlice(cfg, "security_group_ids"),
		"multi_az":           spec.MultiAZ,
		"public_access":      spec.PublicAccess,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func dbOutString(outs map[string]json.RawMessage, key string) string {
	raw, ok := outs[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func dbOutInt(outs map[string]json.RawMessage, key string) int {
	raw, ok := outs[key]
	if !ok {
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0
	}
	return n
}
