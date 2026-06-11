// Package models holds OPORD's domain types. These are persistence- and
// transport-agnostic; ClusterSpec is serialized into clusters.desired_spec
// (jsonb) and maps directly onto the wrapped OpenTofu module inputs/outputs
// (see the k8s-platform reference: opentofu/variables.tf, outputs.tf).
package models

// Role is a node's role in the cluster.
type Role string

const (
	RoleControlPlane Role = "control_plane"
	RoleWorker       Role = "worker"
)

// ProviderType identifies an infrastructure backend.
type ProviderType string

const (
	ProviderVSphere ProviderType = "vsphere"
	ProviderProxmox ProviderType = "proxmox"
	ProviderAWS     ProviderType = "aws"
	ProviderAzure   ProviderType = "azure"
	ProviderGCP     ProviderType = "gcp"
)

// IsManagedK8s reports whether a provider's Kubernetes offering is a managed
// control plane (EKS / AKS / GKE) rather than a self-managed kubeadm cluster
// (vSphere / Proxmox). Managed control planes are operated by the cloud: their
// version auto-upgrades, node pools are owned by the provider, and there is no
// SSH access to the control plane. OPORD therefore (a) skips kubeadm bootstrap,
// (b) does not enumerate individual nodes, and (c) must not treat a benign
// tofu-plan diff (e.g. an auto-upgraded patch version) as real drift.
func IsManagedK8s(providerType string) bool {
	switch ProviderType(providerType) {
	case ProviderAWS, ProviderAzure, ProviderGCP:
		return true
	default:
		return false
	}
}

// ClusterStatus mirrors the clusters.status CHECK constraint.
type ClusterStatus string

const (
	StatusPending       ClusterStatus = "pending"
	StatusProvisioning  ClusterStatus = "provisioning"
	StatusBootstrapping ClusterStatus = "bootstrapping"
	StatusReady         ClusterStatus = "ready"
	StatusDegraded      ClusterStatus = "degraded"
	StatusDestroying    ClusterStatus = "destroying"
	StatusDestroyed     ClusterStatus = "destroyed"
	StatusFailed        ClusterStatus = "failed"
)

// ClusterSpec is the declarative desired state for one cluster.
type ClusterSpec struct {
	KubernetesVersion string `json:"kubernetes_version"`
	CNI               string `json:"cni"` // cilium | calico | flannel
	// MachineType is the worker node size for a MANAGED cluster (EKS/AKS/GKE): a GCP
	// machine type (e2-small), an AWS instance type (t3.medium), or an Azure VM size
	// (Standard_B2s). The kubeadm path (vSphere/Proxmox) ignores it and sizes nodes
	// from Workers.Specs (CPU/Memory). Empty = the provider config's default.
	MachineType  string     `json:"machine_type,omitempty"`
	Template     string     `json:"template"`
	ControlPlane NodeGroup  `json:"control_plane"`
	Workers      NodeGroup  `json:"workers"`
	Networking   Networking `json:"networking"`
	SSHUser      string     `json:"ssh_user"`
	SSHPublicKey string     `json:"ssh_public_key"`

	// Deploy target (ADR-0013): a OPORD-managed account to provision the cluster
	// INTO, reusing the provider's own credentials. Provider-neutral - GCP = the
	// target project id (overrides project_id), Azure = the target subscription id,
	// AWS = the member account (cross-account AssumeRole). Empty = the provider's
	// default. Honored by managed clusters (GKE/AKS/EKS).
	TargetAccount string `json:"target_account,omitempty"`
}

// NodeGroup describes a homogeneous set of nodes (control plane or workers).
type NodeGroup struct {
	Count      int      `json:"count"`
	NamePrefix string   `json:"name_prefix"`
	IPStart    string   `json:"ip_start"` // first IP; subsequent nodes increment the last octet
	Specs      NodeSpec `json:"specs"`
}

// NodeSpec is the hardware sizing for a node.
type NodeSpec struct {
	CPU         int   `json:"cpu"`
	MemoryMB    int   `json:"memory_mb"`
	DiskGB      int   `json:"disk_gb"`
	DataDisksGB []int `json:"data_disks_gb,omitempty"`
}

// Networking captures both the VM-level network config and the cluster CIDRs.
type Networking struct {
	Netmask                  string   `json:"netmask"`
	Gateway                  string   `json:"gateway"`
	DNSServers               []string `json:"dns_servers"`
	DNSSuffix                string   `json:"dns_suffix"`
	ControlPlaneEndpoint     string   `json:"control_plane_endpoint"`
	ControlPlaneEndpointPort int      `json:"control_plane_endpoint_port"`
	PodCIDR                  string   `json:"pod_cidr"`
	ServiceCIDR              string   `json:"service_cidr"`
}

// Node is an observed cluster node, as reported by a provider after provisioning.
type Node struct {
	Name      string `json:"name"`
	Role      Role   `json:"role"`
	IPAddress string `json:"ip_address"`
	VMMoid    string `json:"vm_moid,omitempty"`
}
