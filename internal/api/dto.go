package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/jackc/pgx/v5/pgtype"
)

// DTOs use camelCase JSON to match the web client's TypeScript types
// (web/src/lib/types.ts), so wiring the frontend is mechanical.

type providerDTO struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Server     string            `json:"server"`
	Datacenter string            `json:"datacenter"`
	Region     string            `json:"region,omitempty"`
	SecretRef  string            `json:"secretRef,omitempty"`
	Config     map[string]any    `json:"config,omitempty"`
	Clusters   int               `json:"clusters"`
	CreatedAt  time.Time         `json:"createdAt"`
	Health     providerHealthDTO `json:"health"`
}

// providerHealthDTO is the last persisted connectivity probe, surfaced on every
// read so external monitoring can scrape GET /providers. Status is "" until the
// first check, then "ok" | "failed" | "unsupported".
type providerHealthDTO struct {
	Status    string     `json:"status"`
	Message   string     `json:"message,omitempty"`
	LatencyMs int        `json:"latencyMs,omitempty"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}

type providerReadinessDTO struct {
	Provider    string                      `json:"provider"`
	Type        string                      `json:"type"`
	Status      string                      `json:"status"`
	Checks      []providerReadinessCheckDTO `json:"checks"`
	NextActions []string                    `json:"nextActions"`
}

type providerReadinessCheckDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type clusterDTO struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Environment       string    `json:"environment"`
	Provider          string    `json:"provider"`
	Status            string    `json:"status"`
	KubernetesVersion string    `json:"kubernetesVersion"`
	CNI               string    `json:"cni"`
	ControlPlanes     int       `json:"controlPlanes"`
	Workers           int       `json:"workers"`
	Endpoint          string    `json:"endpoint"`
	Managed           bool      `json:"managed"`
	LastError         string    `json:"lastError,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// clusterLastError pulls the Finding-E provision-failure reason out of a
// cluster's observed_state (failCluster persists `{"error": …}` there).
func clusterLastError(c db.Cluster) string {
	if len(c.ObservedState) == 0 {
		return ""
	}
	var st struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(c.ObservedState, &st)
	return st.Error
}

type nodeDTO struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	IP     string `json:"ip"`
	Status string `json:"status"`
}

