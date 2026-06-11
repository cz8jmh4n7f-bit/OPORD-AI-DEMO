package vsphere

import (
	"encoding/json"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// parseOutputs converts the module's `tofu output -json` payload into a
// ProvisionResult that the Ansible bootstrap phase can consume.
func parseOutputs(outs map[string]json.RawMessage) (*providers.ProvisionResult, error) {
	cpIPs, _ := decodeStringSlice(outs, "control_plane_ips")
	cpNames, _ := decodeStringSlice(outs, "control_plane_names")
	wkIPs, _ := decodeStringSlice(outs, "worker_ips")
	wkNames, _ := decodeStringSlice(outs, "worker_names")

	nodes := make([]models.Node, 0, len(cpNames)+len(wkNames))
	for i, name := range cpNames {
		n := models.Node{Name: name, Role: models.RoleControlPlane}
		if i < len(cpIPs) {
			n.IPAddress = cpIPs[i]
		}
		nodes = append(nodes, n)
	}
	for i, name := range wkNames {
		n := models.Node{Name: name, Role: models.RoleWorker}
		if i < len(wkIPs) {
			n.IPAddress = wkIPs[i]
		}
		nodes = append(nodes, n)
	}

	endpoint, _ := decodeString(outs, "control_plane_endpoint")
	inventory, _ := decodeString(outs, "ansible_inventory")

	raw := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			raw[k] = val
		}
	}

	return &providers.ProvisionResult{
		Nodes:                nodes,
		ControlPlaneEndpoint: endpoint,
		AnsibleInventory:     inventory,
		RawOutputs:           raw,
	}, nil
}

func decodeString(outs map[string]json.RawMessage, key string) (string, error) {
	raw, ok := outs[key]
	if !ok {
		return "", fmt.Errorf("output %q missing", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("decoding output %q: %w", key, err)
	}
	return s, nil
}

func decodeStringSlice(outs map[string]json.RawMessage, key string) ([]string, error) {
	raw, ok := outs[key]
	if !ok {
		return nil, fmt.Errorf("output %q missing", key)
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decoding output %q: %w", key, err)
	}
	return s, nil
}
