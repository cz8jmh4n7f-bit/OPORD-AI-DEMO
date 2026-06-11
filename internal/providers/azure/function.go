package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// FunctionProvisioner: Azure Linux Function App (Consumption plan, scales to
// zero) via modules/azure-functions. Runtime/version map from FunctionSpec's
// AWS-Lambda-shaped "runtime" string (e.g. "python3.12") onto Azure's
// runtime/runtime_version pair ("python" + "3.12").

func (p *Provider) writeFnVars(req providers.FunctionRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildAzureFnVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling function vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-fn-*.tfvars.json")
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

func (p *Provider) PreflightFunction(ctx context.Context, req providers.FunctionRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeFnVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.fnModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionFunction(ctx context.Context, req providers.FunctionRequest) (*providers.FunctionResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.fnModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeFnVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-fn-*.tfplan")
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
	runtime, _ := splitAzureRuntime(req.Spec.Runtime)
	return &providers.FunctionResult{
		ARN:        azureOutString(outs, "function_id"),
		Name:       azureOutString(outs, "function_name"),
		Runtime:    runtime,
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyFunction(ctx context.Context, req providers.FunctionRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.fnModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeFnVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// splitAzureRuntime splits an AWS-Lambda-style "python3.12" string into the
// Azure pair ("python", "3.12"). Returns ("python", "3.12") as a sensible
// default when the input is empty.
func splitAzureRuntime(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "python", "3.12"
	}
	// Try common prefixes. ORDER MATTERS: the longer "nodejs" must be tested
	// before "node", else "nodejs20.x" matches "node" first and leaves "js20.x".
	knownRuntimes := []string{"python", "nodejs", "node", "java", "dotnet", "powershell"}
	for _, r := range knownRuntimes {
		if strings.HasPrefix(s, r) {
			ver := strings.TrimPrefix(s, r)
			// "nodejs" to "node" for Azure; AWS uses nodejs20.x to just "20".
			if r == "nodejs" {
				r = "node"
				ver = strings.TrimSuffix(ver, ".x")
			}
			ver = strings.TrimPrefix(ver, ".")
			if ver == "" {
				ver = "3.12"
			}
			return r, ver
		}
	}
	// Unknown: fall back to python default.
	return "python", "3.12"
}

func buildAzureFnVars(req providers.FunctionRequest) map[string]any {
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

	runtime, version := splitAzureRuntime(spec.Runtime)

	out := map[string]any{
		"location":        location,
		"name_prefix":     namePrefix,
		"environment":     cfgStringDefault(cfg, "environment", "dev"),
		"runtime":         runtime,
		"runtime_version": version,
	}
	// Mirror the AWS Lambda fix: nil maps marshal to JSON null, breaking the
	// module's length() check. Omit env_vars entirely when empty.
	if len(spec.EnvVars) > 0 {
		out["env_vars"] = spec.EnvVars
	}
	return out
}
