package vsphere

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/vcenter"
)

var _ providers.Connectivity = (*Provider)(nil)

// CheckConnection implements providers.Connectivity: it logs in to vCenter via
// the Web Services API (govmomi) and logs straight back out. A successful login
// proves both reachability and credential validity without touching inventory.
// Connection details mirror InspectVMs ("url" override or "server", and the
// allow_unverified_ssl flag).
func (p *Provider) CheckConnection(ctx context.Context, creds map[string]string, config map[string]any) error {
	endpoint, _ := config["url"].(string)
	if endpoint == "" {
		server, _ := config["server"].(string)
		if server == "" {
			return fmt.Errorf("vsphere: provider config has neither 'url' nor 'server'")
		}
		endpoint = "https://" + server + "/sdk"
	}
	insecure := true
	if v, ok := config["allow_unverified_ssl"].(bool); ok {
		insecure = v
	}

	c, err := vcenter.Connect(ctx, vcenter.Config{
		URL:      endpoint,
		User:     creds["user"],
		Password: creds["password"],
		Insecure: insecure,
	})
	if err != nil {
		return err
	}
	_ = c.Close(ctx)
	return nil
}
