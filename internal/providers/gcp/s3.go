package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// S3Provisioner: object storage via modules/gcp-gcs (a Cloud Storage bucket).
// The provider-neutral "s3" primitive maps onto GCS - Name to bucket, Versioning,
// BlockPublicAccess to public_access_prevention, LifecycleGlacierDays to archive. The
// AWS-only KMSKeyARN is ignored (GCS encrypts at rest by default).

var _ providers.S3Provisioner = (*Provider)(nil)

func (p *Provider) writeGCSVars(req providers.S3Request) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildGCSVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcs vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-gcs-*.tfvars.json")
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

// PreflightS3 validates the var mapping + the gcp-gcs module offline.
func (p *Provider) PreflightS3(ctx context.Context, req providers.S3Request) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeGCSVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.gcsModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionS3 creates the bucket (tofu apply) for the workspace.
func (p *Provider) ProvisionS3(ctx context.Context, req providers.S3Request) (*providers.S3Result, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.gcsModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeGCSVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-gcs-*.tfplan")
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
	return &providers.S3Result{
		BucketID:   outString(outs, "bucket_id"),
		BucketARN:  outString(outs, "bucket_arn"),
		DomainName: outString(outs, "bucket_regional_domain_name"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyS3 tears down the bucket for the request's workspace.
func (p *Provider) DestroyS3(ctx context.Context, req providers.S3Request) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.gcsModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeGCSVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildGCSVars maps an S3Request onto the modules/gcp-gcs inputs.
func buildGCSVars(req providers.S3Request) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	name = safeName(name, 50)

	// Bucket location: an explicit gcs_location, else the provider region, else
	// the EU multi-region.
	location := cfgString(cfg, "gcs_location")
	if location == "" {
		location = cfgStringDefault(cfg, "region", "EU")
	}

	return map[string]any{
		"name":                name,
		"location":            location,
		"versioning":          true,
		"block_public_access": true,
		"archive_after_days":  spec.LifecycleGlacierDays,
		"labels": map[string]string{
			"opord_kind":      "s3",
			"opord_workspace": safeName(req.Workspace, 60),
		},
	}
}

// outString extracts a single string-valued tofu output.
func outString(outs map[string]json.RawMessage, key string) string {
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
