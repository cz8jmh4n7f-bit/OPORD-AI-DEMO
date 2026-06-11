package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type secretDTO struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Environment        string    `json:"environment"`
	Provider           string    `json:"provider"`
	TargetAccount      string    `json:"targetAccount,omitempty"`
	Status             string    `json:"status"`
	SecretName         string    `json:"secretName"`
	SecretARN          string    `json:"secretArn,omitempty"`
	URI                string    `json:"uri,omitempty"`
	Description        string    `json:"description,omitempty"`
	KMSKeyARN          string    `json:"kmsKeyArn,omitempty"`
	RecoveryWindowDays int       `json:"recoveryWindowDays,omitempty"`
	RotationDays       int       `json:"rotationDays,omitempty"`
	LastError          string    `json:"lastError,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

func secretToDTO(s orchestrator.SecretSummary) secretDTO {
	r := s.Resource
	secretName := s.Spec.Name
	if secretName == "" {
		secretName = r.Name
	}
	out := secretDTO{
		ID:                 r.ID.String(),
		Name:               r.Name,
		Environment:        r.Environment,
		Provider:           s.Provider,
		TargetAccount:      s.Spec.TargetAccount,
		Status:             r.Status,
		SecretName:         secretName,
		Description:        s.Spec.Description,
		KMSKeyARN:          s.Spec.KMSKeyARN,
		RecoveryWindowDays: s.Spec.RecoveryWindowDays,
		RotationDays:       s.Spec.RotationDays,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
	// observed holds the provider's SecretResult once ready.
	if len(r.Observed) > 0 {
		var sr struct {
			SecretARN string `json:"secret_arn"`
			URI       string `json:"uri"`
			Error     string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &sr); err == nil {
			out.SecretARN = sr.SecretARN
			out.URI = sr.URI
			out.LastError = sr.Error
		}
	}
	return out
}

func (s *Server) listSecrets(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListSecrets(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]secretDTO, 0, len(list))
	for _, item := range list {
		out = append(out, secretToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getSecret(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.SecretStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, secretToDTO(*item))
}

type createSecretReq struct {
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Provider    string            `json:"provider"`
	Spec        models.SecretSpec `json:"spec"`
	DryRun      bool              `json:"dryRun"`
}

func (s *Server) createSecret(w http.ResponseWriter, r *http.Request) {
	var req createSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateSecret(r.Context(), orchestrator.CreateSecretInput{
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

func (s *Server) destroySecret(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.SecretStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteSecretRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroySecretAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
