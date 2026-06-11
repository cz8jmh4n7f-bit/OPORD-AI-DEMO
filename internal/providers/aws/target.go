package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// Deploy-into-managed-account for AWS (ADR-0013). GCP and Azure just override a
// config value (project_id / subscription_id) because one set of provider creds
// reaches every project/subscription. AWS isolates each member account, so OPORD
// instead STS-AssumeRoles into the target account's cross-account role and runs
// tofu with the resulting short-lived creds - the resource lands in the member
// account with no long-lived creds stored there. Member accounts the factory
// creates ship OrganizationAccountAccessRole (Organizations CreateAccount makes
// it); the role name is overridable via the provider config key "assume_role_name".

// awsTargetEnv returns the tofu env for an AWS run. When targetAccount (a 12-digit
// member account id) is set it assumes arn:aws:iam::<account>:role/<role> with the
// provider's base creds and returns the short-lived creds in the env. Empty
// targetAccount = the provider's own account (unchanged base env, prior behavior).
func (p *Provider) awsTargetEnv(ctx context.Context, creds map[string]string, cfg map[string]any, specRegion, targetAccount string) (map[string]string, error) {
	env := awsTofuEnv(creds, cfg, specRegion)
	if targetAccount == "" {
		return env, nil
	}
	roleName := cfgStringDefault(cfg, "assume_role_name", "OrganizationAccountAccessRole")
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", targetAccount, roleName)
	region := firstNonEmpty(env["AWS_REGION"], cfgString(cfg, "region"), "us-east-1")

	tmp, err := p.assumeRoleCreds(ctx, creds, region, roleARN)
	if err != nil {
		return nil, fmt.Errorf("deploy into account %s: assume %s: %w", targetAccount, roleARN, err)
	}
	env["AWS_ACCESS_KEY_ID"] = tmp.AccessKeyID
	env["AWS_SECRET_ACCESS_KEY"] = tmp.SecretAccessKey
	env["AWS_SESSION_TOKEN"] = tmp.SessionToken
	return env, nil
}

// assumeRoleCreds returns short-lived credentials for roleARN, using the provider's
// base creds (or the ambient AWS chain when none are resolved) as the caller - the
// same STS path the account factory uses to reach member accounts (defaultvpc.go).
func (p *Provider) assumeRoleCreds(ctx context.Context, baseCreds map[string]string, region, roleARN string) (awssdk.Credentials, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if ak, sk, tok := awsCredKeys(baseCreds); ak != "" && sk != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(ak, sk, tok)))
	}
	base, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awssdk.Credentials{}, fmt.Errorf("load aws config: %w", err)
	}
	prov := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(base), roleARN, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = "opord-deploy"
	})
	return prov.Retrieve(ctx)
}

// setTargetEnv sets the AWS tofu env on r, assuming the cross-account role when a
// deploy target is given - the compact call site for the provision/destroy methods.
func (p *Provider) setTargetEnv(ctx context.Context, r *tofu.Runner, creds map[string]string, cfg map[string]any, specRegion, targetAccount string) error {
	env, err := p.awsTargetEnv(ctx, creds, cfg, specRegion, targetAccount)
	if err != nil {
		return err
	}
	r.SetEnv(env)
	return nil
}

// applyTargetSubnets returns the provider config with subnet_ids overridden to
// subnets that exist IN the target member account (ADR-0013). The provider's
// configured subnet_ids live in the provider's OWN account, so VPC-bound resources
// (RDS, ElastiCache) deployed into a member account need THAT account's subnets -
// discovered via the cross-account assumed role. The provider's security_group_ids
// (also own-account) are dropped so the resource falls back to the member VPC's
// default SG. No-op when target_account is empty.
func (p *Provider) applyTargetSubnets(ctx context.Context, creds map[string]string, cfg map[string]any, targetAccount string) (map[string]any, error) {
	if targetAccount == "" {
		return cfg, nil
	}
	region := firstNonEmpty(cfgString(cfg, "region"), "us-east-1")
	roleName := cfgStringDefault(cfg, "assume_role_name", "OrganizationAccountAccessRole")
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", targetAccount, roleName)
	tmp, err := p.assumeRoleCreds(ctx, creds, region, roleARN)
	if err != nil {
		return nil, fmt.Errorf("deploy into account %s: assume %s for subnet discovery: %w", targetAccount, roleARN, err)
	}
	awscfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(tmp.AccessKeyID, tmp.SecretAccessKey, tmp.SessionToken)))
	if err != nil {
		return nil, fmt.Errorf("aws config for subnet discovery: %w", err)
	}
	out, err := ec2.NewFromConfig(awscfg).DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, fmt.Errorf("describe subnets in account %s (%s): %w", targetAccount, region, err)
	}
	// A DB subnet group requires every subnet in ONE VPC - pick the VPC with the most
	// subnets (the account-factory secure VPC has 3 across AZs).
	byVPC := map[string][]string{}
	for _, s := range out.Subnets {
		vpc := awssdk.ToString(s.VpcId)
		byVPC[vpc] = append(byVPC[vpc], awssdk.ToString(s.SubnetId))
	}
	var ids []string
	for _, v := range byVPC {
		if len(v) > len(ids) {
			ids = v
		}
	}
	if len(ids) < 2 {
		return nil, fmt.Errorf("account %s (%s) has <2 subnets in a VPC - create it with create_vpc=true so RDS/cache get a subnet group", targetAccount, region)
	}
	out2 := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		out2[k] = v
	}
	out2["subnet_ids"] = ids
	delete(out2, "security_group_ids") // provider SGs live in the provider's account
	return out2, nil
}
