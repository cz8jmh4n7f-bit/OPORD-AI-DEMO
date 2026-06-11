package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// S3Provisioner: Azure Storage Account (Blob) via modules/azure-storage. The
// provider-neutral S3Spec ("object storage") maps onto an Azure Storage
// Account - so the existing first-class /s3 surface (orchestrator + River +
// API + CLI + web) works for Azure too. The AWS-specific S3Spec fields
// (KMSKeyARN, LifecycleGlacierDays) have no Azure equivalent here and are
// ignored; Versioning + BlockPublicAccess + Name do map.

var _ providers.S3Provisioner = (*Provider)(nil)

func (p *Provider) storageModuleDir() string {
	return p.cfg.ModulesDir + "/azure-storage"
}

func (p *Provider) writeStorageVars(req providers.S3Request) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildStorageVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure storage vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-storage-*.tfvars.json")
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

func (p *Provider) PreflightS3(ctx context.Context, req providers.S3Request) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeStorageVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.storageModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionS3(ctx context.Context, req providers.S3Request) (*providers.S3Result, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.storageModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeStorageVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-storage-*.tfplan")
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
	// Map Azure Storage outputs onto the provider-neutral S3Result fields so
	// the shared /s3 surface renders them: account_name to BucketID, account_id
	// (full ARM id) to BucketARN, primary_blob_endpoint to DomainName.
	return &providers.S3Result{
		BucketID:   azureOutString(outs, "account_name"),
		BucketARN:  azureOutString(outs, "account_id"),
		DomainName: azureOutString(outs, "primary_blob_endpoint"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyS3(ctx context.Context, req providers.S3Request) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.storageModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeStorageVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildStorageVars maps the provider-neutral S3Spec onto modules/azure-storage.
func buildStorageVars(req providers.S3Request) map[string]any {
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

	versioning := spec.Versioning
	blockPublic := spec.BlockPublicAccess
	if azureIsProd(cfg) {
		versioning = true
		blockPublic = true
	}

	return map[string]any{
		"location":                 location,
		"name_prefix":              namePrefix,
		"environment":              cfgStringDefault(cfg, "environment", "dev"),
		"versioning":               versioning,
		"allow_blob_public_access": !blockPublic,
		// A default "data" container so the account is immediately usable.
		"containers": []string{"data"},
	}
}
