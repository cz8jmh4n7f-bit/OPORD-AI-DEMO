package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// TableProvisioner: a Firestore database via modules/gcp-firestore. Firestore is
// the GCP managed NoSQL; it is schemaless, so the DynamoDB-shaped hash/range keys
// in TableSpec are ignored (the primitive provisions the database container).

var _ providers.TableProvisioner = (*Provider)(nil)

func (p *Provider) writeTableVars(req providers.TableRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildTableVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp table vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-firestore-*.tfvars.json")
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

func (p *Provider) PreflightTable(ctx context.Context, req providers.TableRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeTableVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.firestoreModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionTable(ctx context.Context, req providers.TableRequest) (*providers.TableResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.firestoreModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeTableVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-firestore-*.tfplan")
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
	return &providers.TableResult{
		ARN:        outString(outs, "arn"),
		Name:       outString(outs, "name"),
		RawOutputs: rawMap(outs),
	}, nil
}

func (p *Provider) DestroyTable(ctx context.Context, req providers.TableRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.firestoreModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeTableVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildTableVars maps a TableRequest onto the modules/gcp-firestore inputs.
func buildTableVars(req providers.TableRequest) map[string]any {
	name := req.Spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	name = safeName(name, 63)
	// Firestore database ids must be 4-63 chars.
	for len(name) < 4 {
		name += "db"
	}
	return map[string]any{
		"name":     name,
		"location": cfgStringDefault(req.Config, "firestore_location", cfgStringDefault(req.Config, "region", "eur3")),
	}
}
