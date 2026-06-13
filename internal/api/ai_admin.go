package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// AI org-administration HTTP surface (ADR-0022): list/invite users, manage org
// roles, create/archive workspaces, and grant/remove workspace access for a
// governable AI provider (Anthropic today). Reads are viewer+, writes operator+
// (wired in server.go), mirroring the rest of the AI routes.

// --- DTOs ---

type aiOrgUserDTO struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	Role    string `json:"role"`
	AddedAt string `json:"addedAt,omitempty"`
}

type aiWorkspaceDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt,omitempty"`
	ArchivedAt string `json:"archivedAt,omitempty"`
	Archived   bool   `json:"archived"`
}

type aiInviteDTO struct {
	InviteID  string `json:"inviteId"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	InvitedAt string `json:"invitedAt,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type aiWorkspaceAccessDTO struct {
	UserID        string `json:"userId"`
	Email         string `json:"email,omitempty"`
	OrgRole       string `json:"orgRole,omitempty"`
	WorkspaceRole string `json:"workspaceRole"`
	Inherited     bool   `json:"inherited"`
}

func orgUserToDTO(u aiproviders.OrgUser) aiOrgUserDTO {
	return aiOrgUserDTO{ID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role), AddedAt: u.AddedAt}
}

func workspaceToDTO(w aiproviders.OrgWorkspace) aiWorkspaceDTO {
	return aiWorkspaceDTO{ID: w.ID, Name: w.Name, CreatedAt: w.CreatedAt, ArchivedAt: w.ArchivedAt, Archived: w.ArchivedAt != ""}
}

func inviteToDTO(i aiproviders.InviteResult) aiInviteDTO {
	return aiInviteDTO{InviteID: i.InviteID, Email: i.Email, Role: string(i.Role), Status: i.Status, InvitedAt: i.InvitedAt, ExpiresAt: i.ExpiresAt}
}

// --- Reads ---

func (s *Server) listAIOrgUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListAIOrgUsers(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiOrgUserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, orgUserToDTO(u))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listAIWorkspaces(w http.ResponseWriter, r *http.Request) {
	wss, err := s.svc.ListAIWorkspaces(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiWorkspaceDTO, 0, len(wss))
	for _, ws := range wss {
		out = append(out, workspaceToDTO(ws))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listAIInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := s.svc.ListAIInvites(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiInviteDTO, 0, len(invites))
	for _, i := range invites {
		out = append(out, inviteToDTO(i))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listAIWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	access, err := s.svc.AIWorkspaceAccess(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiWorkspaceAccessDTO, 0, len(access))
	for _, a := range access {
		out = append(out, aiWorkspaceAccessDTO{
			UserID: a.UserID, Email: a.Email, OrgRole: string(a.OrgRole),
			WorkspaceRole: string(a.WorkspaceRole), Inherited: a.Inherited,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- Writes ---

type inviteUserReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (s *Server) inviteAIOrgUser(w http.ResponseWriter, r *http.Request) {
	var req inviteUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.InviteAIOrgUser(r.Context(), chi.URLParam(r, "name"), req.Email, req.Role)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, inviteToDTO(*res))
}

type setOrgRoleReq struct {
	Role string `json:"role"`
}

func (s *Server) setAIOrgRole(w http.ResponseWriter, r *http.Request) {
	var req setOrgRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	u, err := s.svc.SetAIOrgRole(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "userID"), req.Role)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, orgUserToDTO(*u))
}

func (s *Server) removeAIOrgUser(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RemoveAIOrgUser(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "userID")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

type createWorkspaceReq struct {
	Name string `json:"name"`
}

func (s *Server) createAIWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ws, err := s.svc.CreateAIWorkspace(r.Context(), chi.URLParam(r, "name"), req.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, workspaceToDTO(*ws))
}

func (s *Server) archiveAIWorkspace(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.ArchiveAIWorkspace(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

type grantAccessReq struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

func (s *Server) grantAIWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	var req grantAccessReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.GrantAIWorkspaceAccess(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), req.UserID, req.Role); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "granted"})
}

func (s *Server) removeAIWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RemoveAIWorkspaceMember(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), chi.URLParam(r, "userID")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
