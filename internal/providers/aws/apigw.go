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

// APIGatewayProvisioner: an AWS API Gateway v2 HTTP API + integration (and an
// optional custom domain) via modules/aws-apigw, run in the request's own
// workspace.

var _ providers.APIGatewayProvisioner = (*Provider)(nil)

func (p *Provider) apiGatewayModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "aws-apigw")
}

func (p *Provider) writeAPIGatewayVars(req providers.APIGatewayRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildAPIGatewayVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling apigateway vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-apigw-*.tfvars.json")
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

// PreflightAPIGateway validates the var mapping + the aws-apigw module offline.
func (p *Provider) PreflightAPIGateway(ctx context.Context, req providers.APIGatewayRequest) error {
	_, cleanup, err := p.writeAPIGatewayVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.apiGatewayModuleDir(), p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionAPIGateway creates the HTTP API (tofu apply) for the workspace.
func (p *Provider) ProvisionAPIGateway(ctx context.Context, req providers.APIGatewayRequest) (*providers.APIGatewayResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.apiGatewayModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, firstNonEmpty(req.Spec.Region, ""), req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeAPIGatewayVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-apigw-*.tfplan")
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
	return &providers.APIGatewayResult{
		Endpoint:   dbOutString(outs, "endpoint"),
		APIID:      dbOutString(outs, "api_id"),
		ARN:        dbOutString(outs, "arn"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyAPIGateway tears down the HTTP API for the request's workspace.
func (p *Provider) DestroyAPIGateway(ctx context.Context, req providers.APIGatewayRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.apiGatewayModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, firstNonEmpty(req.Spec.Region, ""), req.Spec.TargetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeAPIGatewayVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildAPIGatewayVars maps an APIGatewayRequest onto the modules/aws-apigw inputs.
func buildAPIGatewayVars(req providers.APIGatewayRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	integrationType := spec.IntegrationType
	if integrationType == "" {
		integrationType = "lambda"
	}
	routeKey := spec.RouteKey
	if routeKey == "" {
		routeKey = "$default"
	}
	vars := map[string]any{
		"region":             firstNonEmpty(spec.Region, cfgString(cfg, "region")),
		"name":               name,
		"integration_type":   integrationType,
		"integration_target": spec.IntegrationTarget,
		"route_key":          routeKey,
		"domain_name":        spec.DomainName,
		"certificate_arn":    spec.CertificateARN,
		"hosted_zone_id":     spec.HostedZoneID,
	}
	return vars
}
