// Package orchestrator holds OPORD's cluster lifecycle logic. It is the single
// reusable home for provisioning behavior so that the CLI now, and the HTTP API
// later, both drive clusters through the same code path (no duplicated logic).
package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/azure"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/events"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// CredentialResolver fetches a provider's credentials (env-backed today,
// Vault-backed later via provider.SecretRef).
type CredentialResolver interface {
	Resolve(ctx context.Context, p db.Provider) (map[string]string, error)
	ResolveConfig(ctx context.Context, p db.Provider) (map[string]any, error)
}

// Enqueuer hands long-running work to a durable job queue (River). It is
// implemented in internal/jobs; the orchestrator only depends on this interface
// so there is no import cycle. When no enqueuer is set, the Service falls back
// to running the work in an in-process goroutine (fine for the CLI).
type Enqueuer interface {
	EnqueueProvisionVM(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyVM(ctx context.Context, name, env string) error
	EnqueueProvisionCluster(ctx context.Context, clusterID, jobID uuid.UUID) error
	EnqueueScaleCluster(ctx context.Context, clusterID, jobID uuid.UUID) error
	EnqueueDestroyCluster(ctx context.Context, name, env string) error
	EnqueueProvisionDatabase(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyDatabase(ctx context.Context, name, env string) error
	EnqueueProvisionStack(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyStack(ctx context.Context, name, env string) error
	EnqueueProvisionTable(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyTable(ctx context.Context, name, env string) error
	EnqueueProvisionFunction(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyFunction(ctx context.Context, name, env string) error
	EnqueueProvisionS3(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyS3(ctx context.Context, name, env string) error
	EnqueueProvisionSecret(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroySecret(ctx context.Context, name, env string) error
	EnqueueProvisionQueue(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyQueue(ctx context.Context, name, env string) error
	EnqueueProvisionCache(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyCache(ctx context.Context, name, env string) error
	EnqueueProvisionProject(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyProject(ctx context.Context, name, env string) error
	EnqueueProvisionAccount(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyAccount(ctx context.Context, name, env string) error
	EnqueueProvisionDNS(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyDNS(ctx context.Context, name, env string) error
	EnqueueProvisionCert(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyCert(ctx context.Context, name, env string) error
	EnqueueProvisionLoadBalancer(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyLoadBalancer(ctx context.Context, name, env string) error
	EnqueueProvisionAPIGateway(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyAPIGateway(ctx context.Context, name, env string) error
	EnqueueProvisionCDN(ctx context.Context, resourceID uuid.UUID) error
	EnqueueDestroyCDN(ctx context.Context, name, env string) error
}

// Allocator hands out and releases CIDR blocks (implemented by *ipam.Pool).
// Optional - only the account factory's secure-VPC layer needs it.
type Allocator interface {
	Allocate(ctx context.Context, owner string) (string, error)
	Release(ctx context.Context, owner string) error
}

// BootstrapConfig configures the provider-agnostic Phase 2 (Kubernetes
// bootstrap via Ansible). It is consumed only by the cluster provision flow;
// standalone VMs ignore it.
type BootstrapConfig struct {
	AnsibleBin    string // ansible-playbook binary (empty => "ansible-playbook")
	AnsibleDir    string // dir holding site.yml + ansible.cfg + roles
	SSHPrivateKey string // path to the SSH key used to reach the nodes
	ArtifactsDir  string // where fetched kubeconfigs are written (empty => os.TempDir)
}

// Service coordinates the cluster lifecycle over the database, the provider
// registry, and credential resolution.
type Service struct {
	q         db.Querier
	registry  *providers.Registry
	creds     CredentialResolver
	log       *slog.Logger
	bootstrap BootstrapConfig
	enqueuer  Enqueuer
	events    *events.Bus
	ticketer  Ticketer
	ipam      Allocator
	entra     *azure.Client
	ai        *aiproviders.Registry
}

// New constructs a Service. A nil logger defaults to slog.Default(). The
// bootstrap config is only needed for live cluster provisioning (Phase 2);
// pass a zero value when only VMs/dry-runs are used.
func New(q db.Querier, registry *providers.Registry, creds CredentialResolver, log *slog.Logger, bootstrap BootstrapConfig) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{q: q, registry: registry, creds: creds, log: log, bootstrap: bootstrap}
}

// SetEnqueuer wires a durable job queue. With it set, async operations enqueue
// River jobs (executed by the worker process); without it they run in-process.
// Called after construction to avoid an import cycle (jobs depends on Service).
func (s *Service) SetEnqueuer(e Enqueuer) { s.enqueuer = e }

// SetEvents wires the connector bus (Slack/SIEM/CMDB notifications). Optional.
func (s *Service) SetEvents(b *events.Bus) { s.events = b }

// Ticketer opens an ITSM ticket (GLPI) for a self-service request. Implemented
// by *glpi.Client; optional (requests work without it, just no ticket).
type Ticketer interface {
	CreateTicket(ctx context.Context, title, content string) (string, error)
}

// SetTicketer wires the ITSM ticket backend used by the request workflow.
func (s *Service) SetTicketer(t Ticketer) { s.ticketer = t }

// SetAllocator wires the CIDR allocator (Vault-backed IPAM) used by the account
// factory's secure-VPC layer. Optional - without it, accounts must supply an
// explicit vpc_cidr (or skip the VPC).
func (s *Service) SetAllocator(a Allocator) { s.ipam = a }

// SetEntra wires the Microsoft Graph client used to automate the Entra side of
// AWS SAML federation (app-role + user assignment). Optional - without it,
// GrantEntraAccess returns a clear "not configured" error and the Entra side
// stays a manual Azure Portal step.
func (s *Service) SetEntra(c *azure.Client) { s.entra = c }

// SetAIProviders wires the AI provider registry. It is separate from the
// infrastructure provider registry by design: AI access is not OpenTofu-backed
// infrastructure provisioning.
func (s *Service) SetAIProviders(r *aiproviders.Registry) { s.ai = r }

// providerCfg returns a provider's effective config: the DB config merged with
// any non-credential keys stored at its Vault SecretRef (region, subnet_ids,
// ou_id, …). Vault is the source of truth - its keys override the DB config -
// so an admin changes provider parameters in one place (Vault). Providers
// without a Vault SecretRef just use the DB config.
func (s *Service) providerCfg(ctx context.Context, p db.Provider) map[string]any {
	cfg := map[string]any{}
	_ = json.Unmarshal(p.Config, &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	if s.creds != nil {
		if vc, err := s.creds.ResolveConfig(ctx, p); err == nil {
			for k, v := range vc {
				cfg[k] = v
			}
		}
	}
	return cfg
}

// emit publishes a lifecycle event to the connector bus (no-op if unset).
func (s *Service) emit(kind, action, name, env, provider, message string) {
	s.events.Publish(events.Event{
		Kind: kind, Action: action, Name: name,
		Environment: env, Provider: provider, Message: message,
	})
}
