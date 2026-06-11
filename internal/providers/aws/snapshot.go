package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// DatabaseSnapshotter: a manual RDS snapshot via modules/aws-db-snapshot, run in
// the request's own workspace.

func (p *Provider) dbSnapshotModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "aws-db-snapshot")
}

func (p *Provider) writeSnapshotVars(req providers.SnapshotRequest) (string, func(), error) {
	noop := func() {}
	vars := map[string]any{
		"region":        cfgString(req.Config, "region"),
		"db_identifier": req.DBIdentifier,
		"snapshot_name": req.SnapshotName,
	}
	data, err := json.Marshal(vars)
	if err != nil {
		return "", noop, fmt.Errorf("marshaling snapshot vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-snap-*.tfvars.json")
	if err != nil {
		return "", noop, err
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		remove()
		return "", noop, err
	}
	if err := f.Close(); err != nil {
		remove()
		return "", noop, err
	}
	return f.Name(), remove, nil
}

// SnapshotDB creates an RDS snapshot (tofu apply) in the request's workspace.
func (p *Provider) SnapshotDB(ctx context.Context, req providers.SnapshotRequest) (*providers.SnapshotResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.dbSnapshotModuleDir(), p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeSnapshotVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-snap-*.tfplan")
	if err != nil {
		return nil, err
	}
	planPath := planFile.Name()
	_ = planFile.Close()
	defer os.Remove(planPath)

	if _, _, err := r.Plan(ctx, varsFile, planPath); err != nil {
		return nil, err
	}
	if _, err := r.Apply(ctx, planPath); err != nil {
		return nil, err
	}
	outs, err := r.Output(ctx)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			raw[k] = val
		}
	}
	return &providers.SnapshotResult{SnapshotID: dbOutString(outs, "snapshot_id"), RawOutputs: raw}, nil
}
