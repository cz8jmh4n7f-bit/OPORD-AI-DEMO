// Package openai implements OPORD's OpenAI governance provider.
package openai

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// Provider grants governance records for OpenAI/ChatGPT access. It validates
// credentials with OpenAI when an API key is configured, but it does not create
// users, projects, or licenses in the provider tenant.
type Provider struct {
	client *http.Client
}

// Register wires OpenAIProvider into the AI provider registry.
func Register(r *aiproviders.Registry) {
	r.Register(aiproviders.ProviderOpenAI, func() aiproviders.Provider {
		return Provider{client: &http.Client{Timeout: 10 * time.Second}}
	})
}

func (Provider) Type() aiproviders.ProviderType { return aiproviders.ProviderOpenAI }

func (p Provider) ValidateCredentials(ctx context.Context, req aiproviders.CredentialRequest) error {
	key := apiKey(req.Credentials, req.Config)
	if key == "" {
		key = strings.TrimSpace(req.Credentials["admin_api_key"])
	}
	if key == "" {
		return fmt.Errorf("openai api key missing (set secret_ref to an OpenBao secret with api_key or admin_api_key, or OPENAI_API_KEY)")
	}
	base := strings.TrimRight(baseURL(req.Config, "https://api.openai.com"), "/")
	// OpenAI admin keys (sk-admin-…) are scoped to the Administration API and CANNOT
	// call /v1/models, so validate them against a lightweight admin endpoint instead -
	// a valid admin key (the one used for Org Admin) then checks GREEN rather than a
	// misleading 403. Project/standard keys validate against /v1/models.
	checkPath, scope := "/v1/models", "model access"
	if strings.HasPrefix(key, "sk-admin-") {
		checkPath, scope = "/v1/organization/projects?limit=1", "org admin"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+checkPath, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	resp, err := p.http().Do(httpReq)
	if err != nil {
		return fmt.Errorf("openai credential check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("openai credential check returned %s for %s - an admin key (sk-admin-…) only reaches /v1/organization/*, a project key (sk-proj-…) reaches /v1/models; store the matching key as admin_api_key / api_key", resp.Status, scope)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openai credential check returned %s", resp.Status)
	}
	return nil
}

func (Provider) ListAvailableServices(context.Context, aiproviders.ServiceListRequest) ([]aiproviders.Service, error) {
	return []aiproviders.Service{
		{
			Name:                  "OpenAI API Access",
			Slug:                  "openai-api-access",
			Category:              "api_access",
			Description:           "Governed access to OpenAI API keys, projects, or model usage.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "ChatGPT Team/Enterprise Access",
			Slug:                  "chatgpt-access",
			Category:              "developer_tool",
			Description:           "Governed ChatGPT seat/access request tracked by OPORD.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "plan", "expires_at"),
			DefaultExpirationDays: 90,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Responses Gateway",
			Slug:                  "openai-responses-gateway",
			Category:              "gateway",
			Description:           "Use OpenAI Responses API through OPORD with audit, budget gates, and DLP controls.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "tools", "max_output_tokens", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Model Permissions",
			Slug:                  "openai-model-permissions",
			Category:              "project_control",
			Description:           "Request an OpenAI project model allowlist or denylist managed through OPORD.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "project_id", "mode", "model_ids", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Hosted Tool Permissions",
			Slug:                  "openai-hosted-tools",
			Category:              "project_control",
			Description:           "Govern access to hosted tools such as web search, file search, code interpreter, image generation, and MCP.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "project_id", "web_search", "file_search", "code_interpreter", "image_generation", "mcp", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Embeddings",
			Slug:                  "openai-embeddings",
			Category:              "model_usage",
			Description:           "Governed access to embedding models for search, ranking, and retrieval workloads.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "estimated_tokens_per_month", "expires_at"),
			DefaultExpirationDays: 60,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Image Generation",
			Slug:                  "openai-image-generation",
			Category:              "media_generation",
			Description:           "Governed access to OpenAI image generation and editing models.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "estimated_images_per_month", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Audio and Transcription",
			Slug:                  "openai-audio",
			Category:              "audio",
			Description:           "Governed access to transcription, translation, speech, and voice workflows.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "estimated_audio_minutes", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI File Search and Vector Stores",
			Slug:                  "openai-file-search",
			Category:              "retrieval",
			Description:           "Governed access to files, vector stores, and file search for retrieval workflows.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "storage_gb", "file_types", "expires_at"),
			DefaultExpirationDays: 60,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Batch Jobs",
			Slug:                  "openai-batch-jobs",
			Category:              "batch",
			Description:           "Governed access to asynchronous batch processing for high-volume model workloads.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "estimated_batch_tokens", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Fine-tuning and Evals",
			Slug:                  "openai-finetuning-evals",
			Category:              "optimization",
			Description:           "Governed access to fine-tuning, graders, and evaluation workflows.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "base_models", "dataset_sensitivity", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "OpenAI Realtime and Voice",
			Slug:                  "openai-realtime-voice",
			Category:              "realtime",
			Description:           "Governed access to low-latency realtime and voice workloads.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "estimated_minutes", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
	}, nil
}

func (p Provider) ListModels(ctx context.Context, req aiproviders.ModelListRequest) ([]aiproviders.Model, error) {
	key := apiKey(req.Credentials, req.Config)
	if key == "" {
		key = strings.TrimSpace(req.Credentials["admin_api_key"])
	}
	if key == "" {
		return nil, fmt.Errorf("openai api key missing (set secret_ref to an OpenBao secret with api_key, or OPENAI_API_KEY)")
	}
	baseURL := baseURL(req.Config, "https://api.openai.com")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	resp, err := p.http().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai model sync failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			hint := "grant the key the api.model.read scope, or store a project key (sk-proj-…) with model access as api_key"
			if strings.HasPrefix(key, "sk-admin-") {
				hint = "this is an OpenAI admin key (sk-admin-…) - it governs the org but can't list models; store a project key (sk-proj-…) with the api.model.read scope as api_key for model sync"
			}
			return nil, fmt.Errorf("openai model sync: %s - %s", resp.Status, hint)
		}
		return nil, fmt.Errorf("openai model sync returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding openai model list: %w", err)
	}
	models := make([]aiproviders.Model, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		models = append(models, aiproviders.Model{
			Model:       item.ID,
			DisplayName: item.ID,
			Modality:    inferOpenAIModality(item.ID),
			Status:      "active",
			Metadata: map[string]any{
				"object":   item.Object,
				"created":  item.Created,
				"owned_by": item.OwnedBy,
				"source":   "openai_models_api",
			},
		})
	}
	return models, nil
}

func (p Provider) ProvisionAccess(ctx context.Context, req aiproviders.ProvisionRequest) (*aiproviders.ProvisionResult, error) {
	return provisionOpenAIAccess(ctx, p, req)
}

func provisionOpenAIAccess(ctx context.Context, p Provider, req aiproviders.ProvisionRequest) (*aiproviders.ProvisionResult, error) {
	accessID := deterministicID("openai", req.RequestID.String(), req.Service.Slug, req.Owner)
	observed := map[string]any{
		"provider":              "openai",
		"provider_access_id":    accessID,
		"service":               req.Service.Slug,
		"owner":                 req.Owner,
		"workspace":             req.Workspace,
		"external_provisioning": "manual",
		"message":               "OpenAI access approved in OPORD; grant/revoke in OpenAI admin console or existing IdP workflow.",
	}
	spec := provisionSpec(req.Spec)
	ac := aiproviders.AdminContext{Credentials: req.Credentials, Config: req.Config}
	projectID := specString(spec, "project_id")
	var createdProjectID string // set only when WE created the project this call
	if projectID == "" && truthy(spec["create_project"]) {
		projectName := firstSpecString(spec, "project_name", "workspace")
		if projectName == "" {
			projectName = req.Workspace
		}
		ws, err := p.CreateWorkspace(ctx, ac, projectName)
		if err != nil {
			return nil, err
		}
		projectID = ws.ID
		createdProjectID = ws.ID
		observed["created_project_id"] = ws.ID
		observed["created_project_name"] = ws.Name
	}
	// Rollback: if we created the project but the permission apply below fails, the
	// empty project would be orphaned - archive it so a partial failure leaves no
	// dangling resource. (Detached from ctx so cleanup runs even on cancellation.)
	applied := false
	defer func() {
		if createdProjectID != "" && !applied {
			_ = p.ArchiveWorkspace(context.WithoutCancel(ctx), ac, createdProjectID)
		}
	}()
	if projectID != "" {
		observed["project_id"] = projectID
		accessID = deterministicID("openai", req.RequestID.String(), req.Service.Slug, projectID, req.Owner)
		observed["provider_access_id"] = accessID
	}

	switch req.Service.Slug {
	case "openai-model-permissions":
		if projectID == "" {
			return nil, fmt.Errorf("project_id or create_project/project_name is required for model permissions")
		}
		modelIDs := specStringSlice(spec, "model_ids")
		if len(modelIDs) == 0 {
			modelIDs = specStringSlice(spec, "models")
		}
		res, err := p.SetProjectModelPermissions(ctx, ac, projectID, aiproviders.ProjectModelPermissions{
			Mode:     firstSpecString(spec, "mode", "permission_mode"),
			ModelIDs: modelIDs,
		})
		if err != nil {
			return nil, err
		}
		observed["external_provisioning"] = "openai_project_model_permissions"
		observed["model_permissions"] = map[string]any{"mode": res.Mode, "model_ids": res.ModelIDs}
		observed["message"] = "OpenAI project model permissions were applied by OPORD."
	case "openai-hosted-tools":
		if projectID == "" {
			return nil, fmt.Errorf("project_id or create_project/project_name is required for hosted tool permissions")
		}
		res, err := p.SetProjectHostedToolPermissions(ctx, ac, projectID, aiproviders.ProjectHostedToolPermissions{
			CodeInterpreter: truthy(spec["code_interpreter"]),
			FileSearch:      truthy(spec["file_search"]),
			ImageGeneration: truthy(spec["image_generation"]),
			MCP:             truthy(spec["mcp"]),
			WebSearch:       truthy(spec["web_search"]),
		})
		if err != nil {
			return nil, err
		}
		observed["external_provisioning"] = "openai_project_hosted_tool_permissions"
		observed["hosted_tool_permissions"] = map[string]any{
			"code_interpreter": res.CodeInterpreter,
			"file_search":      res.FileSearch,
			"image_generation": res.ImageGeneration,
			"mcp":              res.MCP,
			"web_search":       res.WebSearch,
		}
		observed["message"] = "OpenAI project hosted tool permissions were applied by OPORD."
	}

	applied = true // success - keep the project we created
	return &aiproviders.ProvisionResult{
		ProviderAccessID: accessID,
		Observed:         observed,
	}, nil
}

func provisionSpec(raw json.RawMessage) map[string]any {
	var root map[string]any
	_ = json.Unmarshal(raw, &root)
	out := map[string]any{}
	for k, v := range root {
		out[k] = v
	}
	if meta, ok := root["metadata"].(map[string]any); ok {
		for k, v := range meta {
			out[k] = v
		}
	}
	return out
}

func firstSpecString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v := specString(m, key); v != "" {
			return v
		}
	}
	return ""
}

func specString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func specStringSlice(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return v
	case string:
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "yes", "on", "enabled":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func (Provider) RevokeAccess(_ context.Context, req aiproviders.RevokeRequest) error {
	if req.ProviderAccessID == "" {
		return fmt.Errorf("provider access id is required")
	}
	return nil
}

func (Provider) GetUsage(_ context.Context, req aiproviders.UsageRequest) ([]aiproviders.UsageRecord, error) {
	return []aiproviders.UsageRecord{
		{
			Metric:   "tokens",
			Quantity: 0,
			Unit:     "tokens",
			CostUSD:  0,
			Raw: map[string]any{
				"provider_access_id": req.ProviderAccessID,
				"source":             "not_imported",
				"message":            "OpenAI usage import is not implemented in this phase.",
			},
		},
	}, nil
}

func (Provider) GetStatus(_ context.Context, req aiproviders.StatusRequest) (*aiproviders.StatusResult, error) {
	return &aiproviders.StatusResult{
		Status: "active",
		Observed: map[string]any{
			"provider_access_id":    req.ProviderAccessID,
			"external_provisioning": "manual",
		},
	}, nil
}

func (p Provider) http() *http.Client {
	if p.client != nil {
		return p.client
	}
	return http.DefaultClient
}

func apiKey(creds map[string]string, _ map[string]any) string {
	for _, key := range []string{"api_key", "openai_api_key", "token"} {
		if v := strings.TrimSpace(creds[key]); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
}

func baseURL(cfg map[string]any, fallback string) string {
	if v, ok := cfg["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func deterministicID(parts ...string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, ":")))
	return parts[0] + "-" + hex.EncodeToString(sum[:])[:16]
}

func requestSchema(fields ...string) map[string]any {
	return map[string]any{"fields": fields}
}

func inferOpenAIModality(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "embedding"):
		return "embedding"
	case strings.Contains(m, "image"), strings.Contains(m, "dall-e"), strings.Contains(m, "gpt-image"):
		return "image"
	case strings.Contains(m, "audio"), strings.Contains(m, "tts"), strings.Contains(m, "whisper"), strings.Contains(m, "transcribe"):
		return "audio"
	default:
		return "text"
	}
}
