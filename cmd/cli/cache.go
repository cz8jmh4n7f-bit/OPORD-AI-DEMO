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

// --- managed caches (kind=cache resources; AWS ElastiCache / Azure Redis) ---

type cacheFile struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.CacheSpec `json:"spec"`
}

func loadCacheFile(path string) (*cacheFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var cf cacheFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &cf, nil
}

func cacheCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord cache <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return cacheCreate(cfg, args[1:])
	case "list":
		return cacheList(cfg)
	case "status":
		return cacheStatus(cfg, args[1:])
	case "destroy":
		return cacheDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown cache subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func cacheCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("cache create", flag.ContinueOnError)
	file := fs.String("f", "", "path to cache spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	cf, err := loadCacheFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateCache(ctx, orchestrator.CreateCacheInput{
		Name:        cf.Name,
		Environment: cf.Environment,
		Provider:    cf.Provider,
		Spec:        cf.Spec,
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
	fmt.Printf("cache %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord cache status " + r.Name + "`.")
	return nil
}

func cacheList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListCaches(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no caches yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tCACHE\tSTATUS\tCREATED")
	for _, b := range list {
		prov := b.Provider
		if prov == "" {
			prov = "-"
		}
		cache := b.Spec.Name
		if cache == "" {
			cache = b.Resource.Name
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			b.Resource.Name, b.Resource.Environment, prov, cache, b.Resource.Status,
			b.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func cacheStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord cache status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("cache status", flag.ContinueOnError)
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

	b, err := svc.CacheStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := b.Resource
	prov := b.Provider
	if prov == "" {
		prov = "-"
	}
	cache := b.Spec.Name
	if cache == "" {
		cache = r.Name
	}
	fmt.Printf("Cache: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:     %s\n", r.Status)
	fmt.Printf("  Provider:   %s\n", prov)
	fmt.Printf("  Cache name: %s\n", cache)
	fmt.Printf("  Workspace:  %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:    %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:   %s\n", string(r.Observed))
	}
	return nil
}

func cacheDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord cache destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("cache destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy cache %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying cache %q …\n", name)
	if err := svc.DestroyCache(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ cache %q destroyed\n", name)
	return nil
}
