package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// QueueProvisioner: a Pub/Sub topic + pull subscription via modules/gcp-pubsub.
// The provider-neutral "queue" maps onto Pub/Sub; optional DLQ = a dead-letter
// topic + the subscription's dead_letter_policy.

var _ providers.QueueProvisioner = (*Provider)(nil)

func (p *Provider) writeQueueVars(req providers.QueueRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildQueueVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp queue vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-pubsub-*.tfvars.json")
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
	_, cleanup, err := p.writeQueueVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.pubsubModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionQueue(ctx context.Context, req providers.QueueRequest) (*providers.QueueResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.pubsubModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeQueueVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-pubsub-*.tfplan")
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
	return &providers.QueueResult{
		QueueURL:   outString(outs, "queue_url"),
		QueueARN:   outString(outs, "queue_arn"),
		Name:       outString(outs, "name"),
		DLQURL:     outString(outs, "dlq_url"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyQueue(ctx context.Context, req providers.QueueRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.pubsubModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeQueueVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildQueueVars maps a QueueRequest onto the modules/gcp-pubsub inputs. Pub/Sub
// is not FIFO (the spec's FIFO flag is ignored - message ordering is per-key);
// the AWS-only KMS field doesn't apply.
func buildQueueVars(req providers.QueueRequest) map[string]any {
	spec := req.Spec
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	vars := map[string]any{
		"name":        safeName(name, 255),
		"dlq_enabled": spec.DLQEnabled,
		"labels": map[string]string{
			"opord_kind":      "queue",
			"opord_workspace": safeName(req.Workspace, 60),
		},
	}
	if spec.VisibilityTimeoutSeconds > 0 {
		vars["ack_deadline_seconds"] = spec.VisibilityTimeoutSeconds
	}
	if spec.MessageRetentionSeconds > 0 {
		vars["message_retention_seconds"] = spec.MessageRetentionSeconds
	}
	if spec.DLQMaxReceiveCount > 0 {
		vars["dlq_max_delivery_attempts"] = spec.DLQMaxReceiveCount
	}
	return vars
}
