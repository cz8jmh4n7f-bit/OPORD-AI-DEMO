package orchestrator

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// FindAIUsageRecordByImportKey completes the fakeAIQuerier for the import path
// (the request-workflow tests never call it). It scans already-created usage rows
// for the import_key, so a second import of the same data dedupes - exercising the
// idempotency the real SQL FindAIUsageRecordByImportKey provides.
func (f *fakeAIQuerier) FindAIUsageRecordByImportKey(_ context.Context, arg db.FindAIUsageRecordByImportKeyParams) (db.AiUsageRecord, error) {
	for _, u := range f.usageCreates {
		if strings.Contains(string(u.Raw), `"import_key":"`+arg.ImportKey+`"`) {
			return db.AiUsageRecord{ID: uuid.New()}, nil
		}
	}
	return db.AiUsageRecord{}, pgx.ErrNoRows
}

// TestImportAnthropicCosts exercises the real ImportAnthropicCosts against a fake
// Cost Report endpoint serving the documented schema: it asserts the request shape
// (x-api-key admin header, anthropic-version, cost_report path, 1d bucket), the
// cents->USD amount conversion, that the raw record never leaks the key, and that
// a second import dedupes by import_key. Offline (httptest) - a live run needs a
// real sk-ant-admin key.
func TestImportAnthropicCosts(t *testing.T) {
	q := baseFakeAIQuerier(t)
	q.provider.Type = "anthropic"
	q.provider.Name = "anthropic-main"
	q.credential = db.AiProviderCredential{ProviderID: q.provider.ID, SecretRef: "opord/ai/anthropic-main"}

	var gotKey, gotVer, gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVer = r.Header.Get("anthropic-version")
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		// Two cost items: a token line (amount in cents) and a web-search line.
		_, _ = w.Write([]byte(`{
			"data": [{
				"starting_at": "2025-08-01T00:00:00Z",
				"ending_at": "2025-08-02T00:00:00Z",
				"results": [
					{"amount":"123.45","currency":"USD","cost_type":"tokens","description":"Claude Sonnet 4 Usage - Input Tokens","model":"claude-sonnet-4-6","workspace_id":"wrkspc_1","token_type":"uncached_input_tokens"},
					{"amount":"500","currency":"USD","cost_type":"web_search","description":"Web Search","model":"","workspace_id":"wrkspc_1","token_type":""}
				]
			}],
			"has_more": false,
			"next_page": null
		}`))
	}))
	defer srv.Close()
	q.provider.Config = []byte(`{"base_url":"` + srv.URL + `"}`)
	// Two-credential layout (ADR-0022): the secret carries BOTH the inference key
	// (api_key - what check/sync use) and the admin key. The import must pick the
	// admin one.
	svc := New(q, providers.NewRegistry(), fakeAISecretReader{"opord/ai/anthropic-main": {
		"api_key":       "sk-ant-api03-inference",
		"admin_api_key": "sk-ant-admin-test",
	}}, nil, BootstrapConfig{})

	res, err := svc.ImportAnthropicCosts(context.Background(), AnthropicCostImportInput{ProviderName: "anthropic-main"})
	if err != nil {
		t.Fatalf("ImportAnthropicCosts: %v", err)
	}

	// Request shape - and the ADMIN key chosen over the inference key.
	if gotKey != "sk-ant-admin-test" {
		t.Fatalf("x-api-key = %q, want the admin key (admin_api_key preferred)", gotKey)
	}
	if gotVer != "2023-06-01" {
		t.Fatalf("anthropic-version = %q", gotVer)
	}
	if gotPath != "/v1/organizations/cost_report" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotQuery.Get("bucket_width") != "1d" {
		t.Fatalf("bucket_width = %q, want 1d", gotQuery.Get("bucket_width"))
	}

	// Both items imported; amounts are cents -> USD (123.45c = $1.2345; 500c = $5).
	if res.Imported != 2 || res.Skipped != 0 {
		t.Fatalf("imported=%d skipped=%d, want 2/0", res.Imported, res.Skipped)
	}
	if len(q.usageCreates) != 2 {
		t.Fatalf("usageCreates = %d, want 2", len(q.usageCreates))
	}
	if math.Abs(q.usageCreates[0].CostUsd-1.2345) > 1e-9 {
		t.Fatalf("cost[0] = %v, want 1.2345 (cents/100)", q.usageCreates[0].CostUsd)
	}
	if math.Abs(q.usageCreates[1].CostUsd-5.0) > 1e-9 {
		t.Fatalf("cost[1] = %v, want 5.00", q.usageCreates[1].CostUsd)
	}

	// Raw carries the source + model, never the key.
	raw := string(q.usageCreates[0].Raw)
	if !strings.Contains(raw, "anthropic_cost_report") || !strings.Contains(raw, "claude-sonnet-4-6") {
		t.Fatalf("raw missing source/model: %s", raw)
	}
	if strings.Contains(raw, "sk-ant-admin") {
		t.Fatalf("raw leaked the admin key: %s", raw)
	}

	// Idempotent: re-importing the same window skips both rows.
	res2, err := svc.ImportAnthropicCosts(context.Background(), AnthropicCostImportInput{ProviderName: "anthropic-main"})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if res2.Imported != 0 || res2.Skipped != 2 {
		t.Fatalf("second import imported=%d skipped=%d, want 0/2", res2.Imported, res2.Skipped)
	}
}

func TestImportAnthropicCostsRejectsNonAnthropic(t *testing.T) {
	q := baseFakeAIQuerier(t)
	q.provider.Type = "openai"
	q.provider.Name = "openai-main"
	svc := New(q, providers.NewRegistry(), fakeAISecretReader{}, nil, BootstrapConfig{})

	if _, err := svc.ImportAnthropicCosts(context.Background(), AnthropicCostImportInput{ProviderName: "openai-main"}); err == nil {
		t.Fatal("expected an error importing anthropic costs from a non-anthropic provider")
	}
}
