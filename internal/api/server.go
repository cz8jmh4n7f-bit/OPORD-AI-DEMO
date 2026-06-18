// Package api exposes the OPORD AI governance orchestrator over HTTP. Handlers
// are thin: they decode requests, call the same orchestrator.Service the CLI
// uses, and encode DTOs. No business logic lives here.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/auth"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Server holds the dependencies for the HTTP API.
type Server struct {
	svc         *orchestrator.Service
	authResolve auth.Resolver
	authEnabled bool
	log         *slog.Logger
}

// NewServer builds the API server.
func NewServer(svc *orchestrator.Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{svc: svc, log: log}
}

// SetAuth wires API-key authentication. When enabled is false, the API runs
// open (dev mode) with a default admin identity.
func (s *Server) SetAuth(resolve auth.Resolver, enabled bool) {
	s.authResolve = resolve
	s.authEnabled = enabled
}

// Routes returns the configured HTTP handler.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(cors)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Middleware(s.authResolve, s.authEnabled))

		// Reads: viewer and up.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole(auth.RoleViewer))
			r.Get("/me", s.getMe)
			r.Get("/requests", s.listRequests)
			r.Get("/requests/{name}", s.getRequest)
			r.Get("/ai/providers", s.listAIProviders)
			r.Get("/ai/services", s.listAIServices)
			r.Get("/ai/requests", s.listAIRequests)
			r.Get("/ai/instances", s.listAIInstances)
			r.Get("/ai/usage", s.listAIUsage)
			r.Get("/ai/budgets", s.listAIBudgets)
			r.Get("/ai/quotas", s.listAIQuotas)
			r.Get("/ai/policies", s.listAIPolicies)
			r.Get("/ai/models", s.listAIModels)
			r.Get("/ai/renewals", s.listAIRenewals)
			r.Get("/ai/access-review", s.listAIAccessReview)
			r.Get("/ai/audit", s.listAIAudit)
			// AI org administration reads (ADR-0022).
			r.Get("/ai/admin/{name}/users", s.listAIOrgUsers)
			r.Get("/ai/admin/{name}/workspaces", s.listAIWorkspaces)
			r.Get("/ai/admin/{name}/invites", s.listAIInvites)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/access", s.listAIWorkspaceAccess)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/api-keys", s.listAIProjectAPIKeys)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/rate-limits", s.listAIProjectRateLimits)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/model-permissions", s.getAIProjectModelPermissions)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/tool-permissions", s.getAIProjectHostedToolPermissions)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/data-retention", s.getAIProjectDataRetention)
			r.Get("/ai/admin/{name}/workspaces/{wsID}/spend-alerts", s.listAIProjectSpendAlerts)
			// Agent & MCP governance reads + the authorize enforcement check.
			r.Get("/ai/mcp/servers", s.listMCPServers)
			r.Get("/ai/mcp/grants", s.listMCPGrants)
			r.Get("/ai/mcp/authorize", s.authorizeMCP)
			r.Post("/ai/mcp/requests", s.requestMCPAccess) // self-service request (viewer+)
		})

		// Writes: operator and up.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole(auth.RoleOperator))
			r.Post("/requests", s.createRequest)
			r.Post("/requests/{name}/approve", s.approveRequest)
			r.Post("/requests/{name}/reject", s.rejectRequest)
			r.Post("/ai/providers", s.createAIProvider)
			r.Patch("/ai/providers/{name}", s.updateAIProvider)
			r.Delete("/ai/providers/{name}", s.deleteAIProvider)
			r.Post("/ai/providers/{name}/check", s.checkAIProvider)
			r.Post("/ai/providers/{name}/sync", s.syncAIProvider)
			r.Post("/ai/providers/{name}/sync-models", s.syncAIProviderModels)
			r.Post("/ai/requests", s.createAIRequest)
			r.Post("/ai/requests/{name}/approve", s.approveAIRequest)
			r.Post("/ai/requests/{name}/reject", s.rejectAIRequest)
			r.Post("/ai/instances/{id}/revoke", s.revokeAIInstance)
			r.Get("/ai/instances/{id}/secret", s.revealAIInstanceSecret)
			r.Post("/ai/instances/{id}/recertify", s.recertifyAIInstance)
			r.Post("/ai/instances/reap-expired", s.reapExpiredAIInstances)
			// Agent & MCP governance writes.
			r.Post("/ai/mcp/servers", s.registerMCPServer)
			r.Delete("/ai/mcp/servers/{name}", s.deleteMCPServer)
			r.Post("/ai/mcp/grants", s.grantMCPAccess)
			r.Post("/ai/mcp/grants/{id}/approve", s.approveMCPGrant)
			r.Post("/ai/mcp/grants/{id}/reject", s.rejectMCPGrant)
			r.Post("/ai/mcp/grants/{id}/revoke", s.revokeMCPGrant)
			r.Post("/ai/budgets", s.createAIBudget)
			r.Patch("/ai/budgets/{id}", s.updateAIBudget)
			r.Delete("/ai/budgets/{id}", s.deleteAIBudget)
			r.Post("/ai/quotas", s.createAIQuota)
			r.Patch("/ai/quotas/{id}", s.updateAIQuota)
			r.Delete("/ai/quotas/{id}", s.deleteAIQuota)
			r.Post("/ai/policies", s.createAIPolicy)
			r.Patch("/ai/policies/{id}", s.updateAIPolicy)
			r.Delete("/ai/policies/{id}", s.deleteAIPolicy)
			r.Post("/ai/usage/import/openai", s.importOpenAIUsage)
			r.Post("/ai/usage/import/anthropic", s.importAnthropicUsage)
			r.Post("/ai/usage/import/litellm", s.importLiteLLMSpend)
			// AI org administration writes (ADR-0022): invite/role/workspace/access.
			r.Post("/ai/admin/{name}/invites", s.inviteAIOrgUser)
			r.Post("/ai/admin/{name}/users/{userID}/role", s.setAIOrgRole)
			r.Delete("/ai/admin/{name}/users/{userID}", s.removeAIOrgUser)
			r.Post("/ai/admin/{name}/workspaces", s.createAIWorkspace)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/archive", s.archiveAIWorkspace)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/members", s.grantAIWorkspaceAccess)
			r.Delete("/ai/admin/{name}/workspaces/{wsID}/members/{userID}", s.removeAIWorkspaceMember)
			r.Delete("/ai/admin/{name}/workspaces/{wsID}/api-keys/{keyID}", s.deleteAIProjectAPIKey)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/rate-limits/{rateLimitID}", s.updateAIProjectRateLimit)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/model-permissions", s.setAIProjectModelPermissions)
			r.Delete("/ai/admin/{name}/workspaces/{wsID}/model-permissions", s.deleteAIProjectModelPermissions)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/tool-permissions", s.setAIProjectHostedToolPermissions)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/data-retention", s.setAIProjectDataRetention)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/spend-alerts", s.createAIProjectSpendAlert)
			r.Post("/ai/admin/{name}/workspaces/{wsID}/spend-alerts/{alertID}", s.updateAIProjectSpendAlert)
			r.Delete("/ai/admin/{name}/workspaces/{wsID}/spend-alerts/{alertID}", s.deleteAIProjectSpendAlert)
			r.Post("/ai/gateway/openai/responses", s.gatewayOpenAIResponses)
			r.Post("/ai/gateway/anthropic/messages", s.gatewayAnthropicMessages)
		})
	})

	return r
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	// A governance block means the caller isn't allowed - 403, not 400/500.
	if orchestrator.IsAIEnforcementError(err) {
		status = http.StatusForbidden
	}
	// Friendly-ify a Postgres unique-violation (a duplicate name+environment, the
	// most common create error) instead of leaking the raw SQLSTATE 23505 to the UI.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		status = http.StatusConflict
		err = errors.New("a resource with that name already exists in this environment - choose another name, or remove the existing one first (destroyed resources keep their name until removed)")
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
