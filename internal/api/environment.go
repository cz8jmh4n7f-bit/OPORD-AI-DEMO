package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/templates"
)

type blueprintComponentDTO struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type blueprintDTO struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Components  []blueprintComponentDTO `json:"components"`
}

type envComponentDTO struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
	Status   string `json:"status"`
}

type environmentDTO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Blueprint   string            `json:"blueprint"`
	Provider    string            `json:"provider"`
	Status      string            `json:"status"`
	Components  []envComponentDTO `json:"components"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func envToDTO(e orchestrator.EnvironmentSummary) environmentDTO {
	comps := make([]envComponentDTO, 0, len(e.Components))
	for _, c := range e.Components {
		comps = append(comps, envComponentDTO{Name: c.Name, Kind: string(c.Kind), Resource: c.ChildName, Status: c.Status})
	}
	return environmentDTO{
		ID:          e.Env.ID.String(),
		Name:        e.Env.Name,
		Environment: e.Env.Environment,
		Blueprint:   e.Env.Blueprint,
		Provider:    e.Provider,
		Status:      e.Aggregate,
		Components:  comps,
		CreatedAt:   e.Env.CreatedAt,
		UpdatedAt:   e.Env.UpdatedAt,
	}
}

func (s *Server) listBlueprints(w http.ResponseWriter, _ *http.Request) {
	out := make([]blueprintDTO, 0)
	for _, b := range templates.List() {
		comps := make([]blueprintComponentDTO, 0, len(b.Components))
		for _, c := range b.Components {
			comps = append(comps, blueprintComponentDTO{Name: c.Name, Kind: string(c.Kind)})
		}
		out = append(out, blueprintDTO{ID: b.ID, Name: b.Name, Description: b.Description, Components: comps})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := s.svc.ListEnvironments(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]environmentDTO, 0, len(envs))
	for _, e := range envs {
		out = append(out, envToDTO(e))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getEnvironment(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	e, err := s.svc.EnvironmentStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, envToDTO(*e))
}

type createEnvReq struct {
	Name         string             `json:"name"`
	Environment  string             `json:"environment"`
	Provider     string             `json:"provider"`
	Blueprint    string             `json:"blueprint"`
	Template     string             `json:"template"`
	SSHPublicKey string             `json:"sshPublicKey"`
	Components   []models.Component `json:"components"`
	DryRun       bool               `json:"dryRun"`
}

func (s *Server) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var req createEnvReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateEnvironment(r.Context(), orchestrator.CreateEnvironmentInput{
		Name:         req.Name,
		Environment:  req.Environment,
		Provider:     req.Provider,
		Blueprint:    req.Blueprint,
		Template:     req.Template,
		SSHPublicKey: req.SSHPublicKey,
		Components:   req.Components,
		DryRun:       req.DryRun,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if res.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{"dryRun": true, "summaries": res.Summaries})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     res.Env.ID.String(),
		"name":   res.Env.Name,
		"status": res.Env.Status,
	})
}

func (s *Server) destroyEnvironment(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.EnvironmentStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	s.svc.DestroyEnvironmentAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
