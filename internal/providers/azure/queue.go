package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// QueueProvisioner: Azure Service Bus via modules/azure-servicebus. The
// provider-neutral QueueSpec ("message queue") maps onto a Service Bus
// namespace + one queue - so the existing first-class /queues surface works
// for Azure too. AWS-specific QueueSpec fields (FIFO via .fifo suffix, KMS,
// SQS-style retention/visibility seconds) have no direct equivalent and are
// ignored; the namespace SKU + dead-lettering are the Azure analogues.

var _ providers.QueueProvisioner = (*Provider)(nil)

func (p *Provider) servicebusModuleDir() string {
	return p.cfg.ModulesDir + "/azure-servicebus"
}

func (p *Provider) writeServicebusVars(req providers.QueueRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildServicebusVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure servicebus vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-servicebus-*.tfvars.json")
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

func (p *Provider) PreflightQueue(ctx context.Context, req providers.QueueRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeServicebusVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.servicebusModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionQueue(ctx context.Context, req providers.QueueRequest) (*providers.QueueResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.servicebusModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeServicebusVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-servicebus-*.tfplan")
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
	// default_connection_string carries a SAS key - OPORD never persists
	// credentials, so strip it from the raw outputs before they're stored.
	delete(outs, "default_connection_string")
	// Map Service Bus outputs onto the provider-neutral QueueResult:
	// endpoint to QueueURL, namespace_id (full ARM id) to QueueARN,
	// namespace_name to Name. Azure has no separate DLQ URL (dead-lettering is a
	// sub-queue), so DLQURL stays empty.
	return &providers.QueueResult{
		QueueURL:   azureOutString(outs, "endpoint"),
		QueueARN:   azureOutString(outs, "namespace_id"),
		Name:       azureOutString(outs, "namespace_name"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyQueue(ctx context.Context, req providers.QueueRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.servicebusModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeServicebusVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildServicebusVars maps the provider-neutral QueueSpec onto modules/azure-servicebus.
func buildServicebusVars(req providers.QueueRequest) map[string]any {
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

	// The Service Bus namespace is named after the prefix; the queue inside it
	// carries the requested name (sanitised, with a sensible fallback).
	queueName := safePrefix(firstNonEmpty(spec.Name, req.Name, "main"), 40)

	return map[string]any{
		"location":              location,
		"name_prefix":           namePrefix,
		"environment":           cfgStringDefault(cfg, "environment", "dev"),
		"queue_names":           []string{queueName},
		"enable_dead_lettering": spec.DLQEnabled,
	}
}
