package api

import "net/http"

// getCompliance returns the guardrail scorecard for the caller's visible
// resources (tenant-scoped). The scorecard struct is already JSON-tagged, so it
// is written through directly. Read access: viewer and up.
func (s *Server) getCompliance(w http.ResponseWriter, r *http.Request) {
	rep, err := s.svc.ComplianceReport(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
