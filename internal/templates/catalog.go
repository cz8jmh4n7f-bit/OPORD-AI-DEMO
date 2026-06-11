// Package templates holds OPORD's built-in blueprint catalog: golden-path
// environments (EaaS Layer 2) that expand into concrete cluster/VM component
// specs. The orchestrator consumes Expand to create the child resources.
package templates

import (
	"fmt"
	"sort"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
)

// ExpandParams are the user-supplied knobs applied when instantiating a
// blueprint. Everything else comes from sensible golden-path defaults.
type ExpandParams struct {
	Template     string // golden image / Proxmox VMID / AMI (optional override)
	SSHPublicKey string
}

type entry struct {
	id          string
	name        string
	description string
	build       func(ExpandParams) []models.Component
}

// catalog is the ordered set of built-in blueprints.
var catalog = []entry{
	{
		id:          "k8s-small",
		name:        "Small Kubernetes cluster",
		description: "1 control-plane + 2 workers. Good for dev/test.",
		build: func(p ExpandParams) []models.Component {
			return []models.Component{clusterComponent("cluster", 1, 2, p)}
		},
	},
	{
		id:          "k8s-ha",
		name:        "HA Kubernetes cluster",
		description: "3 control-planes + 3 workers. Production-shaped control plane.",
		build: func(p ExpandParams) []models.Component {
			return []models.Component{clusterComponent("cluster", 3, 3, p)}
		},
	},
	{
		id:          "web-app",
		name:        "Web application environment",
		description: "Small Kubernetes cluster + a standalone cache/bastion VM.",
		build: func(p ExpandParams) []models.Component {
			return []models.Component{
				clusterComponent("cluster", 1, 2, p),
				cacheVMComponent("cache", p),
			}
		},
	},
	{
		id:          "aws-web-stack",
		name:        "AWS web stack (EKS + RDS)",
		description: "Managed Kubernetes (EKS) + a Postgres database (RDS). AWS provider only.",
		build: func(p ExpandParams) []models.Component {
			return []models.Component{
				clusterComponent("cluster", 1, 2, p),
				dbComponent("db", p),
			}
		},
	},
}

// List returns the catalog with components expanded under default params (for
// display in `blueprint list` / the web).
func List() []models.Blueprint {
	out := make([]models.Blueprint, 0, len(catalog))
	for _, e := range catalog {
		out = append(out, models.Blueprint{
			ID:          e.id,
			Name:        e.name,
			Description: e.description,
			Components:  e.build(ExpandParams{}),
		})
	}
	return out
}

// Get returns one blueprint's metadata (default-expanded).
func Get(id string) (models.Blueprint, bool) {
	for _, e := range catalog {
		if e.id == id {
			return models.Blueprint{ID: e.id, Name: e.name, Description: e.description, Components: e.build(ExpandParams{})}, true
		}
	}
	return models.Blueprint{}, false
}

// Expand instantiates a blueprint's components with the given params.
func Expand(id string, p ExpandParams) ([]models.Component, error) {
	for _, e := range catalog {
		if e.id == id {
			return e.build(p), nil
		}
	}
	return nil, fmt.Errorf("unknown blueprint %q (try: %s)", id, joinIDs())
}

func joinIDs() string {
	ids := make([]string, 0, len(catalog))
	for _, e := range catalog {
		ids = append(ids, e.id)
	}
	sort.Strings(ids)
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}

func template(p ExpandParams) string {
	if p.Template != "" {
		return p.Template
	}
	return "9000" // Proxmox template VMID default; ignored by managed providers (EKS)
}

// clusterComponent builds a k8s cluster component with golden-path networking
// defaults that validate on both on-prem (Proxmox/vSphere) and managed (EKS)
// providers - managed providers simply ignore the networking fields.
func clusterComponent(name string, cps, workers int, p ExpandParams) models.Component {
	spec := models.ClusterSpec{
		KubernetesVersion: "1.31.0",
		CNI:               "cilium",
		Template:          template(p),
		ControlPlane: models.NodeGroup{
			Count: cps, NamePrefix: "cp", IPStart: "10.0.0.10",
			Specs: models.NodeSpec{CPU: 2, MemoryMB: 4096, DiskGB: 40},
		},
		Workers: models.NodeGroup{
			Count: workers, NamePrefix: "worker", IPStart: "10.0.0.20",
			Specs: models.NodeSpec{CPU: 2, MemoryMB: 4096, DiskGB: 40},
		},
		Networking: models.Networking{
			Netmask: "255.255.255.0", Gateway: "10.0.0.1",
			DNSServers: []string{"1.1.1.1"}, DNSSuffix: "cluster.local",
			ControlPlaneEndpoint: "10.0.0.10", ControlPlaneEndpointPort: 6443,
			PodCIDR: "10.244.0.0/16", ServiceCIDR: "10.96.0.0/12",
		},
		SSHUser:      "debian",
		SSHPublicKey: p.SSHPublicKey,
	}
	return models.Component{Name: name, Kind: models.ComponentCluster, Cluster: &spec}
}

func dbComponent(name string, p ExpandParams) models.Component {
	spec := models.DatabaseSpec{
		Engine: "postgres", Version: "16", InstanceClass: "db.t3.micro",
		StorageGB: 20, DBName: "app", Username: "appuser",
	}
	return models.Component{Name: name, Kind: models.ComponentDatabase, Database: &spec}
}

func cacheVMComponent(name string, p ExpandParams) models.Component {
	spec := models.VMSpec{
		Template: template(p), Count: 1, NamePrefix: name,
		CPU: 2, MemoryMB: 4096, DiskGB: 40,
		IPStart: "10.0.0.30", Netmask: "255.255.255.0", Gateway: "10.0.0.1",
		DNSServers: []string{"1.1.1.1"}, DNSSuffix: "local",
		SSHUser: "debian", SSHPublicKey: p.SSHPublicKey,
	}
	return models.Component{Name: name, Kind: models.ComponentVM, VM: &spec}
}
