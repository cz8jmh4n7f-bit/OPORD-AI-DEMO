package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// ReapCluster is the durable destroy-guard (providers.ClusterReaper). After a tofu
// destroy, it verifies the GKE cluster is actually gone - catching the orphan a
// provision killed mid-apply leaves behind (the cloud cluster exists but tofu state
// was never written, so `tofu destroy` was a silent no-op). See the interface doc.
var _ providers.ClusterReaper = (*Provider)(nil)

func (p *Provider) ReapCluster(ctx context.Context, req providers.Request) error {
	token := req.Credentials["access_token"]
	if token == "" {
		// No in-process OAuth token (keyless ADC on a laptop) - we can't probe the
		// REST API, so skip the guard rather than block the destroy. The token IS
		// present on the OpenBao dynamic-creds path the worker normally runs with.
		return nil
	}
	// Deploy-into a governed project (ADR-0013) overrides where the cluster lives.
	project := cfgString(req.Config, "project_id")
	if t := req.Spec.TargetAccount; t != "" {
		project = t
	}
	if project == "" {
		return nil
	}
	name := gkeClusterName(req)

	status, location := findGKECluster(ctx, token, project, name)
	if location == "" {
		return nil // not found to genuinely gone (the normal case)
	}
	// An in-flight create/delete blocks deletion - return a retryable error so the
	// durable destroy job retries until the operation settles, then deletes.
	switch status {
	case "PROVISIONING", "STOPPING", "RECONCILING":
		return fmt.Errorf("orphaned gke cluster %q is %s (in-flight operation) - retrying destroy", name, status)
	}
	// RUNNING / ERROR / DEGRADED to a real orphan tofu missed; force-delete it.
	url := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s", project, location, name)
	dreq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("reaping orphaned gke cluster %q: %w", name, err)
	}
	dreq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(dreq)
	if err != nil {
		return fmt.Errorf("reaping orphaned gke cluster %q: %w", name, err)
	}
	resp.Body.Close()
	// The delete is async; keep the record "destroying" and retry so the next pass
	// confirms it gone (STOPPING to retry to not-found to nil to marked destroyed).
	return fmt.Errorf("force-deleted orphaned gke cluster %q (tofu state was empty) - retrying to confirm gone", name)
}

// findGKECluster returns a cluster's status + location by name, searching ALL
// locations (we may not know its zone). ("","") when it isn't found.
func findGKECluster(ctx context.Context, token, project, name string) (status, location string) {
	url := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/-/clusters", project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	var out struct {
		Clusters []struct {
			Name     string `json:"name"`
			Status   string `json:"status"`
			Location string `json:"location"`
		} `json:"clusters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", ""
	}
	for _, c := range out.Clusters {
		if c.Name == name {
			return c.Status, c.Location
		}
	}
	return "", ""
}
