package aws

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// SecretProvisioner: managed secret via modules/aws-secret (Secrets Manager).
// OPORD creates the secret container only - the plaintext value is set
// out-of-band, so OPORD never holds credentials. Uniform tofu flow - see module.go.

var _ providers.SecretProvisioner = (*Provider)(nil)

const secretPrefix = "opord-aws-secret"

// PreflightSecret validates the var mapping + the aws-secret module offline.
func (p *Provider) PreflightSecret(ctx context.Context, req providers.SecretRequest) error {
	return p.preflightModule(ctx, p.secretModuleDir, secretPrefix, req.Credentials, req.Config, buildSecretVars(req))
}

// ProvisionSecret creates the secret (tofu apply) for the workspace.
func (p *Provider) ProvisionSecret(ctx context.Context, req providers.SecretRequest) (*providers.SecretResult, error) {
	outs, err := p.applyModule(ctx, p.secretModuleDir, secretPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildSecretVars(req))
	if err != nil {
		return nil, err
	}
	return &providers.SecretResult{
		SecretID:   dbOutString(outs, "secret_id"),
		SecretARN:  dbOutString(outs, "secret_arn"),
		Name:       dbOutString(outs, "secret_name"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroySecret tears down the secret for the request's workspace.
func (p *Provider) DestroySecret(ctx context.Context, req providers.SecretRequest) error {
	return p.destroyModule(ctx, p.secretModuleDir, secretPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildSecretVars(req))
}

// buildSecretVars maps a SecretRequest onto the modules/aws-secret inputs.
func buildSecretVars(req providers.SecretRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	recovery := spec.RecoveryWindowDays
	vars := map[string]any{
		"region":               cfgString(cfg, "region"),
		"name":                 name,
		"description":          spec.Description,
		"kms_key_arn":          spec.KMSKeyARN,
		"recovery_window_days": recovery,
		"rotation_lambda_arn":  spec.RotationLambdaARN,
		"rotation_days":        spec.RotationDays,
		"tags": map[string]string{
			"opord:kind":      "secret",
			"opord:workspace": req.Workspace,
		},
	}
	return vars
}
