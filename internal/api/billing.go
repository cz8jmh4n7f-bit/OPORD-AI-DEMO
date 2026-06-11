package api

import (
	"context"
	"net/http"
	"time"
)

// listProviderBillingScopes lists the billing scopes (Azure MCA invoice sections)
// a provider can create subscriptions under, so the account form can offer a
// picker instead of a pasted URI. Read-only. On error (provider can't list, or
// the billing API is unreachable / SP lacks access) it returns the error and the
// frontend falls back to manual entry.
func (s *Server) listProviderBillingScopes(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	scopes, err := s.svc.ListProviderBillingScopes(ctx, name)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, scopes)
}
