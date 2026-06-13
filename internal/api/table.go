package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type tableDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	HashKey       string    `json:"hashKey"`
	HashKeyType   string    `json:"hashKeyType,omitempty"`
	RangeKey      string    `json:"rangeKey,omitempty"`
	BillingMode   string    `json:"billingMode"`
	ARN           string    `json:"arn,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func tableToDTO(t orchestrator.TableSummary) tableDTO {
	r := t.Resource
	billing := t.Spec.BillingMode
	if billing == "" {
		billing = "PAY_PER_REQUEST"
	}
	out := tableDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      t.Provider,
		TargetAccount: t.Spec.TargetAccount,
		Status:        r.Status,
		HashKey:       t.Spec.HashKey,
		HashKeyType:   t.Spec.HashKeyType,
		RangeKey:      t.Spec.RangeKey,
		BillingMode:   billing,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's TableResult (arn/name) once ready.
	if len(r.Observed) > 0 {
		var tr struct {
			ARN   string `json:"arn"`
			Error string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &tr); err == nil {
			out.ARN = tr.ARN
			out.LastError = tr.Error
		}
	}
	return out
}

func (s *Server) listTables(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListTables(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]tableDTO, 0, len(list))
	for _, t := range list {
		out = append(out, tableToDTO(t))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getTable(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	t, err := s.svc.TableStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, tableToDTO(*t))
}

type createTableReq struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.TableSpec `json:"spec"`
	DryRun      bool             `json:"dryRun"`
}

func (s *Server) createTable(w http.ResponseWriter, r *http.Request) {
	var req createTableReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateTable(r.Context(), orchestrator.CreateTableInput{
		Name:        req.Name,
		Environment: req.Environment,
		Provider:    req.Provider,
		Spec:        req.Spec,
		DryRun:      req.DryRun,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if res.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{"dryRun": true, "summary": res.Summary})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     res.Resource.ID.String(),
		"name":   res.Resource.Name,
		"status": res.Resource.Status,
	})
}

func (s *Server) destroyTable(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.TableStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteTableRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyTableAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
