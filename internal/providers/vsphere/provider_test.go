package vsphere

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

func TestTypeAndRegister(t *testing.T) {
	p := New(Config{ModulesDir: "./modules"})
	if p.Type() != models.ProviderVSphere {
		t.Fatalf("Type() = %q, want %q", p.Type(), models.ProviderVSphere)
	}

	reg := providers.NewRegistry()
	Register(reg, Config{ModulesDir: "./modules"})
	got, err := reg.Get(models.ProviderVSphere)
	if err != nil {
		t.Fatalf("registry.Get: %v", err)
	}
	if got.Type() != models.ProviderVSphere {
		t.Fatalf("registered provider Type() = %q", got.Type())
	}
}

func TestValidate(t *testing.T) {
	p := New(Config{})
	ctx := context.Background()

	good := sampleRequest().Spec
	if err := p.Validate(ctx, good); err != nil {
		t.Fatalf("Validate(good) = %v, want nil", err)
	}

	cases := map[string]func(*models.ClusterSpec){
		"missing template":  func(s *models.ClusterSpec) { s.Template = "" },
		"even cp count":     func(s *models.ClusterSpec) { s.ControlPlane.Count = 2 },
		"zero cp count":     func(s *models.ClusterSpec) { s.ControlPlane.Count = 0 },
		"zero worker count": func(s *models.ClusterSpec) { s.Workers.Count = 0 },
		"missing ip_start":  func(s *models.ClusterSpec) { s.ControlPlane.IPStart = "" },
		"missing endpoint":  func(s *models.ClusterSpec) { s.Networking.ControlPlaneEndpoint = "" },
		"missing gateway":   func(s *models.ClusterSpec) { s.Networking.Gateway = "" },
	}
	for name, mutate := range cases {
		spec := sampleRequest().Spec
		mutate(&spec)
		if err := p.Validate(ctx, spec); err == nil {
			t.Errorf("Validate(%s) = nil, want error", name)
		}
	}
}

func TestParseOutputs(t *testing.T) {
	mk := func(v any) json.RawMessage {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return b
	}
	outs := map[string]json.RawMessage{
		"control_plane_ips":      mk([]string{"10.0.0.80", "10.0.0.81", "10.0.0.82"}),
		"control_plane_names":    mk([]string{"k8s-cp-01", "k8s-cp-02", "k8s-cp-03"}),
		"worker_ips":             mk([]string{"10.0.0.85", "10.0.0.86"}),
		"worker_names":           mk([]string{"k8s-worker-01", "k8s-worker-02"}),
		"control_plane_endpoint": mk("10.0.0.80:6443"),
		"ansible_inventory":      mk("[control_plane_init]\nk8s-cp-01 ansible_host=10.0.0.80\n"),
	}

	res, err := parseOutputs(outs)
	if err != nil {
		t.Fatalf("parseOutputs: %v", err)
	}
	if len(res.Nodes) != 5 {
		t.Fatalf("got %d nodes, want 5", len(res.Nodes))
	}
	if res.Nodes[0].Name != "k8s-cp-01" || res.Nodes[0].Role != models.RoleControlPlane || res.Nodes[0].IPAddress != "10.0.0.80" {
		t.Errorf("node[0] = %+v", res.Nodes[0])
	}
	if res.Nodes[3].Role != models.RoleWorker || res.Nodes[3].Name != "k8s-worker-01" {
		t.Errorf("node[3] = %+v", res.Nodes[3])
	}
	if res.ControlPlaneEndpoint != "10.0.0.80:6443" {
		t.Errorf("endpoint = %q", res.ControlPlaneEndpoint)
	}
	if res.AnsibleInventory == "" {
		t.Errorf("ansible inventory empty")
	}
	if _, ok := res.RawOutputs["control_plane_ips"]; !ok {
		t.Errorf("raw outputs missing control_plane_ips")
	}
}
