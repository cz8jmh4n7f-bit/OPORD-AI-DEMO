package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/templates"
)

// CreateEnvironmentInput is the request to instantiate a composed environment,
// either from a built-in blueprint (Blueprint set) or from explicit Components
// (a custom blueprint, e.g. loaded from a file).
type CreateEnvironmentInput struct {
	Name         string
	Environment  string
	Provider     string
	Blueprint    string
	Template     string
	SSHPublicKey string
	// Components, when non-empty, are used directly (custom blueprint) and the
	// built-in catalog is not consulted.
	Components []models.Component
	DryRun     bool
}

// ComponentStatus is one child resource's live state within an environment.
type ComponentStatus struct {
	Name      string
	Kind      models.ComponentKind
	ChildName string
	Status    string
}

// EnvironmentSummary is an environment enriched with its components' live state
// and an aggregate status.
type EnvironmentSummary struct {
	Env        db.Environment
	Provider   string
	Components []ComponentStatus
	Aggregate  string
}

// CreateEnvironmentResult reports the outcome. For a dry run, Summaries holds the
// per-component preflight lines and nothing is persisted.
type CreateEnvironmentResult struct {
	DryRun    bool
	Summaries []string
	Env       *db.Environment
}

// childName is the deterministic name of a component's child resource.
func childName(envName, component string) string {
	return fmt.Sprintf("%s-%s", envName, component)
}

// validateComponents checks a (possibly user-supplied) component list: unique
// non-empty names and a spec matching each component's kind.
func validateComponents(components []models.Component) error {
	if len(components) == 0 {
		return fmt.Errorf("no components")
	}
	seen := make(map[string]bool, len(components))
	for i, c := range components {
		if c.Name == "" {
			return fmt.Errorf("component %d: name is required", i)
		}
		if seen[c.Name] {
			return fmt.Errorf("duplicate component name %q", c.Name)
		}
		seen[c.Name] = true
		switch c.Kind {
		case models.ComponentCluster:
			if c.Cluster == nil {
				return fmt.Errorf("component %q: kind %q requires a cluster spec", c.Name, c.Kind)
			}
		case models.ComponentVM:
			if c.VM == nil {
				return fmt.Errorf("component %q: kind %q requires a vm spec", c.Name, c.Kind)
			}
		case models.ComponentDatabase:
			if c.Database == nil {
				return fmt.Errorf("component %q: kind %q requires a database spec", c.Name, c.Kind)
			}
		case models.ComponentStack:
			if c.Stack == nil {
				return fmt.Errorf("component %q: kind %q requires a stack spec", c.Name, c.Kind)
			}
		default:
			return fmt.Errorf("component %q: unknown kind %q (want one of %s, %s, %s, %s)", c.Name, c.Kind, models.ComponentCluster, models.ComponentVM, models.ComponentDatabase, models.ComponentStack)
		}
	}
	return nil
}

// CreateEnvironment expands a blueprint into component specs and either dry-runs
// each component offline (persisting nothing) or persists the environment and
// creates each child resource (which then provisions in the background).
func (s *Service) CreateEnvironment(ctx context.Context, in CreateEnvironmentInput) (*CreateEnvironmentResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("environment name and provider are required")
	}
	if in.Blueprint == "" && len(in.Components) == 0 {
		return nil, fmt.Errorf("a blueprint or explicit components are required")
	}
	env := in.Environment
	if env == "" {
		env = "dev"
	}

	// Explicit components (custom blueprint) win; otherwise expand the catalog.
	components := in.Components
	if len(components) == 0 {
		expanded, err := templates.Expand(in.Blueprint, templates.ExpandParams{
			Template:     in.Template,
			SSHPublicKey: in.SSHPublicKey,
		})
		if err != nil {
			return nil, err
		}
		components = expanded
	}
	if in.Blueprint == "" {
		in.Blueprint = "custom"
	}
	if err := validateComponents(components); err != nil {
		return nil, err
	}

	if in.DryRun {
		summaries := make([]string, 0, len(components))
		for _, c := range components {
			line, err := s.dryRunComponent(ctx, in, env, c)
			if err != nil {
				return nil, fmt.Errorf("component %q: %w", c.Name, err)
			}
			summaries = append(summaries, line)
		}
		s.log.Info("environment preflight ok", "name", in.Name, "blueprint", in.Blueprint, "components", len(components))
		return &CreateEnvironmentResult{DryRun: true, Summaries: summaries}, nil
	}

	specJSON, err := json.Marshal(models.EnvironmentSpec{
		Blueprint:  in.Blueprint,
		Provider:   in.Provider,
		Components: components,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling environment spec: %w", err)
	}
	e, err := s.q.CreateEnvironment(ctx, db.CreateEnvironmentParams{
		Name:        in.Name,
		Environment: env,
		Blueprint:   in.Blueprint,
		Spec:        specJSON,
		TenantID:    tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}

	// Compute topological waves now so cycle / unknown-ref errors fail fast,
	// before any resource is created. The provisioning itself runs in the
	// background, wave-by-wave, with placeholder substitution from the previous
	// waves' outputs (ADR-0008).
	waves, err := buildComponentWaves(components)
	if err != nil {
		_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "failed"})
		return nil, fmt.Errorf("environment topology: %w", err)
	}
	_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "provisioning"})
	s.log.Info("environment created", "name", e.Name, "blueprint", in.Blueprint, "components", len(components), "waves", len(waves))
	s.emit("environment", "created", e.Name, env, in.Provider, fmt.Sprintf("blueprint %q, %d component(s), %d wave(s)", in.Blueprint, len(components), len(waves)))
	go s.provisionEnvironmentWaves(in, env, e, components, waves)
	return &CreateEnvironmentResult{Env: &e}, nil
}

