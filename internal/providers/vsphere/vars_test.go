package vsphere

import (
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

func TestNetmaskToBits(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"24", 24},
		{"/25", 25},
		{"255.255.255.0", 24},
		{"255.255.255.128", 25},
		{"255.255.0.0", 16},
		{"255.255.255.255", 32},
		{"", 24},
		{"bogus", 24},
		{"33", 24},
		{"-1", 24},
	}
	for _, c := range cases {
		if got := netmaskToBits(c.in); got != c.want {
			t.Errorf("netmaskToBits(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func sampleRequest() providers.Request {
	return providers.Request{
		Workspace: "cluster-abc",
		Spec: models.ClusterSpec{
			KubernetesVersion: "1.34.3",
			CNI:               "cilium",
			Template:          "debian-12",
			ControlPlane: models.NodeGroup{
				Count: 3, NamePrefix: "k8s-cp", IPStart: "10.0.0.80",
				Specs: models.NodeSpec{CPU: 4, MemoryMB: 8192, DiskGB: 50},
			},
			Workers: models.NodeGroup{
				Count: 2, NamePrefix: "k8s-worker", IPStart: "10.0.0.85",
				Specs: models.NodeSpec{CPU: 8, MemoryMB: 16384, DiskGB: 100, DataDisksGB: []int{200}},
			},
			Networking: models.Networking{
				Netmask: "255.255.255.0", Gateway: "10.0.0.1",
				DNSServers: []string{"10.0.0.1"}, DNSSuffix: "k8s.local",
				ControlPlaneEndpoint: "10.0.0.80", ControlPlaneEndpointPort: 6443,
			},
			SSHUser: "debian", SSHPublicKey: "ssh-rsa AAA",
		},
		Credentials: map[string]string{"user": "admin@vsphere.local", "password": "secret"},
		Config: map[string]any{
			"server": "vc.example.com", "datacenter": "DC", "cluster": "Compute",
			"datastore": "DS1", "network": "vm-network", "folder": "k8s",
			"allow_unverified_ssl": true, "cluster_name": "k8s-dev", "environment": "dev",
		},
	}
}

func TestBuildVars(t *testing.T) {
	vars := buildVars(sampleRequest())

	checks := map[string]any{
		"vsphere_server":              "vc.example.com",
		"vsphere_user":                "admin@vsphere.local",
		"vsphere_password":            "secret",
		"vsphere_datacenter":          "DC",
		"vsphere_cluster":             "Compute",
		"vsphere_datastore":           "DS1",
		"vsphere_network":             "vm-network",
		"vsphere_folder_path":         "k8s",
		"template_name":               "debian-12",
		"control_plane_count":         3,
		"worker_count":                2,
		"cp_name_prefix":              "k8s-cp",
		"worker_name_prefix":          "k8s-worker",
		"cp_ip_start":                 "10.0.0.80",
		"worker_ip_start":             "10.0.0.85",
		"netmask_bits":                24,
		"gateway":                     "10.0.0.1",
		"dns_suffix":                  "k8s.local",
		"control_plane_endpoint":      "10.0.0.80",
		"control_plane_endpoint_port": 6443,
		"ssh_user":                    "debian",
		"ssh_public_key":              "ssh-rsa AAA",
		"cluster_name":                "k8s-dev",
		"environment":                 "dev",
	}
	for k, want := range checks {
		if got := vars[k]; got != want {
			t.Errorf("vars[%q] = %#v, want %#v", k, got, want)
		}
	}

	if vars["vsphere_allow_unverified_ssl"] != true {
		t.Errorf("vsphere_allow_unverified_ssl = %#v, want true", vars["vsphere_allow_unverified_ssl"])
	}

	cp, ok := vars["control_plane_specs"].(map[string]any)
	if !ok || cp["cpu"] != 4 || cp["memory"] != 8192 || cp["disk"] != 50 {
		t.Errorf("control_plane_specs = %#v", vars["control_plane_specs"])
	}
	wk, ok := vars["worker_specs"].(map[string]any)
	if !ok || wk["cpu"] != 8 || wk["memory"] != 16384 || wk["disk"] != 100 {
		t.Errorf("worker_specs = %#v", vars["worker_specs"])
	}

	dataDisks, ok := vars["worker_data_disks"].([]int)
	if !ok || len(dataDisks) != 1 || dataDisks[0] != 200 {
		t.Errorf("worker_data_disks = %#v", vars["worker_data_disks"])
	}
}

func TestBuildVarsDefaults(t *testing.T) {
	req := providers.Request{} // empty
	vars := buildVars(req)

	if vars["control_plane_endpoint_port"] != 6443 {
		t.Errorf("default port = %#v, want 6443", vars["control_plane_endpoint_port"])
	}
	if vars["netmask_bits"] != 24 {
		t.Errorf("default netmask_bits = %#v, want 24", vars["netmask_bits"])
	}
	if dd, ok := vars["worker_data_disks"].([]int); !ok || len(dd) != 0 {
		t.Errorf("default worker_data_disks = %#v, want empty []int", vars["worker_data_disks"])
	}
	// optional keys must be absent when not provided
	if _, present := vars["cluster_name"]; present {
		t.Errorf("cluster_name should be absent when not configured")
	}
	if _, present := vars["environment"]; present {
		t.Errorf("environment should be absent when not configured")
	}
}
