package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type stackDTO struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Environment   string         `json:"environment"`
	Provider      string         `json:"provider"`
	TargetAccount string         `json:"targetAccount,omitempty"`
	Status        string         `json:"status"`
	ModuleDir     string         `json:"moduleDir"`
	Variables     map[string]any `json:"variables,omitempty"`
	Outputs       map[string]any `json:"outputs,omitempty"`
	LastError     string         `json:"lastError,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func stackToDTO(st orchestrator.StackSummary) stackDTO {
	r := st.Resource
	d := stackDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      st.Provider,
		TargetAccount: st.Spec.TargetAccount,
		Status:        r.Status,
		ModuleDir:     st.Spec.ModuleDir,
		Variables:     st.Spec.Variables,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// Observed holds the StackResult ({"outputs": {...}}).
	if len(r.Observed) > 0 {
		var sr struct {
			Outputs map[string]any `json:"outputs"`
			Error   string         `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &sr); err == nil {
			d.Outputs = sr.Outputs
			d.LastError = sr.Error
		}
	}
	return d
}

func (s *Server) listStacks(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListStacks(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]stackDTO, 0, len(list))
	for _, st := range list {
		out = append(out, stackToDTO(st))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getStack(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	st, err := s.svc.StackStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, stackToDTO(*st))
}

type createStackReq struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Provider    string `json:"provider"`
	// Top-level fields are the legacy shape the web form sends. Kept for backward
	// compatibility; new clients may instead send a nested `spec` (consistent with
	// every other create endpoint - Finding C). When both are present, spec wins.
	ModuleDir     string            `json:"moduleDir"`
	Variables     map[string]any    `json:"variables"`
	TargetAccount string            `json:"target_account,omitempty"` // ADR-0013: deploy into a managed account
	Spec          *models.StackSpec `json:"spec,omitempty"`           // nested form (module_dir/variables/target_account)
	DryRun        bool              `json:"dryRun"`
}

func (s *Server) createStack(w http.ResponseWriter, r *http.Request) {
	var req createStackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Accept either the legacy top-level fields or a nested `spec` (Finding C:
	// stack was the only endpoint not taking `spec`). Nested values win when set.
	spec := models.StackSpec{ModuleDir: req.ModuleDir, Variables: req.Variables, TargetAccount: req.TargetAccount}
	if req.Spec != nil {
		if req.Spec.ModuleDir != "" {
			spec.ModuleDir = req.Spec.ModuleDir
		}
		if req.Spec.Variables != nil {
			spec.Variables = req.Spec.Variables
		}
		if req.Spec.TargetAccount != "" {
			spec.TargetAccount = req.Spec.TargetAccount
		}
	}
	res, err := s.svc.CreateStack(r.Context(), orchestrator.CreateStackInput{
		Name:        req.Name,
		Environment: req.Environment,
		Provider:    req.Provider,
		Spec:        spec,
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

func (s *Server) destroyStack(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.StackStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteStackRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyStackAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
