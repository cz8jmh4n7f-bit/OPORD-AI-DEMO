package events

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/glpi"
)

// glpiSink records provisioned resources as Configuration Items in GLPI's CMDB.
// It reacts only to "ready" events (a resource finished provisioning), creating
// one CI per resource via the GLPI REST API.
type glpiSink struct {
	client   *glpi.Client
	itemType string
}

// NewGLPISink builds a GLPI CMDB connector.
func NewGLPISink(url, appToken, userToken, itemType string) Sink {
	if itemType == "" {
		itemType = "Computer"
	}
	return &glpiSink{client: glpi.New(url, appToken, userToken), itemType: itemType}
}

func (g *glpiSink) Name() string { return "glpi" }

func (g *glpiSink) Emit(ctx context.Context, e Event) error {
	// Only materialize a CMDB object once a resource is actually provisioned.
	if e.Action != "ready" {
		return nil
	}
	comment := fmt.Sprintf("OPORD %s on %s (%s). %s", e.Kind, e.Provider, e.Environment, e.Message)
	_, err := g.client.AddItem(ctx, g.itemType, map[string]any{
		"name":    e.Name,
		"comment": comment,
	})
	return err
}
