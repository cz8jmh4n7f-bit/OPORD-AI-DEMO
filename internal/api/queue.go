package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type queueDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Status        string    `json:"status"`
	QueueName     string    `json:"queueName"`
	QueueARN      string    `json:"queueArn,omitempty"`
	QueueURL      string    `json:"queueUrl,omitempty"`
	FIFO          bool      `json:"fifo"`
	DLQEnabled    bool      `json:"dlqEnabled"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func queueToDTO(s orchestrator.QueueSummary) queueDTO {
	r := s.Resource
	queueName := s.Spec.Name
	if queueName == "" {
		queueName = r.Name
	}
	out := queueDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      s.Provider,
		TargetAccount: s.Spec.TargetAccount,
		Status:        r.Status,
		QueueName:     queueName,
		FIFO:          s.Spec.FIFO,
		DLQEnabled:    s.Spec.DLQEnabled,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// observed holds the provider's QueueResult once ready.
	if len(r.Observed) > 0 {
		var qr struct {
			QueueARN string `json:"queue_arn"`
			QueueURL string `json:"queue_url"`
			Error    string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &qr); err == nil {
			out.QueueARN = qr.QueueARN
			out.QueueURL = qr.QueueURL
			out.LastError = qr.Error
		}
	}
	return out
}

func (s *Server) listQueues(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListQueues(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]queueDTO, 0, len(list))
	for _, item := range list {
		out = append(out, queueToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getQueue(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.QueueStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, queueToDTO(*item))
}

type createQueueReq struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.QueueSpec `json:"spec"`
	DryRun      bool             `json:"dryRun"`
}

func (s *Server) createQueue(w http.ResponseWriter, r *http.Request) {
	var req createQueueReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateQueue(r.Context(), orchestrator.CreateQueueInput{
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

func (s *Server) destroyQueue(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.QueueStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteQueueRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyQueueAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
