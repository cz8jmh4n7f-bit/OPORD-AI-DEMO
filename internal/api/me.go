package api

import (
	"net/http"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/auth"
)

type meDTO struct {
	Email  string `json:"email"`
	Tenant string `json:"tenant"`
	Role   string `json:"role"`
}

// getMe returns the authenticated caller's identity (whoami). Used by the web to
// validate an API key on login and to show who's signed in. In dev mode (auth
// off) it returns the injected admin identity.
func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, nil)
		return
	}
	writeJSON(w, http.StatusOK, meDTO{Email: id.Email, Tenant: id.Tenant, Role: string(id.Role)})
}
