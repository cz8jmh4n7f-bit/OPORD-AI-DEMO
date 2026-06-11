package aws

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// TableProvisioner: managed DynamoDB table via modules/aws-dynamodb. Uniform
// tofu flow - see module.go.

var _ providers.TableProvisioner = (*Provider)(nil)

const dynamoPrefix = "opord-aws-dynamodb"

// PreflightTable validates the var mapping + the aws-dynamodb module offline.
func (p *Provider) PreflightTable(ctx context.Context, req providers.TableRequest) error {
	return p.preflightModule(ctx, p.dynamoModuleDir, dynamoPrefix, req.Credentials, req.Config, buildTableVars(req))
}

// ProvisionTable creates the DynamoDB table (tofu apply) for the workspace.
func (p *Provider) ProvisionTable(ctx context.Context, req providers.TableRequest) (*providers.TableResult, error) {
	outs, err := p.applyModule(ctx, p.dynamoModuleDir, dynamoPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildTableVars(req))
	if err != nil {
		return nil, err
	}
	return &providers.TableResult{
		ARN:        dbOutString(outs, "table_arn"),
		Name:       dbOutString(outs, "table_name"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyTable tears down the table for the request's workspace.
func (p *Provider) DestroyTable(ctx context.Context, req providers.TableRequest) error {
	return p.destroyModule(ctx, p.dynamoModuleDir, dynamoPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildTableVars(req))
}

// buildTableVars maps a TableRequest onto the modules/aws-dynamodb inputs.
func buildTableVars(req providers.TableRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	hashType := spec.HashKeyType
	if hashType == "" {
		hashType = "S"
	}
	rangeType := spec.RangeKeyType
	if rangeType == "" {
		rangeType = "S"
	}
	billing := spec.BillingMode
	if billing == "" {
		billing = "PAY_PER_REQUEST"
	}
	return map[string]any{
		"region":         cfgString(cfg, "region"),
		"name":           name,
		"hash_key":       spec.HashKey,
		"hash_key_type":  hashType,
		"range_key":      spec.RangeKey,
		"range_key_type": rangeType,
		"billing_mode":   billing,
		"read_capacity":  spec.ReadCapacity,
		"write_capacity": spec.WriteCapacity,
	}
}
