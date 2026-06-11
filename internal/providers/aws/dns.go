package aws

import (
	"context"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// DNSProvisioner: a Route53 hosted zone via modules/aws-route53-zone. Uniform
// tofu flow - see module.go.

var _ providers.DNSProvisioner = (*Provider)(nil)

const dnsPrefix = "opord-aws-dns"

func (p *Provider) dnsModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "aws-route53-zone")
}

// PreflightDNS validates the var mapping + the aws-route53-zone module offline.
func (p *Provider) PreflightDNS(ctx context.Context, req providers.DNSRequest) error {
	return p.preflightModule(ctx, p.dnsModuleDir(), dnsPrefix, req.Credentials, req.Config, buildDNSVars(req))
}

// ProvisionDNS creates the hosted zone (tofu apply) for the workspace.
func (p *Provider) ProvisionDNS(ctx context.Context, req providers.DNSRequest) (*providers.DNSResult, error) {
	outs, err := p.applyModule(ctx, p.dnsModuleDir(), dnsPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildDNSVars(req))
	if err != nil {
		return nil, err
	}
	return &providers.DNSResult{
		ZoneID:      dbOutString(outs, "zone_id"),
		ZoneName:    dbOutString(outs, "zone_name"),
		NameServers: outStrings(outs, "name_servers"),
		RawOutputs:  rawMap(outs),
	}, nil
}

// DestroyDNS tears down the hosted zone for the request's workspace.
func (p *Provider) DestroyDNS(ctx context.Context, req providers.DNSRequest) error {
	return p.destroyModule(ctx, p.dnsModuleDir(), dnsPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildDNSVars(req))
}

// buildDNSVars maps a DNSRequest onto the modules/aws-route53-zone inputs.
func buildDNSVars(req providers.DNSRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	vars := map[string]any{
		"region":  cfgString(cfg, "region"),
		"name":    name,
		"private": spec.Private,
		"vpc_id":  spec.VPCID,
		"tags": map[string]string{
			"opord:kind":      "dns",
			"opord:workspace": req.Workspace,
		},
	}
	if len(spec.Records) > 0 {
		recs := make([]map[string]any, 0, len(spec.Records))
		for _, rec := range spec.Records {
			recs = append(recs, map[string]any{
				"name":  rec.Name,
				"type":  rec.Type,
				"value": rec.Value,
				"alias": rec.Alias,
				"ttl":   rec.TTL,
			})
		}
		vars["records"] = recs
	}
	return vars
}
