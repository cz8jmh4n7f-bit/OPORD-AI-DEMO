package gcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.CostReporter = (*Provider)(nil)

// ReportCost implements providers.CostReporter for GCP. Unlike AWS Cost Explorer,
// GCP has NO "get my spend" API - actual billed cost lives ONLY in a BigQuery
// billing-export table the operator configures (Billing to Billing export to 
// BigQuery export). OPORD reads it via the BigQuery jobs.query REST API with the
// stdlib (no SDK dep): a single DAILY query grouped by project + service produces
// the rows that the shared providers.AggregateActuals turns into a CostActuals.
// Read-only (a SELECT against the export table). An error means the caller falls
// back to estimates; the missing-config error is actionable and drives a
// "connect" banner.
func (p *Provider) ReportCost(ctx context.Context, q providers.CostQuery) (*providers.CostActuals, error) {
	days := q.Days
	if days <= 0 || days > 365 {
		days = 30
	}

	cfg := q.Config

	// 1. The export table is required - it's the ONLY source of actuals on GCP.
	table := finopsBigQueryTable(cfg)
	if table == "" {
		return nil, fmt.Errorf("GCP cost actuals need a BigQuery billing export - set finops.bigquery_table (project.dataset.table) in the provider config")
	}

	// 2. The query runs/bills in a project: prefer the provider's project_id, else
	// fall back to the table's leading project segment (project.dataset.table).
	project := cfgString(cfg, "project_id")
	if project == "" {
		if seg := strings.SplitN(table, ".", 2); len(seg) == 2 && seg[0] != "" {
			project = seg[0]
		}
	}
	if project == "" {
		return nil, fmt.Errorf("GCP cost actuals need a query project - set project_id in the provider config or use a fully-qualified finops.bigquery_table (project.dataset.table)")
	}

	// 3. OAuth2 access token. Prefer the keyless dynamic token (OpenBao GCP
	// secrets engine, ADR-0010); else mint one from the SA JSON key (same
	// two-legged flow as CheckConnection).
	token, err := gcpCostToken(ctx, q.Credentials)
	if err != nil {
		return nil, err
	}

	// 4 + 5. Run the query and map the result rows.
	rows, err := bigQueryCostRows(ctx, project, table, token, days, q.Account)
	if err != nil {
		return nil, err
	}

	// 6.
	return providers.AggregateActuals(rows, days, time.Now().UTC()), nil
}

