package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type projectDTO struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Environment      string    `json:"environment"`
	Provider         string    `json:"provider"`
	Status           string    `json:"status"`
	AccountID        string    `json:"accountId"`
	Members          []string  `json:"members"`
	GroupName        string    `json:"groupName,omitempty"`
	GroupID          string    `json:"groupId,omitempty"`
	PermissionSetARN string    `json:"permissionSetArn,omitempty"`
	LastError        string    `json:"lastError,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func projectToDTO(p orchestrator.ProjectSummary) projectDTO {
	r := p.Resource
	members := p.Spec.UserNames
	if members == nil {
		members = []string{}
	}
	out := projectDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Environment: r.Environment,
		Provider:    p.Provider,
		Status:      r.Status,
		AccountID:   p.Spec.AccountID,
		Members:     members,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	// observed holds the provider's ProjectResult (group + permission set) once ready.
	if len(r.Observed) > 0 {
		var pr struct {
			GroupID          string `json:"group_id"`
			GroupName        string `json:"group_name"`
			PermissionSetARN string `json:"permission_set_arn"`
			Error            string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &pr); err == nil {
			out.GroupID = pr.GroupID
			out.GroupName = pr.GroupName
			out.PermissionSetARN = pr.PermissionSetARN
			out.LastError = pr.Error
		}
	}
	return out
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListProjects(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]projectDTO, 0, len(list))
	for _, p := range list {
		out = append(out, projectToDTO(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	p, err := s.svc.ProjectStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, projectToDTO(*p))
}

type createProjectReq struct {
	Name        string             `json:"name"`
	Environment string             `json:"environment"`
	Provider    string             `json:"provider"`
	Spec        models.ProjectSpec `json:"spec"`
	DryRun      bool               `json:"dryRun"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateProject(r.Context(), orchestrator.CreateProjectInput{
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

type setMembersReq struct {
	Members []string `json:"members"`
}

// setProjectMembers replaces the project's member list and re-provisions (the
// day-2 add/remove member action).
func (s *Server) setProjectMembers(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	var req setMembersReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.SetProjectMembers(r.Context(), name, env, req.Members); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "provisioning"})
}

func (s *Server) destroyProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.ProjectStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteProjectRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyProjectAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
