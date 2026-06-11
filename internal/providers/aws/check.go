package aws

import (
	"context"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.Connectivity = (*Provider)(nil)

// CheckConnection implements providers.Connectivity via STS GetCallerIdentity -
// the canonical "are these credentials valid?" call: it needs no IAM permission
// and mutates nothing. Credentials come from the resolved map (Vault/env) when
// present, else the ambient AWS chain (same as tofu). STS is global, so any
// region works; we default to us-east-1 when the provider has none configured.
func (p *Provider) CheckConnection(ctx context.Context, creds map[string]string, config map[string]any) error {
	region := cfgString(config, "region")
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	static := false
	if ak, sk, tok := awsCredKeys(creds); ak != "" && sk != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(ak, sk, tok)))
		static = true
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("aws: load config: %w", err)
	}

	// With no provider/Vault keys we fall back to the ambient AWS chain (env vars,
	// shared config, EC2 instance role). Probe it under a short timeout first so a
	// missing chain fails fast with a clear, actionable message instead of hanging
	// ~15s on the EC2 IMDS lookup (the cryptic "no EC2 IMDS role found" timeout).
	if !static {
		credCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := awsCfg.Credentials.Retrieve(credCtx); err != nil {
			return fmt.Errorf("no AWS credentials available - set this provider's secret-ref to an OpenBao path with access_key/secret_key, or run opord-api with AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY")
		}
	}

	if _, err := sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); err != nil {
		return fmt.Errorf("aws: sts get-caller-identity: %w", err)
	}
	return nil
}
