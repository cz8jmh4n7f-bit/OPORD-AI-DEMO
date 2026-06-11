package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.CostReporter = (*Provider)(nil)

// costQueryAPIVersion is the Cost Management Query API version that supports a
// Custom timeframe with Daily granularity grouped by ServiceName.
const costQueryAPIVersion = "2023-03-01"

// ReportCost implements providers.CostReporter via the Azure Cost Management
// Query API. Unlike AWS Cost Explorer (one org-wide payer query), Azure cost
// data is per-subscription, so this reports spend for the single subscription
// this provider manages (config subscription_id). A Custom-timeframe DAILY query
// grouped by ServiceName produces the rows that the shared
// providers.AggregateActuals folds into a CostActuals. Read-only - the SP needs
// "Cost Management Reader" on the subscription; an error means the caller falls
// back to estimates.
func (p *Provider) ReportCost(ctx context.Context, q providers.CostQuery) (*providers.CostActuals, error) {
	days := q.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	now := time.Now().UTC()

	tenantID, clientID, clientSecret := azureCredKeys(q.Credentials)
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("azure: missing service-principal credentials - set this provider's secret-ref to an OpenBao path with tenant_id / client_id / client_secret")
	}

	sub := cfgString(q.Config, "subscription_id")
	if sub == "" {
		sub = firstNonEmpty(q.Credentials["subscription_id"], q.Credentials["arm_subscription_id"])
	}
	if sub == "" {
		return nil, fmt.Errorf("azure: subscription_id not configured - set it in the provider config to report cost actuals")
	}
	// When the caller filters to a different subscription than this provider
	// manages, this provider contributes nothing: return an empty-but-valid
	// report so a multi-cloud merge is unaffected.
	if q.Account != "" && q.Account != sub {
		return providers.AggregateActuals(nil, days, now), nil
	}

	token, err := acquireARMToken(ctx, tenantID, clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	start := now.AddDate(0, 0, -days).Format("2006-01-02")
	end := now.Format("2006-01-02")
	body := map[string]any{
		"type":      "ActualCost",
		"timeframe": "Custom",
		"timePeriod": map[string]any{
			"from": start + "T00:00:00Z",
			"to":   end + "T23:59:59Z",
		},
		"dataset": map[string]any{
			"granularity": "Daily",
			"aggregation": map[string]any{
				"totalCost": map[string]any{"name": "Cost", "function": "Sum"},
			},
			"grouping": []map[string]any{
				{"type": "Dimension", "name": "ServiceName"},
			},
		},
	}

	endpoint := "https://management.azure.com/subscriptions/" + url.PathEscape(sub) +
		"/providers/Microsoft.CostManagement/query?api-version=" + costQueryAPIVersion

	rows, err := costQueryRows(ctx, token, endpoint, body, sub)
	if err != nil {
		return nil, err
	}
	return providers.AggregateActuals(rows, days, now), nil
}

// costMgmtResponse is one page of the Cost Management Query response. Columns
// describe the row tuple; rows are mixed-type arrays (numbers arrive as float64).
type costMgmtResponse struct {
	Properties struct {
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Rows     [][]any `json:"rows"`
		NextLink string  `json:"nextLink"`
	} `json:"properties"`
}

// costQueryRows POSTs the query body to the Cost Management endpoint, maps each
// row into a providers.CostRow, and follows properties.nextLink (POST the SAME
// body to the next URL) until it is empty.
func costQueryRows(ctx context.Context, token, endpoint string, body map[string]any, sub string) ([]providers.CostRow, error) {
	var rows []providers.CostRow
	next := endpoint
	for next != "" {
		page, err := costQueryPage(ctx, token, next, body)
		if err != nil {
			return nil, err
		}

		// Resolve column indexes by name (order is not guaranteed).
		idxCost, idxDate, idxService := -1, -1, -1
		for i, c := range page.Properties.Columns {
			switch c.Name {
			case "Cost":
				idxCost = i
			case "UsageDate":
				idxDate = i
			case "ServiceName":
				idxService = i
			}
		}

		for _, r := range page.Properties.Rows {
			var cost float64
			if idxCost >= 0 && idxCost < len(r) {
				cost, _ = r[idxCost].(float64)
			}
			date := ""
			if idxDate >= 0 && idxDate < len(r) {
				if d, ok := r[idxDate].(float64); ok {
					date = formatUsageDate(int(d))
				}
			}
			service := ""
			if idxService >= 0 && idxService < len(r) {
				service, _ = r[idxService].(string)
			}
			rows = append(rows, providers.CostRow{
				Date:        date,
				Account:     sub,
				AccountName: sub,
				Service:     service,
				USD:         cost,
			})
		}

		next = page.Properties.NextLink
	}
	return rows, nil
}

// costQueryPage POSTs the body to one URL (the initial endpoint or a nextLink)
// and decodes a single page. On non-2xx it returns the response body as the
// error (typically "the SP lacks Cost Management Reader" - actionable).
func costQueryPage(ctx context.Context, token, endpoint string, body map[string]any) (*costMgmtResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("azure: marshal cost query: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("azure: build cost query request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: cost query request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Surface Azure's structured error (code + message) when present, else
		// the raw body - it usually says the SP needs "Cost Management Reader".
		var aer struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &aer)
		if aer.Error.Code != "" {
			return nil, fmt.Errorf("azure: cost query failed (%s): %s", aer.Error.Code, aer.Error.Message)
		}
		return nil, fmt.Errorf("azure: cost query failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var page costMgmtResponse
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("azure: parse cost query response: %w", err)
	}
	return &page, nil
}

// formatUsageDate turns the Cost Management UsageDate integer (e.g. 20260605)
// into a YYYY-MM-DD string. An unparseable value yields "" (the row still
// contributes to totals, just not to the daily trend for that day).
func formatUsageDate(d int) string {
	if d <= 0 {
		return ""
	}
	t, err := time.Parse("20060102", fmt.Sprintf("%08d", d))
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}