type jobDTO struct {
	ID         string     `json:"id"`
	Cluster    string     `json:"cluster"`
	Operation  string     `json:"operation"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Error      *string    `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type liveVMDTO struct {
	Name       string `json:"name"`
	PowerState string `json:"powerState"`
	IP         string `json:"ip"`
	NumCPU     int    `json:"numCpu"`
	MemoryMB   int    `json:"memoryMb"`
}

type clusterDetailDTO struct {
	clusterDTO
	KubeconfigRef string      `json:"kubeconfigRef,omitempty"`
	Nodes         []nodeDTO   `json:"nodes"`
	Jobs          []jobDTO    `json:"jobs"`
	LiveVMs       []liveVMDTO `json:"liveVMs,omitempty"`
	LiveError     string      `json:"liveError,omitempty"`
}

func specOf(c db.Cluster) models.ClusterSpec {
	var s models.ClusterSpec
	_ = json.Unmarshal(c.DesiredSpec, &s)
	return s
}

func endpointOf(spec models.ClusterSpec) string {
	if spec.Networking.ControlPlaneEndpoint == "" {
		return ""
	}
	port := spec.Networking.ControlPlaneEndpointPort
	if port == 0 {
		port = 6443
	}
	return fmt.Sprintf("%s:%d", spec.Networking.ControlPlaneEndpoint, port)
}

func clusterSummaryToDTO(s orchestrator.ClusterSummary) clusterDTO {
	c := s.Cluster
	spec := specOf(c)
	return clusterDTO{
		ID:                c.ID.String(),
		Name:              c.Name,
		Environment:       c.Environment,
		Provider:          s.Provider,
		Status:            c.Status,
		KubernetesVersion: spec.KubernetesVersion,
		CNI:               spec.CNI,
		ControlPlanes:     s.ControlPlanes,
		Workers:           s.Workers,
		Endpoint:          endpointOf(spec),
		Managed:           models.IsManagedK8s(s.ProviderType),
		LastError:         clusterLastError(c),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

func clusterDetailToDTO(d *orchestrator.ClusterDetail) clusterDetailDTO {
	c := d.Cluster
	base := clusterDTO{
		ID:                c.ID.String(),
		Name:              c.Name,
		Environment:       c.Environment,
		Provider:          d.Provider,
		Status:            c.Status,
		KubernetesVersion: d.Spec.KubernetesVersion,
		CNI:               d.Spec.CNI,
		ControlPlanes:     d.Spec.ControlPlane.Count,
		Workers:           d.Spec.Workers.Count,
		Endpoint:          endpointOf(d.Spec),
		Managed:           models.IsManagedK8s(d.ProviderType),
		LastError:         clusterLastError(c),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}

	nodes := make([]nodeDTO, 0, len(d.Nodes))
	for _, n := range d.Nodes {
		ip := ""
		if n.IpAddress != nil {
			ip = *n.IpAddress
		}
		nodes = append(nodes, nodeDTO{Name: n.Name, Role: n.Role, IP: ip, Status: n.Status})
	}

	jobs := make([]jobDTO, 0, len(d.Jobs))
	for _, j := range d.Jobs {
		jobs = append(jobs, jobDTO{
			ID:         j.ID.String(),
			Cluster:    c.Name,
			Operation:  j.Operation,
			Status:     j.Status,
			StartedAt:  tsPtr(j.StartedAt),
			FinishedAt: tsPtr(j.FinishedAt),
			Error:      j.Error,
			CreatedAt:  j.CreatedAt,
		})
	}

	live := make([]liveVMDTO, 0, len(d.LiveVMs))
	for _, vm := range d.LiveVMs {
		live = append(live, liveVMDTO{
			Name:       vm.Name,
			PowerState: vm.PowerState,
			IP:         vm.IP,
			NumCPU:     vm.NumCPU,
			MemoryMB:   vm.MemoryMB,
		})
	}

	kubeconfig := ""
	if c.KubeconfigRef != nil {
		kubeconfig = *c.KubeconfigRef
	}

	return clusterDetailDTO{clusterDTO: base, KubeconfigRef: kubeconfig, Nodes: nodes, Jobs: jobs, LiveVMs: live, LiveError: d.LiveError}
}

func providerToDTO(p db.Provider, clusterCount int) providerDTO {
	var cfg map[string]any
	_ = json.Unmarshal(p.Config, &cfg)
	server, _ := cfg["server"].(string)
	if server == "" {
		// Proxmox records its API URL under "endpoint"; show it in the same column.
		server, _ = cfg["endpoint"].(string)
	}
	datacenter, _ := cfg["datacenter"].(string)
	region, _ := cfg["region"].(string)
	if region == "" && p.Type == "azure" {
		region, _ = cfg["location"].(string)
	}
	return providerDTO{
		ID:         p.ID.String(),
		Name:       p.Name,
		Type:       p.Type,
		Server:     server,
		Datacenter: datacenter,
		Region:     region,
		SecretRef:  p.SecretRef,
		Config:     cfg,
		Clusters:   clusterCount,
		CreatedAt:  p.CreatedAt,
		Health: providerHealthDTO{
			Status:    p.LastCheckStatus,
			Message:   p.LastCheckMessage,
			LatencyMs: int(p.LastCheckLatencyMs),
			CheckedAt: tsPtr(p.LastCheckAt),
		},
	}
}

// providerCheckDTO is the response to POST /providers/{name}/check - the live
// probe result. (The same outcome is also persisted and surfaced via the
// provider's `health` field on subsequent reads.)
type providerCheckDTO struct {
	Provider  string     `json:"provider"`
	Type      string     `json:"type"`
	OK        bool       `json:"ok"`
	Status    string     `json:"status"`
	Message   string     `json:"message"`
	LatencyMs int        `json:"latencyMs"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}

func providerCheckToDTO(c *orchestrator.ProviderCheck) providerCheckDTO {
	out := providerCheckDTO{
		Provider:  c.Provider,
		Type:      c.Type,
		OK:        c.OK,
		Status:    c.Status,
		Message:   c.Message,
		LatencyMs: c.LatencyMs,
	}
	if !c.CheckedAt.IsZero() {
		t := c.CheckedAt
		out.CheckedAt = &t
	}
	return out
}

func providerReadinessToDTO(r *orchestrator.ProviderReadiness) providerReadinessDTO {
	checks := make([]providerReadinessCheckDTO, 0, len(r.Checks))
	for _, check := range r.Checks {
		checks = append(checks, providerReadinessCheckDTO{
			ID:      check.ID,
			Label:   check.Label,
			Status:  check.Status,
			Message: check.Message,
		})
	}
	return providerReadinessDTO{
		Provider:    r.Provider,
		Type:        r.Type,
		Status:      r.Status,
		Checks:      checks,
		NextActions: r.NextActions,
	}
}

func tsPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
