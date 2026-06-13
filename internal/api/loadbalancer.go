package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type loadBalancerDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	DNSName       string    `json:"dnsName,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func loadBalancerToDTO(s orchestrator.LoadBalancerSummary) loadBalancerDTO {
	r := s.Resource
	out := loadBalancerDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      s.Provider,
		TargetAccount: s.Spec.TargetAccount,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's LoadBalancerResult once ready.
	if len(r.Observed) > 0 {
		var lr struct {
			DNSName string `json:"dns_name"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(r.Observed, &lr); err == nil {
			out.DNSName = lr.DNSName
			out.LastError = lr.Error
		}
	}
	return out
}

func (s *Server) listLoadBalancers(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListLoadBalancers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]loadBalancerDTO, 0, len(list))
	for _, item := range list {
		out = append(out, loadBalancerToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getLoadBalancer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.LoadBalancerStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, loadBalancerToDTO(*item))
}

type createLoadBalancerReq struct {
	Name        string                  `json:"name"`
	Environment string                  `json:"environment"`
	Provider    string                  `json:"provider"`
	Spec        models.LoadBalancerSpec `json:"spec"`
	DryRun      bool                    `json:"dryRun"`
}

func (s *Server) createLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req createLoadBalancerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateLoadBalancer(r.Context(), orchestrator.CreateLoadBalancerInput{
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

func (s *Server) destroyLoadBalancer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.LoadBalancerStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteLoadBalancerRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyLoadBalancerAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
