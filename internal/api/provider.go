package api

import (
	"encoding/json"
	"net/http"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type updateProviderReq struct {
	Name      *string        `json:"name"`
	Type      *string        `json:"type"`
	Config    map[string]any `json:"config"`
	SecretRef *string        `json:"secretRef"`
}

// updateProvider merges config + secret-ref changes into an existing provider.
func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	var req updateProviderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p, err := s.svc.UpdateProvider(r.Context(), name, orchestrator.ProviderUpdate{
		Name:      req.Name,
		Type:      req.Type,
		Config:    req.Config,
		SecretRef: req.SecretRef,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, providerToDTO(p, 0))
}

// checkProvider runs a live reachability + credential probe against the
// provider's backend and returns the result. The outcome is also persisted, so
// it shows up in the provider's `health` field on subsequent GET /providers
// reads (for monitoring). A missing provider / unregistered type is a 404;
// a failed probe is still a 200 with status:"failed" (it's a valid result).
func (s *Server) checkProvider(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	res, err := s.svc.CheckProviderConnection(r.Context(), name)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, providerCheckToDTO(res))
}

func (s *Server) getProviderReadiness(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	res, err := s.svc.ProviderReadiness(r.Context(), name)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, providerReadinessToDTO(res))
}

// deleteProvider removes a provider (refused by the orchestrator if clusters or
// resources still reference it).
func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	if err := s.svc.DeleteProvider(r.Context(), name); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "deleted"})
}
