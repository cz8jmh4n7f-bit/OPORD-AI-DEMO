package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AIBudgetInput struct {
	Scope            string
	ScopeRef         string
	LimitUSD         float64
	Period           string
	SoftThresholdPct int32
	HardThresholdPct int32
}

type AIBudgetSummary struct {
	Budget       db.AiBudget
	ActualUSD    float64
	RemainingUSD float64
	UsagePct     float64
	Status       string
}

type AIQuotaInput struct {
	ServiceSlug   string
	Metric        string
	LimitQuantity float64
	Period        string
	Enforcement   string
}

type AIAccessPolicyInput struct {
	Name   string
	Rules  map[string]any
	Status string
}

type OpenAICostImportInput struct {
	ProviderName string
	Start        time.Time
	End          time.Time
}

type OpenAICostImportResult struct {
	ProviderName string
	Imported     int
	Skipped      int
	PeriodStart  time.Time
	PeriodEnd    time.Time
}

type AIGatewayResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

func validAIScope(scope string) bool {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "global", "provider", "owner", "workspace", "tenant":
		return true
	default:
		return false
	}
}

func (s *Service) CreateAIBudget(ctx context.Context, in AIBudgetInput) (db.AiBudget, error) {
	scope := strings.TrimSpace(in.Scope)
	if scope == "" {
		scope = "global"
	}
	period := strings.TrimSpace(in.Period)
	if period == "" {
		period = "monthly"
	}
	soft := in.SoftThresholdPct
	if soft == 0 {
		soft = 80
	}
	hard := in.HardThresholdPct
	if hard == 0 {
		hard = 100
	}
	if in.LimitUSD <= 0 {
		return db.AiBudget{}, fmt.Errorf("limit_usd must be greater than zero")
	}
	if soft < 0 || hard < 0 || soft > 100 || hard > 100 {
		return db.AiBudget{}, fmt.Errorf("threshold percentages must be between 0 and 100")
	}
	if soft > hard {
		return db.AiBudget{}, fmt.Errorf("soft_threshold_pct (%d) must not exceed hard_threshold_pct (%d)", soft, hard)
	}
	if !validAIScope(scope) {
		return db.AiBudget{}, fmt.Errorf("scope must be one of global, provider, owner, workspace, tenant")
	}
	// A non-global scope without a reference would silently apply to everything
	// (e.g. an "owner" budget with a blank owner). Require the reference.
	if scope != "global" && strings.TrimSpace(in.ScopeRef) == "" {
		return db.AiBudget{}, fmt.Errorf("scope %q requires a scope reference (the %s it applies to)", scope, scope)
	}
	b, err := s.q.CreateAIBudget(ctx, db.CreateAIBudgetParams{
		TenantID:         tenantForCreate(ctx),
		Scope:            scope,
		ScopeRef:         strings.TrimSpace(in.ScopeRef),
		LimitUsd:         in.LimitUSD,
		Period:           period,
		SoftThresholdPct: soft,
		HardThresholdPct: hard,
	})
	if err != nil {
		return db.AiBudget{}, fmt.Errorf("creating ai budget: %w", err)
	}
	s.emitAIAudit(ctx, "ai_budget", b.ID, "created", "AI budget created", map[string]any{"scope": b.Scope, "scope_ref": b.ScopeRef, "limit_usd": b.LimitUsd}, "")
	return b, nil
}

func (s *Service) ListAIBudgetSummaries(ctx context.Context) ([]AIBudgetSummary, error) {
	budgets, err := s.q.ListAIBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ai budgets: %w", err)
	}
	usage, err := s.ListAIUsageRecords(ctx)
	if err != nil {
		return nil, err
	}
	tid, scoped := scopeTenant(ctx)
	out := make([]AIBudgetSummary, 0, len(budgets))
	for _, b := range budgets {
		if scoped && !tenantVisible(b.TenantID, tid) {
			continue
		}
		actual := aiBudgetActualUSD(b, usage)
		remaining := b.LimitUsd - actual
		pct := 0.0
		if b.LimitUsd > 0 {
			pct = (actual / b.LimitUsd) * 100
		}
		status := "ok"
		if pct >= float64(b.HardThresholdPct) {
			status = "hard_limit"
		} else if pct >= float64(b.SoftThresholdPct) {
			status = "warning"
		}
		out = append(out, AIBudgetSummary{Budget: b, ActualUSD: actual, RemainingUSD: remaining, UsagePct: pct, Status: status})
	}
	return out, nil
}

