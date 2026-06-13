// Command opord is the OPORD CLI (Milestone 6). Business logic lives in
// internal/orchestrator so the future HTTP API can reuse it; this package is a
// thin layer that parses flags, calls the Service, and renders output.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/anthropic"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/litellm"
	aimock "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/mock"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/openai"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/azure"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/config"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/glpi"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/ipam"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/aws"
	azureprov "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/azure"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/gcp"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/proxmox"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/vsphere"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/templates"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/vcenter"
	"sigs.k8s.io/yaml"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return printHelp()
	}

	switch args[0] {
	case "version":
		fmt.Println("opord dev (Milestone 6: CLI)")
		return nil
	case "providers":
		return listProviderTypes(cfg)
	case "provider":
		return providerCmd(cfg, args[1:])
	case "cluster":
		return clusterCmd(cfg, args[1:])
	case "vm":
		return vmCmd(cfg, args[1:])
	case "env", "environment":
		return envCmd(cfg, args[1:])
	case "db", "database":
		return dbCmd(cfg, args[1:])
	case "stack":
		return stackCmd(cfg, args[1:])
	case "table":
		return tableCmd(cfg, args[1:])
	case "function", "fn":
		return functionCmd(cfg, args[1:])
	case "s3", "bucket":
		return s3Cmd(cfg, args[1:])
	case "secret":
		return secretCmd(cfg, args[1:])
	case "queue":
		return queueCmd(cfg, args[1:])
	case "cache":
		return cacheCmd(cfg, args[1:])
	case "project":
		return projectCmd(cfg, args[1:])
	case "account":
		return accountCmd(cfg, args[1:])
	case "entra":
		return entraCmd(cfg, args[1:])
	case "ai":
		return aiCmd(cfg, args[1:])
	case "request", "req":
		return requestCmd(cfg, args[1:])
	case "cost":
		return costReport(cfg)
	case "backups":
		return backupsList(cfg)
	case "tenant":
		return tenantCmd(cfg, args[1:])
	case "user":
		return userCmd(cfg, args[1:])
	case "blueprint", "blueprints":
		return blueprintList()
	case "vcenter":
		return vcenterCmd(cfg, args[1:])
	case "help", "":
		return printHelp()
	default:
		return fmt.Errorf("unknown command %q (try: opord help)", args[0])
	}
}

func printHelp() error {
	fmt.Println(`usage: opord <command>

commands:
  version              print version
  providers            list available provider types (plugins)
  provider add ...     register a provider instance (needs DATABASE_URL)
  provider list        list registered provider instances
  cluster create ...   create a cluster from a spec (-f spec.yaml [--dry-run])
  cluster list         list clusters
  cluster status NAME  show cluster detail, nodes, and jobs ([--env <env>])
  cluster scale NAME   change worker count and re-provision (--workers N [--env <env>])
  cluster destroy NAME tofu destroy a cluster ([--env <env>] [--yes])
  vm create ...        provision standalone VM(s) from a spec (-f spec.yaml [--dry-run])
  vm list              list VM resources
  vm status NAME       show one VM's detail ([--env <env>])
  vm scale NAME ...    change VM count and re-provision (--count N [--env <env>])
  vm destroy NAME      tofu destroy a VM ([--env <env>] [--yes])
  db create ...        provision a managed database from a spec (-f spec.yaml [--dry-run])
  db list              list managed databases
  db status NAME       show one database's detail ([--env <env>])
  db scale NAME ...    change RDS class/storage and re-provision (--instance-class X --storage N)
  db backup NAME       take an RDS snapshot ([--env <env>])
  db destroy NAME      tofu destroy a database ([--env <env>] [--yes])
  backups              list database backups/snapshots
  stack create ...     provision an arbitrary OpenTofu module (-f spec.yaml [--dry-run])
  stack list           list generic stacks
  stack status NAME    show one stack's detail + outputs ([--env <env>])
  stack destroy NAME   tofu destroy a stack ([--env <env>] [--yes])
  s3 create ...        provision an S3 bucket from a spec (-f spec.yaml [--dry-run])
  s3 list              list S3 buckets
  s3 status NAME       show one bucket's detail ([--env <env>])
  s3 destroy NAME      tofu destroy a bucket ([--env <env>] [--yes])
  project create ...   provision an access-vending project (-f spec.yaml [--dry-run])
  project list         list access-vending projects
  project status NAME  show one project's detail + members ([--env <env>])
  project members NAME manage members (--add a,b | --remove c | --set a,b,c)
  project destroy NAME tofu destroy a project ([--env <env>] [--yes])
  account create ...   provision a member AWS account + baseline (-f spec.yaml [--dry-run])
  account list         list provisioned accounts
  account status NAME  show one account's detail + per-layer status ([--env <env>])
  account destroy NAME tear down account layers ([--env <env>] [--yes]; account closure is separate)
  entra grant ...      assign Entra users to an AWS SAML enterprise app (--app-id --role-arn --provider-arn --user a@x,b@y [--invite])
  entra grant-group .. assign an Entra group to an app for GCP WIF / SSO (--app-id --group-id)
  ai providers         list AI governance providers (ai services = catalog)
  ai provider add ...  register an AI provider (--name --type --secret-ref [--scopes])
  ai provider check N  validate an AI provider's creds (ai provider sync N imports catalog)
  ai request ...       request AI access (--name --service --owner [--workspace] [--expires])
  ai approve NAME      approve+grant (ai reject NAME); ai instances; ai revoke ID
  ai usage             AI usage records (ai audit = governance trail)
  request create ...   submit a request for approval (-f req.yaml)
  request list         list requests
  request status NAME  show one request ([--env <env>])
  request approve NAME approve + provision a request ([--by <user>] [--env <env>])
  request reject NAME  reject a request ([--by <user>] [--reason <text>] [--env <env>])
  cost                 estimated monthly cost of all active resources
  tenant add NAME      create a tenant; tenant list
  user add ...         create a user + API key (--email --tenant --role); user list
  blueprints           list built-in environment blueprints (golden paths)
  env create ...       create an environment from a blueprint (--name --provider --blueprint [--dry-run])
  env list             list composed environments
  env status NAME      show an environment's components + status ([--env <env>])
  env destroy NAME     tofu destroy an environment's components ([--env <env>] [--yes])
  vcenter check ...    live vCenter check via API (--provider NAME | --url URL)
  help                 show this help`)
	return nil
}

// --- provider type plugins (in-code registry) ---

