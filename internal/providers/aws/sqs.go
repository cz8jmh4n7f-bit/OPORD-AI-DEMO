package aws

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// QueueProvisioner: message queue via modules/aws-sqs (SQS, standard or FIFO,
// with an optional sibling dead-letter queue + redrive policy). Uniform tofu
// flow - see module.go.

var _ providers.QueueProvisioner = (*Provider)(nil)

const sqsPrefix = "opord-aws-sqs"

// PreflightQueue validates the var mapping + the aws-sqs module offline.
func (p *Provider) PreflightQueue(ctx context.Context, req providers.QueueRequest) error {
	return p.preflightModule(ctx, p.sqsModuleDir, sqsPrefix, req.Credentials, req.Config, buildQueueVars(req))
}

// ProvisionQueue creates the queue (tofu apply) for the workspace.
func (p *Provider) ProvisionQueue(ctx context.Context, req providers.QueueRequest) (*providers.QueueResult, error) {
	outs, err := p.applyModule(ctx, p.sqsModuleDir, sqsPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildQueueVars(req))
	if err != nil {
		return nil, err
	}
	return &providers.QueueResult{
		QueueURL:   dbOutString(outs, "queue_url"),
		QueueARN:   dbOutString(outs, "queue_arn"),
		Name:       dbOutString(outs, "queue_name"),
		DLQURL:     dbOutString(outs, "dlq_url"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyQueue tears down the queue for the request's workspace.
func (p *Provider) DestroyQueue(ctx context.Context, req providers.QueueRequest) error {
	return p.destroyModule(ctx, p.sqsModuleDir, sqsPrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildQueueVars(req))
}

// buildQueueVars maps a QueueRequest onto the modules/aws-sqs inputs. Zero
// values fall back to the module defaults so an unset spec yields a sane
// standard queue.
func buildQueueVars(req providers.QueueRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	vars := map[string]any{
		"region":      cfgString(cfg, "region"),
		"name":        name,
		"fifo":        spec.FIFO,
		"dlq_enabled": spec.DLQEnabled,
		"kms_key_arn": spec.KMSKeyARN,
		"tags": map[string]string{
			"opord:kind":      "queue",
			"opord:workspace": req.Workspace,
		},
	}
	// Only set the numeric tunables when provided, else let the module defaults stand.
	if spec.VisibilityTimeoutSeconds > 0 {
		vars["visibility_timeout_seconds"] = spec.VisibilityTimeoutSeconds
	}
	if spec.MessageRetentionSeconds > 0 {
		vars["message_retention_seconds"] = spec.MessageRetentionSeconds
	}
	if spec.MaxMessageSizeBytes > 0 {
		vars["max_message_size_bytes"] = spec.MaxMessageSizeBytes
	}
	if spec.ReceiveWaitTimeSeconds > 0 {
		vars["receive_wait_time_seconds"] = spec.ReceiveWaitTimeSeconds
	}
	if spec.DLQMaxReceiveCount > 0 {
		vars["dlq_max_receive_count"] = spec.DLQMaxReceiveCount
	}
	return vars
}
