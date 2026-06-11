package main

// `opord ai ...` - CLI surface for AI governance, mirroring the API/web. AI
// access reuses the generic request/approval workflow (kind=ai_service), so the
// approve/reject subcommands call the same Service methods as `opord request`.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/config"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

func aiCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord ai <providers|provider|services|request|requests|approve|reject|instances|revoke|usage|audit>")
	}
	switch args[0] {
	case "providers":
		return aiProvidersList(cfg)
	case "provider":
		return aiProviderCmd(cfg, args[1:])
	case "services", "catalog":
		return aiServicesList(cfg)
	case "request":
		return aiRequestCreate(cfg, args[1:])
	case "requests":
		return aiRequestsList(cfg)
	case "approve":
		return aiDecide(cfg, args[1:], true)
	case "reject":
		return aiDecide(cfg, args[1:], false)
	case "instances":
		return aiInstancesList(cfg)
	case "revoke":
		return aiRevoke(cfg, args[1:])
	case "usage":
		return aiUsageList(cfg)
	case "audit":
		return aiAuditList(cfg)
	default:
		return fmt.Errorf("unknown ai subcommand %q (want providers|provider|services|request|requests|approve|reject|instances|revoke|usage|audit)", args[0])
	}
}

func aiProviderCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord ai provider <add|check|sync>")
	}
	switch args[0] {
	case "add":
		return aiProviderAdd(cfg, args[1:])
	case "check":
		return aiProviderCheck(cfg, args[1:])
	case "sync":
		return aiProviderSync(cfg, args[1:])
	default:
		return fmt.Errorf("unknown ai provider subcommand %q (want add|check|sync)", args[0])
	}
}

func aiProvidersList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAIProviders(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no AI providers registered")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tADDED")
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Type, p.Status, p.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func aiProviderAdd(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("ai provider add", flag.ContinueOnError)
	name := fs.String("name", "", "provider name (required)")
	ptype := fs.String("type", "", "provider type: mock_ai|openai|anthropic|gemini|github_copilot|cursor (required)")
	secretRef := fs.String("secret-ref", "", "OpenBao KV path holding the api key (e.g. opord/ai/openai-main)")
	scopes := fs.String("scopes", "", "comma-separated scopes (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *ptype == "" {
		return errors.New("--name and --type are required")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	var sc []string
	if strings.TrimSpace(*scopes) != "" {
		for _, s := range strings.Split(*scopes, ",") {
			if s = strings.TrimSpace(s); s != "" {
				sc = append(sc, s)
			}
		}
	}
	p, err := svc.CreateAIProvider(ctx, orchestrator.AIProviderInput{
		Name:      *name,
		Type:      *ptype,
		SecretRef: *secretRef,
		Scopes:    sc,
	})
	if err != nil {
		return err
	}
	fmt.Printf("✓ AI provider %q (%s) registered; catalog services synced\n", p.Name, p.Type)
	if *secretRef == "" {
		fmt.Println("  note: no --secret-ref set - `ai provider check` will report a missing key")
	}
	return nil
}

func aiProviderCheck(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord ai provider check <name>")
	}
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.CheckAIProvider(ctx, args[0]); err != nil {
		return fmt.Errorf("✗ %s: %w", args[0], err)
	}
	fmt.Printf("✓ %s: credentials OK\n", args[0])
	return nil
}

func aiProviderSync(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord ai provider sync <name>")
	}
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.SyncAIProviderServicesByName(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("✓ %s: catalog services synced\n", args[0])
	return nil
}

func aiServicesList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAIServices(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no AI services in the catalog (add a provider to sync its services)")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tNAME\tPROVIDER\tCATEGORY\tAPPROVAL\tDEFAULT")
	for _, s := range list {
		approval := "auto"
		if s.RequiresApproval {
			approval = "required"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%dd\n",
			s.Slug, s.Name, s.ProviderName, s.Category, approval, s.DefaultExpirationDays)
	}
	return w.Flush()
}

func aiRequestCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("ai request", flag.ContinueOnError)
	name := fs.String("name", "", "request name (required)")
	service := fs.String("service", "", "service slug, e.g. openai-api-access (required)")
	requester := fs.String("requester", "cli", "who is requesting")
	owner := fs.String("owner", "", "access owner (defaults to requester)")
	workspace := fs.String("workspace", "", "workspace/project label")
	justification := fs.String("justification", "", "why access is needed")
	expires := fs.String("expires", "", "expiry: RFC3339 or YYYY-MM-DD (default: service policy)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *service == "" {
		return errors.New("--name and --service are required")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	req, err := svc.CreateAIRequest(ctx, orchestrator.CreateAIRequestInput{
		Name:          *name,
		Requester:     *requester,
		ServiceSlug:   *service,
		Owner:         *owner,
		Workspace:     *workspace,
		Justification: *justification,
		ExpiresAt:     *expires,
	})
	if err != nil {
		return err
	}
	fmt.Printf("request %q submitted (status: %s)\n", req.Name, req.Status)
	fmt.Println("  approve with `opord ai approve " + req.Name + "`")
	return nil
}

func aiRequestsList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAIRequests(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no AI requests yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROVIDER\tREQUESTER\tSTATUS\tCREATED")
	for _, r := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.Name, r.Provider, r.Requester, r.Status, r.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

// aiDecide approves or rejects an AI request. AI requests reuse the generic
// request workflow, so this calls the same Approve/RejectRequest as `opord request`.
func aiDecide(cfg *config.Config, args []string, approve bool) error {
	verb := "approve"
	if !approve {
		verb = "reject"
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("usage: opord ai %s <name> [--by <user>] [--env <env>]", verb)
	}
	name := args[0]
	fs := flag.NewFlagSet("ai "+verb, flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	by := fs.String("by", "cli", "who is deciding")
	reason := fs.String("reason", "", "rejection reason")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if approve {
		if err := svc.ApproveRequest(ctx, name, *env, *by); err != nil {
			return err
		}
		fmt.Printf("✓ AI request %q approved - access granted\n", name)
		return nil
	}
	if err := svc.RejectRequest(ctx, name, *env, *by, *reason); err != nil {
		return err
	}
	fmt.Printf("✓ AI request %q rejected\n", name)
	return nil
}

func aiInstancesList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAIInstances(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no AI access instances")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSERVICE\tOWNER\tSTATUS\tACCESS_ID\tEXPIRES")
	for _, i := range list {
		exp := "-"
		if i.ExpiresAt.Valid {
			exp = i.ExpiresAt.Time.Format("2006-01-02")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			i.ID, i.ServiceSlug, i.Owner, i.Status, i.ProviderAccessID, exp)
	}
	return w.Flush()
}

func aiRevoke(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord ai revoke <instance-id> [--by <user>]")
	}
	id, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("invalid instance id %q: %w", args[0], err)
	}
	fs := flag.NewFlagSet("ai revoke", flag.ContinueOnError)
	by := fs.String("by", "cli", "who is revoking")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	inst, err := svc.RevokeAIInstance(ctx, id, *by)
	if err != nil {
		return err
	}
	fmt.Printf("✓ instance %s %s\n", inst.ID, inst.Status)
	return nil
}

func aiUsageList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAIUsageRecords(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no AI usage records")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tOWNER\tMETRIC\tQTY\tUNIT\tCOST_USD\tPERIOD")
	for _, u := range list {
		owner := "-"
		if u.Owner != nil && *u.Owner != "" {
			owner = *u.Owner
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%g\t%s\t%.2f\t%s..%s\n",
			u.ProviderName, owner, u.Metric, u.Quantity, u.Unit, u.CostUsd,
			u.PeriodStart.Format("2006-01-02"), u.PeriodEnd.Format("2006-01-02"))
	}
	return w.Flush()
}

func aiAuditList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAIAuditEvents(ctx, 100)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no AI audit events")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tACTOR\tSUBJECT\tACTION\tMESSAGE")
	for _, e := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.CreatedAt.Format(time.RFC3339), e.Actor, e.SubjectType, e.Action, e.Message)
	}
	return w.Flush()
}
