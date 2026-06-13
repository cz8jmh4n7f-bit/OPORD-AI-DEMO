package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	aimock "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/mock"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/auth"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

type fakeAIQuerier struct {
	db.Querier
	provider db.AiProvider
	service  db.GetAIServiceRow
	request  db.Request
	instance db.AiServiceInstance
	listRows []db.ListAIServiceInstancesRow
	budgets  []db.AiBudget
	usage    []db.ListAIUsageRecordsRow
	policies []db.AiAccessPolicy
	quotas   []db.AiQuota
	audit    []db.AiAuditEvent

	createdRequest  *db.CreateRequestParams
	setRequest      *db.SetRequestResourceParams
	revokedInstance bool
	usageCreates    []db.CreateAIUsageRecordParams
	modelUpserts    []db.UpsertAIModelCatalogParams
	credential      db.AiProviderCredential
}

func (f *fakeAIQuerier) CreateRequest(_ context.Context, arg db.CreateRequestParams) (db.Request, error) {
	f.createdRequest = &arg
	id := uuid.New()
	f.request = db.Request{
		ID:          id,
		Name:        arg.Name,
		Environment: arg.Environment,
		Requester:   arg.Requester,
		Kind:        arg.Kind,
		Provider:    arg.Provider,
		Spec:        arg.Spec,
		Status:      "pending_approval",
		TenantID:    arg.TenantID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	return f.request, nil
}

func (f *fakeAIQuerier) GetRequestByName(_ context.Context, _ db.GetRequestByNameParams) (db.Request, error) {
	return f.request, nil
}

func (f *fakeAIQuerier) DecideRequest(_ context.Context, arg db.DecideRequestParams) (db.Request, error) {
	f.request.Status = arg.Status
	f.request.DecidedBy = arg.DecidedBy
	return f.request, nil
}

func (f *fakeAIQuerier) SetRequestResource(_ context.Context, arg db.SetRequestResourceParams) (db.Request, error) {
	f.setRequest = &arg
	f.request.ResourceRef = arg.ResourceRef
	f.request.Status = arg.Status
	return f.request, nil
}

func (f *fakeAIQuerier) GetAIServiceBySlug(_ context.Context, _ string) (db.GetAIServiceBySlugRow, error) {
	return db.GetAIServiceBySlugRow(f.service), nil
}

func (f *fakeAIQuerier) GetAIService(_ context.Context, _ uuid.UUID) (db.GetAIServiceRow, error) {
	return f.service, nil
}

func (f *fakeAIQuerier) GetAIProvider(_ context.Context, _ uuid.UUID) (db.AiProvider, error) {
	return f.provider, nil
}

func (f *fakeAIQuerier) GetAIProviderByName(_ context.Context, _ string) (db.AiProvider, error) {
	return f.provider, nil
}

func (f *fakeAIQuerier) GetAIProviderCredentialByProvider(context.Context, uuid.UUID) (db.AiProviderCredential, error) {
	return f.credential, nil
}

func (f *fakeAIQuerier) CreateAIServiceInstance(_ context.Context, arg db.CreateAIServiceInstanceParams) (db.AiServiceInstance, error) {
	f.instance = db.AiServiceInstance{
		ID:               uuid.New(),
		ServiceID:        arg.ServiceID,
		RequestID:        arg.RequestID,
		ProviderAccessID: arg.ProviderAccessID,
		Owner:            arg.Owner,
		TenantID:         arg.TenantID,
		Workspace:        arg.Workspace,
		Status:           arg.Status,
		Spec:             arg.Spec,
		Observed:         arg.Observed,
		ProvisionedAt:    arg.ProvisionedAt,
		ExpiresAt:        arg.ExpiresAt,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	return f.instance, nil
}

func (f *fakeAIQuerier) CreateAIUsageRecord(_ context.Context, arg db.CreateAIUsageRecordParams) (db.AiUsageRecord, error) {
	f.usageCreates = append(f.usageCreates, arg)
	return db.AiUsageRecord{ID: uuid.New()}, nil
}

func (f *fakeAIQuerier) GetAIServiceInstance(context.Context, uuid.UUID) (db.AiServiceInstance, error) {
	return f.instance, nil
}

func (f *fakeAIQuerier) RevokeAIServiceInstance(_ context.Context, id uuid.UUID) (db.AiServiceInstance, error) {
	f.revokedInstance = true
	f.instance.ID = id
	f.instance.Status = "revoked"
	f.instance.RevokedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return f.instance, nil
}

func (f *fakeAIQuerier) ListAIServiceInstances(context.Context) ([]db.ListAIServiceInstancesRow, error) {
	return f.listRows, nil
}

func (f *fakeAIQuerier) ListAIBudgets(context.Context) ([]db.AiBudget, error) {
	return f.budgets, nil
}

func (f *fakeAIQuerier) ListAIUsageRecords(context.Context) ([]db.ListAIUsageRecordsRow, error) {
	return f.usage, nil
}

// Governance enforcement (ai_enforce.go) consults policies + quotas on every
// AI request; the fake returns none so these tests stay focused on the request
// workflow (governance is exercised separately in ai_enforce_test.go).
func (f *fakeAIQuerier) ListAIAccessPolicies(context.Context) ([]db.AiAccessPolicy, error) {
	return f.policies, nil
}

func (f *fakeAIQuerier) ListAIQuotas(context.Context) ([]db.AiQuota, error) {
	return f.quotas, nil
}

func (f *fakeAIQuerier) UpsertAIModelCatalog(_ context.Context, arg db.UpsertAIModelCatalogParams) (db.AiModelCatalog, error) {
	f.modelUpserts = append(f.modelUpserts, arg)
	return db.AiModelCatalog{ID: uuid.New(), ProviderID: arg.ProviderID, Model: arg.Model}, nil
}

func (f *fakeAIQuerier) CreateAIAuditEvent(_ context.Context, arg db.CreateAIAuditEventParams) (db.AiAuditEvent, error) {
	ev := db.AiAuditEvent{
		ID:          uuid.New(),
		Actor:       arg.Actor,
		TenantID:    arg.TenantID,
		SubjectType: arg.SubjectType,
		SubjectID:   arg.SubjectID,
		Action:      arg.Action,
		Message:     arg.Message,
		Fields:      arg.Fields,
		CreatedAt:   time.Now(),
	}
	f.audit = append(f.audit, ev)
	return ev, nil
}

func newAITestService(q *fakeAIQuerier) *Service {
	reg := aiproviders.NewRegistry()
	aimock.Register(reg)
	svc := New(q, providers.NewRegistry(), nil, nil, BootstrapConfig{})
	svc.SetAIProviders(reg)
	return svc
}

func baseFakeAIQuerier(t *testing.T) *fakeAIQuerier {
	t.Helper()
	providerID := uuid.New()
	serviceID := uuid.New()
	schema, _ := json.Marshal(map[string]any{"fields": []string{"owner"}})
	return &fakeAIQuerier{
		provider: db.AiProvider{ID: providerID, Name: "mock-ai", Type: "mock_ai", Config: []byte(`{"mvp":true}`)},
		service: db.GetAIServiceRow{
			ID:                    serviceID,
			ProviderID:            providerID,
			Name:                  "OpenAI API Access (Mock)",
			Slug:                  "openai-api-mock",
			Category:              "api_access",
			Description:           "Mock",
			RequestSchema:         schema,
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
			Status:                "active",
			ProviderName:          "mock-ai",
			ProviderType:          "mock_ai",
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
		},
	}
}

func TestCreateAIRequestUsesGenericRequestWorkflow(t *testing.T) {
	q := baseFakeAIQuerier(t)
	svc := newAITestService(q)

	req, err := svc.CreateAIRequest(context.Background(), CreateAIRequestInput{
		Name: "ai-access", Requester: "alice@example.com", ServiceSlug: "openai-api-mock", Owner: "team-a", Workspace: "lab",
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Kind != "ai_service" {
		t.Fatalf("kind = %q, want ai_service", req.Kind)
	}
	if q.createdRequest == nil || q.createdRequest.Provider != "mock-ai" {
		t.Fatalf("generic request not created with mock provider: %+v", q.createdRequest)
	}
	if len(q.audit) != 1 || q.audit[0].Action != "created" {
		t.Fatalf("audit events = %+v, want created", q.audit)
	}
}

func TestApproveAIRequestCreatesInstanceAndCompletesRequest(t *testing.T) {
	q := baseFakeAIQuerier(t)
	spec, _ := json.Marshal(AIRequestSpec{ServiceSlug: "openai-api-mock", Owner: "team-a", Workspace: "lab"})
	q.request = db.Request{
		ID: uuid.New(), Name: "ai-access", Environment: "dev", Requester: "alice@example.com",
		Kind: "ai_service", Provider: "mock-ai", Spec: spec, Status: "pending_approval", CreatedAt: time.Now(),
	}
	svc := newAITestService(q)

	if err := svc.ApproveRequest(context.Background(), "ai-access", "dev", "operator@example.com"); err != nil {
		t.Fatal(err)
	}
	if q.instance.ID == uuid.Nil || q.instance.Status != "active" {
		t.Fatalf("instance = %+v, want active instance", q.instance)
	}
	if q.setRequest == nil || q.setRequest.Status != "completed" || q.setRequest.ResourceRef != q.instance.ID.String() {
		t.Fatalf("request update = %+v, want completed with instance id %s", q.setRequest, q.instance.ID)
	}
	if len(q.audit) == 0 || q.audit[len(q.audit)-1].Action != "created" {
		t.Fatalf("audit events = %+v, want instance created", q.audit)
	}
}

func TestRevokeAIInstanceRecordsRevocationAndAudit(t *testing.T) {
	q := baseFakeAIQuerier(t)
	instanceID := uuid.New()
	q.instance = db.AiServiceInstance{
		ID: instanceID, ServiceID: q.service.ID, ProviderAccessID: "mock-ai-123", Status: "active",
	}
	svc := newAITestService(q)

	inst, err := svc.RevokeAIInstance(context.Background(), instanceID, "operator@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !q.revokedInstance || inst.Status != "revoked" {
		t.Fatalf("revoked=%v instance=%+v", q.revokedInstance, inst)
	}
	if len(q.audit) != 1 || q.audit[0].Action != "revoked" {
		t.Fatalf("audit events = %+v, want revoked", q.audit)
	}
}

func TestAIExpiresAtAcceptsDateOnly(t *testing.T) {
	got, err := aiExpiresAt("2026-07-07", time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC), 30)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Format("2006-01-02") != "2026-07-07" {
		t.Fatalf("expires_at = %v, want 2026-07-07", got)
	}
}

func TestRejectAIProviderSecretConfig(t *testing.T) {
	if err := rejectAIProviderSecretConfig(map[string]any{"api_key": "sk-test"}); err == nil {
		t.Fatal("expected api_key in provider config to be rejected")
	}
	if err := rejectAIProviderSecretConfig(map[string]any{"base_url": "https://api.openai.com"}); err != nil {
		t.Fatalf("non-secret config rejected: %v", err)
	}
}

func TestCreateAIRequestRejectsBadExpiresAt(t *testing.T) {
	q := baseFakeAIQuerier(t)
	svc := newAITestService(q)
	_, err := svc.CreateAIRequest(context.Background(), CreateAIRequestInput{
		Name: "ai-access", Requester: "alice@example.com", ServiceSlug: "openai-api-mock",
		Owner: "team-a", Workspace: "lab", ExpiresAt: "not-a-date",
	})
	if err == nil || !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("err = %v, want an expires_at format error", err)
	}
}

func TestRevokeAIInstanceRejectsTerminalState(t *testing.T) {
	q := baseFakeAIQuerier(t)
	id := uuid.New()
	q.instance = db.AiServiceInstance{ID: id, ServiceID: q.service.ID, ProviderAccessID: "mock-ai-123", Status: "revoked"}
	svc := newAITestService(q)

	_, err := svc.RevokeAIInstance(context.Background(), id, "operator@example.com")
	if err == nil || !strings.Contains(err.Error(), "not revocable") {
		t.Fatalf("err = %v, want a not-revocable error", err)
	}
	if q.revokedInstance {
		t.Fatal("RevokeAIServiceInstance must not run for a terminal-state instance")
	}
}

func TestListAIInstancesTenantScoped(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	q := baseFakeAIQuerier(t)
	q.listRows = []db.ListAIServiceInstancesRow{
		{ID: uuid.New(), TenantID: pgtype.UUID{Bytes: tenantA, Valid: true}, Status: "active"},
		{ID: uuid.New(), TenantID: pgtype.UUID{Bytes: tenantB, Valid: true}, Status: "active"},
		{ID: uuid.New(), Status: "active"},
	}
	svc := newAITestService(q)

	rows, err := svc.ListAIInstances(ctxWithIdentity(t, auth.Identity{
		Email: "alice@example.com", TenantID: tenantA, Role: auth.RoleOperator,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || uuid.UUID(rows[0].TenantID.Bytes) != tenantA {
		t.Fatalf("rows = %+v, want only tenant A", rows)
	}
}

func TestAIBudgetSummaryCalculatesCurrentPeriodSpend(t *testing.T) {
	q := baseFakeAIQuerier(t)
	q.budgets = []db.AiBudget{{
		ID: uuid.New(), Scope: "provider", ScopeRef: "mock-ai", LimitUsd: 10,
		Period: "monthly", SoftThresholdPct: 50, HardThresholdPct: 90,
	}}
	q.usage = []db.ListAIUsageRecordsRow{
		{ProviderID: q.provider.ID, ProviderName: "mock-ai", PeriodStart: time.Now(), CostUsd: 6},
		{ProviderID: q.provider.ID, ProviderName: "other", PeriodStart: time.Now(), CostUsd: 100},
	}
	svc := newAITestService(q)

	rows, err := svc.ListAIBudgetSummaries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ActualUSD != 6 || rows[0].Status != "warning" {
		t.Fatalf("budget summary = %+v, want 6 USD warning", rows)
	}
}

func TestSyncAIProviderModelsUpsertsCatalog(t *testing.T) {
	q := baseFakeAIQuerier(t)
	svc := newAITestService(q)

	if err := svc.SyncAIProviderModelsByName(context.Background(), "mock-ai"); err != nil {
		t.Fatal(err)
	}
	if len(q.modelUpserts) < 2 {
		t.Fatalf("model upserts = %+v, want mock catalog entries", q.modelUpserts)
	}
	if len(q.audit) == 0 || q.audit[len(q.audit)-1].Action != "models_synced" {
		t.Fatalf("audit events = %+v, want models_synced", q.audit)
	}
}

func TestGatewayOpenAIResponsesRecordsUsageWithoutPromptLeak(t *testing.T) {
	q := baseFakeAIQuerier(t)
	q.provider.Type = "openai"
	q.provider.Name = "openai-main"
	q.credential = db.AiProviderCredential{ProviderID: q.provider.ID, SecretRef: "opord/ai/openai-main"}
	var gotAuth string
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %s, want /v1/responses", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","model":"gpt-test","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`))
	}))
	defer openai.Close()
	q.provider.Config = []byte(`{"base_url":"` + openai.URL + `"}`)
	svc := New(q, providers.NewRegistry(), fakeAISecretReader{"opord/ai/openai-main": {"api_key": "sk-test"}}, nil, BootstrapConfig{})

	res, err := svc.GatewayOpenAIResponses(context.Background(), "openai-main", []byte(`{"model":"gpt-test","input":"secret prompt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(res.Body), "resp_test") {
		t.Fatalf("gateway response = %d %s", res.StatusCode, string(res.Body))
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
	if len(q.usageCreates) != 1 || q.usageCreates[0].Quantity != 7 {
		t.Fatalf("usage creates = %+v, want 7 tokens", q.usageCreates)
	}
	if strings.Contains(string(q.usageCreates[0].Raw), "secret prompt") {
		t.Fatalf("gateway usage raw leaked prompt: %s", string(q.usageCreates[0].Raw))
	}
}

type fakeAISecretReader map[string]map[string]string

func (f fakeAISecretReader) Resolve(context.Context, db.Provider) (map[string]string, error) {
	return nil, nil
}

func (f fakeAISecretReader) ResolveConfig(context.Context, db.Provider) (map[string]any, error) {
	return nil, nil
}

func (f fakeAISecretReader) ReadSecret(_ context.Context, path string) (map[string]string, error) {
	return f[path], nil
}

func ctxWithIdentity(t *testing.T, id auth.Identity) context.Context {
	t.Helper()
	var got context.Context
	resolve := func(context.Context, string) (auth.Identity, bool) { return id, true }
	auth.Middleware(resolve, true)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), requestWithToken())
	if got == nil {
		t.Fatal("identity middleware did not set context")
	}
	return got
}

func requestWithToken() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer test-token")
	return r
}
