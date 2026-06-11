package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

type vmDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Environment   string    `json:"environment"`
	Provider      string    `json:"provider"`
	TargetAccount string    `json:"targetAccount,omitempty"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	Template      string    `json:"template"`
	Count         int       `json:"count"`
	CPU           int       `json:"cpu"`
	MemoryMB      int       `json:"memoryMb"`
	DiskGB        int       `json:"diskGb"`
	InstanceType  string    `json:"instanceType,omitempty"`
	IPStart       string    `json:"ipStart,omitempty"`
	PublicIP      bool      `json:"publicIp"`
	TTLHours      int       `json:"ttlHours"`
	PublicIPs     []string  `json:"publicIps,omitempty"`
	PrivateIPs    []string  `json:"privateIps,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func vmToDTO(v orchestrator.VMSummary) vmDTO {
	r := v.Resource
	// observed holds the provider's VMResult (assigned IDs/IPs) from the last
	// successful provision - surface the real IPs, not just the spec's ip_start.
	var obs struct {
		PublicIPs  []string `json:"public_ips"`
		PrivateIPs []string `json:"private_ips"`
		Error      string   `json:"error"` // Finding E: provision-failure reason
	}
	if len(r.Observed) > 0 {
		_ = json.Unmarshal(r.Observed, &obs)
	}
	return vmDTO{
		ID:            r.ID.String(),
		Name:          r.Name,
		Environment:   r.Environment,
		Provider:      v.Provider,
		TargetAccount: v.Spec.TargetAccount,
		Kind:          r.Kind,
		Status:        r.Status,
		Template:      v.Spec.Template,
		Count:         v.Spec.Count,
		CPU:           v.Spec.CPU,
		MemoryMB:      v.Spec.MemoryMB,
		DiskGB:        v.Spec.DiskGB,
		InstanceType:  v.Spec.InstanceType,
		IPStart:       v.Spec.IPStart,
		PublicIP:      v.Spec.PublicIP,
		TTLHours:      v.Spec.TTLHours,
		PublicIPs:     obs.PublicIPs,
		PrivateIPs:    obs.PrivateIPs,
		LastError:     obs.Error,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

type createVMReq struct {
	Name        string        `json:"name"`
	Environment string        `json:"environment"`
	Provider    string        `json:"provider"`
	Spec        models.VMSpec `json:"spec"`
	DryRun      bool          `json:"dryRun"`
}

func (s *Server) listVMs(w http.ResponseWriter, r *http.Request) {
	vms, err := s.svc.ListVMs(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]vmDTO, 0, len(vms))
	for _, v := range vms {
		out = append(out, vmToDTO(v))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getVM(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	v, err := s.svc.VMStatus(r.Context(), name, env)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, vmToDTO(*v))
}

func (s *Server) createVM(w http.ResponseWriter, r *http.Request) {
	var req createVMReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.svc.CreateVM(r.Context(), orchestrator.CreateVMInput{
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

type scaleVMReq struct {
	Count int `json:"count"`
}

// scaleVM changes a VM's count and re-provisions (day-2 op).
func (s *Server) scaleVM(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	var req scaleVMReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ScaleVM(r.Context(), name, env, req.Count); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "count": req.Count, "status": "provisioning"})
}

// destroyVM tears down a vm resource (tofu destroy). The lookup runs
// synchronously (so a missing VM returns 404), but the apply itself - which can
// take minutes - runs in the background; status flows destroying -> destroyed/failed.
//
// With ?purge=true it instead forgets the tracking row entirely (no tofu). This
// is allowed only for terminal VMs (destroyed/failed) - DeleteVMRecord enforces
// that - so users can clear out tombstoned rows from the list.
func (s *Server) destroyVM(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "dev"
	}
	if _, err := s.svc.VMStatus(r.Context(), name, env); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if r.URL.Query().Get("purge") == "true" {
		if err := s.svc.DeleteVMRecord(r.Context(), name, env); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "status": "removed"})
		return
	}
	s.svc.DestroyVMAsync(name, env)
	writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "status": "destroying"})
}
