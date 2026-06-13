package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type requestDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	Requester   string    `json:"requester"`
	Kind        string    `json:"kind"`
	Provider    string    `json:"provider"`
	Blueprint   string    `json:"blueprint,omitempty"`
	Status      string    `json:"status"`
	TicketRef   string    `json:"ticketRef,omitempty"`
	ResourceRef string    `json:"resourceRef,omitempty"`
	DecidedBy   string    `json:"decidedBy,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func requestToDTO(r db.Request) requestDTO {
	return requestDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Environment: r.Environment,
		Requester:   r.Requester,
		Kind:        r.Kind,
		Provider:    r.Provider,
		Blueprint:   r.Blueprint,
		Status:      r.Status,
		TicketRef:   r.TicketRef,
		ResourceRef: r.ResourceRef,
		DecidedBy:   r.DecidedBy,
		Reason:      r.Reason,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (s *Server) listRequests(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListRequests(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]requestDTO, 0, len(list))
	for _, rq := range list {
		out = append(out, requestToDTO(rq))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getRequest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	rq, err := s.svc.RequestStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, requestToDTO(*rq))
}

type createRequestReq struct {
	Name        string          `json:"name"`
	Environment string          `json:"environment"`
	Requester   string          `json:"requester"`
	Kind        string          `json:"kind"`
	Provider    string          `json:"provider"`
	Blueprint   string          `json:"blueprint"`
	Spec        json.RawMessage `json:"spec"`
}

func (s *Server) createRequest(w http.ResponseWriter, r *http.Request) {
	var req createRequestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	rq, err := s.svc.CreateRequest(r.Context(), orchestrator.CreateRequestInput{
		Name:        req.Name,
		Environment: req.Environment,
		Requester:   req.Requester,
		Kind:        req.Kind,
		Provider:    req.Provider,
		Blueprint:   req.Blueprint,
		Spec:        req.Spec,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, requestToDTO(*rq))
}

type decisionReq struct {
	By     string `json:"by"`
	Reason string `json:"reason"`
}

func (s *Server) approveRequest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := queryEnv(r)
	var body decisionReq
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.By == "" {
		body.By = "api"
	}
	if err := s.svc.ApproveRequest(r.Context(), name, env, body.By); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "approved"})
}

func (s *Server) rejectRequest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := queryEnv(r)
	var body decisionReq
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.By == "" {
		body.By = "api"
	}
	if err := s.svc.RejectRequest(r.Context(), name, env, body.By, body.Reason); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "rejected"})
}

func queryEnv(r *http.Request) string {
	env := r.URL.Query().Get("env")
	if env == "" {
		return "dev"
	}
	return env
}
