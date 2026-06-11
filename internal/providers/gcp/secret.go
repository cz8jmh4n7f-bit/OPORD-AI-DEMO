package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// SecretProvisioner: a Secret Manager secret container via modules/gcp-secret.
// OPORD provisions the container only - the plaintext value is set out-of-band,
// so OPORD never holds it (same contract as AWS Secrets Manager / Azure KV).

var _ providers.SecretProvisioner = (*Provider)(nil)

func (p *Provider) writeSecretVars(req providers.SecretRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildSecretVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp secret vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-secret-*.tfvars.json")
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

func (p *Provider) PreflightSecret(ctx context.Context, req providers.SecretRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeSecretVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.secretModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionSecret(ctx context.Context, req providers.SecretRequest) (*providers.SecretResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.secretModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeSecretVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-secret-*.tfplan")
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
	return &providers.SecretResult{
		SecretID:   outString(outs, "secret_id"),
		SecretARN:  outString(outs, "secret_arn"),
		Name:       outString(outs, "name"),
		URI:        outString(outs, "uri"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroySecret(ctx context.Context, req providers.SecretRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.secretModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeSecretVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildSecretVars maps a SecretRequest onto the modules/gcp-secret inputs. The
// AWS/Azure-only KMS / rotation / recovery-window fields don't apply.
func buildSecretVars(req providers.SecretRequest) map[string]any {
	name := req.Spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	return map[string]any{
		"name": safeName(name, 255),
		"labels": map[string]string{
			"opord_kind":      "secret",
			"opord_workspace": safeName(req.Workspace, 60),
		},
	}
}
