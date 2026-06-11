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

// FunctionProvisioner: a 2nd-gen Cloud Function via modules/gcp-function. With no
// external source the module zips a built-in hello handler so the function is
// immediately deployable (mirrors the AWS Lambda card).

var _ providers.FunctionProvisioner = (*Provider)(nil)

func (p *Provider) writeFunctionVars(req providers.FunctionRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildFunctionVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp function vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-fn-*.tfvars.json")
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
	_, cleanup, err := p.writeFunctionVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.functionModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionFunction(ctx context.Context, req providers.FunctionRequest) (*providers.FunctionResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.functionModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeFunctionVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-fn-*.tfplan")
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
	return &providers.FunctionResult{
		ARN:        outString(outs, "arn"),
		Name:       outString(outs, "name"),
		Runtime:    gcpRuntime(req.Spec.Runtime),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyFunction(ctx context.Context, req providers.FunctionRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.functionModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeFunctionVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildFunctionVars maps a FunctionRequest onto the modules/gcp-function inputs.
func buildFunctionVars(req providers.FunctionRequest) map[string]any {
	spec := req.Spec
	name := req.Name
	if name == "" {
		name = "opord-" + req.Workspace
	}
	entry := spec.Handler
	if entry == "" {
		entry = "hello"
	}
	memory := spec.MemoryMB
	if memory <= 0 {
		memory = 256
	}
	timeout := spec.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}

	// Spec region wins over the provider's configured region (mirrors the VM
	// provider); empty falls back to the provider config, then europe-west1.
	region := spec.Region
	if region == "" {
		region = cfgStringDefault(req.Config, "region", "europe-west1")
	}

	vars := map[string]any{
		"name":        safeName(name, 60),
		"region":      region,
		"runtime":     gcpRuntime(spec.Runtime),
		"entry_point": entry,
		"memory_mb":   memory,
		"timeout_sec": timeout,
		"labels": map[string]string{
			"opord_kind":      "function",
			"opord_workspace": safeName(req.Workspace, 60),
		},
	}
	// An external source (existing GCS object) overrides the built-in hello.
	if spec.S3Bucket != "" && spec.S3Key != "" {
		vars["source_bucket"] = spec.S3Bucket
		vars["source_object"] = spec.S3Key
	}
	// Only emit env_vars when non-empty (a nil map marshals to null).
	if len(spec.EnvVars) > 0 {
		vars["env_vars"] = spec.EnvVars
	}
	return vars
}

// gcpRuntime maps a loose runtime ("python3.12", "nodejs20.x", "go1.22") onto a
// Cloud Functions runtime string ("python312", "nodejs20", "go122").
func gcpRuntime(rt string) string {
	rt = strings.TrimSpace(strings.ToLower(rt))
	if rt == "" {
		return "python312"
	}
	rt = strings.TrimSuffix(rt, ".x")
	for _, fam := range []string{"python", "nodejs", "go", "java", "dotnet", "ruby", "php"} {
		if strings.HasPrefix(rt, fam) {
			ver := strings.TrimPrefix(rt, fam)
			ver = strings.ReplaceAll(ver, ".", "")
			return fam + ver
		}
	}
	return strings.ReplaceAll(rt, ".", "")
}
