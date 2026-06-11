package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.CostReporter = (*Provider)(nil)

const ceMetric = "UnblendedCost"

// ReportCost implements providers.CostReporter via AWS Cost Explorer. CE is a global
// service whose endpoint lives in us-east-1; the data is org-wide from the payer
// account, so one query reports spend across every linked account. A single DAILY
// query grouped by LINKED_ACCOUNT + SERVICE produces the rows that the shared
// providers.AggregateActuals turns into a CostActuals. Read-only
// (ce:GetCostAndUsage) - an error means the caller falls back to estimates.
func (p *Provider) ReportCost(ctx context.Context, q providers.CostQuery) (*providers.CostActuals, error) {
	days := q.Days
	if days <= 0 || days > 365 {
		days = 30
	}

	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion("us-east-1")}
	if ak, sk, tok := awsCredKeys(q.Credentials); ak != "" && sk != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(ak, sk, tok)))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws: load config: %w", err)
	}
	ce := costexplorer.NewFromConfig(cfg)

	now := time.Now().UTC()
	period := &cetypes.DateInterval{
		Start: aws.String(now.AddDate(0, 0, -days).Format("2006-01-02")),
		End:   aws.String(now.AddDate(0, 0, 1).Format("2006-01-02")), // End is exclusive; +1 day includes today
	}
	var filter *cetypes.Expression
	if q.Account != "" {
		filter = &cetypes.Expression{
			Dimensions: &cetypes.DimensionValues{Key: cetypes.DimensionLinkedAccount, Values: []string{q.Account}},
		}
	}

	rows, err := ceRows(ctx, ce, period, filter)
	if err != nil {
		return nil, err
	}
	return providers.AggregateActuals(rows, days, now), nil
}

// ceRows runs a DAILY GetCostAndUsage grouped by LINKED_ACCOUNT + SERVICE, follows
// pagination, and maps each cell into a providers.CostRow (account name resolved
// from the response's dimension attributes).
func ceRows(ctx context.Context, ce *costexplorer.Client, period *cetypes.DateInterval, filter *cetypes.Expression) ([]providers.CostRow, error) {
	var rows []providers.CostRow
	names := map[string]string{}
	var page *string
	for {
		resp, err := ce.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
			TimePeriod:  period,
			Granularity: cetypes.GranularityDaily,
			Metrics:     []string{ceMetric},
			GroupBy: []cetypes.GroupDefinition{
				{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
				{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
			},
			Filter:        filter,
			NextPageToken: page,
		})
		if err != nil {
			return nil, fmt.Errorf("aws cost explorer: %w", err)
		}
		for _, a := range resp.DimensionValueAttributes {
			if a.Value != nil {
				names[*a.Value] = a.Attributes["description"]
			}
		}
		for _, r := range resp.ResultsByTime {
			date := ""
			if r.TimePeriod != nil && r.TimePeriod.Start != nil {
				date = *r.TimePeriod.Start
			}
			for _, g := range r.Groups {
				if len(g.Keys) < 2 {
					continue
				}
				m, ok := g.Metrics[ceMetric]
				if !ok || m.Amount == nil {
					continue
				}
				amt, _ := strconv.ParseFloat(*m.Amount, 64)
				rows = append(rows, providers.CostRow{Date: date, Account: g.Keys[0], Service: g.Keys[1], USD: amt})
			}
		}
		if resp.NextPageToken == nil || *resp.NextPageToken == "" {
			break
		}
		page = resp.NextPageToken
	}
	// Fill account names (attributes accumulate across pages, so a row built before
	// its account's attribute arrived would otherwise miss the name).
	for i := range rows {
		rows[i].AccountName = names[rows[i].Account]
	}
	return rows, nil
}
