// Package ansible is a thin wrapper around the ansible-playbook CLI for the
// provider-agnostic Phase 2 (Kubernetes bootstrap) of cluster delivery. It
// consumes the inventory emitted by a provider's ProvisionResult.
package ansible

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Runner executes ansible-playbook from a fixed directory (so ansible.cfg and
// roles_path resolve correctly).
type Runner struct {
	bin string
	dir string
	log *slog.Logger
}

// New returns a Runner. Empty bin defaults to "ansible-playbook"; nil logger
// defaults to slog.Default().
func New(bin, dir string, log *slog.Logger) *Runner {
	if bin == "" {
		bin = "ansible-playbook"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Runner{bin: bin, dir: dir, log: log}
}

// Options controls a single ansible-playbook invocation.
type Options struct {
	Playbook      string // path to the playbook (relative to dir or absolute)
	InventoryPath string // path to the inventory file
	PrivateKey    string // SSH private key for reaching the nodes
	Limit         string // optional --limit pattern
	ExtraVars     map[string]string
}

// Playbook runs ansible-playbook with the given options and returns combined
// output. A non-zero exit becomes an error.
func (r *Runner) Playbook(ctx context.Context, opts Options) (string, error) {
	if opts.Playbook == "" {
		return "", errors.New("ansible: playbook is required")
	}
	if opts.InventoryPath == "" {
		return "", errors.New("ansible: inventory path is required")
	}

	args := []string{"-i", opts.InventoryPath}
	if opts.PrivateKey != "" {
		args = append(args, "--private-key", opts.PrivateKey)
	}
	if opts.Limit != "" {
		args = append(args, "--limit", opts.Limit)
	}
	// Deterministic extra-var ordering for stable logs/tests.
	keys := make([]string, 0, len(opts.ExtraVars))
	for k := range opts.ExtraVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, opts.ExtraVars[k]))
	}
	args = append(args, opts.Playbook)

	r.log.Debug("ansible-playbook", slog.String("dir", r.dir), slog.String("args", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "ANSIBLE_FORCE_COLOR=0")

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	out := buf.String()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return out, fmt.Errorf("ansible-playbook exited %d: %s", ee.ExitCode(), out)
		}
		return out, fmt.Errorf("starting ansible-playbook: %w", err)
	}
	return out, nil
}

// WriteInventory writes inventory content (e.g. ProvisionResult.AnsibleInventory)
// to a temp file and returns its path plus a cleanup func that is always safe to
// call.
func WriteInventory(content string) (path string, cleanup func(), err error) {
	noop := func() {}
	f, err := os.CreateTemp("", "opord-inventory-*.ini")
	if err != nil {
		return "", noop, fmt.Errorf("creating inventory file: %w", err)
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		remove()
		return "", noop, fmt.Errorf("writing inventory: %w", err)
	}
	if err := f.Close(); err != nil {
		remove()
		return "", noop, fmt.Errorf("closing inventory: %w", err)
	}
	return f.Name(), remove, nil
}
