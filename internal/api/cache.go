package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type cacheDTO struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Environment         string    `json:"environment"`
	Provider            string    `json:"provider"`
	TargetAccount       string    `json:"targetAccount,omitempty"`
	Status              string    `json:"status"`
	CacheName           string    `json:"cacheName"`
	PrimaryEndpoint     string    `json:"primaryEndpoint,omitempty"`
	Port                int       `json:"port,omitempty"`
	EngineVersion       string    `json:"engineVersion,omitempty"`
	NodeType            string    `json:"nodeType,omitempty"`
	NumCacheNodes       int       `json:"numCacheNodes,omitempty"`
	InTransitEncryption bool      `json:"inTransitEncryption"`
	LastError           string    `json:"lastError,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func cacheToDTO(s orchestrator.CacheSummary) cacheDTO {
	r := s.Resource
	cacheName := s.Spec.Name
	if cacheName == "" {
		cacheName = r.Name
	}
	out := cacheDTO{
		ID:                  r.ID.String(),
		Name:                r.Name,
		Environment:         r.Environment,
		Provider:            s.Provider,
		TargetAccount:       s.Spec.TargetAccount,
		Status:              r.Status,
		CacheName:           cacheName,
		EngineVersion:       s.Spec.EngineVersion,
		NodeType:            s.Spec.NodeType,
		NumCacheNodes:       s.Spec.NumCacheNodes,
		InTransitEncryption: s.Spec.InTransitEncryption,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}
	// observed holds the provider's CacheResult once ready.
	if len(r.Observed) > 0 {
		var cr struct {
			PrimaryEndpoint string `json:"primary_endpoint"`
			Port            int    `json:"port"`
			Error           string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &cr); err == nil {
			out.PrimaryEndpoint = cr.PrimaryEndpoint
			out.Port = cr.Port
			out.LastError = cr.Error
		}
	}
	return out
}

func (s *Server) listCaches(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListCaches(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]cacheDTO, 0, len(list))
	for _, item := range list {
		out = append(out, cacheToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getCache(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.CacheStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, cacheToDTO(*item))
}

type createCacheReq struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.CacheSpec `json:"spec"`
	DryRun      bool             `json:"dryRun"`
}

func (s *Server) createCache(w http.ResponseWriter, r *http.Request) {
	var req createCacheReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateCache(r.Context(), orchestrator.CreateCacheInput{
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

func (s *Server) destroyCache(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.CacheStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteCacheRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyCacheAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
