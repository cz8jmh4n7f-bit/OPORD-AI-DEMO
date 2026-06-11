package vcenter

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
)

// TestClientAgainstSimulator drives the client against govmomi's in-process
// vCenter simulator (vcsim) - no real vCenter required. The default model
// provides datacenter "DC0", datastore "LocalDS_0", and several VMs.
func TestClientAgainstSimulator(t *testing.T) {
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		c := newClient(vc)

		vms, err := c.ListVMs(ctx)
		if err != nil {
			t.Fatalf("ListVMs: %v", err)
		}
		if len(vms) == 0 {
			t.Fatal("expected simulator VMs, got 0")
		}
		for _, vm := range vms {
			if vm.Name == "" {
				t.Errorf("VM with empty name: %+v", vm)
			}
		}

		if err := c.CheckInventory(ctx, "DC0", "LocalDS_0", "", vms[0].Name); err != nil {
			t.Errorf("CheckInventory(valid): %v", err)
		}
		if err := c.CheckInventory(ctx, "DC0", "no-such-ds", "", ""); err == nil {
			t.Error("CheckInventory(missing datastore): expected error, got nil")
		}
		if err := c.CheckInventory(ctx, "no-such-dc", "", "", ""); err == nil {
			t.Error("CheckInventory(missing datacenter): expected error, got nil")
		}

		if _, err := c.RecentTasks(ctx); err != nil {
			t.Errorf("RecentTasks: %v", err)
		}
	})
}
