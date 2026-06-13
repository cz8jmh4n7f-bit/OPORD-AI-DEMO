package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type certDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	Domain        string    `json:"domain"`
	CertStatus    string    `json:"certStatus,omitempty"`
	ARN           string    `json:"arn,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func certToDTO(c orchestrator.CertSummary) certDTO {
	r := c.Resource
	out := certDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      c.Provider,
		TargetAccount: c.Spec.TargetAccount,
		Status:        r.Status,
		Domain:        c.Spec.Domain,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's CertResult once ready.
	if len(r.Observed) > 0 {
		var cr struct {
			ARN    string `json:"arn"`
			Domain string `json:"domain"`
			Status string `json:"status"`
			Error  string `json:"error"` // provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &cr); err == nil {
			out.ARN = cr.ARN
			if cr.Domain != "" {
				out.Domain = cr.Domain
			}
			out.CertStatus = cr.Status
			out.LastError = cr.Error
		}
	}
	return out
}

func (s *Server) listCert(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListCert(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]certDTO, 0, len(list))
	for _, item := range list {
		out = append(out, certToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getCert(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.CertStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, certToDTO(*item))
}

type createCertReq struct {
	Name        string          `json:"name"`
	Environment string          `json:"environment"`
	Provider    string          `json:"provider"`
	Spec        models.CertSpec `json:"spec"`
	DryRun      bool            `json:"dryRun"`
}

func (s *Server) createCert(w http.ResponseWriter, r *http.Request) {
	var req createCertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateCert(r.Context(), orchestrator.CreateCertInput{
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

func (s *Server) destroyCert(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.CertStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteCertRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyCertAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
