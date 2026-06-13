package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// listProviderClusterVersions lists the managed-k8s versions a provider's cloud
// currently offers (GKE/AKS/EKS) so the cluster form can show a live version
// picker instead of free-text. Read-only; on error (provider can't list / cloud
// unreachable / keyless ADC) the frontend falls back to "(provider default)".
func (s *Server) listProviderClusterVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	region := r.URL.Query().Get("region")

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	versions, err := s.svc.ListProviderClusterVersions(ctx, name, region)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, versions)
}