// buildComponentWaves computes the wave order from a flat list of components,
// reading placeholder references out of each component's serialized spec.
func buildComponentWaves(components []models.Component) ([][]string, error) {
	names := make([]string, 0, len(components))
	refs := map[string][]string{}
	for _, c := range components {
		names = append(names, c.Name)
		raw, err := json.Marshal(c)
		if err != nil {
			return nil, fmt.Errorf("marshal component %q: %w", c.Name, err)
		}
		rs, err := componentRefs(raw)
		if err != nil {
			return nil, fmt.Errorf("scan component %q for references: %w", c.Name, err)
		}
		refs[c.Name] = rs
	}
	return componentWaves(refs, names)
}

// provisionEnvironmentWaves drives the wave-based provisioning in a goroutine:
// for each wave, substitute placeholders -> create -> wait for ready -> read
// outputs -> next wave. A failure in any wave marks the environment failed.
func (s *Service) provisionEnvironmentWaves(in CreateEnvironmentInput, env string, e db.Environment, components []models.Component, waves [][]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	byName := make(map[string]models.Component, len(components))
	for _, c := range components {
		byName[c.Name] = c
	}
	outputs := map[string]map[string]any{}

	for waveIdx, wave := range waves {
		s.log.Info("environment wave start", "env", e.Name, "wave", waveIdx, "components", wave)

		for _, name := range wave {
			c := byName[name]
			resolved, err := s.resolveComponentRefs(c, outputs)
			if err != nil {
				s.log.Error("environment substitution failed", "comp", name, "err", err)
				_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "failed"})
				return
			}
			if err := s.createComponent(ctx, in, env, resolved); err != nil {
				s.log.Error("environment component create failed", "comp", name, "err", err)
				_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "failed"})
				return
			}
		}

		for _, name := range wave {
			c := byName[name]
			cn := childName(e.Name, name)
			if err := s.waitComponentReady(ctx, c.Kind, cn, env); err != nil {
				s.log.Error("environment component not ready", "comp", name, "err", err)
				_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "failed"})
				return
			}
			outputs[name] = s.readComponentOutputs(ctx, c.Kind, cn, env)
		}
	}
	_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "ready"})
	s.log.Info("environment provisioning complete", "name", e.Name, "blueprint", in.Blueprint)
}

// resolveComponentRefs substitutes ${other.outputs.field} placeholders in a
// component's spec from the running outputs map. No-op when the component
// contains no placeholders.
func (s *Service) resolveComponentRefs(c models.Component, outputs map[string]map[string]any) (models.Component, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	sub, err := substituteRefs(raw, outputs)
	if err != nil {
		return c, err
	}
	if string(sub) == string(raw) {
		return c, nil
	}
	var out models.Component
	if err := json.Unmarshal(sub, &out); err != nil {
		return c, fmt.Errorf("unmarshal resolved component %q: %w", c.Name, err)
	}
	return out, nil
}

