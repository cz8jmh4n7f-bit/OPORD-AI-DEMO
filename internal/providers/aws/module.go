package aws

import (
	"context"
	"encoding/json"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// Shared tofu plumbing for the "uniform" AWS catalog primitives - those whose
// provision/destroy/preflight is exactly: write vars -> init -> select workspace
// -> plan -> apply -> read outputs, with no resource-specific extra steps. s3 /
// secret / sqs / dynamodb / lambda / cache / dns all share this; vm, eks, rds,
// account, project, and cert keep their own flows (subnet discovery, multi-layer
// sequencing, or a per-resource region override). Extracted to drop ~50 lines of
// identical boilerplate per file while preserving behavior. (The temp-tfvars
// writer, writeTfvars, already lived in account.go - reused here.)

// preflightModule validates a module + its vars offline (no backend, no STS) -
// the shared body of every PreflightX on a uniform resource.
func (p *Provider) preflightModule(ctx context.Context, moduleDir, prefix string, creds map[string]string, cfg, vars map[string]any) error {
	_, cleanup, err := writeTfvars(prefix, vars)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	r.SetEnv(awsTofuEnv(creds, cfg, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// applyModule runs the standard provision flow (init -> workspace -> plan ->
// apply) and returns the module outputs. targetAccount "" = the provider's own
// account; otherwise it assume-roles into the member account (ADR-0013).
func (p *Provider) applyModule(ctx context.Context, moduleDir, prefix, workspace string, creds map[string]string, cfg map[string]any, targetAccount string, vars map[string]any) (map[string]json.RawMessage, error) {
	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	if err := p.setTargetEnv(ctx, r, creds, cfg, "", targetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := writeTfvars(prefix, vars)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", prefix+"-*.tfplan")
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
	return r.Output(ctx)
}

// destroyModule runs the standard destroy flow for a uniform resource.
func (p *Provider) destroyModule(ctx context.Context, moduleDir, prefix, workspace string, creds map[string]string, cfg map[string]any, targetAccount string, vars map[string]any) error {
	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	if err := p.setTargetEnv(ctx, r, creds, cfg, "", targetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := writeTfvars(prefix, vars)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}
