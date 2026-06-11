package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type accountDTO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Provider    string            `json:"provider"`
	Status      string            `json:"status"`
	CSAID       string            `json:"csaId"`
	CloudName   string            `json:"cloudName"`
	AccountID   string            `json:"accountId,omitempty"`
	CreateVPC   bool              `json:"createVpc"`
	Layers      map[string]string `json:"layers,omitempty"`
	LastError   string            `json:"lastError,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func accountToDTO(a orchestrator.AccountSummary) accountDTO {
	r := a.Resource
	out := accountDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Environment: r.Environment,
		Provider:    a.Provider,
		Status:      r.Status,
		CSAID:       a.Spec.CSAID,
		CloudName:   a.Spec.CloudName,
		CreateVPC:   a.Spec.CreateVPC,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	// observed holds the provider's AccountResult (account_id + per-layer status).
	if len(r.Observed) > 0 {
		var ar struct {
			AccountID string            `json:"account_id"`
			Layers    map[string]string `json:"layers"`
			Error     string            `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &ar); err == nil {
			out.AccountID = ar.AccountID
			out.Layers = ar.Layers
			out.LastError = ar.Error
		}
	}
	return out
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListAccounts(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]accountDTO, 0, len(list))
	for _, a := range list {
		out = append(out, accountToDTO(a))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	a, err := s.svc.AccountStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, accountToDTO(*a))
}

type createAccountReq struct {
	Name        string             `json:"name"`
	Environment string             `json:"environment"`
	Provider    string             `json:"provider"`
	Spec        models.AccountSpec `json:"spec"`
	DryRun      bool               `json:"dryRun"`
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateAccount(r.Context(), orchestrator.CreateAccountInput{
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

func (s *Server) destroyAccount(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.AccountStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteAccountRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyAccountAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