func aiBudgetActualUSD(b db.AiBudget, usage []db.ListAIUsageRecordsRow) float64 {
	start := aiBudgetPeriodStart(b.Period)
	var total float64
	for _, u := range usage {
		if u.PeriodStart.Before(start) || !aiBudgetScopeMatches(b, u) {
			continue
		}
		total += u.CostUsd
	}
	return total
}

func aiBudgetPeriodStart(period string) time.Time {
	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "yearly", "annual":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
}

func aiBudgetScopeMatches(b db.AiBudget, u db.ListAIUsageRecordsRow) bool {
	ref := strings.TrimSpace(b.ScopeRef)
	switch strings.ToLower(strings.TrimSpace(b.Scope)) {
	case "", "global":
		return true
	case "provider":
		return ref == "" || strings.EqualFold(u.ProviderName, ref) || u.ProviderID.String() == ref
	case "owner":
		return u.Owner != nil && strings.EqualFold(*u.Owner, ref)
	case "workspace":
		return u.Workspace != nil && strings.EqualFold(*u.Workspace, ref)
	case "tenant":
		if ref == "" {
			return !u.TenantID.Valid
		}
		return u.TenantID.Valid && uuid.UUID(u.TenantID.Bytes).String() == ref
	default:
		return false
	}
}

func (s *Service) CreateAIQuota(ctx context.Context, in AIQuotaInput) (db.AiQuota, error) {
	metric := strings.ToLower(strings.TrimSpace(in.Metric))
	if metric == "" {
		metric = "instances"
	}
	if metric == "cost" {
		metric = "cost_usd"
	}
	switch metric {
	case "instances", "seats", "tokens", "requests", "cost_usd":
	default:
		return db.AiQuota{}, fmt.Errorf("metric must be one of instances, seats, tokens, requests, cost_usd")
	}
	period := strings.ToLower(strings.TrimSpace(in.Period))
	if period == "" {
		period = "monthly"
	}
	if period != "daily" && period != "monthly" && period != "yearly" && period != "annual" {
		return db.AiQuota{}, fmt.Errorf("period must be daily, monthly, or yearly")
	}
	enforcement := strings.ToLower(strings.TrimSpace(in.Enforcement))
	if enforcement == "" {
		enforcement = "warn"
	}
	if enforcement != "warn" && enforcement != "block" {
		return db.AiQuota{}, fmt.Errorf("enforcement must be warn or block")
	}
	if in.LimitQuantity <= 0 {
		return db.AiQuota{}, fmt.Errorf("limit_quantity must be greater than zero")
	}
	var serviceID pgtype.UUID
	if strings.TrimSpace(in.ServiceSlug) != "" {
		svc, err := s.q.GetAIServiceBySlug(ctx, strings.TrimSpace(in.ServiceSlug))
		if err != nil {
			return db.AiQuota{}, fmt.Errorf("ai service %q not found: %w", in.ServiceSlug, err)
		}
		serviceID = pgtype.UUID{Bytes: svc.ID, Valid: true}
	}
	q, err := s.q.CreateAIQuota(ctx, db.CreateAIQuotaParams{
		ServiceID:     serviceID,
		TenantID:      tenantForCreate(ctx),
		Metric:        metric,
		LimitQuantity: in.LimitQuantity,
		Period:        period,
		Enforcement:   enforcement,
	})
	if err != nil {
		return db.AiQuota{}, fmt.Errorf("creating ai quota: %w", err)
	}
	s.emitAIAudit(ctx, "ai_quota", q.ID, "created", "AI quota created", map[string]any{"metric": q.Metric, "limit_quantity": q.LimitQuantity, "period": q.Period}, "")
	return q, nil
}

