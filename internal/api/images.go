package api

import (
	"context"
	"net/http"
	"time"
)

// listProviderImages lists bootable images (AWS AMIs) for a provider so the web
// VM form can offer an AMI picker. Read-only. On error (provider can't list, or
// AWS is unreachable / has no creds) it returns the error and the frontend falls
// back to manual ID entry.
func (s *Server) listProviderImages(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	region := r.URL.Query().Get("region")
	owner := r.URL.Query().Get("owner")

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	imgs, err := s.svc.ListProviderImages(ctx, name, region, owner)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, imgs)
}