func buildRegistry(cfg *config.Config) *providers.Registry {
	reg := providers.NewRegistry()
	vsphere.Register(reg, vsphere.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	proxmox.Register(reg, proxmox.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	aws.Register(reg, aws.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	azureprov.Register(reg, azureprov.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	gcp.Register(reg, gcp.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	return reg
}

func listProviderTypes(cfg *config.Config) error {
	reg := buildRegistry(cfg)
	fmt.Println("available provider types (plugins):")
	for _, t := range reg.Types() {
		fmt.Printf("  - %s\n", t)
	}
	return nil
}

// --- provider instances (DB-backed) ---

func providerCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord provider <add|list|update|delete|check>")
	}
	switch args[0] {
	case "add":
		return providerAdd(cfg, args[1:])
	case "list":
		return providerList(cfg)
	case "update":
		return providerUpdate(cfg, args[1:])
	case "delete", "rm":
		return providerDelete(cfg, args[1:])
	case "check":
		return providerCheck(cfg, args[1:])
	default:
		return fmt.Errorf("unknown provider subcommand %q (want add|list|update|delete|check)", args[0])
	}
}

// providerCheck runs a live connectivity + credential probe against a provider
// (handy for cron-based monitoring). Exits non-zero when the backend is
// unreachable so it composes with shell `&&` / alerting.
func providerCheck(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("provider check", flag.ContinueOnError)
	name := fs.String("name", "", "provider name (required)")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*name, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("provider name is required (positional or --name)")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CheckProviderConnection(ctx, *name)
	if err != nil {
		return err
	}
	switch res.Status {
	case "ok":
		fmt.Printf("%s (%s): OK - reachable, credentials valid (%dms)\n", res.Provider, res.Type, res.LatencyMs)
		return nil
	case "unsupported":
		fmt.Printf("%s (%s): connection check not supported\n", res.Provider, res.Type)
		return nil
	default:
		return fmt.Errorf("%s (%s): FAILED - %s", res.Provider, res.Type, res.Message)
	}
}

// providerUpdate merges only the flags you pass into the provider's config /
// secret-ref (unset flags are left untouched). Name may be positional.
func providerUpdate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("provider update", flag.ContinueOnError)
	name := fs.String("name", "", "provider name (required)")
	secretRef := fs.String("secret-ref", "", "OpenBao/Vault KV path holding credentials")
	region := fs.String("region", "", "cloud region (e.g. eu-central-1)")
	server := fs.String("server", "", "server FQDN/IP")
	datacenter := fs.String("datacenter", "", "datacenter")
	computeCluster := fs.String("cluster", "", "compute cluster")
	datastore := fs.String("datastore", "", "datastore")
	network := fs.String("network", "", "network / port group")
	folder := fs.String("folder", "", "VM folder path")
	configJSON := fs.String("config", "", `extra provider config as JSON, merged (e.g. '{"subnet_ids":["subnet-a"]}')`)
	// Allow a leading positional name (`provider update NAME --flags…`); Go's flag
	// package would otherwise stop parsing at the first non-flag argument.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*name, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("provider name is required (positional or --name)")
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	vals := map[string]string{
		"server": *server, "datacenter": *datacenter, "cluster": *computeCluster,
		"datastore": *datastore, "network": *network, "folder": *folder, "region": *region,
	}
	cfgMap := map[string]any{}
	for flagName, v := range vals {
		if set[flagName] {
			cfgMap[flagName] = v
		}
	}
	if set["config"] {
		var extra map[string]any
		if err := json.Unmarshal([]byte(*configJSON), &extra); err != nil {
			return fmt.Errorf("--config must be valid JSON: %w", err)
		}
		for k, v := range extra {
			cfgMap[k] = v
		}
	}
	var secretRefPtr *string
	if set["secret-ref"] {
		secretRefPtr = secretRef
	}
	if len(cfgMap) == 0 && secretRefPtr == nil {
		return errors.New("nothing to update (pass --secret-ref / --region / --config / --server / ...)")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	p, err := svc.UpdateProvider(ctx, *name, orchestrator.ProviderUpdate{Config: cfgMap, SecretRef: secretRefPtr})
	if err != nil {
		return err
	}
	fmt.Printf("provider %q updated\n", p.Name)
	return nil
}

// providerDelete removes a provider (refused if clusters/resources depend on it).
func providerDelete(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("provider delete", flag.ContinueOnError)
	name := fs.String("name", "", "provider name (required)")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*name, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("provider name is required (positional or --name)")
	}
	if !*yes {
		fmt.Printf("Delete provider %q? This cannot be undone. Re-run with --yes to confirm.\n", *name)
		return nil
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := svc.DeleteProvider(ctx, *name); err != nil {
		return err
	}
	fmt.Printf("provider %q deleted\n", *name)
	return nil
}

func providerAdd(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("provider add", flag.ContinueOnError)
	name := fs.String("name", "", "unique provider name (required)")
	ptype := fs.String("type", "vsphere", "provider type: vsphere|proxmox|aws")
	server := fs.String("server", "", "vCenter server FQDN/IP")
	datacenter := fs.String("datacenter", "", "vSphere datacenter")
	computeCluster := fs.String("cluster", "", "vSphere compute cluster")
	datastore := fs.String("datastore", "", "vSphere datastore")
	network := fs.String("network", "", "vSphere network / port group")
	folder := fs.String("folder", "", "VM folder path")
	region := fs.String("region", "", "cloud region (AWS, e.g. eu-central-1)")
	configJSON := fs.String("config", "", `extra provider config as JSON, merged (e.g. '{"subnet_ids":["subnet-a"],"ou_id":"ou-x"}')`)
	insecure := fs.Bool("allow-unverified-ssl", true, "allow unverified vCenter TLS")
	secretRef := fs.String("secret-ref", "", "OpenBao/Vault KV path holding credentials (recorded for later)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("--name is required")
	}

	providerConfig := map[string]any{}
	if *ptype == "vsphere" || *ptype == "proxmox" {
		providerConfig = map[string]any{
			"server":               *server,
			"datacenter":           *datacenter,
			"cluster":              *computeCluster,
			"datastore":            *datastore,
			"network":              *network,
			"folder":               *folder,
			"allow_unverified_ssl": *insecure,
		}
	}
	if *region != "" {
		providerConfig["region"] = *region
	}
	// --config merges arbitrary keys (AWS subnet_ids / ou_id / node_instance_type, …).
	if *configJSON != "" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(*configJSON), &extra); err != nil {
			return fmt.Errorf("--config must be valid JSON: %w", err)
		}
		for k, v := range extra {
			providerConfig[k] = v
		}
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	p, err := svc.AddProvider(ctx, orchestrator.ProviderInput{
		Name:      *name,
		Type:      *ptype,
		Config:    providerConfig,
		SecretRef: *secretRef,
	})
	if err != nil {
		return err
	}
	fmt.Printf("provider %q (%s) registered: %s\n", p.Name, p.Type, p.ID)
	return nil
}

func providerList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListProviders(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no providers registered")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tSECRET_REF\tCREATED")
	for _, p := range list {
		secret := p.SecretRef
		if secret == "" {
			secret = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Type, secret, p.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

// --- clusters ---

type clusterFile struct {
	Name        string             `json:"name"`
	Environment string             `json:"environment"`
	Provider    string             `json:"provider"`
	Spec        models.ClusterSpec `json:"spec"`
}

func loadClusterFile(path string) (*clusterFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var cf clusterFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &cf, nil
}

func clusterCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord cluster <create|list|status>")
	}
	switch args[0] {
	case "create":
		return clusterCreate(cfg, args[1:])
	case "list":
		return clusterList(cfg)
	case "status":
		return clusterStatus(cfg, args[1:])
	case "scale":
		return clusterScale(cfg, args[1:])
	case "destroy":
		return clusterDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown cluster subcommand %q (want create|list|status|scale|destroy)", args[0])
	}
}

func clusterScale(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord cluster scale <name> --workers N [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("cluster scale", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	workers := fs.Int("workers", 0, "new worker count (required, >= 1)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *workers < 1 {
		return errors.New("--workers >= 1 is required")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.ScaleCluster(ctx, name, *env, *workers); err != nil {
		return err
	}
	fmt.Printf("✓ cluster %q scaling to %d workers - re-provisioning in the background\n", name, *workers)
	return nil
}

func clusterCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("cluster create", flag.ContinueOnError)
	file := fs.String("f", "", "path to cluster spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	workers := fs.Int("workers", 0, "override the worker count from the spec")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}

	cf, err := loadClusterFile(*file)
	if err != nil {
		return err
	}
	if *workers > 0 {
		cf.Spec.Workers.Count = *workers
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateCluster(ctx, orchestrator.CreateClusterInput{
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
		fmt.Println("✓ dry-run OK - spec valid, module validated, nothing changed")
		if res.Preflight != nil {
			fmt.Printf("  %s\n", res.Preflight.Summary)
		}
		fmt.Println("  (no provider API contacted; run without --dry-run once the provider API is reachable)")
		return nil
	}

	fmt.Printf("cluster %q registered (status: %s)\n", res.Cluster.Name, res.Cluster.Status)
	fmt.Printf("  id=%s  workspace=%s  job=%s\n", res.Cluster.ID, res.Cluster.TofuWorkspace, res.JobID)
	fmt.Printf("  provisioning runs in the background (Tofu apply + Ansible); poll `opord cluster status %s`.\n", res.Cluster.Name)
	return nil
}

func clusterDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord cluster destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("cluster destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy cluster %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying cluster %q … (tofu destroy can take a few minutes)\n", name)
	if err := svc.DestroyCluster(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ cluster %q destroyed\n", name)
	return nil
}

func clusterList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListClusters(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no clusters yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tSTATUS\tNODES\tCREATED")
	for _, c := range list {
		prov := c.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%dcp/%dw\t%s\n",
			c.Cluster.Name, c.Cluster.Environment, prov, c.Cluster.Status,
			c.ControlPlanes, c.Workers, c.Cluster.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func clusterStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord cluster status <name> [--env <env>] [--live]")
	}
	name := args[0]
	fs := flag.NewFlagSet("cluster status", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	live := fs.Bool("live", false, "query the provider's vCenter API for live VM state")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	d, err := svc.ClusterStatus(ctx, name, *env, *live)
	if err != nil {
		return err
	}
	c := d.Cluster

	provLine := d.Provider
	if provLine == "" {
		provLine = "-"
	}
	if d.ProviderType != "" {
		provLine = fmt.Sprintf("%s (%s)", provLine, d.ProviderType)
	}
	endpoint := "-"
	if d.Spec.Networking.ControlPlaneEndpoint != "" {
		port := d.Spec.Networking.ControlPlaneEndpointPort
		if port == 0 {
			port = 6443
		}
		endpoint = fmt.Sprintf("%s:%d", d.Spec.Networking.ControlPlaneEndpoint, port)
	}
	kube := "-"
	if c.KubeconfigRef != nil && *c.KubeconfigRef != "" {
		kube = *c.KubeconfigRef
	}

	fmt.Printf("Cluster: %s (%s)\n", c.Name, c.Environment)
	fmt.Printf("  Status:     %s\n", c.Status)
	fmt.Printf("  Provider:   %s\n", provLine)
	fmt.Printf("  Kubernetes: v%s\n", d.Spec.KubernetesVersion)
	fmt.Printf("  Endpoint:   %s\n", endpoint)
	fmt.Printf("  Workspace:  %s\n", c.TofuWorkspace)
	fmt.Printf("  Kubeconfig: %s\n", kube)
	fmt.Printf("  Created:    %s\n", c.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:    %s\n", c.UpdatedAt.Format(time.RFC3339))

	fmt.Printf("\nNodes (%d):\n", len(d.Nodes))
	if len(d.Nodes) == 0 {
		fmt.Println("  (none yet - provisioning has not run)")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tROLE\tIP\tSTATUS")
		for _, n := range d.Nodes {
			ip := "-"
			if n.IpAddress != nil && *n.IpAddress != "" {
				ip = *n.IpAddress
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", n.Name, n.Role, ip, n.Status)
		}
		_ = w.Flush()
	}

	fmt.Printf("\nJobs (%d):\n", len(d.Jobs))
	if len(d.Jobs) == 0 {
		fmt.Println("  (none)")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "  OPERATION\tSTATUS\tCREATED")
		for _, j := range d.Jobs {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", j.Operation, j.Status, j.CreatedAt.Format(time.RFC3339))
		}
		_ = w.Flush()
		for _, j := range d.Jobs {
			if j.Error != nil && *j.Error != "" {
				fmt.Printf("\n  last error (%s job): %s\n", j.Operation, *j.Error)
				break
			}
		}
	}

	if *live {
		fmt.Printf("\nLive provider inventory (%d VMs):\n", len(d.LiveVMs))
		switch {
		case d.LiveError != "":
			fmt.Printf("  (unavailable: %s)\n", d.LiveError)
		case len(d.LiveVMs) == 0:
			fmt.Println("  (none)")
		default:
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tPOWER\tIP\tCPU\tMEM(MB)")
			for _, vm := range d.LiveVMs {
				ip := vm.IP
				if ip == "" {
					ip = "-"
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%d\n", vm.Name, vm.PowerState, ip, vm.NumCPU, vm.MemoryMB)
			}
			_ = w.Flush()
		}
	}
	return nil
}

// --- standalone VMs (kind=vm resources) ---

type vmFile struct {
	Name        string        `json:"name"`
	Environment string        `json:"environment"`
	Provider    string        `json:"provider"`
	Spec        models.VMSpec `json:"spec"`
}

func loadVMFile(path string) (*vmFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var vf vmFile
	if err := yaml.Unmarshal(data, &vf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &vf, nil
}

func vmCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord vm <create|list|status|scale|destroy>")
	}
	switch args[0] {
	case "create":
		return vmCreate(cfg, args[1:])
	case "list":
		return vmList(cfg)
	case "status":
		return vmStatus(cfg, args[1:])
	case "scale":
		return vmScale(cfg, args[1:])
	case "destroy":
		return vmDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown vm subcommand %q (want create|list|status|scale|destroy)", args[0])
	}
}

func vmScale(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord vm scale <name> --count N [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("vm scale", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	count := fs.Int("count", 0, "new VM count (required, >= 1)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *count < 1 {
		return errors.New("--count >= 1 is required")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.ScaleVM(ctx, name, *env, *count); err != nil {
		return err
	}
	fmt.Printf("✓ vm %q scaling to %d - re-provisioning in the background\n", name, *count)
	return nil
}

func vmCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("vm create", flag.ContinueOnError)
	file := fs.String("f", "", "path to VM spec YAML (required)")
	count := fs.Int("count", 0, "override the VM count from the spec")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}

	vf, err := loadVMFile(*file)
	if err != nil {
		return err
	}
	if *count > 0 {
		vf.Spec.Count = *count
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateVM(ctx, orchestrator.CreateVMInput{
		Name:        vf.Name,
		Environment: vf.Environment,
		Provider:    vf.Provider,
		Spec:        vf.Spec,
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
	fmt.Printf("vm %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background (tofu apply); check `opord vm status " + r.Name + "`.")
	return nil
}

func vmList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListVMs(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no VMs yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tSTATUS\tCOUNT\tTTL(h)\tCREATED")
	for _, v := range list {
		prov := v.Provider
		if prov == "" {
			prov = "-"
		}
		ttl := "-"
		if v.Spec.TTLHours > 0 {
			ttl = fmt.Sprintf("%d", v.Spec.TTLHours)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			v.Resource.Name, v.Resource.Environment, prov, v.Resource.Status,
			v.Spec.Count, ttl, v.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func vmStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord vm status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("vm status", flag.ContinueOnError)
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

	v, err := svc.VMStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := v.Resource
	prov := v.Provider
	if prov == "" {
		prov = "-"
	}
	sizing := fmt.Sprintf("%d vCPU / %d MB / %d GB", v.Spec.CPU, v.Spec.MemoryMB, v.Spec.DiskGB)
	if v.Spec.InstanceType != "" {
		sizing = fmt.Sprintf("%s (%d GB disk)", v.Spec.InstanceType, v.Spec.DiskGB)
	}
	ttl := "never"
	if v.Spec.TTLHours > 0 {
		ttl = fmt.Sprintf("%dh (auto-destroy)", v.Spec.TTLHours)
	}

	fmt.Printf("VM: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Template:  %s\n", v.Spec.Template)
	fmt.Printf("  Count:     %d\n", v.Spec.Count)
	fmt.Printf("  Sizing:    %s\n", sizing)
	fmt.Printf("  TTL:       %s\n", ttl)
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:   %s\n", r.UpdatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" {
		fmt.Printf("  Observed:  %s\n", string(r.Observed))
	}
	return nil
}

func vmDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord vm destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("vm destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy VM %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying %q … (tofu destroy can take a few minutes)\n", name)
	if err := svc.DestroyVM(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ vm %q destroyed\n", name)
	return nil
}

// --- tenants & users (RBAC) ---

func tenantCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord tenant <add|list>")
	}
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	switch args[0] {
	case "add":
		if len(args) < 2 {
			return errors.New("usage: opord tenant add <name>")
		}
		t, err := svc.CreateTenant(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("tenant %q created (id=%s)\n", t.Name, t.ID)
		return nil
	case "list":
		list, err := svc.ListTenants(ctx)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("no tenants yet")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tID\tCREATED")
		for _, t := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, t.ID, t.CreatedAt.Format(time.RFC3339))
		}
		return w.Flush()
	default:
		return fmt.Errorf("unknown tenant subcommand %q (want add|list)", args[0])
	}
}

func userCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord user <add|list>")
	}
	switch args[0] {
	case "add":
		return userAdd(cfg, args[1:])
	case "list":
		return userList(cfg)
	default:
		return fmt.Errorf("unknown user subcommand %q (want add|list)", args[0])
	}
}

func userAdd(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	email := fs.String("email", "", "user email (required)")
	tenant := fs.String("tenant", "", "tenant name (required)")
	role := fs.String("role", "viewer", "role: admin|operator|viewer")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *tenant == "" {
		return errors.New("--email and --tenant are required")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	u, key, err := svc.CreateUser(ctx, *email, *tenant, *role)
	if err != nil {
		return err
	}
	fmt.Printf("user %q created (tenant %s, role %s)\n", u.Email, *tenant, u.Role)
	fmt.Printf("  API key (shown once): %s\n", key)
	fmt.Println("  use it as: Authorization: Bearer <key>")
	return nil
}

func userList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no users yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "EMAIL\tTENANT\tROLE\tCREATED")
	for _, u := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.User.Email, u.Tenant, u.User.Role, u.User.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

// --- cost (FinOps estimate) ---

func costReport(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	rep, err := svc.CostReport(ctx)
	if err != nil {
		return err
	}
	if len(rep.Lines) == 0 {
		fmt.Println("no active resources to cost")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tKIND\tPROVIDER\tENV\tSTATUS\tEST $/MO")
	for _, l := range rep.Lines {
		prov := l.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t$%.2f\n", l.Name, l.Kind, prov, l.Environment, l.Status, l.MonthlyUSD)
	}
	_ = w.Flush()
	fmt.Printf("\nTotal estimated: $%.2f/mo  (rough estimate; stacks excluded)\n", rep.TotalUSD)
	return nil
}

// --- self-service requests (approval workflow) ---

type requestFile struct {
	Name        string          `json:"name"`
	Environment string          `json:"environment"`
	Requester   string          `json:"requester"`
	Kind        string          `json:"kind"`
	Provider    string          `json:"provider"`
	Blueprint   string          `json:"blueprint"`
	Spec        json.RawMessage `json:"spec"`
}

func requestCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord request <create|list|status|approve|reject>")
	}
	switch args[0] {
	case "create":
		return requestCreate(cfg, args[1:])
	case "list":
		return requestList(cfg)
	case "status":
		return requestStatus(cfg, args[1:])
	case "approve":
		return requestDecide(cfg, args[1:], true)
	case "reject":
		return requestDecide(cfg, args[1:], false)
	default:
		return fmt.Errorf("unknown request subcommand %q (want create|list|status|approve|reject)", args[0])
	}
}

func requestCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("request create", flag.ContinueOnError)
	file := fs.String("f", "", "path to request YAML (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <request.yaml> is required")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("reading request file: %w", err)
	}
	var rf requestFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return fmt.Errorf("parsing request %q: %w", *file, err)
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	req, err := svc.CreateRequest(ctx, orchestrator.CreateRequestInput{
		Name:        rf.Name,
		Environment: rf.Environment,
		Requester:   rf.Requester,
		Kind:        rf.Kind,
		Provider:    rf.Provider,
		Blueprint:   rf.Blueprint,
		Spec:        rf.Spec,
	})
	if err != nil {
		return err
	}
	fmt.Printf("request %q submitted (status: %s)\n", req.Name, req.Status)
	if req.TicketRef != "" {
		fmt.Printf("  GLPI ticket: %s\n", req.TicketRef)
	}
	fmt.Println("  approve with `opord request approve " + req.Name + "`")
	return nil
}

func requestList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListRequests(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no requests yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tKIND\tREQUESTER\tSTATUS\tTICKET\tCREATED")
	for _, r := range list {
		ticket := r.TicketRef
		if ticket == "" {
			ticket = "-"
		}
		req := r.Requester
		if req == "" {
			req = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Name, r.Environment, r.Kind, req, r.Status, ticket, r.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func requestStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord request status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("request status", flag.ContinueOnError)
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

	r, err := svc.RequestStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	fmt.Printf("Request: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Kind:      %s\n", r.Kind)
	fmt.Printf("  Provider:  %s\n", r.Provider)
	if r.Blueprint != "" {
		fmt.Printf("  Blueprint: %s\n", r.Blueprint)
	}
	fmt.Printf("  Requester: %s\n", r.Requester)
	if r.TicketRef != "" {
		fmt.Printf("  Ticket:    %s\n", r.TicketRef)
	}
	if r.ResourceRef != "" {
		fmt.Printf("  Resource:  %s\n", r.ResourceRef)
	}
	if r.DecidedBy != "" {
		fmt.Printf("  Decided:   %s\n", r.DecidedBy)
	}
	if r.Reason != "" {
		fmt.Printf("  Reason:    %s\n", r.Reason)
	}
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	return nil
}

func requestDecide(cfg *config.Config, args []string, approve bool) error {
	verb := "approve"
	if !approve {
		verb = "reject"
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("usage: opord request %s <name> [--by <user>] [--env <env>]", verb)
	}
	name := args[0]
	fs := flag.NewFlagSet("request "+verb, flag.ContinueOnError)
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
		fmt.Printf("✓ request %q approved - provisioning started\n", name)
		return nil
	}
	if err := svc.RejectRequest(ctx, name, *env, *by, *reason); err != nil {
		return err
	}
	fmt.Printf("✓ request %q rejected\n", name)
	return nil
}

// --- generic stacks (kind=stack resources: arbitrary OpenTofu) ---

type stackFile struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.StackSpec `json:"spec"`
}

func loadStackFile(path string) (*stackFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var sf stackFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &sf, nil
}

func stackCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord stack <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return stackCreate(cfg, args[1:])
	case "list":
		return stackList(cfg)
	case "status":
		return stackStatus(cfg, args[1:])
	case "destroy":
		return stackDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown stack subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func stackCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("stack create", flag.ContinueOnError)
	file := fs.String("f", "", "path to stack spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the module offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	sf, err := loadStackFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateStack(ctx, orchestrator.CreateStackInput{
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
		fmt.Println("✓ dry-run OK - module valid, nothing changed")
		fmt.Printf("  %s\n", res.Summary)
		return nil
	}
	r := res.Resource
	fmt.Printf("stack %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background (tofu apply); check `opord stack status " + r.Name + "`.")
	return nil
}

func stackList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListStacks(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no stacks yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tMODULE\tSTATUS\tCREATED")
	for _, st := range list {
		prov := st.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			st.Resource.Name, st.Resource.Environment, prov, st.Spec.ModuleDir, st.Resource.Status,
			st.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func stackStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord stack status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("stack status", flag.ContinueOnError)
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

	st, err := svc.StackStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := st.Resource
	prov := st.Provider
	if prov == "" {
		prov = "-"
	}
	fmt.Printf("Stack: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Module:    %s\n", st.Spec.ModuleDir)
	fmt.Printf("  Variables: %d\n", len(st.Spec.Variables))
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Outputs:   %s\n", string(r.Observed))
	}
	return nil
}

func stackDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord stack destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("stack destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy stack %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying stack %q … (tofu destroy can take a few minutes)\n", name)
	if err := svc.DestroyStack(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ stack %q destroyed\n", name)
	return nil
}

// --- managed databases (kind=database resources) ---

type dbFile struct {
	Name        string              `json:"name"`
	Environment string              `json:"environment"`
	Provider    string              `json:"provider"`
	Spec        models.DatabaseSpec `json:"spec"`
}

func loadDBFile(path string) (*dbFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var df dbFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &df, nil
}

func dbCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord db <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return dbCreate(cfg, args[1:])
	case "list":
		return dbList(cfg)
	case "status":
		return dbStatus(cfg, args[1:])
	case "scale":
		return dbScale(cfg, args[1:])
	case "backup":
		return dbBackup(cfg, args[1:])
	case "destroy":
		return dbDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown db subcommand %q (want create|list|status|scale|backup|destroy)", args[0])
	}
}

func dbBackup(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord db backup <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("db backup", flag.ContinueOnError)
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

	b, err := svc.BackupDatabase(ctx, name, *env)
	if err != nil {
		return err
	}
	fmt.Printf("backup of %q started (id=%s, status: %s)\n", name, b.ID, b.Status)
	fmt.Println("  RDS snapshot runs in the background; check `opord backups`.")
	return nil
}

func backupsList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListBackups(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no backups yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "RESOURCE\tKIND\tENV\tPROVIDER\tSNAPSHOT\tSTATUS\tCREATED")
	for _, b := range list {
		snap := b.SnapshotID
		if snap == "" {
			snap = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			b.ResourceName, b.ResourceKind, b.Environment, b.Provider, snap, b.Status, b.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func dbScale(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord db scale <name> [--instance-class X] [--storage N] [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("db scale", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	class := fs.String("instance-class", "", "new RDS instance class (e.g. db.t3.small)")
	storage := fs.Int("storage", 0, "new storage GB (can only grow)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *class == "" && *storage <= 0 {
		return errors.New("provide --instance-class and/or --storage")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.ScaleDatabase(ctx, name, *env, *class, *storage); err != nil {
		return err
	}
	fmt.Printf("✓ database %q scaling - re-provisioning in the background\n", name)
	return nil
}

func dbCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("db create", flag.ContinueOnError)
	file := fs.String("f", "", "path to database spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	df, err := loadDBFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateDatabase(ctx, orchestrator.CreateDatabaseInput{
		Name:        df.Name,
		Environment: df.Environment,
		Provider:    df.Provider,
		Spec:        df.Spec,
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
	fmt.Printf("database %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background (RDS apply); check `opord db status " + r.Name + "`.")
	return nil
}

func dbList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListDatabases(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no databases yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tENGINE\tSTATUS\tCREATED")
	for _, d := range list {
		prov := d.Provider
		if prov == "" {
			prov = "-"
		}
		engine := d.Spec.Engine
		if engine == "" {
			engine = "postgres"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Resource.Name, d.Resource.Environment, prov, engine, d.Resource.Status,
			d.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func dbStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord db status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("db status", flag.ContinueOnError)
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

	d, err := svc.DatabaseStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := d.Resource
	prov := d.Provider
	if prov == "" {
		prov = "-"
	}
	engine := d.Spec.Engine
	if engine == "" {
		engine = "postgres"
	}
	fmt.Printf("Database: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Engine:    %s %s\n", engine, d.Spec.Version)
	fmt.Printf("  Sizing:    %s / %d GB\n", d.Spec.InstanceClass, d.Spec.StorageGB)
	fmt.Printf("  DB name:   %s (user %s)\n", d.Spec.DBName, d.Spec.Username)
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:  %s\n", string(r.Observed))
	}
	return nil
}

func dbDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord db destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("db destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy database %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying database %q … (tofu destroy can take a few minutes)\n", name)
	if err := svc.DestroyDatabase(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ database %q destroyed\n", name)
	return nil
}

// --- managed tables (kind=table resources; AWS DynamoDB) ---

type tableFile struct {
	Name        string           `json:"name"`
	Environment string           `json:"environment"`
	Provider    string           `json:"provider"`
	Spec        models.TableSpec `json:"spec"`
}

func loadTableFile(path string) (*tableFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var tf tableFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &tf, nil
}

func tableCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord table <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return tableCreate(cfg, args[1:])
	case "list":
		return tableList(cfg)
	case "status":
		return tableStatus(cfg, args[1:])
	case "destroy":
		return tableDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown table subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func tableCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("table create", flag.ContinueOnError)
	file := fs.String("f", "", "path to table spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	tf, err := loadTableFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateTable(ctx, orchestrator.CreateTableInput{
		Name:        tf.Name,
		Environment: tf.Environment,
		Provider:    tf.Provider,
		Spec:        tf.Spec,
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
	fmt.Printf("table %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord table status " + r.Name + "`.")
	return nil
}

func tableList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListTables(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no tables yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tHASH KEY\tSTATUS\tCREATED")
	for _, t := range list {
		prov := t.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			t.Resource.Name, t.Resource.Environment, prov, t.Spec.HashKey, t.Resource.Status,
			t.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func tableStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord table status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("table status", flag.ContinueOnError)
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

	t, err := svc.TableStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := t.Resource
	prov := t.Provider
	if prov == "" {
		prov = "-"
	}
	billing := t.Spec.BillingMode
	if billing == "" {
		billing = "PAY_PER_REQUEST"
	}
	fmt.Printf("Table: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Hash key:  %s (%s)\n", t.Spec.HashKey, t.Spec.HashKeyType)
	if t.Spec.RangeKey != "" {
		fmt.Printf("  Range key: %s (%s)\n", t.Spec.RangeKey, t.Spec.RangeKeyType)
	}
	fmt.Printf("  Billing:   %s\n", billing)
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:  %s\n", string(r.Observed))
	}
	return nil
}

func tableDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord table destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("table destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy table %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying table %q …\n", name)
	if err := svc.DestroyTable(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ table %q destroyed\n", name)
	return nil
}

// --- object storage buckets (kind=s3 resources; AWS S3) ---

type s3File struct {
	Name        string        `json:"name"`
	Environment string        `json:"environment"`
	Provider    string        `json:"provider"`
	Spec        models.S3Spec `json:"spec"`
}

func loadS3File(path string) (*s3File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var sf s3File
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &sf, nil
}

func s3Cmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord s3 <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return s3Create(cfg, args[1:])
	case "list":
		return s3List(cfg)
	case "status":
		return s3Status(cfg, args[1:])
	case "destroy":
		return s3Destroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown s3 subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func s3Create(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("s3 create", flag.ContinueOnError)
	file := fs.String("f", "", "path to s3 spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	sf, err := loadS3File(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateS3(ctx, orchestrator.CreateS3Input{
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
	fmt.Printf("s3 bucket %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord s3 status " + r.Name + "`.")
	return nil
}

func s3List(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListS3(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no s3 buckets yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tBUCKET\tSTATUS\tCREATED")
	for _, b := range list {
		prov := b.Provider
		if prov == "" {
			prov = "-"
		}
		bucket := b.Spec.Name
		if bucket == "" {
			bucket = b.Resource.Name
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			b.Resource.Name, b.Resource.Environment, prov, bucket, b.Resource.Status,
			b.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func s3Status(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord s3 status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("s3 status", flag.ContinueOnError)
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

	b, err := svc.S3Status(ctx, name, *env)
	if err != nil {
		return err
	}
	r := b.Resource
	prov := b.Provider
	if prov == "" {
		prov = "-"
	}
	bucket := b.Spec.Name
	if bucket == "" {
		bucket = r.Name
	}
	fmt.Printf("S3 bucket: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:      %s\n", r.Status)
	fmt.Printf("  Provider:    %s\n", prov)
	fmt.Printf("  Bucket name: %s\n", bucket)
	fmt.Printf("  Private:     true\n")
	fmt.Printf("  Versioned:   true\n")
	fmt.Printf("  Workspace:   %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:     %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:    %s\n", string(r.Observed))
	}
	return nil
}

func s3Destroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord s3 destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("s3 destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy s3 bucket %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying s3 bucket %q …\n", name)
	if err := svc.DestroyS3(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ s3 bucket %q destroyed\n", name)
	return nil
}

// --- serverless functions (kind=function resources; AWS Lambda) ---

type functionFile struct {
	Name        string              `json:"name"`
	Environment string              `json:"environment"`
	Provider    string              `json:"provider"`
	Spec        models.FunctionSpec `json:"spec"`
}

func loadFunctionFile(path string) (*functionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var ff functionFile
	if err := yaml.Unmarshal(data, &ff); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &ff, nil
}

func functionCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord function <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return functionCreate(cfg, args[1:])
	case "list":
		return functionList(cfg)
	case "status":
		return functionStatus(cfg, args[1:])
	case "destroy":
		return functionDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown function subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func functionCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("function create", flag.ContinueOnError)
	file := fs.String("f", "", "path to function spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	ff, err := loadFunctionFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateFunction(ctx, orchestrator.CreateFunctionInput{
		Name:        ff.Name,
		Environment: ff.Environment,
		Provider:    ff.Provider,
		Spec:        ff.Spec,
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
	fmt.Printf("function %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord function status " + r.Name + "`.")
	return nil
}

func functionList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListFunctions(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no functions yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tRUNTIME\tSTATUS\tCREATED")
	for _, f := range list {
		prov := f.Provider
		if prov == "" {
			prov = "-"
		}
		runtime := f.Spec.Runtime
		if runtime == "" {
			runtime = "python3.12"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			f.Resource.Name, f.Resource.Environment, prov, runtime, f.Resource.Status,
			f.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func functionStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord function status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("function status", flag.ContinueOnError)
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

	f, err := svc.FunctionStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := f.Resource
	prov := f.Provider
	if prov == "" {
		prov = "-"
	}
	runtime := f.Spec.Runtime
	if runtime == "" {
		runtime = "python3.12"
	}
	fmt.Printf("Function: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Runtime:   %s (handler %s)\n", runtime, f.Spec.Handler)
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:  %s\n", string(r.Observed))
	}
	return nil
}

func functionDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord function destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("function destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy function %q (env %q)? This runs `tofu destroy` and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying function %q …\n", name)
	if err := svc.DestroyFunction(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ function %q destroyed\n", name)
	return nil
}

// --- access-vending projects (kind=project; AWS IAM Identity Center) ---

type projectFile struct {
	Name        string             `json:"name"`
	Environment string             `json:"environment"`
	Provider    string             `json:"provider"`
	Spec        models.ProjectSpec `json:"spec"`
}

func loadProjectFile(path string) (*projectFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var pf projectFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &pf, nil
}

func projectCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord project <create|list|status|members|destroy>")
	}
	switch args[0] {
	case "create":
		return projectCreate(cfg, args[1:])
	case "list":
		return projectList(cfg)
	case "status":
		return projectStatus(cfg, args[1:])
	case "members":
		return projectMembers(cfg, args[1:])
	case "destroy":
		return projectDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown project subcommand %q (want create|list|status|members|destroy)", args[0])
	}
}

func projectCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("project create", flag.ContinueOnError)
	file := fs.String("f", "", "path to project spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	pf, err := loadProjectFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateProject(ctx, orchestrator.CreateProjectInput{
		Name:        pf.Name,
		Environment: pf.Environment,
		Provider:    pf.Provider,
		Spec:        pf.Spec,
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
	fmt.Printf("project %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord project status " + r.Name + "`.")
	return nil
}

func projectList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListProjects(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no projects yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tACCOUNT\tMEMBERS\tSTATUS\tCREATED")
	for _, p := range list {
		prov := p.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			p.Resource.Name, p.Resource.Environment, prov, p.Spec.AccountID, len(p.Spec.UserNames), p.Resource.Status,
			p.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func projectStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord project status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("project status", flag.ContinueOnError)
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

	p, err := svc.ProjectStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := p.Resource
	prov := p.Provider
	if prov == "" {
		prov = "-"
	}
	fmt.Printf("Project: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Account:   %s\n", p.Spec.AccountID)
	if len(p.Spec.UserNames) > 0 {
		fmt.Printf("  Members:   %s\n", strings.Join(p.Spec.UserNames, ", "))
	} else {
		fmt.Printf("  Members:   (none)\n")
	}
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:  %s\n", string(r.Observed))
	}
	return nil
}

func projectMembers(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord project members <name> (--add a,b | --remove c | --set a,b,c) [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("project members", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	add := fs.String("add", "", "comma-separated usernames to add")
	remove := fs.String("remove", "", "comma-separated usernames to remove")
	set := fs.String("set", "", "comma-separated usernames to set as the full member list")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *add == "" && *remove == "" && *set == "" {
		return errors.New("provide --add, --remove, or --set")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	cur, err := svc.ProjectStatus(ctx, name, *env)
	if err != nil {
		return err
	}

	var members []string
	if *set != "" {
		members = csvList(*set)
	} else {
		keep := map[string]struct{}{}
		for _, m := range cur.Spec.UserNames {
			keep[m] = struct{}{}
		}
		for _, a := range csvList(*add) {
			keep[a] = struct{}{}
		}
		for _, r := range csvList(*remove) {
			delete(keep, r)
		}
		for m := range keep {
			members = append(members, m)
		}
	}

	if err := svc.SetProjectMembers(ctx, name, *env, members); err != nil {
		return err
	}
	fmt.Printf("✓ project %q members updated (%d) - re-provisioning in the background\n", name, len(members))
	return nil
}

// csvList splits a comma-separated flag value into trimmed, non-empty entries.
func csvList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func projectDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord project destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("project destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy project %q (env %q)? This revokes the access (tofu destroy) and cannot be undone. [y/N]: ", name, *env)
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

	fmt.Printf("destroying project %q …\n", name)
	if err := svc.DestroyProject(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ project %q destroyed\n", name)
	return nil
}

// --- member AWS accounts (kind=account; the OPORD account factory) ---

type accountFile struct {
	Name        string             `json:"name"`
	Environment string             `json:"environment"`
	Provider    string             `json:"provider"`
	Spec        models.AccountSpec `json:"spec"`
}

func loadAccountFile(path string) (*accountFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}
	var af accountFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}
	return &af, nil
}

func accountCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord account <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return accountCreate(cfg, args[1:])
	case "list":
		return accountList(cfg)
	case "status":
		return accountStatus(cfg, args[1:])
	case "destroy":
		return accountDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown account subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func accountCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("account create", flag.ContinueOnError)
	file := fs.String("f", "", "path to account spec YAML (required)")
	dryRun := fs.Bool("dry-run", false, "validate the spec offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("-f <spec.yaml> is required")
	}
	af, err := loadAccountFile(*file)
	if err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateAccount(ctx, orchestrator.CreateAccountInput{
		Name:        af.Name,
		Environment: af.Environment,
		Provider:    af.Provider,
		Spec:        af.Spec,
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
	fmt.Printf("account %q registered (status: %s)\n", r.Name, r.Status)
	fmt.Printf("  id=%s  workspace=%s\n", r.ID, r.TofuWorkspace)
	fmt.Println("  provisioning runs in the background; check `opord account status " + r.Name + "`.")
	return nil
}

func accountList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListAccounts(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no accounts yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tPROVIDER\tCSA\tCLOUD\tSTATUS\tCREATED")
	for _, a := range list {
		prov := a.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			a.Resource.Name, a.Resource.Environment, prov, a.Spec.CSAID, a.Spec.CloudName, a.Resource.Status,
			a.Resource.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func accountStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord account status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("account status", flag.ContinueOnError)
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

	a, err := svc.AccountStatus(ctx, name, *env)
	if err != nil {
		return err
	}
	r := a.Resource
	prov := a.Provider
	if prov == "" {
		prov = "-"
	}
	fmt.Printf("Account: %s (%s)\n", r.Name, r.Environment)
	fmt.Printf("  Status:    %s\n", r.Status)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  CSA:       %s   Cloud: %s\n", a.Spec.CSAID, a.Spec.CloudName)
	fmt.Printf("  Workspace: %s\n", r.TofuWorkspace)
	fmt.Printf("  Created:   %s\n", r.CreatedAt.Format(time.RFC3339))
	if len(r.Observed) > 0 && string(r.Observed) != "null" && string(r.Observed) != "{}" {
		fmt.Printf("  Observed:  %s\n", string(r.Observed))
	}
	return nil
}

func accountDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord account destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("account destroy", flag.ContinueOnError)
	env := fs.String("env", "dev", "environment")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Tear down account %q layers (env %q)? This runs `tofu destroy` on the baseline/VPC/security layers (the AWS account itself is NOT closed). [y/N]: ", name, *env)
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

	fmt.Printf("destroying account %q layers …\n", name)
	if err := svc.DestroyAccount(ctx, name, *env); err != nil {
		return err
	}
	fmt.Printf("✓ account %q layers destroyed (account not closed - see decommission runbook)\n", name)
	return nil
}

// --- composed environments (blueprints / golden paths) ---

func blueprintList() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tCOMPONENTS\tDESCRIPTION")
	for _, b := range templates.List() {
		kinds := make([]string, 0, len(b.Components))
		for _, c := range b.Components {
			kinds = append(kinds, fmt.Sprintf("%s:%s", c.Name, c.Kind))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.ID, b.Name, strings.Join(kinds, ","), b.Description)
	}
	return w.Flush()
}

func envCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord env <create|list|status|destroy>")
	}
	switch args[0] {
	case "create":
		return envCreate(cfg, args[1:])
	case "list":
		return envList(cfg)
	case "status":
		return envStatus(cfg, args[1:])
	case "destroy":
		return envDestroy(cfg, args[1:])
	default:
		return fmt.Errorf("unknown env subcommand %q (want create|list|status|destroy)", args[0])
	}
}

func envCreate(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("env create", flag.ContinueOnError)
	file := fs.String("f", "", "path to a custom environment YAML (components or blueprint+overrides)")
	name := fs.String("name", "", "environment name")
	provider := fs.String("provider", "", "registered provider name")
	blueprint := fs.String("blueprint", "", "built-in blueprint id (see `opord blueprints`)")
	environment := fs.String("env", "dev", "environment tier")
	sshKey := fs.String("ssh-key", "", "SSH public key for the nodes")
	template := fs.String("template", "", "override golden image / VMID / AMI")
	dryRun := fs.Bool("dry-run", false, "validate every component offline; do not provision or persist")
	if err := fs.Parse(args); err != nil {
		return err
	}

	in := orchestrator.CreateEnvironmentInput{
		Name:         *name,
		Environment:  *environment,
		Provider:     *provider,
		Blueprint:    *blueprint,
		Template:     *template,
		SSHPublicKey: *sshKey,
		DryRun:       *dryRun,
	}
	// A spec file fills/overrides fields and may carry explicit components.
	if *file != "" {
		ef, err := loadEnvFile(*file)
		if err != nil {
			return err
		}
		if ef.Name != "" {
			in.Name = ef.Name
		}
		if ef.Environment != "" {
			in.Environment = ef.Environment
		}
		if ef.Provider != "" {
			in.Provider = ef.Provider
		}
		if ef.Blueprint != "" {
			in.Blueprint = ef.Blueprint
		}
		if ef.Template != "" {
			in.Template = ef.Template
		}
		if ef.SSHPublicKey != "" {
			in.SSHPublicKey = ef.SSHPublicKey
		}
		in.Components = ef.Components
	}
	if in.Name == "" || in.Provider == "" {
		return errors.New("name and provider are required (flags or -f file)")
	}
	if in.Blueprint == "" && len(in.Components) == 0 {
		return errors.New("provide --blueprint or a -f file with components")
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.CreateEnvironment(ctx, in)
	if err != nil {
		return err
	}

	label := in.Blueprint
	if label == "" {
		label = "custom"
	}
	if res.DryRun {
		fmt.Printf("✓ dry-run OK - blueprint %q valid (%d component(s)), nothing changed\n", label, len(res.Summaries))
		for _, line := range res.Summaries {
			fmt.Printf("  - %s\n", line)
		}
		return nil
	}
	fmt.Printf("environment %q created from blueprint %q (status: %s)\n", res.Env.Name, res.Env.Blueprint, res.Env.Status)
	fmt.Println("  components provision in the background; check `opord env status " + res.Env.Name + "`.")
	return nil
}

type envFile struct {
	Name         string             `json:"name"`
	Environment  string             `json:"environment"`
	Provider     string             `json:"provider"`
	Blueprint    string             `json:"blueprint"`
	Template     string             `json:"template"`
	SSHPublicKey string             `json:"ssh_public_key"`
	Components   []models.Component `json:"components"`
}

func loadEnvFile(path string) (*envFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading env file: %w", err)
	}
	var ef envFile
	if err := yaml.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parsing env file %q: %w", path, err)
	}
	return &ef, nil
}

func envList(cfg *config.Config) error {
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no environments yet")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENV\tBLUEPRINT\tPROVIDER\tSTATUS\tCOMPONENTS\tCREATED")
	for _, e := range list {
		prov := e.Provider
		if prov == "" {
			prov = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			e.Env.Name, e.Env.Environment, e.Env.Blueprint, prov, e.Aggregate,
			len(e.Components), e.Env.CreatedAt.Format(time.RFC3339))
	}
	return w.Flush()
}

func envStatus(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord env status <name> [--env <env>]")
	}
	name := args[0]
	fs := flag.NewFlagSet("env status", flag.ContinueOnError)
	environment := fs.String("env", "dev", "environment tier")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	e, err := svc.EnvironmentStatus(ctx, name, *environment)
	if err != nil {
		return err
	}
	prov := e.Provider
	if prov == "" {
		prov = "-"
	}
	fmt.Printf("Environment: %s (%s)\n", e.Env.Name, e.Env.Environment)
	fmt.Printf("  Blueprint: %s\n", e.Env.Blueprint)
	fmt.Printf("  Provider:  %s\n", prov)
	fmt.Printf("  Status:    %s\n", e.Aggregate)
	fmt.Printf("  Created:   %s\n", e.Env.CreatedAt.Format(time.RFC3339))
	fmt.Printf("\nComponents (%d):\n", len(e.Components))
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "  COMPONENT\tKIND\tRESOURCE\tSTATUS")
	for _, c := range e.Components {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", c.Name, c.Kind, c.ChildName, c.Status)
	}
	return w.Flush()
}

func envDestroy(cfg *config.Config, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: opord env destroy <name> [--env <env>] [--yes]")
	}
	name := args[0]
	fs := flag.NewFlagSet("env destroy", flag.ContinueOnError)
	environment := fs.String("env", "dev", "environment tier")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if !*yes {
		fmt.Printf("Destroy environment %q (env %q) and ALL its components? This runs `tofu destroy`. [y/N]: ", name, *environment)
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

	fmt.Printf("destroying environment %q … (tofu destroy per component)\n", name)
	if err := svc.DestroyEnvironment(ctx, name, *environment); err != nil {
		return err
	}
	fmt.Printf("✓ environment %q destroyed\n", name)
	return nil
}

// --- vcenter (live vSphere Web Services API via govmomi) ---

func vcenterCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord vcenter check [--provider NAME | --url URL]")
	}
	switch args[0] {
	case "check":
		return vcenterCheck(cfg, args[1:])
	default:
		return fmt.Errorf("unknown vcenter subcommand %q (want check)", args[0])
	}
}

func vcenterCheck(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("vcenter check", flag.ContinueOnError)
	provider := fs.String("provider", "", "registered provider name (uses its server + env creds)")
	urlFlag := fs.String("url", "", "direct vCenter SDK URL, e.g. https://host:8989/sdk")
	user := fs.String("user", "", "username (with --url)")
	pass := fs.String("password", "", "password (with --url)")
	insecure := fs.Bool("insecure", true, "skip TLS verification")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	var vcfg vcenter.Config

	switch {
	case *urlFlag != "":
		vcfg = vcenter.Config{URL: *urlFlag, User: *user, Password: *pass, Insecure: *insecure}
	case *provider != "":
		svc, cleanup, err := newService(ctx, cfg)
		if err != nil {
			return err
		}
		defer cleanup()
		p, err := svc.Provider(ctx, *provider)
		if err != nil {
			return fmt.Errorf("provider %q: %w", *provider, err)
		}
		var pc map[string]any
		_ = json.Unmarshal(p.Config, &pc)
		server, _ := pc["server"].(string)
		if server == "" {
			return fmt.Errorf("provider %q has no 'server' in config", *provider)
		}
		cr, err := creds.NewResolver(cfg.VaultAddr, cfg.VaultToken, cfg.VaultKVMount, newLogger(cfg)).Resolve(ctx, p)
		if err != nil {
			return err
		}
		insec := *insecure
		if v, ok := pc["allow_unverified_ssl"].(bool); ok {
			insec = v
		}
		vcfg = vcenter.Config{URL: "https://" + server + "/sdk", User: cr["user"], Password: cr["password"], Insecure: insec}
	default:
		return errors.New("need --provider <name> or --url <url>")
	}

	c, err := vcenter.Connect(ctx, vcfg)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close(ctx) }()

	vms, err := c.ListVMs(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("VMs (%d):\n", len(vms))
	if len(vms) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tPOWER\tIP\tCPU\tMEM(MB)")
		for _, vm := range vms {
			ip := vm.IP
			if ip == "" {
				ip = "-"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%d\n", vm.Name, vm.PowerState, ip, vm.NumCPU, vm.MemoryMB)
		}
		_ = w.Flush()
	}

	tasks, err := c.RecentTasks(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\nRecent tasks (%d):\n", len(tasks))
	if len(tasks) == 0 {
		fmt.Println("  (none)")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "  TASK\tENTITY\tSTATE\tPROGRESS")
		for _, t := range tasks {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%d%%\n", t.DescriptionID, t.EntityName, t.State, t.Progress)
		}
		_ = w.Flush()
	}
	return nil
}

// --- shared helpers ---

// --- entra: automate the Entra (Azure AD) side of AWS SAML federation ---

func entraCmd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: opord entra grant|grant-group ...")
	}
	switch args[0] {
	case "grant":
		return entraGrant(cfg, args[1:])
	case "grant-group":
		return entraGrantGroup(cfg, args[1:])
	default:
		return fmt.Errorf("unknown entra subcommand %q (want grant, grant-group)", args[0])
	}
}

func entraGrant(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("entra grant", flag.ContinueOnError)
	appID := fs.String("app-id", "", "enterprise app (client/app) id (required)")
	roleArn := fs.String("role-arn", "", "AWS IAM role ARN to grant (required)")
	providerArn := fs.String("provider-arn", "", "AWS IAM SAML provider ARN (required)")
	roleName := fs.String("role-name", "ReadOnly", "label for the app role")
	users := fs.String("user", "", "comma-separated user emails/UPNs to assign (required)")
	invite := fs.Bool("invite", false, "invite the users as B2B guests before assigning")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *appID == "" || *roleArn == "" || *providerArn == "" || *users == "" {
		return errors.New("--app-id, --role-arn, --provider-arn and --user are required")
	}

	var entraUsers []orchestrator.EntraUser
	for _, e := range strings.Split(*users, ",") {
		if e = strings.TrimSpace(e); e != "" {
			entraUsers = append(entraUsers, orchestrator.EntraUser{Email: e, Role: *roleName, Invite: *invite})
		}
	}

	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := svc.GrantEntraAccess(ctx, orchestrator.GrantEntraAccessInput{
		AppID: *appID,
		Roles: map[string]string{*roleName: *roleArn + "," + *providerArn},
		Users: entraUsers,
	})
	if err != nil {
		return err
	}
	fmt.Printf("✓ entra: app role %q ensured (id %s)\n", *roleName, res.AppRoleIDs[*roleName])
	if len(res.Invited) > 0 {
		fmt.Printf("  invited as guests: %s\n", strings.Join(res.Invited, ", "))
	}
	fmt.Printf("  assigned: %s\n", strings.Join(res.Assigned, ", "))
	fmt.Println(" to users can now SSO via the enterprise app and assume the role in AWS.")
	return nil
}

// entraGrantGroup assigns an Entra group to a workforce/enterprise app (e.g. the
// GCP Workforce Identity Federation app, ADR-0012) so its members can authenticate.
func entraGrantGroup(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("entra grant-group", flag.ContinueOnError)
	appID := fs.String("app-id", "", "workforce/enterprise app (client/app) id (required)")
	groupID := fs.String("group-id", "", "Entra group object id to assign (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *appID == "" || *groupID == "" {
		return errors.New("--app-id and --group-id are required")
	}
	ctx := context.Background()
	svc, cleanup, err := newService(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := svc.GrantEntraGroupToApp(ctx, *appID, *groupID); err != nil {
		return err
	}
	fmt.Printf("✓ entra: group %s assigned to app %s\n", *groupID, *appID)
	fmt.Println(" to the group's members can now authenticate via the app (e.g. GCP WIF sign-in).")
	return nil
}

func newService(ctx context.Context, cfg *config.Config) (*orchestrator.Service, func(), error) {
	if cfg.DatabaseURL == "" {
		return nil, nil, errors.New("DATABASE_URL is not set (see .env.example)")
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	resolver := creds.NewResolver(cfg.VaultAddr, cfg.VaultToken, cfg.VaultKVMount, newLogger(cfg))
	if pass := creds.StateEncryptionPassphrase(context.Background(), resolver); pass != "" {
		tofu.SetStateEncryptionPassphrase(pass)
	}
	svc := orchestrator.New(db.New(pool), buildRegistry(cfg), resolver, newLogger(cfg), orchestrator.BootstrapConfig{
		AnsibleBin:    cfg.AnsibleBin,
		AnsibleDir:    cfg.AnsibleDir,
		SSHPrivateKey: cfg.SSHPrivateKey,
		ArtifactsDir:  cfg.ArtifactsDir,
	})
	if cfg.GLPIURL != "" && cfg.GLPIAppToken != "" && cfg.GLPIUserToken != "" {
		svc.SetTicketer(glpi.New(cfg.GLPIURL, cfg.GLPIAppToken, cfg.GLPIUserToken))
	}
	if cfg.VaultAddr != "" && cfg.VaultToken != "" {
		if cidrPool, err := ipam.NewVaultPool(cfg.VaultAddr, cfg.VaultToken, "opord-vpc-cidr-pools", "aws-vpc-cidr-pools", newLogger(cfg)); err == nil {
			svc.SetAllocator(cidrPool)
		}
	}
	// Microsoft Graph creds: env override, else Vault KV at "opord/azure/graph"
	// (keys tenant_id / client_id / client_secret) - Vault stays the source of truth.
	azCfg := azure.Config{TenantID: cfg.AzureTenantID, ClientID: cfg.AzureClientID, ClientSecret: cfg.AzureClientSecret}
	if azCfg.TenantID == "" || azCfg.ClientID == "" || azCfg.ClientSecret == "" {
		if sec, err := resolver.ReadSecret(ctx, "opord/azure/graph"); err == nil && sec != nil {
			if azCfg.TenantID == "" {
				azCfg.TenantID = sec["tenant_id"]
			}
			if azCfg.ClientID == "" {
				azCfg.ClientID = sec["client_id"]
			}
			if azCfg.ClientSecret == "" {
				azCfg.ClientSecret = sec["client_secret"]
			}
		}
	}
	if azCfg.TenantID != "" && azCfg.ClientID != "" && azCfg.ClientSecret != "" {
		svc.SetEntra(azure.New(azCfg, newLogger(cfg)))
	}
	// AI governance providers (MockAI + OpenAI/Anthropic governance), mirroring
	// cmd/api so `opord ai ...` resolves the same registry in-process.
	aiReg := aiproviders.NewRegistry()
	aimock.Register(aiReg)
	openai.Register(aiReg)
	anthropic.Register(aiReg)
	litellm.Register(aiReg)
	svc.SetAIProviders(aiReg)
	return svc, func() { pool.Close() }, nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
