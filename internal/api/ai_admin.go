package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/go-chi/chi/v5"
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

func (s *Server) listAIProjectAPIKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.ListAIProjectAPIKeys(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiProjectAPIKeyDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, aiProjectAPIKeyToDTO(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listAIProjectRateLimits(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.ListAIProjectRateLimits(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiProjectRateLimitDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, aiProjectRateLimitToDTO(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getAIProjectModelPermissions(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.GetAIProjectModelPermissions(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectModelPermissionsToDTO(res))
}

func (s *Server) getAIProjectHostedToolPermissions(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.GetAIProjectHostedToolPermissions(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectHostedToolPermissionsToDTO(res))
}

func (s *Server) getAIProjectDataRetention(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.GetAIProjectDataRetention(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectDataRetentionToDTO(res))
}

func (s *Server) listAIProjectSpendAlerts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.ListAIProjectSpendAlerts(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]aiProjectSpendAlertDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, aiProjectSpendAlertToDTO(row))
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

func (s *Server) deleteAIProjectAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteAIProjectAPIKey(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), chi.URLParam(r, "keyID")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type updateProjectRateLimitReq struct {
	MaxRequestsPer1Minute       *float64 `json:"maxRequestsPer1Minute"`
	MaxTokensPer1Minute         *float64 `json:"maxTokensPer1Minute"`
	MaxRequestsPer1Day          *float64 `json:"maxRequestsPer1Day"`
	MaxImagesPer1Minute         *float64 `json:"maxImagesPer1Minute"`
	MaxAudioMegabytesPer1Minute *float64 `json:"maxAudioMegabytesPer1Minute"`
	Batch1DayMaxInputTokens     *float64 `json:"batch1DayMaxInputTokens"`
}

func (s *Server) updateAIProjectRateLimit(w http.ResponseWriter, r *http.Request) {
	var req updateProjectRateLimitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.UpdateAIProjectRateLimit(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), chi.URLParam(r, "rateLimitID"), aiproviders.ProjectRateLimitUpdate{
		MaxRequestsPer1Minute:       req.MaxRequestsPer1Minute,
		MaxTokensPer1Minute:         req.MaxTokensPer1Minute,
		MaxRequestsPer1Day:          req.MaxRequestsPer1Day,
		MaxImagesPer1Minute:         req.MaxImagesPer1Minute,
		MaxAudioMegabytesPer1Minute: req.MaxAudioMegabytesPer1Minute,
		Batch1DayMaxInputTokens:     req.Batch1DayMaxInputTokens,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectRateLimitToDTO(*res))
}

func (s *Server) setAIProjectModelPermissions(w http.ResponseWriter, r *http.Request) {
	var req aiProjectModelPermissionsDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.SetAIProjectModelPermissions(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), aiproviders.ProjectModelPermissions{
		Mode: req.Mode, ModelIDs: req.ModelIDs,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectModelPermissionsToDTO(res))
}

func (s *Server) deleteAIProjectModelPermissions(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteAIProjectModelPermissions(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) setAIProjectHostedToolPermissions(w http.ResponseWriter, r *http.Request) {
	var req aiProjectHostedToolPermissionsDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.SetAIProjectHostedToolPermissions(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), aiproviders.ProjectHostedToolPermissions{
		CodeInterpreter: req.CodeInterpreter, FileSearch: req.FileSearch, ImageGeneration: req.ImageGeneration, MCP: req.MCP, WebSearch: req.WebSearch,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectHostedToolPermissionsToDTO(res))
}

func (s *Server) setAIProjectDataRetention(w http.ResponseWriter, r *http.Request) {
	var req aiProjectDataRetentionDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.SetAIProjectDataRetention(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), aiproviders.ProjectDataRetention{Type: req.Type})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectDataRetentionToDTO(res))
}

type projectSpendAlertReq struct {
	ThresholdCents float64  `json:"thresholdCents"`
	ThresholdUSD   float64  `json:"thresholdUsd"`
	Recipients     []string `json:"recipients"`
	SubjectPrefix  string   `json:"subjectPrefix"`
}

func (r projectSpendAlertReq) input() (aiproviders.ProjectSpendAlertInput, error) {
	cents := r.ThresholdCents
	if cents == 0 && r.ThresholdUSD > 0 {
		cents = r.ThresholdUSD * 100
	}
	recipients := make([]string, 0, len(r.Recipients))
	for _, email := range r.Recipients {
		trimmed := strings.TrimSpace(email)
		if trimmed == "" {
			continue
		}
		if _, err := mail.ParseAddress(trimmed); err != nil {
			return aiproviders.ProjectSpendAlertInput{}, fmt.Errorf("invalid recipient email %q", trimmed)
		}
		recipients = append(recipients, trimmed)
	}
	return aiproviders.ProjectSpendAlertInput{ThresholdCents: cents, Recipients: recipients, SubjectPrefix: r.SubjectPrefix}, nil
}

func (s *Server) createAIProjectSpendAlert(w http.ResponseWriter, r *http.Request) {
	var req projectSpendAlertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in, err := req.input()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateAIProjectSpendAlert(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, aiProjectSpendAlertToDTO(*res))
}

func (s *Server) updateAIProjectSpendAlert(w http.ResponseWriter, r *http.Request) {
	var req projectSpendAlertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in, err := req.input()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.UpdateAIProjectSpendAlert(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), chi.URLParam(r, "alertID"), in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, aiProjectSpendAlertToDTO(*res))
}

func (s *Server) deleteAIProjectSpendAlert(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteAIProjectSpendAlert(r.Context(), chi.URLParam(r, "name"), chi.URLParam(r, "wsID"), chi.URLParam(r, "alertID")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
