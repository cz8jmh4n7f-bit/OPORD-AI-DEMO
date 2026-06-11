package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type cdnDTO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Environment    string    `json:"environment"`
	Provider       string    `json:"provider"`
	TargetAccount  string    `json:"targetAccount,omitempty"`
	Status         string    `json:"status"`
	DomainName     string    `json:"domainName,omitempty"`
	DistributionID string    `json:"distributionId,omitempty"`
	LastError      string    `json:"lastError,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func cdnToDTO(s orchestrator.CDNSummary) cdnDTO {
	r := s.Resource
	out := cdnDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      s.Provider,
		TargetAccount: s.Spec.TargetAccount,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's CDNResult once ready.
	if len(r.Observed) > 0 {
		var cr struct {
			DomainName     string `json:"domain_name"`
			DistributionID string `json:"distribution_id"`
			Error          string `json:"error"` // provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &cr); err == nil {
			out.DomainName = cr.DomainName
			out.DistributionID = cr.DistributionID
			out.LastError = cr.Error
		}
	}
	return out
}

func (s *Server) listCDN(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListCDN(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]cdnDTO, 0, len(list))
	for _, item := range list {
		out = append(out, cdnToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getCDN(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.CDNStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, cdnToDTO(*item))
}

type createCDNReq struct {
	Name        string         `json:"name"`
	Environment string         `json:"environment"`
	Provider    string         `json:"provider"`
	Spec        models.CDNSpec `json:"spec"`
	DryRun      bool           `json:"dryRun"`
}

func (s *Server) createCDN(w http.ResponseWriter, r *http.Request) {
	var req createCDNReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateCDN(r.Context(), orchestrator.CreateCDNInput{
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

func (s *Server) destroyCDN(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.CDNStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteCDNRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyCDNAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
