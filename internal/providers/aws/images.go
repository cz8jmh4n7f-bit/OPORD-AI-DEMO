package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.ImageLister = (*Provider)(nil)

const maxImages = 200

// ListImages returns available AMIs in a region via EC2 DescribeImages.
// Credentials come from the ambient AWS chain (env vars, shared config, IAM
// role) - the same chain OpenTofu uses - so only the region is threaded through.
// Owner defaults to "self" (the account's own images), which keeps the list
// bounded; pass "amazon" or an account ID to browse public images.
func (p *Provider) ListImages(ctx context.Context, req providers.ImageRequest) ([]providers.Image, error) {
	region := req.Region
	if region == "" {
		region = cfgString(req.Config, "region")
	}
	if region == "" {
		return nil, fmt.Errorf("aws: region is required to list images")
	}

	// Use static credentials resolved from Vault/env when present; otherwise fall
	// back to the ambient AWS credential chain (shared config, IAM role, ...).
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if ak, sk, tok := awsCredKeys(req.Credentials); ak != "" && sk != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(ak, sk, tok)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws: load config: %w", err)
	}
	client := ec2.NewFromConfig(awsCfg)

	out, err := client.DescribeImages(ctx, describeInput(req.Owner))
	if err != nil {
		return nil, fmt.Errorf("aws: describe images in %s: %w", region, err)
	}

	imgs := make([]providers.Image, 0, len(out.Images))
	for _, im := range out.Images {
		imgs = append(imgs, providers.Image{
			ID:          awssdk.ToString(im.ImageId),
			Name:        awssdk.ToString(im.Name),
			Description: awssdk.ToString(im.Description),
			CreatedAt:   awssdk.ToString(im.CreationDate),
			Arch:        string(im.Architecture),
		})
	}
	// Newest first. CreationDate is ISO-8601, so lexical sort is chronological.
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].CreatedAt > imgs[j].CreatedAt })
	if len(imgs) > maxImages {
		imgs = imgs[:maxImages]
	}
	return imgs, nil
}

// describeInput builds the DescribeImages query. owner "public"/"os" returns a
// curated set of common public OS families (Amazon Linux 2023, Ubuntu LTS,
// Debian 12) via their well-known owner accounts + name patterns; any other
// value (default "self") lists that owner's own machine images.
func describeInput(owner string) *ec2.DescribeImagesInput {
	switch owner {
	case "public", "os":
		return &ec2.DescribeImagesInput{
			// amazon, canonical (Ubuntu), debian - stable owner account IDs.
			Owners: []string{"137112412989", "099720109477", "136693071363"},
			Filters: []ec2types.Filter{
				{Name: awssdk.String("state"), Values: []string{"available"}},
				{Name: awssdk.String("image-type"), Values: []string{"machine"}},
				{Name: awssdk.String("architecture"), Values: []string{"x86_64"}},
				{Name: awssdk.String("name"), Values: []string{
					"al2023-ami-2023.*-kernel-*-x86_64",
					"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*",
					"ubuntu/images/hvm-ssd-*/ubuntu-noble-24.04-amd64-server-*",
					"debian-12-amd64-*",
				}},
			},
		}
	default:
		if owner == "" {
			owner = "self"
		}
		return &ec2.DescribeImagesInput{
			Owners: []string{owner},
			Filters: []ec2types.Filter{
				{Name: awssdk.String("state"), Values: []string{"available"}},
				{Name: awssdk.String("image-type"), Values: []string{"machine"}},
			},
		}
	}
}
