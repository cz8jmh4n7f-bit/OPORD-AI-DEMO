package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type functionDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	Runtime       string    `json:"runtime"`
	Handler       string    `json:"handler,omitempty"`
	MemoryMB      int       `json:"memoryMb,omitempty"`
	ARN           string    `json:"arn,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func functionToDTO(f orchestrator.FunctionSummary) functionDTO {
	r := f.Resource
	runtime := f.Spec.Runtime
	if runtime == "" {
		runtime = "python3.12"
	}
	out := functionDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      f.Provider,
		TargetAccount: f.Spec.TargetAccount,
		Status:        r.Status,
		Runtime:       runtime,
		Handler:       f.Spec.Handler,
		MemoryMB:      f.Spec.MemoryMB,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's FunctionResult (arn) once ready.
	if len(r.Observed) > 0 {
		var fr struct {
			ARN   string `json:"arn"`
			Error string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &fr); err == nil {
			out.ARN = fr.ARN
			out.LastError = fr.Error
		}
	}
	return out
}

func (s *Server) listFunctions(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListFunctions(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]functionDTO, 0, len(list))
	for _, f := range list {
		out = append(out, functionToDTO(f))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	f, err := s.svc.FunctionStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, functionToDTO(*f))
}

type createFunctionReq struct {
	Name        string              `json:"name"`
	Environment string              `json:"environment"`
	Provider    string              `json:"provider"`
	Spec        models.FunctionSpec `json:"spec"`
	DryRun      bool                `json:"dryRun"`
}

func (s *Server) createFunction(w http.ResponseWriter, r *http.Request) {
	var req createFunctionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateFunction(r.Context(), orchestrator.CreateFunctionInput{
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

func (s *Server) destroyFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.FunctionStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteFunctionRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyFunctionAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
