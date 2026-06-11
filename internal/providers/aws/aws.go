// Package aws implements an infrastructure provider for AWS. It currently
// supports the standalone-VM blueprint (modules/aws-vm = EC2 instances via the
// hashicorp/aws OpenTofu provider). Managed Kubernetes (EKS) is not implemented
// yet, so the k8s-shaped Provider methods return a clear error.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// Config configures the AWS provider.
type Config struct {
	ModulesDir   string
	TofuBin      string
	StateConnStr string
	Logger       *slog.Logger
}

// Provider wraps the AWS OpenTofu modules.
type Provider struct {
	cfg              Config
	vmModuleDir      string
	eksModuleDir     string
	rdsModuleDir     string
	dynamoModuleDir  string
	lambdaModuleDir  string
	s3ModuleDir      string
	secretModuleDir  string
	sqsModuleDir     string
	cacheModuleDir   string
	ssoProjectModDir string
	// account-factory layer modules (L1 master + L2-L5 member-level)
	accountModDir          string
	accountBaselineModDir  string
	accountAccessModDir    string
	secureVpcModDir        string
	securityBaselineModDir string
	log                    *slog.Logger
}

var (
	_ providers.Provider            = (*Provider)(nil)
	_ providers.VMProvisioner       = (*Provider)(nil)
	_ providers.DatabaseProvisioner = (*Provider)(nil)
	_ providers.StackProvisioner    = (*Provider)(nil)
	_ providers.DatabaseSnapshotter = (*Provider)(nil)
)

// New constructs an AWS provider.
func New(cfg Config) *Provider {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Provider{
		cfg:                    cfg,
		vmModuleDir:            filepath.Join(cfg.ModulesDir, "aws-vm"),
		eksModuleDir:           filepath.Join(cfg.ModulesDir, "aws-eks"),
		rdsModuleDir:           filepath.Join(cfg.ModulesDir, "aws-rds"),
		dynamoModuleDir:        filepath.Join(cfg.ModulesDir, "aws-dynamodb"),
		lambdaModuleDir:        filepath.Join(cfg.ModulesDir, "aws-lambda"),
		s3ModuleDir:            filepath.Join(cfg.ModulesDir, "aws-s3-bucket"),
		secretModuleDir:        filepath.Join(cfg.ModulesDir, "aws-secret"),
		sqsModuleDir:           filepath.Join(cfg.ModulesDir, "aws-sqs"),
		cacheModuleDir:         filepath.Join(cfg.ModulesDir, "aws-elasticache"),
		ssoProjectModDir:       filepath.Join(cfg.ModulesDir, "aws-sso-project"),
		accountModDir:          filepath.Join(cfg.ModulesDir, "aws-account"),
		accountBaselineModDir:  filepath.Join(cfg.ModulesDir, "aws-account-baseline"),
		accountAccessModDir:    filepath.Join(cfg.ModulesDir, "aws-account-access"),
		secureVpcModDir:        filepath.Join(cfg.ModulesDir, "aws-secure-vpc"),
		securityBaselineModDir: filepath.Join(cfg.ModulesDir, "aws-security-baseline"),
		log:                    log,
	}
}

// Register adds the AWS provider factory to a registry.
func Register(reg *providers.Registry, cfg Config) {
	reg.Register(models.ProviderAWS, func() providers.Provider { return New(cfg) })
}

func (p *Provider) Type() models.ProviderType { return models.ProviderAWS }

// The k8s-shaped Provider methods (Validate/Preflight/Plan/Provision/Destroy)
// wrap modules/aws-eks and live in eks.go.

// PreflightVM validates the vm var mapping + the aws-vm module offline.
func (p *Provider) PreflightVM(ctx context.Context, req providers.VMRequest) error {
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return fmt.Errorf("marshaling vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-vm-*.tfvars.json")
	if err != nil {
		return fmt.Errorf("creating vars file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing vars file: %w", err)
	}
	_ = f.Close()

	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// buildVMVars maps a VMRequest onto the modules/aws-vm OpenTofu inputs. Cloud
// sizing is by instance type (from config); the spec's template becomes the AMI.
func buildVMVars(req providers.VMRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	// Instance type: per-deploy spec wins, else provider config, else default.
	instanceType := spec.InstanceType
	if instanceType == "" {
		instanceType = cfgStringDefault(cfg, "instance_type", "t3.micro")
	}

	// Region: per-deploy spec wins, else the provider's configured region.
	region := spec.Region
	if region == "" {
		region = cfgString(cfg, "region")
	}

	return map[string]any{
		"region":              region,
		"ami":                 spec.Template,
		"instance_type":       instanceType,
		"vm_count":            spec.Count,
		"name_prefix":         spec.NamePrefix,
		"subnet_id":           cfgString(cfg, "subnet_id"),
		"security_group_ids":  cfgStringSlice(cfg, "security_group_ids"),
		"key_name":            cfgString(cfg, "key_name"),
		"root_volume_gb":      spec.DiskGB,
		"associate_public_ip": spec.PublicIP || cfgBool(cfg, "associate_public_ip", false),
	}
}

// awsCredKeys extracts AWS access key / secret / session token from a resolved
// credentials map (from the creds resolver - Vault or env), tolerating common
// key aliases so users can store them however they like in Vault.
func awsCredKeys(creds map[string]string) (accessKey, secretKey, sessionToken string) {
	accessKey = firstNonEmpty(creds["access_key"], creds["access_key_id"], creds["aws_access_key_id"])
	secretKey = firstNonEmpty(creds["secret_key"], creds["secret_access_key"], creds["aws_secret_access_key"])
	sessionToken = firstNonEmpty(creds["session_token"], creds["aws_session_token"])
	return
}

// awsTofuEnv maps resolved credentials + region onto the standard AWS_* env vars
// the tofu AWS provider reads. Empty values are omitted, so a provider without a
// Vault secret falls back to the process's ambient AWS credential chain (env,
// shared config, IAM role) - preserving the prior behavior.
func awsTofuEnv(creds map[string]string, cfg map[string]any, specRegion string) map[string]string {
	env := map[string]string{}
	ak, sk, tok := awsCredKeys(creds)
	if ak != "" {
		env["AWS_ACCESS_KEY_ID"] = ak
	}
	if sk != "" {
		env["AWS_SECRET_ACCESS_KEY"] = sk
	}
	if tok != "" {
		env["AWS_SESSION_TOKEN"] = tok
	}
	region := specRegion
	if region == "" {
		region = cfgString(cfg, "region")
	}
	if region != "" {
		env["AWS_REGION"] = region
		env["AWS_DEFAULT_REGION"] = region
	}
	return env
}

func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func cfgStringDefault(cfg map[string]any, key, def string) string {
	if s := cfgString(cfg, key); s != "" {
		return s
	}
	return def
}

func cfgBool(cfg map[string]any, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	if v, ok := cfg[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func cfgStringSlice(cfg map[string]any, key string) []string {
	out := []string{}
	if cfg == nil {
		return out
	}
	switch v := cfg[key].(type) {
	case []string:
		return v
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
