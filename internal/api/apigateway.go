package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type apiGatewayDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	Endpoint      string    `json:"endpoint,omitempty"`
	APIID         string    `json:"apiId,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func apiGatewayToDTO(s orchestrator.APIGatewaySummary) apiGatewayDTO {
	r := s.Resource
	out := apiGatewayDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      s.Provider,
		TargetAccount: s.Spec.TargetAccount,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's APIGatewayResult once ready.
	if len(r.Observed) > 0 {
		var gr struct {
			Endpoint string `json:"endpoint"`
			APIID    string `json:"api_id"`
			Error    string `json:"error"` // provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &gr); err == nil {
			out.Endpoint = gr.Endpoint
			out.APIID = gr.APIID
			out.LastError = gr.Error
		}
	}
	return out
}

func (s *Server) listAPIGateways(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListAPIGateways(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]apiGatewayDTO, 0, len(list))
	for _, item := range list {
		out = append(out, apiGatewayToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getAPIGateway(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.APIGatewayStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, apiGatewayToDTO(*item))
}

type createAPIGatewayReq struct {
	Name        string                `json:"name"`
	Environment string                `json:"environment"`
	Provider    string                `json:"provider"`
	Spec        models.APIGatewaySpec `json:"spec"`
	DryRun      bool                  `json:"dryRun"`
}

func (s *Server) createAPIGateway(w http.ResponseWriter, r *http.Request) {
	var req createAPIGatewayReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateAPIGateway(r.Context(), orchestrator.CreateAPIGatewayInput{
		Name:        req.Name,
		Environment: req.Environment,
		Provider:    req.Provider,
		Spec:        req.Spec,
		DryRun:      req.DryRun,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if res.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{"dryRun": true, "summary": res.Summary})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     res.Resource.ID.String(),
		"name":   res.Resource.Name,
		"status": res.Resource.Status,
	})
}

func (s *Server) destroyAPIGateway(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.APIGatewayStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteAPIGatewayRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyAPIGatewayAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
