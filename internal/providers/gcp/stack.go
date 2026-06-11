package gcp

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

// StackProvisioner: run an arbitrary OpenTofu root module against GCP (the
// "anything in GCP" escape hatch). OPORD copies the module to a temp workdir,
// injects a workspace-isolated pg backend, supplies the variables, and runs
// init/plan/apply/destroy with the GCP provider's resolved credentials.

var _ providers.StackProvisioner = (*Provider)(nil)

const stackBackendFile = "opord_backend.tf"
const stackBackendHCL = "terraform {\n  backend \"pg\" {}\n}\n"

// copyStackModule copies a root module directory to a fresh temp workdir
// (skipping caches/VCS). The returned cleanup removes the workdir.
func copyStackModule(src string) (string, func(), error) {
	noop := func() {}
	info, err := os.Stat(src)
	if err != nil {
		return "", noop, fmt.Errorf("stack module %q: %w", src, err)
	}
	if !info.IsDir() {
		return "", noop, fmt.Errorf("stack module %q is not a directory", src)
	}
	dst, err := os.MkdirTemp("", "opord-gcp-stack-*")
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

func prepareGCPStack(req providers.StackRequest, withBackend bool) (string, func(), error) {
	workdir, cleanup, err := copyStackModule(req.Spec.ModuleDir)
	if err != nil {
		return "", cleanup, err
	}
	if withBackend {
		if err := os.WriteFile(filepath.Join(workdir, stackBackendFile), []byte(stackBackendHCL), 0o644); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("writing backend override: %w", err)
		}
	}
	return workdir, cleanup, nil
}

func writeGCPStackVars(req providers.StackRequest) (string, func(), error) {
	noop := func() {}
	vars := req.Spec.Variables
	if vars == nil {
		vars = map[string]any{}
	}
	data, err := json.Marshal(vars)
	if err != nil {
		return "", noop, fmt.Errorf("marshaling stack vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-stack-*.tfvars.json")
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

func (p *Provider) PreflightStack(ctx context.Context, req providers.StackRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	workdir, cleanup, err := prepareGCPStack(req, false)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, workdir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionStack(ctx context.Context, req providers.StackRequest) (*providers.StackResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	workdir, cleanup, err := prepareGCPStack(req, true)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, workdir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, varsCleanup, err := writeGCPStackVars(req)
	if err != nil {
		return nil, err
	}
	defer varsCleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-stack-*.tfplan")
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

func (p *Provider) DestroyStack(ctx context.Context, req providers.StackRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	workdir, cleanup, err := prepareGCPStack(req, true)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, workdir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, varsCleanup, err := writeGCPStackVars(req)
	if err != nil {
		return err
	}
	defer varsCleanup()
	return r.Destroy(ctx, varsFile)
}
