package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type s3DTO struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Environment          string    `json:"environment"`
	Provider             string    `json:"provider"`
	TargetAccount        string    `json:"targetAccount,omitempty"`
	Status               string    `json:"status"`
	BucketName           string    `json:"bucketName"`
	BucketARN            string    `json:"bucketArn,omitempty"`
	DomainName           string    `json:"domainName,omitempty"`
	Versioning           bool      `json:"versioning"`
	BlockPublicAccess    bool      `json:"blockPublicAccess"`
	KMSKeyARN            string    `json:"kmsKeyArn,omitempty"`
	LifecycleGlacierDays int       `json:"lifecycleGlacierDays,omitempty"`
	LastError            string    `json:"lastError,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func s3ToDTO(s orchestrator.S3Summary) s3DTO {
	r := s.Resource
	bucketName := s.Spec.Name
	if bucketName == "" {
		bucketName = r.Name
	}
	out := s3DTO{
		ID:                   r.ID.String(),
		Name:                 r.Name,
		Environment:          r.Environment,
		Provider:             s.Provider,
		TargetAccount:        s.Spec.TargetAccount,
		Status:               r.Status,
		BucketName:           bucketName,
		Versioning:           true,
		BlockPublicAccess:    true,
		KMSKeyARN:            s.Spec.KMSKeyARN,
		LifecycleGlacierDays: s.Spec.LifecycleGlacierDays,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
	// observed holds the provider's S3Result once ready.
	if len(r.Observed) > 0 {
		var sr struct {
			BucketARN  string `json:"bucket_arn"`
			DomainName string `json:"bucket_regional_domain_name"`
			Error      string `json:"error"` // Finding E: provision-failure reason
		}
		if err := json.Unmarshal(r.Observed, &sr); err == nil {
			out.BucketARN = sr.BucketARN
			out.DomainName = sr.DomainName
			out.LastError = sr.Error
		}
	}
	return out
}

func (s *Server) listS3(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListS3(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]s3DTO, 0, len(list))
	for _, item := range list {
		out = append(out, s3ToDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getS3(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	item, err := s.svc.S3Status(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, s3ToDTO(*item))
}

type createS3Req struct {
	Name        string        `json:"name"`
	Environment string        `json:"environment"`
	Provider    string        `json:"provider"`
	Spec        models.S3Spec `json:"spec"`
	DryRun      bool          `json:"dryRun"`
}

func (s *Server) createS3(w http.ResponseWriter, r *http.Request) {
	var req createS3Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateS3(r.Context(), orchestrator.CreateS3Input{
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

func (s *Server) destroyS3(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.S3Status(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteS3Record(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyS3Async(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
