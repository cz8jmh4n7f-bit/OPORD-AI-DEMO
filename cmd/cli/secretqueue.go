package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/config"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"sigs.k8s.io/yaml"
)

// --- managed secrets (kind=secret resources; AWS Secrets Manager / Azure Key Vault) ---

type secretFile struct {
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Provider    string            `json:"provider"`
	Spec        models.SecretSpec `json:"spec"`
}

func loadSecretFile(path string) (*secretFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var sf secretFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &sf, nil
}

func secretCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord secret <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return secretCreate(cfg, args[1:])
	case "list":
		return secretList(cfg)
	case "status":
		return secretStatus(cfg, args[1:])
	case "destroy":
		return secretDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown secret subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func secretCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("secret create", flag.ContinueOnError)
	file := fs.String("f", "", "path to secret spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	sf, err := loadSecretFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateSecret(ctx, orchestrator.CreateSecretInput{
		Name:        sf.Name,
		Environment: sf.Environment,
		Provider:    sf.Provider,
		Spec:        sf.Spec,
		DryRun:      *dryRun,
	})
	if err != nil {
		return err
	}
	if res.DryRun {
		fmt.Println("✓ dry-run OK - spec valid, preflight passed, nothing changed")
		fmt.Printf("  %s\n", res.Summary)
		return nil
	}
	r := res.Resource
	fmt.Printf("secret %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord secret status " + r.Name + "`.")
	return nil
}

func secretList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListSecrets(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no secrets yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tSECRET\tSTATUS\tCREATED")
	for _, b := range list {
		prov := b.Provider
		if prov == "" {
			prov = "-"
		}
		secret := b.Spec.Name
		if secret == "" {
			secret = b.Resource.Name
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			b.Resource.Name, b.Resource.Environment, prov, secret, b.Resource.Status,
			b.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func secretStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord secret status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("secret status", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	b, err := svc.SecretStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := b.Resource
	prov := b.Provider
	if prov == "" {
		prov = "-"
	}
	secret := b.Spec.Name
	if secret == "" {
		secret = r.Name
	}
	fmt.Printf("Secret: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:      %s\n", r.Status)
	fmt.Printf("  Provider:    %s\n", prov)
	fmt.Printf("  Secret name: %s\n", secret)
	fmt.Printf("  Workspace:   %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:     %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:    %s\n", string(r.Observed))
	}
	return nil
}

func secretDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord secret destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("secret destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy secret %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Printf("destroying secret %q …\n", name)
	if err := svc.DestroySecret(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ secret %q destroyed\n", name)
	return nil
}

// --- message queues (kind=queue resources; AWS SQS / Azure Service Bus) ---

type queueFile struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.QueueSpec `json:"spec"`
}

func loadQueueFile(path string) (*queueFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var qf queueFile
	if err := yaml.Unmarshal(data, &qf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &qf, nil
}

func queueCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord queue <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return queueCreate(cfg, args[1:])
	case "list":
		return queueList(cfg)
	case "status":
		return queueStatus(cfg, args[1:])
	case "destroy":
		return queueDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown queue subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func queueCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("queue create", flag.ContinueOnError)
	file := fs.String("f", "", "path to queue spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	qf, err := loadQueueFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateQueue(ctx, orchestrator.CreateQueueInput{
		Name:        qf.Name,
		Environment: qf.Environment,
		Provider:    qf.Provider,
		Spec:        qf.Spec,
		DryRun:      *dryRun,
	})
	if err != nil {
		return err
	}
	if res.DryRun {
		fmt.Println("✓ dry-run OK - spec valid, preflight passed, nothing changed")
		fmt.Printf("  %s\n", res.Summary)
		return nil
	}
	r := res.Resource
	fmt.Printf("queue %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord queue status " + r.Name + "`.")
	return nil
}

func queueList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListQueues(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no queues yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tQUEUE\tFIFO\tSTATUS\tCREATED")
	for _, b := range list {
		prov := b.Provider
		if prov == "" {
			prov = "-"
		}
		queue := b.Spec.Name
		if queue == "" {
			queue = b.Resource.Name
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			b.Resource.Name, b.Resource.Environment, prov, queue, b.Spec.FIFO, b.Resource.Status,
			b.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func queueStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord queue status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("queue status", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	b, err := svc.QueueStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := b.Resource
	prov := b.Provider
	if prov == "" {
		prov = "-"
	}
	queue := b.Spec.Name
	if queue == "" {
		queue = r.Name
	}
	fmt.Printf("Queue: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:     %s\n", r.Status)
	fmt.Printf("  Provider:   %s\n", prov)
	fmt.Printf("  Queue name: %s\n", queue)
	fmt.Printf("  FIFO:       %t\n", b.Spec.FIFO)
	fmt.Printf("  DLQ:        %t\n", b.Spec.DLQEnabled)
	fmt.Printf("  Workspace:  %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:    %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:   %s\n", string(r.Observed))
	}
	return nil
}

func queueDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord queue destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("queue destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy queue %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
			fmt.Println("aborted")
			return nil
		}
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Printf("destroying queue %q …\n", name)
	if err := svc.DestroyQueue(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ queue %q destroyed\n", name)
	return nil
}
