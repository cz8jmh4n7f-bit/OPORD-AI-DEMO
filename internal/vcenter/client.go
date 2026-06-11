// Package vcenter is a read-only client for the vSphere Web Services API
// (via govmomi). OPORD uses it to track vCenter Tasks (job progress) and report
// live VM/inventory state - a complement to Tofu, which drives provisioning.
// It is verified against the in-process simulator (vcsim) so it needs no real
// vCenter to test.
package vcenter

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// Config describes how to reach a vCenter (or vcsim) endpoint.
type Config struct {
	URL      string // e.g. https://vcenter.example.com/sdk
	User     string
	Password string
	Insecure bool
}

// Client is a connected vSphere Web Services API client.
type Client struct {
	vim    *vim25.Client
	finder *find.Finder
}

// VMInfo is a flattened view of a virtual machine.
type VMInfo struct {
	Name       string
	PowerState string
	IP         string
	UUID       string
	NumCPU     int
	MemoryMB   int
}

// TaskInfo is a flattened view of a vCenter Task (used for job progress).
type TaskInfo struct {
	DescriptionID string
	State         string // queued | running | success | error
	Progress      int32  // 0-100 while running
	EntityName    string
}

// Connect logs in to vCenter and returns a Client. Close it when done.
func Connect(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("vcenter: URL is required")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("vcenter: parsing URL: %w", err)
	}
	if cfg.User != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	}
	gc, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("vcenter: connecting: %w", err)
	}
	return newClient(gc.Client), nil
}

// newClient wraps an existing vim25 client (used by Connect and by tests that
// drive the embedded simulator).
func newClient(vc *vim25.Client) *Client {
	return &Client{vim: vc, finder: find.NewFinder(vc, true)}
}

// Close logs out the session.
func (c *Client) Close(ctx context.Context) error {
	return session.NewManager(c.vim).Logout(ctx)
}

// ListVMs returns all virtual machines visible to the session.
func (c *Client) ListVMs(ctx context.Context) ([]VMInfo, error) {
	m := view.NewManager(c.vim)
	v, err := m.CreateContainerView(ctx, c.vim.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("vcenter: creating view: %w", err)
	}
	defer func() { _ = v.Destroy(ctx) }()

	var vms []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary"}, &vms); err != nil {
		return nil, fmt.Errorf("vcenter: retrieving VMs: %w", err)
	}

	out := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		info := VMInfo{
			Name:       vm.Summary.Config.Name,
			PowerState: string(vm.Summary.Runtime.PowerState),
			UUID:       vm.Summary.Config.Uuid,
			NumCPU:     int(vm.Summary.Config.NumCpu),
			MemoryMB:   int(vm.Summary.Config.MemorySizeMB),
		}
		if vm.Summary.Guest != nil {
			info.IP = vm.Summary.Guest.IpAddress
		}
		out = append(out, info)
	}
	return out, nil
}

// RecentTasks returns the vCenter recent-task list (the basis for job progress).
func (c *Client) RecentTasks(ctx context.Context) ([]TaskInfo, error) {
	pc := property.DefaultCollector(c.vim)

	var tm mo.TaskManager
	if err := pc.RetrieveOne(ctx, *c.vim.ServiceContent.TaskManager, []string{"recentTask"}, &tm); err != nil {
		return nil, fmt.Errorf("vcenter: reading task manager: %w", err)
	}
	if len(tm.RecentTask) == 0 {
		return []TaskInfo{}, nil
	}

	var tasks []mo.Task
	if err := pc.Retrieve(ctx, tm.RecentTask, []string{"info"}, &tasks); err != nil {
		return nil, fmt.Errorf("vcenter: retrieving tasks: %w", err)
	}

	out := make([]TaskInfo, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, TaskInfo{
			DescriptionID: t.Info.DescriptionId,
			State:         string(t.Info.State),
			Progress:      t.Info.Progress,
			EntityName:    t.Info.EntityName,
		})
	}
	return out, nil
}

// CheckInventory verifies the named vSphere objects exist (a real-vCenter
// preflight). Empty names are skipped.
func (c *Client) CheckInventory(ctx context.Context, datacenter, datastore, network, template string) error {
	dc, err := c.finder.Datacenter(ctx, datacenter)
	if err != nil {
		return fmt.Errorf("datacenter %q: %w", datacenter, err)
	}
	c.finder.SetDatacenter(dc)

	var missing []string
	if datastore != "" {
		if _, err := c.finder.Datastore(ctx, datastore); err != nil {
			missing = append(missing, "datastore "+datastore)
		}
	}
	if network != "" {
		if _, err := c.finder.Network(ctx, network); err != nil {
			missing = append(missing, "network "+network)
		}
	}
	if template != "" {
		if _, err := c.finder.VirtualMachine(ctx, template); err != nil {
			missing = append(missing, "template "+template)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("not found in %q: %s", datacenter, strings.Join(missing, ", "))
	}
	return nil
}
