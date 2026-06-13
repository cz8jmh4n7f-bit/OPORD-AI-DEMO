package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type dnsDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	ZoneName      string    `json:"zoneName,omitempty"`
	NameServers   []string  `json:"nameServers,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func dnsToDTO(s orchestrator.DNSSummary) dnsDTO {
	r := s.Resource
	zoneName := s.Spec.Name
	if zoneName == "" {
		zoneName = r.Name
	}
	out := dnsDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      s.Provider,
		TargetAccount: s.Spec.TargetAccount,
		Status:        r.Status,
		ZoneName:      zoneName,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's DNSResult once ready.
	if len(r.Observed) > 0 {
		var dr struct {
			ZoneName    string   `json:"zone_name"`
			NameServers []string `json:"name_servers"`
			Error       string   `json:"error"` // provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &dr); err == nil {
			if dr.ZoneName != "" {
				out.ZoneName = dr.ZoneName
			}
			out.NameServers = dr.NameServers
			out.LastError = dr.Error
		}
	}
	return out
}

func (s *Server) listDNS(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListDNS(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]dnsDTO, 0, len(list))
	for _, item := range list {
		out = append(out, dnsToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getDNS(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.DNSStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, dnsToDTO(*item))
}

type createDNSReq struct {
	Name        string         `json:"name"`
	Environment string         `json:"environment"`
	Provider    string         `json:"provider"`
	Spec        models.DNSSpec `json:"spec"`
	DryRun      bool           `json:"dryRun"`
}

func (s *Server) createDNS(w http.ResponseWriter, r *http.Request) {
	var req createDNSReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateDNS(r.Context(), orchestrator.CreateDNSInput{
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

func (s *Server) destroyDNS(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.DNSStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteDNSRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyDNSAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
