package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// StackProvisioner: run an arbitrary OpenTofu root module (the "anything" escape
// hatch). OPORD copies the module to a temp workdir, injects a workspace-isolated
// pg backend, supplies the variables, and runs init/plan/apply/destroy.

const opordBackendFile = "opord_backend.tf"
const opordBackendHCL = "terraform {\n  backend \"pg\" {}\n}\n"

// copyModule copies a root module directory to a fresh temp workdir (skipping
// caches/VCS). The returned cleanup removes the workdir.
func copyModule(src string) (string, func(), error) {
	noop := func() {}
	info, err := os.Stat(src)
	if err != nil {
		return "", noop, fmt.Errorf("stack module %q: %w", src, err)
	}
	if !info.IsDir() {
		return "", noop, fmt.Errorf("stack module %q is not a directory", src)
	}
	dst, err := os.MkdirTemp("", "opord-stack-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(dst) }
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if base := d.Name(); base == ".terraform" || base == ".git" {
				return fs.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		cleanup()
		return "", noop, fmt.Errorf("copying stack module: %w", err)
	}
	return dst, cleanup, nil
}

// prepareStack copies the module and (when withBackend) injects the pg backend.
func prepareStack(req providers.StackRequest, withBackend bool) (string, func(), error) {
	workdir, cleanup, err := copyModule(req.Spec.ModuleDir)
	if err != nil {
		return "", cleanup, err
	}
	if withBackend {
		if err := os.WriteFile(filepath.Join(workdir, opordBackendFile), []byte(opordBackendHCL), 0o644); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("writing backend override: %w", err)
		}
	}
	return workdir, cleanup, nil
}

func writeStackVars(req providers.StackRequest) (string, func(), error) {
	noop := func() {}
	vars := req.Spec.Variables
	if vars == nil {
		vars = map[string]any{}
	}
	data, err := json.Marshal(vars)
	if err != nil {
		return "", noop, fmt.Errorf("marshaling stack vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-stack-*.tfvars.json")
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

// PreflightStack validates the module offline (no backend, no apply).
func (p *Provider) PreflightStack(ctx context.Context, req providers.StackRequest) error {
	workdir, cleanup, err := prepareStack(req, false)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, workdir, p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionStack runs tofu apply on the module and returns its outputs.
func (p *Provider) ProvisionStack(ctx context.Context, req providers.StackRequest) (*providers.StackResult, error) {
	workdir, cleanup, err := prepareStack(req, true)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, workdir, p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, varsCleanup, err := writeStackVars(req)
	if err != nil {
		return nil, err
	}
	defer varsCleanup()

	planFile, err := os.CreateTemp("", "opord-stack-*.tfplan")
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
	outputs := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			outputs[k] = val
		}
	}
	return &providers.StackResult{Outputs: outputs}, nil
}

// DestroyStack runs tofu destroy for the request's workspace.
func (p *Provider) DestroyStack(ctx context.Context, req providers.StackRequest) error {
	workdir, cleanup, err := prepareStack(req, true)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, workdir, p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, varsCleanup, err := writeStackVars(req)
	if err != nil {
		return err
	}
	defer varsCleanup()
	return r.Destroy(ctx, varsFile)
}