// gcpCostToken returns an OAuth2 access token for the BigQuery REST call. It
// reuses the same auth precedence as CheckConnection: a short-lived dynamic
// access_token first (the common, live-proven keyless path), else a token minted
// from the service-account JSON key via gcpAccessToken (defined in check.go).
func gcpCostToken(ctx context.Context, creds map[string]string) (string, error) {
	if tok := creds["access_token"]; tok != "" {
		return tok, nil
	}
	saJSON := gcpCredKeys(creds)
	if saJSON == "" {
		return "", fmt.Errorf("gcp: no credentials for cost query - provide a dynamic access_token (OpenBao GCP engine) or the service-account JSON key")
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return "", fmt.Errorf("gcp: service-account key is not valid JSON: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return "", fmt.Errorf("gcp: service-account key is missing client_email / private_key")
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return gcpAccessToken(tctx, sa.ClientEmail, sa.PrivateKey, tokenURI)
}

// finopsBigQueryTable resolves the billing-export table `project.dataset.table`
// from the provider's finops config, tolerant of nil/typed maps. It accepts either
// a fully-qualified `finops.bigquery_table`, or - the shape the GCP billing-export
// config actually uses - the discrete keys finops.billing_project_id (or
// project_id) + finops.bigquery_dataset + finops.detailed_export_table
// (or export_table).
func finopsBigQueryTable(cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	fin, ok := cfg["finops"].(map[string]any)
	if !ok {
		return ""
	}
	str := func(k string) string {
		if v, ok := fin[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	if t := str("bigquery_table"); t != "" {
		return t
	}
	proj := str("billing_project_id")
	if proj == "" {
		proj = str("project_id")
	}
	dataset := str("bigquery_dataset")
	table := str("detailed_export_table")
	if table == "" {
		table = str("export_table")
	}
	if proj != "" && dataset != "" && table != "" {
		return proj + "." + dataset + "." + table
	}
	return ""
}

// bqQueryRequest is the BigQuery jobs.query request body.
type bqQueryRequest struct {
	Query         string             `json:"query"`
	UseLegacySQL  bool               `json:"useLegacySql"`
	TimeoutMs     int                `json:"timeoutMs"`
	ParameterMode string             `json:"parameterMode,omitempty"`
	QueryParams   []bqQueryParameter `json:"queryParameters,omitempty"`
}

type bqQueryParameter struct {
	Name           string                `json:"name"`
	ParameterType  bqParameterType       `json:"parameterType"`
	ParameterValue bqQueryParameterValue `json:"parameterValue"`
}

type bqParameterType struct {
	Type string `json:"type"`
}

type bqQueryParameterValue struct {
	Value string `json:"value"`
}

// bqQueryResponse is the subset of the jobs.query response OPORD reads.
type bqQueryResponse struct {
	JobComplete bool `json:"jobComplete"`
	Schema      struct {
		Fields []struct {
			Name string `json:"name"`
		} `json:"fields"`
	} `json:"schema"`
	Rows []struct {
		F []struct {
			V json.RawMessage `json:"v"`
		} `json:"f"`
	} `json:"rows"`
	Error  *bqErrorProto  `json:"error"`
	Errors []bqErrorProto `json:"errors"`
}

type bqErrorProto struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// bigQueryCostRows runs a DAILY cost query against the billing-export table and
// maps each result row into a providers.CostRow. The table name is interpolated
// directly into FROM (it comes from trusted operator config and table names
// cannot be query parameters); the trailing-days window and optional project
// filter are NAMED query parameters (@days INT64, @account STRING) to avoid
// injection.
func bigQueryCostRows(ctx context.Context, project, table, token string, days int, account string) ([]providers.CostRow, error) {
	// Backtick-quote the FROM reference (a billing-export table name contains the
	// version suffix). Guard against a stray backtick in operator config.
	if strings.Contains(table, "`") {
		return nil, fmt.Errorf("gcp: invalid finops.bigquery_table %q", table)
	}

	var sb strings.Builder
	sb.WriteString("SELECT project.id AS account, ANY_VALUE(project.name) AS account_name, service.description AS service, ")
	sb.WriteString("FORMAT_TIMESTAMP('%Y-%m-%d', usage_start_time) AS day, SUM(cost) AS cost ")
	sb.WriteString("FROM `" + table + "` ")
	sb.WriteString("WHERE usage_start_time >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL @days DAY) ")
	if account != "" {
		sb.WriteString("AND project.id = @account ")
	}
	sb.WriteString("GROUP BY account, service, day")

	reqBody := bqQueryRequest{
		Query:         sb.String(),
		UseLegacySQL:  false,
		TimeoutMs:     30000,
		ParameterMode: "NAMED",
		QueryParams: []bqQueryParameter{
			{
				Name:           "days",
				ParameterType:  bqParameterType{Type: "INT64"},
				ParameterValue: bqQueryParameterValue{Value: strconv.Itoa(days)},
			},
		},
	}
	if account != "" {
		reqBody.QueryParams = append(reqBody.QueryParams, bqQueryParameter{
			Name:           "account",
			ParameterType:  bqParameterType{Type: "STRING"},
			ParameterValue: bqQueryParameterValue{Value: account},
		})
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gcp: marshal bigquery request: %w", err)
	}

	endpoint := "https://bigquery.googleapis.com/bigquery/v2/projects/" + project + "/queries"
	qctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(qctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gcp: build bigquery request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcp: bigquery query: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out bqQueryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("gcp: bigquery query failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("gcp: parse bigquery response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if msg := bqErrorMessage(&out); msg != "" {
			return nil, fmt.Errorf("gcp: bigquery query failed: %s", msg)
		}
		return nil, fmt.Errorf("gcp: bigquery query failed (HTTP %d)", resp.StatusCode)
	}
	// A 200 can still carry an error proto.
	if msg := bqErrorMessage(&out); msg != "" {
		return nil, fmt.Errorf("gcp: bigquery query error: %s", msg)
	}
	if !out.JobComplete {
		return nil, fmt.Errorf("bigquery query did not complete")
	}

	// Map columns by name from the schema (order is query-defined but resolve by
	// name to be robust).
	idx := map[string]int{}
	for i, f := range out.Schema.Fields {
		idx[f.Name] = i
	}
	iAccount, iName, iService, iDay, iCost := idx["account"], idx["account_name"], idx["service"], idx["day"], idx["cost"]

	rows := make([]providers.CostRow, 0, len(out.Rows))
	for _, r := range out.Rows {
		usd, _ := strconv.ParseFloat(bqCell(r.F, iCost), 64)
		rows = append(rows, providers.CostRow{
			Date:        bqCell(r.F, iDay),
			Account:     bqCell(r.F, iAccount),
			AccountName: bqCell(r.F, iName),
			Service:     bqCell(r.F, iService),
			USD:         usd,
		})
	}
	return rows, nil
}

// bqCell returns the string value of cell i (BigQuery REST encodes every scalar
// value as a JSON string), or "" when the index is out of range or the value is
// JSON null.
func bqCell(f []struct {
	V json.RawMessage `json:"v"`
}, i int) string {
	if i < 0 || i >= len(f) {
		return ""
	}
	raw := f[i].V
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// bqErrorMessage extracts a human error message from a jobs.query response (the
// top-level error proto or the first row-level error), preferring whichever is
// set - these usually say the table doesn't exist or perms are missing.
func bqErrorMessage(out *bqQueryResponse) string {
	if out.Error != nil && out.Error.Message != "" {
		return out.Error.Message
	}
	for _, e := range out.Errors {
		if e.Message != "" {
			return e.Message
		}
	}
	return ""
}
