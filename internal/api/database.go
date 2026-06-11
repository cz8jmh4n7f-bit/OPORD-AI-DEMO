package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type databaseDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	Engine        string    `json:"engine"`
	Version       string    `json:"version,omitempty"`
	InstanceClass string    `json:"instanceClass,omitempty"`
	StorageGB     int       `json:"storageGb"`
	DBName        string    `json:"dbName"`
	Endpoint      string    `json:"endpoint,omitempty"`
	Port          int       `json:"port,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func dbToDTO(d orchestrator.DatabaseSummary) databaseDTO {
	r := d.Resource
	out := databaseDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      d.Provider,
		TargetAccount: d.Spec.TargetAccount,
		Status:        r.Status,
		Engine:        d.Spec.Engine,
		Version:       d.Spec.Version,
		InstanceClass: d.Spec.InstanceClass,
		StorageGB:     d.Spec.StorageGB,
		DBName:        d.Spec.DBName,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's DBResult (endpoint/port) once ready.
	if len(r.Observed) > 0 {
		var dr struct {
			Endpoint string `json:"endpoint"`
			Port     int    `json:"port"`
			Error    string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &dr); err == nil {
			out.Endpoint = dr.Endpoint
			out.Port = dr.Port
			out.LastError = dr.Error
		}
	}
	return out
}

func (s *Server) listDatabases(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListDatabases(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]databaseDTO, 0, len(list))
	for _, d := range list {
		out = append(out, dbToDTO(d))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getDatabase(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	d, err := s.svc.DatabaseStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, dbToDTO(*d))
}

type createDatabaseReq struct {
	Name        string              `json:"name"`
	Environment string              `json:"environment"`
	Provider    string              `json:"provider"`
	Spec        models.DatabaseSpec `json:"spec"`
	DryRun      bool                `json:"dryRun"`
}

// createDatabase registers a managed database and provisions it (RDS apply) in
// the background. The apply runs in this (long-lived) API process - databases
// are not yet wired to the River queue (see CLAUDE.md item 38 / task 71).
func (s *Server) createDatabase(w http.ResponseWriter, r *http.Request) {
	var req createDatabaseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateDatabase(r.Context(), orchestrator.CreateDatabaseInput{
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

func (s *Server) destroyDatabase(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.DatabaseStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteDatabaseRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyDatabaseAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
