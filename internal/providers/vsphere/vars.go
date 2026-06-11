package vsphere

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// buildVars maps a provision Request to the OpenTofu variables consumed by
// modules/vsphere-k8s. Connection details come from req.Config, credentials
// from req.Credentials, and sizing/networking from the spec.
func buildVars(req providers.Request) map[string]any {
	spec := req.Spec
	cfg := req.Config
	creds := req.Credentials

	dataDisks := spec.Workers.Specs.DataDisksGB
	if dataDisks == nil {
		dataDisks = []int{}
	}

	port := spec.Networking.ControlPlaneEndpointPort
	if port == 0 {
		port = 6443
	}

	vars := map[string]any{
		"vsphere_server":               cfgString(cfg, "server"),
		"vsphere_user":                 creds["user"],
		"vsphere_password":             creds["password"],
		"vsphere_allow_unverified_ssl": cfgBool(cfg, "allow_unverified_ssl", true),
		"vsphere_datacenter":           cfgString(cfg, "datacenter"),
		"vsphere_cluster":              cfgString(cfg, "cluster"),
		"vsphere_datastore":            cfgString(cfg, "datastore"),
		"vsphere_network":              cfgString(cfg, "network"),
		"vsphere_folder_path":          cfgString(cfg, "folder"),

		"template_name": spec.Template,

		"control_plane_count": spec.ControlPlane.Count,
		"worker_count":        spec.Workers.Count,
		"control_plane_specs": map[string]any{
			"cpu":    spec.ControlPlane.Specs.CPU,
			"memory": spec.ControlPlane.Specs.MemoryMB,
			"disk":   spec.ControlPlane.Specs.DiskGB,
		},
		"worker_specs": map[string]any{
			"cpu":    spec.Workers.Specs.CPU,
			"memory": spec.Workers.Specs.MemoryMB,
			"disk":   spec.Workers.Specs.DiskGB,
		},
		"worker_data_disks": dataDisks,

		"cp_name_prefix":     spec.ControlPlane.NamePrefix,
		"worker_name_prefix": spec.Workers.NamePrefix,

		"cp_ip_start":     spec.ControlPlane.IPStart,
		"worker_ip_start": spec.Workers.IPStart,
		"netmask_bits":    netmaskToBits(spec.Networking.Netmask),
		"gateway":         spec.Networking.Gateway,
		"dns_servers":     spec.Networking.DNSServers,
		"dns_suffix":      spec.Networking.DNSSuffix,

		"control_plane_endpoint":      spec.Networking.ControlPlaneEndpoint,
		"control_plane_endpoint_port": port,

		"ssh_user":       spec.SSHUser,
		"ssh_public_key": spec.SSHPublicKey,
	}

	// Optional, only set when provided so the module defaults apply otherwise.
	if name := cfgString(cfg, "cluster_name"); name != "" {
		vars["cluster_name"] = name
	}
	if env := cfgString(cfg, "environment"); env != "" {
		vars["environment"] = env
	}
	return vars
}

func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func cfgBool(cfg map[string]any, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	v, ok := cfg[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		if b, err := strconv.ParseBool(t); err == nil {
			return b
		}
	}
	return def
}

// netmaskToBits accepts a prefix length ("24", "/24") or a dotted-decimal mask
// ("255.255.255.0") and returns the prefix length in bits. Invalid input falls
// back to 24.
func netmaskToBits(s string) int {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "/"))
	if s == "" {
		return 24
	}
	if strings.Contains(s, ".") {
		bits := 0
		for _, part := range strings.Split(s, ".") {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 || n > 255 {
				return 24
			}
			bits += onesCount(n)
		}
		return bits
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 32 {
		return 24
	}
	return n
}

func onesCount(n int) int {
	count := 0
	for n > 0 {
		count += n & 1
		n >>= 1
	}
	return count
}
