package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// CDNProvisioner: a CloudFront distribution via modules/aws-cloudfront. CloudFront
// is a global service (no VPC/subnets), so the var mapping stays simple like s3.

var _ providers.CDNProvisioner = (*Provider)(nil)

func (p *Provider) cdnModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "aws-cloudfront")
}

func (p *Provider) writeCDNVars(req providers.CDNRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildCDNVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling cdn vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-cdn-*.tfvars.json")
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

// PreflightCDN validates the var mapping + the aws-cloudfront module offline.
func (p *Provider) PreflightCDN(ctx context.Context, req providers.CDNRequest) error {
	_, cleanup, err := p.writeCDNVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.cdnModuleDir(), p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionCDN creates the distribution (tofu apply) for the workspace.
func (p *Provider) ProvisionCDN(ctx context.Context, req providers.CDNRequest) (*providers.CDNResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.cdnModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeCDNVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-cdn-*.tfplan")
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
	return &providers.CDNResult{
		DomainName:     dbOutString(outs, "domain_name"),
		DistributionID: dbOutString(outs, "distribution_id"),
		ARN:            dbOutString(outs, "arn"),
		HostedZoneID:   dbOutString(outs, "hosted_zone_id"),
		RawOutputs:     rawMap(outs),
	}, nil
}

// DestroyCDN tears down the distribution for the request's workspace.
func (p *Provider) DestroyCDN(ctx context.Context, req providers.CDNRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.cdnModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeCDNVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildCDNVars maps a CDNRequest onto the modules/aws-cloudfront inputs.
func buildCDNVars(req providers.CDNRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := req.Name
	if name == "" {
		name = spec.Name
	}
	originType := spec.OriginType
	if originType == "" {
		originType = "s3"
	}
	priceClass := spec.PriceClass
	if priceClass == "" {
		priceClass = "PriceClass_100"
	}
	vars := map[string]any{
		"region":              cfgString(cfg, "region"),
		"name":                name,
		"origin_type":         originType,
		"origin_domain":       spec.OriginDomain,
		"certificate_arn":     spec.CertificateARN,
		"default_root_object": spec.DefaultRootObject,
		"price_class":         priceClass,
		"tags": map[string]string{
			"opord:kind":      "cdn",
			"opord:workspace": req.Workspace,
		},
	}
	// Omit aliases when empty: a nil slice marshals to JSON null, which overrides
	// the module's default and breaks the distribution's CNAME list.
	if len(spec.Aliases) > 0 {
		vars["aliases"] = spec.Aliases
	}
	return vars
}
