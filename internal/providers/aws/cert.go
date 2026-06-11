package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// CertProvisioner: TLS certificate (AWS ACM) via modules/aws-acm-cert.

var _ providers.CertProvisioner = (*Provider)(nil)

func (p *Provider) certModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "aws-acm-cert")
}

// certRegion picks the cert's region. ACM certs are regional; a cert used by an
// ALB or API Gateway lives in the resource's region, but a cert used by
// CloudFront MUST be in us-east-1 - so ForCloudFront wins over an explicit Region.
func certRegion(spec models.CertSpec, cfg map[string]any) string {
	if spec.ForCloudFront {
		return "us-east-1"
	}
	if spec.Region != "" {
		return spec.Region
	}
	return cfgString(cfg, "region")
}

func (p *Provider) writeCertVars(req providers.CertRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildCertVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling cert vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-cert-*.tfvars.json")
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

// PreflightCert validates the var mapping + the aws-acm-cert module offline.
func (p *Provider) PreflightCert(ctx context.Context, req providers.CertRequest) error {
	_, cleanup, err := p.writeCertVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.certModuleDir(), p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, certRegion(req.Spec, req.Config)))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionCert requests the certificate (tofu apply) for the workspace.
func (p *Provider) ProvisionCert(ctx context.Context, req providers.CertRequest) (*providers.CertResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.certModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, certRegion(req.Spec, req.Config), req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeCertVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-cert-*.tfplan")
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
	return &providers.CertResult{
		ARN:        dbOutString(outs, "arn"),
		Domain:     dbOutString(outs, "domain"),
		Status:     dbOutString(outs, "status"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyCert tears down the certificate for the request's workspace.
func (p *Provider) DestroyCert(ctx context.Context, req providers.CertRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.certModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, certRegion(req.Spec, req.Config), req.Spec.TargetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeCertVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildCertVars maps a CertRequest onto the modules/aws-acm-cert inputs.
func buildCertVars(req providers.CertRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	vars := map[string]any{
		"region":             certRegion(spec, cfg),
		"domain":             spec.Domain,
		"validation_zone_id": spec.ValidationZoneID,
		"tags": map[string]string{
			"opord:kind":      "cert",
			"opord:workspace": req.Workspace,
		},
	}
	// A nil slice marshals to JSON null, which breaks the module's list(string)
	// variable - omit it entirely when there are no SANs.
	if len(spec.SubjectAlternativeNames) > 0 {
		vars["subject_alternative_names"] = spec.SubjectAlternativeNames
	}
	return vars
}