func (s *Service) ListAIQuotas(ctx context.Context) ([]db.AiQuota, error) {
	rows, err := s.q.ListAIQuotas(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ai quotas: %w", err)
	}
	tid, scoped := scopeTenant(ctx)
	if !scoped {
		return rows, nil
	}
	out := make([]db.AiQuota, 0, len(rows))
	for _, r := range rows {
		if tenantVisible(r.TenantID, tid) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Service) CreateAIAccessPolicy(ctx context.Context, in AIAccessPolicyInput) (db.AiAccessPolicy, error) {
	if strings.TrimSpace(in.Name) == "" {
		return db.AiAccessPolicy{}, fmt.Errorf("policy name is required")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	rules := in.Rules
	if rules == nil {
		rules = map[string]any{}
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return db.AiAccessPolicy{}, fmt.Errorf("marshaling policy rules: %w", err)
	}
	// Guard the deny-all footgun: an ACTIVE deny rule with no selectors matches
	// every request and blocks ALL AI access. Require at least one selector, an
	// explicit allow, or a disabled status.
	if status == "active" {
		var rule aiPolicyRule
		if json.Unmarshal(raw, &rule) == nil && rule.isDeny() &&
			len(rule.Providers) == 0 && len(rule.Categories) == 0 && len(rule.Services) == 0 && len(rule.OwnerDomains) == 0 {
			return db.AiAccessPolicy{}, fmt.Errorf("this deny rule has no selectors and would block EVERY AI request; add at least one of providers/categories/services/owner_domains, or create it with status=disabled")
		}
	}
	p, err := s.q.CreateAIAccessPolicy(ctx, db.CreateAIAccessPolicyParams{
		Name:     strings.TrimSpace(in.Name),
		TenantID: tenantForCreate(ctx),
		Rules:    raw,
		Status:   status,
	})
	if err != nil {
		return db.AiAccessPolicy{}, fmt.Errorf("creating ai access policy: %w", err)
	}
	s.emitAIAudit(ctx, "ai_policy", p.ID, "created", "AI access policy created", map[string]any{"name": p.Name, "status": p.Status}, "")
	return p, nil
}

func (s *Service) ListAIAccessPolicies(ctx context.Context) ([]db.AiAccessPolicy, error) {
	rows, err := s.q.ListAIAccessPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ai access policies: %w", err)
	}
	tid, scoped := scopeTenant(ctx)
	if !scoped {
		return rows, nil
	}
	out := make([]db.AiAccessPolicy, 0, len(rows))
	for _, r := range rows {
		if tenantVisible(r.TenantID, tid) {
			out = append(out, r)
		}
	}
	return out, nil
}

// DeleteAIBudget removes a budget so a mistaken one can be undone.
func (s *Service) DeleteAIBudget(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteAIBudget(ctx, id); err != nil {
		return fmt.Errorf("deleting ai budget: %w", err)
	}
	s.emitAIAudit(ctx, "ai_budget", id, "deleted", "AI budget deleted", nil, "")
	return nil
}

// DeleteAIQuota removes a quota.
func (s *Service) DeleteAIQuota(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteAIQuota(ctx, id); err != nil {
		return fmt.Errorf("deleting ai quota: %w", err)
	}
	s.emitAIAudit(ctx, "ai_quota", id, "deleted", "AI quota deleted", nil, "")
	return nil
}

// DeleteAIAccessPolicy removes a policy - the escape hatch for an accidental
// deny-all that would otherwise block every AI request with no way to undo it.
func (s *Service) DeleteAIAccessPolicy(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteAIAccessPolicy(ctx, id); err != nil {
		return fmt.Errorf("deleting ai access policy: %w", err)
	}
	s.emitAIAudit(ctx, "ai_policy", id, "deleted", "AI access policy deleted", nil, "")
	return nil
}

// ExpireAIInstances flips active/suspended instances whose expiry has passed to
// 'expired' and returns how many. Meant for a periodic reaper so a grant does
// not stay "active" forever past its expires_at.
func (s *Service) ExpireAIInstances(ctx context.Context) (int, error) {
	rows, err := s.q.ExpireAIServiceInstances(ctx)
	if err != nil {
		return 0, fmt.Errorf("expiring ai instances: %w", err)
	}
	for _, r := range rows {
		s.emitAIAudit(ctx, "ai_instance", r.ID, "expired", "AI access instance expired", map[string]any{"owner": r.Owner}, "system")
	}
	return len(rows), nil
}

func (s *Service) SyncAIProviderModelsByName(ctx context.Context, name string) error {
	p, err := s.q.GetAIProviderByName(ctx, name)
	if err != nil {
		return fmt.Errorf("ai provider %q not found: %w", name, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && p.TenantID.Valid && !tenantVisible(p.TenantID, tid) {
		return fmt.Errorf("ai provider %q not found", name)
	}
	prov, err := s.aiProvider(p.Type)
	if err != nil {
		return err
	}
	modelProvider, ok := prov.(aiproviders.ModelCatalogProvider)
	if !ok {
		return fmt.Errorf("ai provider %q does not expose a model catalog", p.Type)
	}
	models, err := modelProvider.ListModels(ctx, aiproviders.ModelListRequest{Credentials: s.aiCredentials(ctx, p), Config: aiProviderConfig(p)})
	if err != nil {
		s.emitAIAudit(ctx, "ai_provider", p.ID, "model_sync_failed", err.Error(), map[string]any{"name": p.Name, "type": p.Type}, "")
		return err
	}
	for _, m := range models {
		if strings.TrimSpace(m.Model) == "" {
			continue
		}
		meta := m.Metadata
		if meta == nil {
			meta = map[string]any{}
		}
		raw, _ := json.Marshal(meta)
		_, err := s.q.UpsertAIModelCatalog(ctx, db.UpsertAIModelCatalogParams{
			ProviderID:  p.ID,
			Model:       m.Model,
			DisplayName: firstNonEmpty(m.DisplayName, m.Model),
			Modality:    firstNonEmpty(m.Modality, "text"),
			Status:      firstNonEmpty(m.Status, "active"),
			Metadata:    raw,
		})
		if err != nil {
			return fmt.Errorf("upserting ai model %q: %w", m.Model, err)
		}
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "models_synced", "AI provider models synced", map[string]any{"name": p.Name, "count": len(models)}, "")
	return nil
}

func (s *Service) ListAIModelCatalog(ctx context.Context) ([]db.ListAIModelCatalogRow, error) {
	rows, err := s.q.ListAIModelCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ai model catalog: %w", err)
	}
	return rows, nil
}

func (s *Service) ListAIExpiringInstances(ctx context.Context, days int32) ([]db.ListAIExpiringInstancesRow, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := s.q.ListAIExpiringInstances(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("listing ai renewals: %w", err)
	}
	tid, scoped := scopeTenant(ctx)
	if !scoped {
		return rows, nil
	}
	out := make([]db.ListAIExpiringInstancesRow, 0, len(rows))
	for _, r := range rows {
		if tenantVisible(r.TenantID, tid) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Service) ImportOpenAICosts(ctx context.Context, in OpenAICostImportInput) (OpenAICostImportResult, error) {
	name := strings.TrimSpace(in.ProviderName)
	if name == "" {
		return OpenAICostImportResult{}, fmt.Errorf("provider name is required")
	}
	p, err := s.q.GetAIProviderByName(ctx, name)
	if err != nil {
		return OpenAICostImportResult{}, fmt.Errorf("ai provider %q not found: %w", name, err)
	}
	if p.Type != string(aiproviders.ProviderOpenAI) {
		return OpenAICostImportResult{}, fmt.Errorf("ai provider %q is %q, not openai", p.Name, p.Type)
	}
	creds := s.aiCredentials(ctx, p)
	key := firstNonEmpty(creds["api_key"], creds["openai_api_key"], creds["token"])
	if key == "" {
		return OpenAICostImportResult{}, fmt.Errorf("openai admin api key missing in secret_ref")
	}
	start, end := in.Start, in.End
	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -7)
	}
	if end.IsZero() {
		end = time.Now()
	}
	if !end.After(start) {
		return OpenAICostImportResult{}, fmt.Errorf("end must be after start")
	}
	cfg := aiProviderConfig(p)
	baseURL := "https://api.openai.com"
	if v, ok := cfg["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		baseURL = strings.TrimSpace(v)
	}
	u, _ := url.Parse(strings.TrimRight(baseURL, "/") + "/v1/organization/costs")
	q := u.Query()
	q.Set("start_time", fmt.Sprintf("%d", start.Unix()))
	q.Set("end_time", fmt.Sprintf("%d", end.Unix()))
	q.Set("bucket_width", "1d")
	q.Set("limit", "180")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return OpenAICostImportResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return OpenAICostImportResult{}, fmt.Errorf("openai costs import failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return OpenAICostImportResult{}, fmt.Errorf("openai costs import returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload openAICostsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return OpenAICostImportResult{}, fmt.Errorf("decoding openai costs: %w", err)
	}
	result := OpenAICostImportResult{ProviderName: p.Name, PeriodStart: start, PeriodEnd: end}
	for _, bucket := range payload.Data {
		bs := time.Unix(bucket.StartTime, 0)
		be := time.Unix(bucket.EndTime, 0)
		for idx, item := range bucket.Results {
			importKey := fmt.Sprintf("openai-costs:%d:%d:%s:%s:%d", bucket.StartTime, bucket.EndTime, item.ProjectID, item.LineItem, idx)
			if _, err := s.q.FindAIUsageRecordByImportKey(ctx, db.FindAIUsageRecordByImportKeyParams{
				ProviderID: p.ID, PeriodStart: bs, PeriodEnd: be, Metric: "cost_usd", ImportKey: importKey,
			}); err == nil {
				result.Skipped++
				continue
			} else if err != pgx.ErrNoRows {
				return result, fmt.Errorf("checking ai usage import key: %w", err)
			}
			raw, _ := json.Marshal(map[string]any{
				"source":     "openai_costs_api",
				"import_key": importKey,
				"currency":   item.Amount.Currency,
				"project_id": item.ProjectID,
				"line_item":  item.LineItem,
			})
			if _, err := s.q.CreateAIUsageRecord(ctx, db.CreateAIUsageRecordParams{
				ProviderID: p.ID, PeriodStart: bs, PeriodEnd: be, Metric: "cost_usd", Quantity: item.Amount.Value, Unit: "usd", CostUsd: item.Amount.Value, Raw: raw,
			}); err != nil {
				return result, fmt.Errorf("creating openai cost usage record: %w", err)
			}
			result.Imported++
		}
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "usage_imported", "OpenAI costs imported", map[string]any{"provider": p.Name, "imported": result.Imported, "skipped": result.Skipped}, "")
	return result, nil
}

type openAICostsPayload struct {
	Data []struct {
		StartTime int64 `json:"start_time"`
		EndTime   int64 `json:"end_time"`
		Results   []struct {
			Amount struct {
				Value    float64 `json:"value"`
				Currency string  `json:"currency"`
			} `json:"amount"`
			LineItem  string `json:"line_item"`
			ProjectID string `json:"project_id"`
		} `json:"results"`
	} `json:"data"`
}

// AnthropicCostImportInput / Result mirror the OpenAI cost import above.
type AnthropicCostImportInput struct {
	ProviderName string
	Start        time.Time
	End          time.Time
}
type AnthropicCostImportResult struct {
	ProviderName string
	Imported     int
	Skipped      int
	PeriodStart  time.Time
	PeriodEnd    time.Time
}

// ImportAnthropicCosts pulls org-level spend from the Anthropic Admin Cost Report
// API (GET /v1/organizations/cost_report) and records it as ai_usage_records - the
// Anthropic twin of ImportOpenAICosts. Differences from the OpenAI path, per the
// Anthropic docs: auth is the `x-api-key` header (needs an ADMIN key,
// sk-ant-admin..., in the provider's secret_ref) + `anthropic-version`; the time
// params are RFC 3339 strings (not Unix); `amount` comes back as a decimal string
// in the lowest currency unit (cents), so it is divided by 100 to USD; daily
// buckets only; grouped by workspace_id + description for per-model/line-item
// detail; paginates on has_more/next_page. Idempotent via a per-row import_key.
func (s *Service) ImportAnthropicCosts(ctx context.Context, in AnthropicCostImportInput) (AnthropicCostImportResult, error) {
	name := strings.TrimSpace(in.ProviderName)
	if name == "" {
		return AnthropicCostImportResult{}, fmt.Errorf("provider name is required")
	}
	p, err := s.q.GetAIProviderByName(ctx, name)
	if err != nil {
		return AnthropicCostImportResult{}, fmt.Errorf("ai provider %q not found: %w", name, err)
	}
	if p.Type != string(aiproviders.ProviderAnthropic) {
		return AnthropicCostImportResult{}, fmt.Errorf("ai provider %q is %q, not anthropic", p.Name, p.Type)
	}
	creds := s.aiCredentials(ctx, p)
	key := firstNonEmpty(creds["api_key"], creds["anthropic_api_key"], creds["token"])
	if key == "" {
		return AnthropicCostImportResult{}, fmt.Errorf("anthropic admin api key (sk-ant-admin...) missing in secret_ref")
	}
	start, end := in.Start, in.End
	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -7)
	}
	if end.IsZero() {
		end = time.Now()
	}
	if !end.After(start) {
		return AnthropicCostImportResult{}, fmt.Errorf("end must be after start")
	}
	cfg := aiProviderConfig(p)
	baseURL := "https://api.anthropic.com"
	if v, ok := cfg["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		baseURL = strings.TrimSpace(v)
	}

	result := AnthropicCostImportResult{ProviderName: p.Name, PeriodStart: start, PeriodEnd: end}
	page := ""
	// Daily buckets cap at 31; bound the pagination loop defensively regardless.
	for iter := 0; iter < 40; iter++ {
		u, _ := url.Parse(strings.TrimRight(baseURL, "/") + "/v1/organizations/cost_report")
		q := u.Query()
		q.Set("starting_at", start.UTC().Format(time.RFC3339))
		q.Set("ending_at", end.UTC().Format(time.RFC3339))
		q.Set("bucket_width", "1d")
		q.Add("group_by[]", "workspace_id")
		q.Add("group_by[]", "description")
		q.Set("limit", "31")
		if page != "" {
			q.Set("page", page)
		}
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return result, err
		}
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return result, fmt.Errorf("anthropic cost import failed: %w", err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			msg := string(body)
			if len(msg) > 512 {
				msg = msg[:512]
			}
			return result, fmt.Errorf("anthropic cost import returned %s: %s", resp.Status, strings.TrimSpace(msg))
		}
		var payload anthropicCostReport
		if err := json.Unmarshal(body, &payload); err != nil {
			return result, fmt.Errorf("decoding anthropic cost report: %w", err)
		}
		for _, bucket := range payload.Data {
			bs, _ := time.Parse(time.RFC3339, bucket.StartingAt)
			be, _ := time.Parse(time.RFC3339, bucket.EndingAt)
			for idx, item := range bucket.Results {
				cents, perr := strconv.ParseFloat(strings.TrimSpace(item.Amount), 64)
				if perr != nil {
					continue
				}
				usd := cents / 100.0
				importKey := fmt.Sprintf("anthropic-cost:%s:%s:%s:%s:%s:%d",
					bucket.StartingAt, item.WorkspaceID, item.Description, item.CostType, item.TokenType, idx)
				if _, err := s.q.FindAIUsageRecordByImportKey(ctx, db.FindAIUsageRecordByImportKeyParams{
					ProviderID: p.ID, PeriodStart: bs, PeriodEnd: be, Metric: "cost_usd", ImportKey: importKey,
				}); err == nil {
					result.Skipped++
					continue
				} else if err != pgx.ErrNoRows {
					return result, fmt.Errorf("checking ai usage import key: %w", err)
				}
				raw, _ := json.Marshal(map[string]any{
					"source":       "anthropic_cost_report",
					"import_key":   importKey,
					"currency":     item.Currency,
					"workspace_id": item.WorkspaceID,
					"description":  item.Description,
					"model":        item.Model,
					"cost_type":    item.CostType,
				})
				if _, err := s.q.CreateAIUsageRecord(ctx, db.CreateAIUsageRecordParams{
					ProviderID: p.ID, PeriodStart: bs, PeriodEnd: be, Metric: "cost_usd", Quantity: usd, Unit: "usd", CostUsd: usd, Raw: raw,
				}); err != nil {
					return result, fmt.Errorf("creating anthropic cost usage record: %w", err)
				}
				result.Imported++
			}
		}
		if !payload.HasMore || strings.TrimSpace(payload.NextPage) == "" {
			break
		}
		page = payload.NextPage
	}
	s.emitAIAudit(ctx, "ai_provider", p.ID, "usage_imported", "Anthropic costs imported", map[string]any{"provider": p.Name, "imported": result.Imported, "skipped": result.Skipped}, "")
	return result, nil
}

