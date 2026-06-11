package api

import (
	"encoding/json"
	"net/http"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

// grantEntraReq drives POST /entra/grant: assign Entra users to an AWS SAML
// enterprise app (ensure the app role for the AWS role, then assign each user).
type grantEntraReq struct {
	AppID       string   `json:"appId"`
	RoleArn     string   `json:"roleArn"`
	ProviderArn string   `json:"providerArn"`
	RoleName    string   `json:"roleName"`
	Users       []string `json:"users"`
	Invite      bool     `json:"invite"`
}

func (s *Server) grantEntra(w http.ResponseWriter, r *http.Request) {
	var req grantEntraReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.RoleName == "" {
		req.RoleName = "ReadOnly"
	}
	users := make([]orchestrator.EntraUser, 0, len(req.Users))
	for _, e := range req.Users {
		if e != "" {
			users = append(users, orchestrator.EntraUser{Email: e, Role: req.RoleName, Invite: req.Invite})
		}
	}
	res, err := s.svc.GrantEntraAccess(r.Context(), orchestrator.GrantEntraAccessInput{
		AppID: req.AppID,
		Roles: map[string]string{req.RoleName: req.RoleArn + "," + req.ProviderArn},
		Users: users,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"appRoleId": res.AppRoleIDs[req.RoleName],
		"assigned":  res.Assigned,
		"invited":   res.Invited,
	})
}
