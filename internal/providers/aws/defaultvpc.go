package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// deleteDefaultVPCs strips the default VPC (and its dependencies - default
// subnets + the attached internet gateway) from EVERY enabled region of the
// member account. Default VPCs ship with a permissive default security group
// and are a recurring attack surface, so the account factory's setup phase
// removes them (the reference's "delete default VPCs, all regions").
//
// Why Go (not a tofu module): the aws_default_vpc resource only *adopts* an
// existing default VPC into state - it does not delete it on apply - so there
// is no clean declarative way to guarantee "no default VPC" at provision time.
// The AWS SDK path (same approach OPORD already uses for the AMI lister and the
// connectivity check) is exact and idempotent.
//
// Cross-account, STS-only (DR-3): the master credentials assume the member
// account's bootstrap role; the resulting short-lived credentials drive a
// per-region EC2 client. Idempotent - regions without a default VPC are
// skipped, so re-running after a partial run is safe.
func (p *Provider) deleteDefaultVPCs(ctx context.Context, masterCreds map[string]string, baseRegion, assumeRoleArn string) error {
	if assumeRoleArn == "" {
		return fmt.Errorf("delete default vpcs: assume_role_arn is required")
	}
	if baseRegion == "" {
		baseRegion = "us-east-1"
	}

	// Base config from static master creds (fall back to the ambient chain).
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(baseRegion)}
	if ak, sk, tok := awsCredKeys(masterCreds); ak != "" && sk != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(ak, sk, tok)))
	}
	base, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("aws: load config: %w", err)
	}

	// Hop into the member account via STS AssumeRole (no long-lived creds).
	stsClient := sts.NewFromConfig(base)
	assumed := stscreds.NewAssumeRoleProvider(stsClient, assumeRoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = "opord-delete-default-vpcs"
	})
	memberCfg := base.Copy()
	memberCfg.Credentials = awssdk.NewCredentialsCache(assumed)

	// Only regions the account can actually use (opt-in regions excluded unless
	// enabled). The default (nil AllRegions) already filters to enabled regions.
	regClient := ec2.NewFromConfig(memberCfg, func(o *ec2.Options) { o.Region = baseRegion })
	regOut, err := regClient.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return fmt.Errorf("describe regions: %w", err)
	}

	// Best-effort across regions: log each failure, keep going, surface the first.
	var firstErr error
	deleted := 0
	for _, r := range regOut.Regions {
		region := awssdk.ToString(r.RegionName)
		removed, err := p.deleteDefaultVPCInRegion(ctx, memberCfg, region)
		if err != nil {
			p.log.Error("delete default vpc failed", "region", region, "err", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", region, err)
			}
			continue
		}
		if removed {
			deleted++
		}
	}
	p.log.Info("default-vpc sweep complete", "regions", len(regOut.Regions), "deleted", deleted)
	return firstErr
}

// deleteDefaultVPCInRegion removes the default VPC in one region. Returns
// removed=false (no error) when the region has no default VPC (idempotent).
func (p *Provider) deleteDefaultVPCInRegion(ctx context.Context, cfg awssdk.Config, region string) (bool, error) {
	c := ec2.NewFromConfig(cfg, func(o *ec2.Options) { o.Region = region })

	vpcs, err := c.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{{Name: awssdk.String("isDefault"), Values: []string{"true"}}},
	})
	if err != nil {
		return false, fmt.Errorf("describe vpcs: %w", err)
	}
	if len(vpcs.Vpcs) == 0 {
		return false, nil // already gone - idempotent
	}
	vpcID := awssdk.ToString(vpcs.Vpcs[0].VpcId)

	// 1) Delete default subnets (one per AZ).
	subs, err := c.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{{Name: awssdk.String("vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		return false, fmt.Errorf("describe subnets: %w", err)
	}
	for _, s := range subs.Subnets {
		if _, err := c.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: s.SubnetId}); err != nil {
			return false, fmt.Errorf("delete subnet %s: %w", awssdk.ToString(s.SubnetId), err)
		}
	}

	// 2) Detach + delete the internet gateway(s).
	igws, err := c.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []ec2types.Filter{{Name: awssdk.String("attachment.vpc-id"), Values: []string{vpcID}}},
	})
	if err != nil {
		return false, fmt.Errorf("describe internet gateways: %w", err)
	}
	for _, igw := range igws.InternetGateways {
		id := awssdk.ToString(igw.InternetGatewayId)
		if _, err := c.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId, VpcId: awssdk.String(vpcID),
		}); err != nil {
			return false, fmt.Errorf("detach internet gateway %s: %w", id, err)
		}
		if _, err := c.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
		}); err != nil {
			return false, fmt.Errorf("delete internet gateway %s: %w", id, err)
		}
	}

	// 3) Delete the VPC. Its default route table, NACL and security group are
	// removed automatically with it.
	if _, err := c.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: awssdk.String(vpcID)}); err != nil {
		return false, fmt.Errorf("delete vpc %s: %w", vpcID, err)
	}
	p.log.Info("deleted default vpc", "region", region, "vpc", vpcID)
	return true, nil
}
