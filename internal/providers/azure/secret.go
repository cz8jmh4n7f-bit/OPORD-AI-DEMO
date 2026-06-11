package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// SecretProvisioner: Azure Key Vault via modules/azure-keyvault. The
// provider-neutral SecretSpec ("managed secret") maps onto a Key Vault - the
// secret container - so the existing first-class /secrets surface works for
// Azure too. OPORD provisions the vault only; secret values are set
// out-of-band. The AWS-specific SecretSpec fields (KMSKeyARN, rotation,
// RecoveryWindowDays) have no equivalent here and are ignored.

var _ providers.SecretProvisioner = (*Provider)(nil)

func (p *Provider) keyvaultModuleDir() string {
	return p.cfg.ModulesDir + "/azure-keyvault"
}

func (p *Provider) writeKeyvaultVars(req providers.SecretRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildKeyvaultVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure keyvault vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-keyvault-*.tfvars.json")
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
	_, cleanup, err := p.writeKeyvaultVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.keyvaultModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionSecret(ctx context.Context, req providers.SecretRequest) (*providers.SecretResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.keyvaultModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeKeyvaultVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-keyvault-*.tfplan")
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
	// Map Key Vault outputs onto the provider-neutral SecretResult: vault_name to 
	// Name/SecretID, vault_id (full ARM id) to SecretARN, vault_uri to URI.
	return &providers.SecretResult{
		SecretID:   azureOutString(outs, "vault_name"),
		SecretARN:  azureOutString(outs, "vault_id"),
		Name:       azureOutString(outs, "vault_name"),
		URI:        azureOutString(outs, "vault_uri"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroySecret(ctx context.Context, req providers.SecretRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.keyvaultModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeKeyvaultVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildKeyvaultVars maps the provider-neutral SecretSpec onto modules/azure-keyvault.
// Dev/sandbox stays easy to destroy; prod enables purge protection by default.
func buildKeyvaultVars(req providers.SecretRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	location := cfgString(cfg, "location")
	if location == "" {
		location = "westeurope"
	}

	namePrefix := spec.Name
	if namePrefix == "" {
		namePrefix = req.Name
	}
	if namePrefix == "" {
		namePrefix = "opord-" + safePrefix(req.Workspace, 12)
	} else {
		namePrefix = safePrefix(namePrefix, 18)
	}

	return map[string]any{
		"location":                 location,
		"name_prefix":              namePrefix,
		"environment":              cfgStringDefault(cfg, "environment", "dev"),
		"purge_protection_enabled": cfgBoolDefault(cfg, "azure_keyvault_purge_protection", azureIsProd(cfg)),
	}
}
