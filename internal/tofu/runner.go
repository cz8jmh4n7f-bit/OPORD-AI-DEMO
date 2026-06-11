// Package tofu is a thin wrapper around the OpenTofu CLI. It shells out rather
// than reimplementing Tofu, preserving the behavior of the wrapped modules.
// Secrets are passed via -var-file (a JSON file), never on argv.
package tofu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Runner executes tofu commands in a fixed working directory (a module dir).
type Runner struct {
	bin       string
	workDir   string
	log       *slog.Logger
	env       []string // extra "KEY=value" env appended to each tofu subprocess
	workspace string   // when set, passed as TF_WORKSPACE (see SelectWorkspace)
}

// New returns a Runner. An empty bin defaults to "tofu"; a nil logger defaults
// to slog.Default().
func New(bin, workDir string, log *slog.Logger) *Runner {
	if bin == "" {
		bin = "tofu"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Runner{bin: bin, workDir: workDir, log: log}
}

// SetEnv adds extra environment variables to every tofu subprocess this Runner
// spawns (e.g. AWS_ACCESS_KEY_ID resolved from Vault). Empty values are skipped,
// so callers can pass a sparse map without clobbering the ambient environment.
// Called before any exec; later keys override earlier ones via the OS rule that
// the last assignment wins.
func (r *Runner) SetEnv(kv map[string]string) {
	for k, v := range kv {
		if v != "" {
			r.env = append(r.env, k+"="+v)
		}
	}
}

type result struct {
	stdout   string
	stderr   string
	exitCode int
}

// exec runs `tofu <args...>` in the work dir. A non-zero tofu exit is reported
// via result.exitCode (not a Go error); a Go error means the process could not
// be started.
func (r *Runner) exec(ctx context.Context, args ...string) (result, error) {
	r.log.Debug("tofu", slog.String("dir", r.workDir), slog.String("args", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = r.workDir
	cmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1", "TF_INPUT=0")
	cmd.Env = append(cmd.Env, r.env...)
	// Select the workspace via the environment, NOT `tofu workspace select` (which
	// writes the shared .terraform/environment file in the module dir and races when
	// provisions run concurrently in the SAME module dir). TF_WORKSPACE is honored by
	// every command and is process-local, so concurrent runs never collide. Skipped
	// for `workspace` subcommands, which reject a TF_WORKSPACE override.
	if r.workspace != "" && (len(args) == 0 || args[0] != "workspace") {
		cmd.Env = append(cmd.Env, "TF_WORKSPACE="+r.workspace)
	}
	// OpenTofu native state encryption (when a passphrase is configured) - encrypts
	// the pg-backend state + plan files at rest. No-op when disabled.
	if tfe := stateEncryptionEnv(); tfe != "" {
		cmd.Env = append(cmd.Env, tfe)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := result{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.exitCode = ee.ExitCode()
			return res, nil
		}
		return res, fmt.Errorf("starting tofu %s: %w", args[0], err)
	}
	return res, nil
}

// Init runs `tofu init`, wiring the pg backend from backendConfig
// (e.g. {"conn_str": "postgres://..."}).
func (r *Runner) Init(ctx context.Context, backendConfig map[string]string) error {
	// -reconfigure so switching a module dir between backendless (Preflight) and
	// the pg backend (Provision) never fails on a backend change.
	args := []string{"init", "-input=false", "-no-color", "-reconfigure"}
	for k, v := range backendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s=%s", k, v))
	}
	res, err := r.exec(ctx, args...)
	if err != nil {
		return err
	}
	if res.exitCode != 0 {
		return fmt.Errorf("tofu init failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
	return nil
}

// InitBackendless runs `tofu init -backend=false` - installs providers and
// validates module wiring without configuring remote state. Used for offline
// preflight (no Postgres, no provider API contacted).
func (r *Runner) InitBackendless(ctx context.Context) error {
	res, err := r.exec(ctx, "init", "-input=false", "-no-color", "-backend=false", "-reconfigure")
	if err != nil {
		return err
	}
	if res.exitCode != 0 {
		return fmt.Errorf("tofu init failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
	return nil
}

// Validate runs `tofu validate` (offline; does not contact the backend or the
// provider's API).
func (r *Runner) Validate(ctx context.Context) error {
	res, err := r.exec(ctx, "validate", "-no-color")
	if err != nil {
		return err
	}
	if res.exitCode != 0 {
		return fmt.Errorf("tofu validate failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
	return nil
}

// SelectWorkspace makes the runner operate on the named workspace. It ensures the
// workspace exists (a one-time, idempotent `tofu workspace new`) and then targets it
// via TF_WORKSPACE on every subsequent command (see exec) - NOT `tofu workspace
// select`, whose write to the shared .terraform/environment file in the module dir
// races when provisions run concurrently in the SAME module dir. That race is what
// left deploy-into-member RDS state written to the wrong/empty workspace (empty
// outputs) and orphaned the instances on destroy (destroy ran against an empty
// state). TF_WORKSPACE is process-local, so concurrent same-module runs never collide.
func (r *Runner) SelectWorkspace(ctx context.Context, name string) error {
	// Create the workspace if absent. This runs while r.workspace is still empty, so
	// the command carries no TF_WORKSPACE (which `workspace new` rejects). The
	// pg-backend state row is workspace-specific so concurrent creates of DIFFERENT
	// workspaces don't collide; the command's .terraform/environment write is harmless
	// because TF_WORKSPACE overrides workspace selection for every real op below.
	created, err := r.exec(ctx, "workspace", "new", "-no-color", name)
	if err != nil {
		return err
	}
	if created.exitCode != 0 && !strings.Contains(created.stderr, "already exists") {
		return fmt.Errorf("tofu workspace new %q failed: %s", name, tail(created.stderr, created.stdout))
	}
	r.workspace = name
	return nil
}

// Plan runs `tofu plan -detailed-exitcode`. hasChanges is true when tofu reports
// pending changes (exit code 2). When planOut is non-empty the plan is saved
// there for a subsequent Apply.
func (r *Runner) Plan(ctx context.Context, varsFile, planOut string) (hasChanges bool, output string, err error) {
	// -lock-timeout=30s lets us tolerate a brief in-flight lock (e.g. a stale
	// lock left after a laptop wake-up or a colliding job retry) instead of
	// failing the apply immediately.
	args := []string{"plan", "-input=false", "-no-color", "-detailed-exitcode", "-lock-timeout=30s"}
	if varsFile != "" {
		args = append(args, "-var-file="+varsFile)
	}
	if planOut != "" {
		args = append(args, "-out="+planOut)
	}
	res, err := r.exec(ctx, args...)
	if err != nil {
		return false, res.stdout, err
	}
	switch res.exitCode {
	case 0:
		return false, res.stdout, nil
	case 2:
		return true, res.stdout, nil
	default:
		return false, res.stdout, fmt.Errorf("tofu plan failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
}

// Apply applies a saved plan file (or the current config if planFile is empty).
func (r *Runner) Apply(ctx context.Context, planFile string) (string, error) {
	args := []string{"apply", "-input=false", "-no-color", "-auto-approve", "-lock-timeout=30s"}
	if planFile != "" {
		args = append(args, planFile)
	}
	res, err := r.exec(ctx, args...)
	if err != nil {
		return res.stdout, err
	}
	if res.exitCode != 0 {
		return res.stdout, fmt.Errorf("tofu apply failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
	return res.stdout, nil
}

// Destroy tears down all resources tracked by the current workspace.
func (r *Runner) Destroy(ctx context.Context, varsFile string) error {
	args := []string{"destroy", "-input=false", "-no-color", "-auto-approve", "-lock-timeout=30s"}
	if varsFile != "" {
		args = append(args, "-var-file="+varsFile)
	}
	res, err := r.exec(ctx, args...)
	if err != nil {
		return err
	}
	if res.exitCode != 0 {
		return fmt.Errorf("tofu destroy failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
	return nil
}

// DestroyTarget destroys only the given resource address (plus its dependents),
// leaving the rest of the workspace's resources in state and in the cloud. The GCP
// account factory uses it to delete just the project - which removes all child
// resources - without a full `tofu destroy` choking on the un-deletable KMS
// keyring or racing the per-CSA folder (which can't be deleted while its project
// is pending-deletion).
func (r *Runner) DestroyTarget(ctx context.Context, varsFile, target string) error {
	args := []string{"destroy", "-input=false", "-no-color", "-auto-approve", "-lock-timeout=30s", "-target=" + target}
	if varsFile != "" {
		args = append(args, "-var-file="+varsFile)
	}
	res, err := r.exec(ctx, args...)
	if err != nil {
		return err
	}
	if res.exitCode != 0 {
		return fmt.Errorf("tofu destroy -target=%s failed (exit %d): %s", target, res.exitCode, tail(res.stderr, res.stdout))
	}
	return nil
}

// Output returns `tofu output -json` decoded to a map of name -> raw JSON value.
func (r *Runner) Output(ctx context.Context) (map[string]json.RawMessage, error) {
	res, err := r.exec(ctx, "output", "-json", "-no-color")
	if err != nil {
		return nil, err
	}
	if res.exitCode != 0 {
		return nil, fmt.Errorf("tofu output failed (exit %d): %s", res.exitCode, tail(res.stderr, res.stdout))
	}
	var raw map[string]struct {
		Value     json.RawMessage `json:"value"`
		Sensitive bool            `json:"sensitive"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &raw); err != nil {
		return nil, fmt.Errorf("parsing tofu outputs: %w", err)
	}
	out := make(map[string]json.RawMessage, len(raw))
	for k, v := range raw {
		out[k] = v.Value
	}
	return out, nil
}

// tail joins non-empty parts and trims the result to a bounded length, keeping
// the tail (where tofu errors usually are).
func tail(parts ...string) string {
	// Cap EACH part separately: a long stdout (e.g. a tofu plan diff) must not
	// push the stderr error - where tofu actually reports the failure - out of
	// the window. Called as tail(stderr, stdout), so the error is shown first.
	const perPart = 3000
	var b strings.Builder
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if len(s) > perPart {
			s = "..." + s[len(s)-perPart:]
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s)
	}
	return b.String()
}
