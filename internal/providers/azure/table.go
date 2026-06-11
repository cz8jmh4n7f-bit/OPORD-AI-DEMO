package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// TableProvisioner: Azure Cosmos DB SQL container via modules/azure-cosmos.
// Cosmos has 5 APIs; SQL (Core) is the one that maps cleanly onto OPORD's
// DynamoDB-style TableSpec (hash key -> partition key path).

func (p *Provider) writeCosmosVars(req providers.TableRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildCosmosVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling cosmos vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-cosmos-*.tfvars.json")
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

func (p *Provider) PreflightTable(ctx context.Context, req providers.TableRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeCosmosVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.cosmosModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionTable(ctx context.Context, req providers.TableRequest) (*providers.TableResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.cosmosModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeCosmosVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-cosmos-*.tfplan")
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
	// Cosmos's ARM resource ID stands in for the AWS-shaped "ARN" field, so the
	// TableResult is comparable across providers.
	return &providers.TableResult{
		ARN:        azureOutString(outs, "account_id"),
		Name:       azureOutString(outs, "container_name"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyTable(ctx context.Context, req providers.TableRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.cosmosModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeCosmosVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildCosmosVars maps the provider-neutral TableSpec onto modules/azure-cosmos
// inputs. Cosmos's partition key path takes the place of DynamoDB's hash key.
func buildCosmosVars(req providers.TableRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	location := cfgString(cfg, "location")
	if location == "" {
		location = "westeurope"
	}

	namePrefix := req.Name
	if namePrefix == "" {
		namePrefix = "opord-" + safePrefix(req.Workspace, 12)
	} else {
		namePrefix = safePrefix(namePrefix, 30)
	}

	tableName := spec.Name
	if tableName == "" {
		tableName = req.Name
	}
	if tableName == "" {
		tableName = "items"
	}

	// Cosmos partition keys are paths (e.g. /id). DynamoDB's HashKey is an
	// attribute name; translate by prefixing "/".
	partitionKey := spec.HashKey
	if partitionKey == "" {
		partitionKey = "id"
	}
	if partitionKey[0] != '/' {
		partitionKey = "/" + partitionKey
	}

	billing := spec.BillingMode
	if billing == "" || billing == "PAY_PER_REQUEST" {
		billing = "SERVERLESS"
	}
	if configured := cfgString(cfg, "cosmos_billing_mode"); configured != "" {
		billing = configured
	}

	throughput := spec.ReadCapacity
	if throughput < 400 {
		throughput = 400
	}

	return map[string]any{
		"location":       location,
		"name_prefix":    namePrefix,
		"environment":    cfgStringDefault(cfg, "environment", "dev"),
		"table_name":     tableName,
		"partition_key":  partitionKey,
		"billing_mode":   billing,
		"throughput":     throughput,
		"max_throughput": 4000,
	}
}