// anthropicCostReport models the Admin Cost Report response. `amount` is a decimal
// string in cents (e.g. "123.45" = $1.2345); `starting_at`/`ending_at` are RFC 3339.
type anthropicCostReport struct {
	Data []struct {
		StartingAt string `json:"starting_at"`
		EndingAt   string `json:"ending_at"`
		Results    []struct {
			Amount      string `json:"amount"`
			Currency    string `json:"currency"`
			CostType    string `json:"cost_type"`
			Description string `json:"description"`
			Model       string `json:"model"`
			WorkspaceID string `json:"workspace_id"`
			TokenType   string `json:"token_type"`
		} `json:"results"`
	} `json:"data"`
	HasMore  bool   `json:"has_more"`
	NextPage string `json:"next_page"`
}

func (s *Service) GatewayOpenAIResponses(ctx context.Context, providerName string, payload []byte) (AIGatewayResponse, error) {
	name := strings.TrimSpace(providerName)
	if name == "" {
		name = "openai-main"
	}
	p, err := s.q.GetAIProviderByName(ctx, name)
	if err != nil {
		return AIGatewayResponse{}, fmt.Errorf("ai provider %q not found: %w", name, err)
	}
	if p.Type != string(aiproviders.ProviderOpenAI) {
		return AIGatewayResponse{}, fmt.Errorf("ai provider %q is %q, not openai", p.Name, p.Type)
	}
	creds := s.aiCredentials(ctx, p)
	key := firstNonEmpty(creds["api_key"], creds["openai_api_key"], creds["token"])
	if key == "" {
		return AIGatewayResponse{}, fmt.Errorf("openai api key missing in secret_ref")
	}
	// Spend gate: block the proxy when the provider/global budget is exhausted.
	if err := s.evaluateGatewayBudget(ctx, p.Name); err != nil {
		return AIGatewayResponse{}, err
	}
	cfg := aiProviderConfig(p)
	baseURL := "https://api.openai.com"
	if v, ok := cfg["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		baseURL = strings.TrimSpace(v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/responses", bytes.NewReader(payload))
	if err != nil {
		return AIGatewayResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.emitAIAudit(ctx, "ai_gateway", uuid.Nil, "request_failed", err.Error(), map[string]any{"provider": p.Name}, "")
		return AIGatewayResponse{}, fmt.Errorf("openai gateway request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return AIGatewayResponse{}, fmt.Errorf("reading openai gateway response: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	fields := map[string]any{"provider": p.Name, "status_code": resp.StatusCode}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.recordGatewayUsage(ctx, p.ID, body)
		s.emitAIAudit(ctx, "ai_gateway", uuid.Nil, "request_completed", "OpenAI gateway request completed", fields, "")
	} else {
		s.emitAIAudit(ctx, "ai_gateway", uuid.Nil, "request_rejected", "OpenAI gateway request returned an error", fields, "")
	}
	return AIGatewayResponse{StatusCode: resp.StatusCode, ContentType: contentType, Body: body}, nil
}

func (s *Service) recordGatewayUsage(ctx context.Context, providerID uuid.UUID, body []byte) {
	var payload struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens  float64 `json:"input_tokens"`
			OutputTokens float64 `json:"output_tokens"`
			TotalTokens  float64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}
	total := payload.Usage.TotalTokens
	if total == 0 {
		total = payload.Usage.InputTokens + payload.Usage.OutputTokens
	}
	if total == 0 {
		return
	}
	now := time.Now()
	raw, _ := json.Marshal(map[string]any{
		"source":        "opord_gateway_lite",
		"model":         payload.Model,
		"input_tokens":  payload.Usage.InputTokens,
		"output_tokens": payload.Usage.OutputTokens,
	})
	_, _ = s.q.CreateAIUsageRecord(ctx, db.CreateAIUsageRecordParams{
		ProviderID: providerID, PeriodStart: now, PeriodEnd: now, Metric: "tokens", Quantity: total, Unit: "tokens", CostUsd: 0, Raw: raw,
	})
}
