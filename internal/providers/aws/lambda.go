package aws

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// FunctionProvisioner: managed AWS Lambda via modules/aws-lambda. Uniform tofu
// flow - see module.go.

var _ providers.FunctionProvisioner = (*Provider)(nil)

const lambdaPrefix = "opord-aws-lambda"

// PreflightFunction validates the var mapping + the aws-lambda module offline.
func (p *Provider) PreflightFunction(ctx context.Context, req providers.FunctionRequest) error {
	return p.preflightModule(ctx, p.lambdaModuleDir, lambdaPrefix, req.Credentials, req.Config, buildFunctionVars(req))
}

// ProvisionFunction creates the Lambda function (tofu apply) for the workspace.
func (p *Provider) ProvisionFunction(ctx context.Context, req providers.FunctionRequest) (*providers.FunctionResult, error) {
	outs, err := p.applyModule(ctx, p.lambdaModuleDir, lambdaPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildFunctionVars(req))
	if err != nil {
		return nil, err
	}
	runtime := req.Spec.Runtime
	if runtime == "" {
		runtime = "python3.12"
	}
	return &providers.FunctionResult{
		ARN:        dbOutString(outs, "function_arn"),
		Name:       dbOutString(outs, "function_name"),
		Runtime:    runtime,
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyFunction tears down the function for the request's workspace.
func (p *Provider) DestroyFunction(ctx context.Context, req providers.FunctionRequest) error {
	return p.destroyModule(ctx, p.lambdaModuleDir, lambdaPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildFunctionVars(req))
}

// buildFunctionVars maps a FunctionRequest onto the modules/aws-lambda inputs.
func buildFunctionVars(req providers.FunctionRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := req.Name
	if name == "" {
		name = "opord-" + req.Workspace
	}
	runtime := spec.Runtime
	if runtime == "" {
		runtime = "python3.12"
	}
	handler := spec.Handler
	if handler == "" {
		handler = "index.handler"
	}
	memory := spec.MemoryMB
	if memory < 1 {
		memory = 128
	}
	timeout := spec.TimeoutSec
	if timeout < 1 {
		timeout = 10
	}
	vars := map[string]any{
		"region":      cfgString(cfg, "region"),
		"name":        name,
		"runtime":     runtime,
		"handler":     handler,
		"memory_mb":   memory,
		"timeout_sec": timeout,
		"s3_bucket":   spec.S3Bucket,
		"s3_key":      spec.S3Key,
	}
	// Only emit env_vars when set: a nil Go map marshals to JSON null, which
	// would override the module's default ({}) and break length(var.env_vars).
	if len(spec.EnvVars) > 0 {
		vars["env_vars"] = spec.EnvVars
	}
	return vars
}