// waitComponentReady polls a component's status until it reaches "ready" or
// "failed", or the context deadline fires. 10s polling interval.
func (s *Service) waitComponentReady(ctx context.Context, kind models.ComponentKind, name, env string) error {
	for {
		var status string
		switch kind {
		case models.ComponentCluster:
			if cl, err := s.q.GetClusterByName(ctx, db.GetClusterByNameParams{Name: name, Environment: env}); err == nil {
				status = cl.Status
			}
		case models.ComponentVM, models.ComponentDatabase, models.ComponentStack:
			if r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env}); err == nil {
				status = r.Status
			}
		}
		switch status {
		case "ready":
			return nil
		case "failed":
			return fmt.Errorf("component %q (%s) failed", name, kind)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// readComponentOutputs returns a component's tofu outputs once it is ready,
// normalised for placeholder access. Stack outputs are hoisted from the nested
// "raw_outputs" key so users write ${comp.outputs.bucket_arn} directly.
func (s *Service) readComponentOutputs(ctx context.Context, kind models.ComponentKind, name, env string) map[string]any {
	var observed []byte
	switch kind {
	case models.ComponentVM, models.ComponentDatabase, models.ComponentStack:
		if r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env}); err == nil {
			observed = r.Observed
		}
	}
	out := map[string]any{}
	if len(observed) == 0 {
		return out
	}
	_ = json.Unmarshal(observed, &out)
	if kind == models.ComponentStack {
		if outs, ok := out["outputs"].(map[string]any); ok {
			return outs
		}
	}
	return out
}

func (s *Service) dryRunComponent(ctx context.Context, in CreateEnvironmentInput, env string, c models.Component) (string, error) {
	cn := childName(in.Name, c.Name)
	switch c.Kind {
	case models.ComponentCluster:
		res, err := s.CreateCluster(ctx, CreateClusterInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.Cluster, DryRun: true})
		if err != nil {
			return "", err
		}
		summary := "cluster valid"
		if res.Preflight != nil {
			summary = res.Preflight.Summary
		}
		return fmt.Sprintf("%s (cluster): %s", c.Name, summary), nil
	case models.ComponentVM:
		res, err := s.CreateVM(ctx, CreateVMInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.VM, DryRun: true})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s (vm): %s", c.Name, res.Summary), nil
	case models.ComponentDatabase:
		res, err := s.CreateDatabase(ctx, CreateDatabaseInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.Database, DryRun: true})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s (database): %s", c.Name, res.Summary), nil
	case models.ComponentStack:
		res, err := s.CreateStack(ctx, CreateStackInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.Stack, DryRun: true})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s (stack): %s", c.Name, res.Summary), nil
	default:
		return "", fmt.Errorf("unknown component kind %q", c.Kind)
	}
}

func (s *Service) createComponent(ctx context.Context, in CreateEnvironmentInput, env string, c models.Component) error {
	cn := childName(in.Name, c.Name)
	switch c.Kind {
	case models.ComponentCluster:
		_, err := s.CreateCluster(ctx, CreateClusterInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.Cluster})
		return err
	case models.ComponentVM:
		_, err := s.CreateVM(ctx, CreateVMInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.VM})
		return err
	case models.ComponentDatabase:
		_, err := s.CreateDatabase(ctx, CreateDatabaseInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.Database})
		return err
	case models.ComponentStack:
		_, err := s.CreateStack(ctx, CreateStackInput{Name: cn, Environment: env, Provider: in.Provider, Spec: *c.Stack})
		return err
	default:
		return fmt.Errorf("unknown component kind %q", c.Kind)
	}
}

// componentStatuses looks up each component's child resource and reports its
// status (with "missing" when the child can't be found).
func (s *Service) componentStatuses(ctx context.Context, e db.Environment) []ComponentStatus {
	var spec models.EnvironmentSpec
	_ = json.Unmarshal(e.Spec, &spec)
	out := make([]ComponentStatus, 0, len(spec.Components))
	for _, c := range spec.Components {
		cn := childName(e.Name, c.Name)
		cs := ComponentStatus{Name: c.Name, Kind: c.Kind, ChildName: cn, Status: "missing"}
		switch c.Kind {
		case models.ComponentCluster:
			if cl, err := s.q.GetClusterByName(ctx, db.GetClusterByNameParams{Name: cn, Environment: e.Environment}); err == nil {
				cs.Status = cl.Status
			}
		case models.ComponentVM, models.ComponentDatabase, models.ComponentStack:
			if r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: cn, Environment: e.Environment}); err == nil {
				cs.Status = r.Status
			}
		}
		out = append(out, cs)
	}
	return out
}

// aggregateStatus rolls up component statuses into one environment status.
// envStatus is the env's currently-stored status: during a destroy lifecycle a
// "missing" component (lookup returned no row) is treated as already destroyed,
// since the component either never came up or was already torn down.
func aggregateStatus(comps []ComponentStatus, envStatus string) string {
	if len(comps) == 0 {
		return "pending"
	}
	inDestroy := envStatus == "destroying" || envStatus == "destroyed"

	statuses := make([]string, len(comps))
	for i, c := range comps {
		s := c.Status
		if s == "missing" && inDestroy {
			s = "destroyed"
		}
		statuses[i] = s
	}
	has := func(want string) bool {
		for _, s := range statuses {
			if s == want {
				return true
			}
		}
		return false
	}
	all := func(want string) bool {
		for _, s := range statuses {
			if s != want {
				return false
			}
		}
		return true
	}
	switch {
	case has("failed"):
		return "failed"
	case has("destroying"):
		return "destroying"
	case all("destroyed"):
		return "destroyed"
	case has("pending") || has("provisioning") || has("bootstrapping") || has("missing"):
		return "provisioning"
	case all("ready"):
		return "ready"
	default:
		return "degraded"
	}
}

