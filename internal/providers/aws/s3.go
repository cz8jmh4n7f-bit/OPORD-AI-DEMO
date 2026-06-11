package aws

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// S3Provisioner: object storage bucket via modules/aws-s3-bucket. Uniform tofu
// flow - see module.go.

var _ providers.S3Provisioner = (*Provider)(nil)

const s3Prefix = "opord-aws-s3"

// PreflightS3 validates the var mapping + the aws-s3-bucket module offline.
func (p *Provider) PreflightS3(ctx context.Context, req providers.S3Request) error {
	return p.preflightModule(ctx, p.s3ModuleDir, s3Prefix, req.Credentials, req.Config, buildS3Vars(req))
}

// ProvisionS3 creates the bucket (tofu apply) for the workspace.
func (p *Provider) ProvisionS3(ctx context.Context, req providers.S3Request) (*providers.S3Result, error) {
	outs, err := p.applyModule(ctx, p.s3ModuleDir, s3Prefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildS3Vars(req))
	if err != nil {
		return nil, err
	}
	return &providers.S3Result{
		BucketID:   dbOutString(outs, "bucket_id"),
		BucketARN:  dbOutString(outs, "bucket_arn"),
		DomainName: dbOutString(outs, "bucket_regional_domain_name"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyS3 tears down the bucket for the request's workspace. AWS requires the
// bucket to be empty before deletion; the module intentionally does not force
// delete user data.
func (p *Provider) DestroyS3(ctx context.Context, req providers.S3Request) error {
	return p.destroyModule(ctx, p.s3ModuleDir, s3Prefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildS3Vars(req))
}

// buildS3Vars maps an S3Request onto the modules/aws-s3-bucket inputs.
func buildS3Vars(req providers.S3Request) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	// Safe catalog defaults: with the current bool-shaped model we cannot tell
	// omitted from explicit false, so the first-class primitive keeps buckets
	// private and versioned. Use the generic Stack escape hatch for unusual
	// public/static-website buckets until the model grows tri-state booleans.
	versioning := true
	blockPublicAccess := true
	vars := map[string]any{
		"region":                 cfgString(cfg, "region"),
		"name":                   name,
		"versioning":             versioning,
		"block_public_access":    blockPublicAccess,
		"kms_key_arn":            spec.KMSKeyARN,
		"lifecycle_glacier_days": spec.LifecycleGlacierDays,
		"tags": map[string]string{
			"opord:kind":      "s3",
			"opord:workspace": req.Workspace,
		},
	}
	return vars
}