// ListEnvironments returns all environments with component statuses + aggregate.
func (s *Service) ListEnvironments(ctx context.Context) ([]EnvironmentSummary, error) {
	envs, err := s.q.ListEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}
	tid, scoped := scopeTenant(ctx)
	out := make([]EnvironmentSummary, 0, len(envs))
	for _, e := range envs {
		if scoped && !tenantVisible(e.TenantID, tid) {
			continue
		}
		comps := s.componentStatuses(ctx, e)
		agg := aggregateStatus(comps, e.Status)
		s.refreshEnvStatus(ctx, e, agg)
		out = append(out, EnvironmentSummary{Env: e, Provider: providerOf(e), Components: comps, Aggregate: agg})
	}
	return out, nil
}

// EnvironmentStatus returns one environment with live component state.
func (s *Service) EnvironmentStatus(ctx context.Context, name, env string) (*EnvironmentSummary, error) {
	if env == "" {
		env = "dev"
	}
	e, err := s.q.GetEnvironmentByName(ctx, db.GetEnvironmentByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("environment %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(e.TenantID, tid) {
		return nil, fmt.Errorf("environment %q (env %q) not found", name, env)
	}
	comps := s.componentStatuses(ctx, e)
	agg := aggregateStatus(comps, e.Status)
	s.refreshEnvStatus(ctx, e, agg)
	return &EnvironmentSummary{Env: e, Provider: providerOf(e), Components: comps, Aggregate: agg}, nil
}

// refreshEnvStatus persists the computed aggregate when it differs from stored.
// Terminal / destroy-lifecycle statuses are authoritative: never auto-overwrite
// them with a computed value (a stray list query mid-destroy must not clobber
// what the DestroyEnvironment goroutine just wrote).
func (s *Service) refreshEnvStatus(ctx context.Context, e db.Environment, agg string) {
	switch e.Status {
	case "destroying", "destroyed", "failed":
		return
	}
	if agg != "" && agg != e.Status {
		_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: agg})
	}
}

func providerOf(e db.Environment) string {
	var spec models.EnvironmentSpec
	_ = json.Unmarshal(e.Spec, &spec)
	return spec.Provider
}

// DestroyEnvironment tears down every component (tofu destroy) and marks the
// environment destroyed. Synchronous: components are destroyed in order.
func (s *Service) DestroyEnvironment(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	e, err := s.q.GetEnvironmentByName(ctx, db.GetEnvironmentByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("environment %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(e.TenantID, tid) {
		return fmt.Errorf("environment %q (env %q) not found", name, env)
	}
	var spec models.EnvironmentSpec
	_ = json.Unmarshal(e.Spec, &spec)

	_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "destroying"})
	s.log.Info("environment destroy started", "name", e.Name, "components", len(spec.Components))

	var firstErr error
	for _, c := range spec.Components {
		cn := childName(e.Name, c.Name)
		var derr error
		switch c.Kind {
		case models.ComponentCluster:
			derr = s.DestroyCluster(ctx, cn, env)
		case models.ComponentVM:
			derr = s.DestroyVM(ctx, cn, env)
		case models.ComponentDatabase:
			derr = s.DestroyDatabase(ctx, cn, env)
		case models.ComponentStack:
			derr = s.DestroyStack(ctx, cn, env)
		}
		if derr != nil {
			s.log.Error("component destroy failed", "component", c.Name, "err", derr)
			if firstErr == nil {
				firstErr = derr
			}
		}
	}
	if firstErr != nil {
		_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "failed"})
		return fmt.Errorf("environment destroy had failures: %w", firstErr)
	}
	_, _ = s.q.UpdateEnvironmentStatus(ctx, db.UpdateEnvironmentStatusParams{ID: e.ID, Status: "destroyed"})
	s.log.Info("environment destroy complete", "name", e.Name)
	s.emit("environment", "destroyed", e.Name, env, providerOf(e), "")
	return nil
}

// DestroyEnvironmentAsync runs DestroyEnvironment on a background context. Used
// by the HTTP API so the request returns immediately while components tear down.
func (s *Service) DestroyEnvironmentAsync(name, env string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		if err := s.DestroyEnvironment(ctx, name, env); err != nil {
			s.log.Error("async environment destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}
