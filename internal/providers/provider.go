// Package providers defines the infrastructure-backend abstraction. A Provider
// is responsible only for Phase 1 of cluster delivery: provisioning compute and
// network and reporting the resulting nodes. Kubernetes bootstrap (Ansible) is a
// separate, provider-agnostic step that consumes a ProvisionResult.
//
// Adding a new backend (Proxmox, AWS, ...) must not require changes outside this
// package's subtree - see docs/adr/0002-provider-abstraction.md.
package providers

import (
	"context"
	"fmt"
	"sort"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
)

// Provider abstracts an infrastructure backend (vSphere, Proxmox, ...).
type Provider interface {
	// Type returns the backend identifier, matching models.ProviderType.
	Type() models.ProviderType

	// Validate checks a spec against this provider's constraints. No side effects.
	Validate(ctx context.Context, spec models.ClusterSpec) error

	// Preflight runs offline checks (module validity + var mapping) WITHOUT
	// contacting the backend's API. Safe to run before a provider lab exists.
	Preflight(ctx context.Context, req Request) (*PreflightResult, error)

	// Plan computes the changes required to reach the desired spec (e.g. tofu plan).
	Plan(ctx context.Context, req Request) (*PlanResult, error)

	// Provision reconciles infrastructure to match the spec (e.g. tofu apply).
	// Must be idempotent: safe to re-run against existing state.
	Provision(ctx context.Context, req Request) (*ProvisionResult, error)

	// Destroy tears down all infrastructure tracked by the request's workspace.
	Destroy(ctx context.Context, req Request) error
}

// Request carries everything a provider needs for a single operation. Secrets are
// resolved from Vault by the caller; providers never talk to Vault directly.
type Request struct {
	// Workspace isolates Tofu state for one cluster (one workspace == one cluster).
	Workspace string
	// Name is the cluster's friendly name (used by providers that name cloud
	// resources after it, e.g. the EKS cluster name).
	Name string
	Spec models.ClusterSpec
	// Credentials resolved from Vault (e.g. vsphere_user, vsphere_password).
	Credentials map[string]string
	// Config is the non-secret connection config (datacenter, datastore, network, ...).
	Config map[string]any
}

// PreflightResult summarizes an offline pre-flight check.
type PreflightResult struct {
	Workspace   string
	ModuleValid bool
	Summary     string
}

// PlanResult summarizes a dry-run.
type PlanResult struct {
	Summary    string // human-readable plan output
	HasChanges bool
}

// ProvisionResult is the Phase-1 output that feeds the Ansible bootstrap step.
type ProvisionResult struct {
	Nodes                []models.Node  `json:"nodes"`
	ControlPlaneEndpoint string         `json:"control_plane_endpoint"` // "host:port"
	AnsibleInventory     string         `json:"ansible_inventory"`      // rendered INI from the tofu module
	RawOutputs           map[string]any `json:"raw_outputs"`            // full `tofu output -json` for auditing

	// Managed marks a managed control plane (e.g. EKS): there is no SSH-based
	// kubeadm bootstrap, so the orchestrator skips Phase 2 (Ansible).
	Managed bool `json:"managed"`
	// Kubeconfig is how to reach the cluster once ready. For managed providers
	// this is typically an instruction (e.g. `aws eks update-kubeconfig ...`)
	// rather than a fetched file, since tokens are short-lived.
	Kubeconfig string `json:"kubeconfig"`
}

// LiveNode is a live VM reported by a provider's backend API.
type LiveNode struct {
	Name       string `json:"name"`
	PowerState string `json:"powerState"`
	IP         string `json:"ip"`
	NumCPU     int    `json:"numCpu"`
	MemoryMB   int    `json:"memoryMb"`
}

// Inspector is an OPTIONAL provider capability: reporting live VM state from the
// backend's API (e.g. the vSphere Web Services API). Callers type-assert for it.
type Inspector interface {
	InspectVMs(ctx context.Context, req Request) ([]LiveNode, error)
}

// Connectivity is an OPTIONAL provider capability: a live reachability +
// credential check against the backend (vCenter login, AWS STS
// GetCallerIdentity, Proxmox auth ticket). Used by provider health monitoring;
// callers type-assert for it and report "unsupported" when absent. The check is
// read-only - it must never mutate infrastructure.
type Connectivity interface {
	// CheckConnection verifies the backend is reachable and the credentials are
	// valid. Returns nil on success; a descriptive error otherwise.
	CheckConnection(ctx context.Context, creds map[string]string, config map[string]any) error
}

// Image is a bootable machine image (e.g. an AWS AMI) reported by a provider's
// API, so the UI can offer a picker instead of free-text IDs.
type Image struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	Arch        string `json:"arch"`
}

// ImageRequest carries what a provider needs to list bootable images. AWS reads
// credentials from the ambient chain (like tofu), so only Region/Owner matter.
type ImageRequest struct {
	Region      string
	Owner       string // "self" | "amazon" | account-id; the provider picks a default when empty
	Credentials map[string]string
	Config      map[string]any
}

// ImageLister is an OPTIONAL provider capability: list bootable images (e.g. AWS
// AMIs). Providers that boot from named templates (vSphere/Proxmox) don't
// implement it; callers type-assert and fall back to free-text entry.
type ImageLister interface {
	ListImages(ctx context.Context, req ImageRequest) ([]Image, error)
}

// ClusterVersionRequest carries what a provider needs to list the managed-k8s
// versions its cloud currently offers. Region/location matters (GKE versions vary
// by location); creds/config are read the same way as provisioning.
type ClusterVersionRequest struct {
	Region      string
	Credentials map[string]string
	Config      map[string]any
}

// ClusterVersionLister is an OPTIONAL provider capability: list the Kubernetes
// versions the managed control plane (GKE/AKS/EKS) offers RIGHT NOW, so the cluster
// form can show a live picker instead of free-text (which lets users type
// non-existent versions). The returned list is major.minor or full versions the
// service accepts as min_master_version; newest first. Providers without a managed
// k8s version API don't implement it; callers fall back to "(provider default)".
type ClusterVersionLister interface {
	ListClusterVersions(ctx context.Context, req ClusterVersionRequest) ([]string, error)
}

// ClusterReaper is an OPTIONAL provider capability: a durable destroy-guard for
// MANAGED clusters (GKE/EKS/AKS). A provision killed mid-apply can create the cloud
// cluster WITHOUT writing tofu state, so the subsequent `tofu destroy` is a silent
// no-op (empty state) and the orchestrator would wrongly mark the record destroyed
// while the real cluster keeps running - an orphan that costs money. After a destroy,
// the orchestrator calls ReapCluster to VERIFY the cluster is actually gone from the
// cloud (by name); it force-deletes an orphan it finds. It returns:
//   - nil when the cluster is gone (the normal case - tofu already removed it),
//   - a (retryable) error when an in-flight create/delete operation blocks the delete,
//     so the durable destroy job retries until the cloud confirms it gone.
//
// Providers without a managed-k8s control-plane API don't implement it (no-op).
type ClusterReaper interface {
	ReapCluster(ctx context.Context, req Request) error
}

// BillingScope is a place a new account/subscription can be billed to (e.g. an
// Azure MCA invoice section), so the account form can offer a picker instead of
// asking the user to paste a long resource-id URI.
type BillingScope struct {
	ID   string `json:"id"`   // the value the spec needs (e.g. the invoice-section resource id)
	Name string `json:"name"` // human label (e.g. "Billing account / Profile / Invoice section")
	Type string `json:"type"` // e.g. "invoiceSection"
}

// BillingScopeRequest carries what a provider needs to list billing scopes. The
// provider reads credentials/config the same way it does for provisioning.
type BillingScopeRequest struct {
	Credentials map[string]string
	Config      map[string]any
}

// BillingScopeLister is an OPTIONAL provider capability: list the billing scopes
// the credentials can create accounts/subscriptions under (Azure MCA invoice
// sections). Providers without a billing API (AWS/GCP/vSphere/Proxmox) don't
// implement it; callers type-assert and fall back to manual entry.
type BillingScopeLister interface {
	ListBillingScopes(ctx context.Context, req BillingScopeRequest) ([]BillingScope, error)
}

// VMRequest carries everything a provider needs to provision standalone VMs
// (the generic "vm" blueprint), independent of the k8s-shaped ClusterSpec.
type VMRequest struct {
	Workspace   string
	Spec        models.VMSpec
	Credentials map[string]string
	Config      map[string]any
}

// VMProvisioner is an OPTIONAL provider capability for standalone-VM
// provisioning against the provider's vm blueprint.
type VMProvisioner interface {
	// PreflightVM runs offline checks (module validity) without the backend API.
	PreflightVM(ctx context.Context, req VMRequest) error
	// ProvisionVM creates the VMs (tofu apply) and returns their identifiers.
	ProvisionVM(ctx context.Context, req VMRequest) (*VMResult, error)
	// DestroyVM tears down the VMs (tofu destroy) for the request's workspace.
	DestroyVM(ctx context.Context, req VMRequest) error
}

// VMResult is the output of provisioning standalone VMs.
type VMResult struct {
	Names      []string       `json:"names"`
	IDs        []string       `json:"ids"`
	PrivateIPs []string       `json:"private_ips"`
	PublicIPs  []string       `json:"public_ips"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// DBRequest carries everything a provider needs to provision a managed database
// (the "database" blueprint), independent of the k8s-shaped ClusterSpec.
type DBRequest struct {
	Workspace   string
	Name        string
	Spec        models.DatabaseSpec
	Credentials map[string]string
	Config      map[string]any
}

// DatabaseProvisioner is an OPTIONAL provider capability for managed databases
// (e.g. AWS RDS). Providers without a managed-database service don't implement it.
type DatabaseProvisioner interface {
	// PreflightDB runs offline checks (module validity) without the backend API.
	PreflightDB(ctx context.Context, req DBRequest) error
	// ProvisionDB creates the database (tofu apply) and returns its endpoint.
	ProvisionDB(ctx context.Context, req DBRequest) (*DBResult, error)
	// DestroyDB tears down the database (tofu destroy) for the request's workspace.
	DestroyDB(ctx context.Context, req DBRequest) error
}

// DBResult is the output of provisioning a managed database.
type DBResult struct {
	Endpoint string `json:"endpoint"`
	Port     int    `json:"port"`
	// Password is the generated master password (Cloud SQL / Azure Flexible Server
	// must set one at create time, unlike RDS's managed master password). json:"-"
	// so it is NEVER serialized into resources.observed; the orchestrator writes it
	// to the secrets store (OpenBao) and then drops it.
	Password   string         `json:"-"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// TableRequest carries everything a provider needs to provision a managed NoSQL
// table (the "table" blueprint - AWS DynamoDB today).
type TableRequest struct {
	Workspace   string
	Name        string
	Spec        models.TableSpec
	Credentials map[string]string
	Config      map[string]any
}

// TableProvisioner is an OPTIONAL provider capability for managed NoSQL tables
// (e.g. AWS DynamoDB). Providers without one don't implement it.
type TableProvisioner interface {
	// PreflightTable runs offline checks (module validity) without the backend API.
	PreflightTable(ctx context.Context, req TableRequest) error
	// ProvisionTable creates the table (tofu apply) and returns its identifiers.
	ProvisionTable(ctx context.Context, req TableRequest) (*TableResult, error)
	// DestroyTable tears down the table (tofu destroy) for the request's workspace.
	DestroyTable(ctx context.Context, req TableRequest) error
}

// TableResult is the output of provisioning a managed table.
type TableResult struct {
	ARN        string         `json:"arn"`
	Name       string         `json:"name"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// FunctionRequest carries everything a provider needs to provision a serverless
// function (the "function" blueprint - AWS Lambda today).
type FunctionRequest struct {
	Workspace   string
	Name        string
	Spec        models.FunctionSpec
	Credentials map[string]string
	Config      map[string]any
}

// FunctionProvisioner is an OPTIONAL provider capability for serverless functions
// (e.g. AWS Lambda). Providers without one don't implement it.
type FunctionProvisioner interface {
	// PreflightFunction runs offline checks (module validity) without the backend API.
	PreflightFunction(ctx context.Context, req FunctionRequest) error
	// ProvisionFunction creates the function (tofu apply) and returns its identifiers.
	ProvisionFunction(ctx context.Context, req FunctionRequest) (*FunctionResult, error)
	// DestroyFunction tears down the function (tofu destroy) for the request's workspace.
	DestroyFunction(ctx context.Context, req FunctionRequest) error
}

// FunctionResult is the output of provisioning a serverless function.
type FunctionResult struct {
	ARN        string         `json:"arn"`
	Name       string         `json:"name"`
	Runtime    string         `json:"runtime"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// S3Request carries everything a provider needs to provision object storage
// (the "s3" blueprint - AWS S3 today).
type S3Request struct {
	Workspace   string
	Name        string
	Spec        models.S3Spec
	Credentials map[string]string
	Config      map[string]any
}

// S3Provisioner is an OPTIONAL provider capability for object storage buckets.
// Providers without one don't implement it.
type S3Provisioner interface {
	// PreflightS3 runs offline checks (module validity) without the backend API.
	PreflightS3(ctx context.Context, req S3Request) error
	// ProvisionS3 creates the bucket (tofu apply) and returns its identifiers.
	ProvisionS3(ctx context.Context, req S3Request) (*S3Result, error)
	// DestroyS3 tears down the bucket (tofu destroy) for the request's workspace.
	DestroyS3(ctx context.Context, req S3Request) error
}

// S3Result is the output of provisioning an object storage bucket.
type S3Result struct {
	BucketID   string         `json:"bucket_id"`
	BucketARN  string         `json:"bucket_arn"`
	DomainName string         `json:"bucket_regional_domain_name"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// SecretRequest carries everything a provider needs to provision a managed
// secret (the "secret" blueprint - AWS Secrets Manager / Azure Key Vault).
type SecretRequest struct {
	Workspace   string
	Name        string
	Spec        models.SecretSpec
	Credentials map[string]string
	Config      map[string]any
}

// SecretProvisioner is an OPTIONAL provider capability for managed secrets.
// OPORD provisions the secret container only - the plaintext VALUE is set
// out-of-band (console / CLI / Vault-sync), so OPORD never holds credentials.
type SecretProvisioner interface {
	// PreflightSecret runs offline checks (module validity) without the backend API.
	PreflightSecret(ctx context.Context, req SecretRequest) error
	// ProvisionSecret creates the secret (tofu apply) and returns its identifiers.
	ProvisionSecret(ctx context.Context, req SecretRequest) (*SecretResult, error)
	// DestroySecret tears down the secret (tofu destroy) for the request's workspace.
	DestroySecret(ctx context.Context, req SecretRequest) error
}

// SecretResult is the output of provisioning a managed secret.
type SecretResult struct {
	SecretID   string         `json:"secret_id"`
	SecretARN  string         `json:"secret_arn"`
	Name       string         `json:"name"`
	URI        string         `json:"uri"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// QueueRequest carries everything a provider needs to provision a message
// queue (the "queue" blueprint - AWS SQS / Azure Service Bus).
type QueueRequest struct {
	Workspace   string
	Name        string
	Spec        models.QueueSpec
	Credentials map[string]string
	Config      map[string]any
}

// QueueProvisioner is an OPTIONAL provider capability for message queues.
// Providers without one don't implement it.
type QueueProvisioner interface {
	// PreflightQueue runs offline checks (module validity) without the backend API.
	PreflightQueue(ctx context.Context, req QueueRequest) error
	// ProvisionQueue creates the queue (tofu apply) and returns its identifiers.
	ProvisionQueue(ctx context.Context, req QueueRequest) (*QueueResult, error)
	// DestroyQueue tears down the queue (tofu destroy) for the request's workspace.
	DestroyQueue(ctx context.Context, req QueueRequest) error
}

// QueueResult is the output of provisioning a message queue.
type QueueResult struct {
	QueueURL   string         `json:"queue_url"`
	QueueARN   string         `json:"queue_arn"`
	Name       string         `json:"name"`
	DLQURL     string         `json:"dlq_url"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// CacheRequest carries everything a provider needs to provision an in-memory
// cache (the "cache" blueprint - AWS ElastiCache / Azure Cache for Redis).
type CacheRequest struct {
	Workspace   string
	Name        string
	Spec        models.CacheSpec
	Credentials map[string]string
	Config      map[string]any
}

// CacheProvisioner is an OPTIONAL provider capability for managed in-memory
// caches (Redis). Providers without one don't implement it.
type CacheProvisioner interface {
	// PreflightCache runs offline checks (module validity) without the backend API.
	PreflightCache(ctx context.Context, req CacheRequest) error
	// ProvisionCache creates the cache (tofu apply) and returns its endpoints.
	ProvisionCache(ctx context.Context, req CacheRequest) (*CacheResult, error)
	// DestroyCache tears down the cache (tofu destroy) for the request's workspace.
	DestroyCache(ctx context.Context, req CacheRequest) error
}

// CacheResult is the output of provisioning an in-memory cache. The access
// key / auth token is intentionally NOT included - OPORD never persists creds.
type CacheResult struct {
	PrimaryEndpoint string         `json:"primary_endpoint"`
	ReaderEndpoint  string         `json:"reader_endpoint"`
	Port            int            `json:"port"`
	ID              string         `json:"id"`
	RawOutputs      map[string]any `json:"raw_outputs"`
}

// ProjectRequest carries everything a provider needs to provision an
// access-vending project (the "project" blueprint - AWS IAM Identity Center today).
type ProjectRequest struct {
	Workspace   string
	Name        string
	Spec        models.ProjectSpec
	Credentials map[string]string
	Config      map[string]any
}

// ProjectProvisioner is an OPTIONAL provider capability for access-vending
// projects (e.g. AWS Identity Center). Providers without one don't implement it.
type ProjectProvisioner interface {
	// PreflightProject runs offline checks (module validity) without the backend API.
	PreflightProject(ctx context.Context, req ProjectRequest) error
	// ProvisionProject creates/updates the project (tofu apply) and returns its identifiers.
	ProvisionProject(ctx context.Context, req ProjectRequest) (*ProjectResult, error)
	// DestroyProject tears down the project (tofu destroy) for the request's workspace.
	DestroyProject(ctx context.Context, req ProjectRequest) error
}

// ProjectResult is the output of provisioning an access-vending project.
type ProjectResult struct {
	GroupID          string         `json:"group_id"`
	GroupName        string         `json:"group_name"`
	PermissionSetARN string         `json:"permission_set_arn"`
	AccountID        string         `json:"account_id"`
	MemberCount      int            `json:"member_count"`
	RawOutputs       map[string]any `json:"raw_outputs"`
}

// AccountRequest carries everything a provider needs to provision a member AWS
// account and its baseline layers (the "account" blueprint - AWS Organizations).
type AccountRequest struct {
	Workspace     string             // base workspace prefix; layers derive <prefix>-<layer>
	Name          string             // resource name
	Spec          models.AccountSpec // desired state
	Credentials   map[string]string  // master-account STS creds (from Vault AWS engine)
	Config        map[string]any     // provider config (region, default OU, SAML inputs)
	AllocatedCIDR string             // /22 from IPAM, set by the orchestrator when CreateVPC
}

// AccountProvisioner is an OPTIONAL provider capability for provisioning member
// accounts (AWS Organizations + cross-account baseline). The implementation
// sequences the layers (L1 create to L2-L6 via AssumeRole) and reports per-layer
// status. Providers without one don't implement it.
type AccountProvisioner interface {
	// PreflightAccount validates the spec + layer modules offline.
	PreflightAccount(ctx context.Context, req AccountRequest) error
	// ProvisionAccount creates the account and runs the enabled layers.
	ProvisionAccount(ctx context.Context, req AccountRequest) (*AccountResult, error)
	// DestroyAccount tears down the layers (and optionally closes the account).
	DestroyAccount(ctx context.Context, req AccountRequest) error
}

// AccountResult is the output of provisioning a member account.
type AccountResult struct {
	AccountID     string            `json:"account_id"`
	AccessRoleARN string            `json:"access_role_arn"`
	AllocatedCIDR string            `json:"allocated_cidr,omitempty"` // /22 from IPAM (for release on destroy)
	Layers        map[string]string `json:"layers"`                   // layer name -> status/summary
	RawOutputs    map[string]any    `json:"raw_outputs"`
}

// StackRequest carries everything a provider needs to run a generic OpenTofu
// stack (the "anything" blueprint).
type StackRequest struct {
	Workspace   string
	Name        string
	Spec        models.StackSpec
	Credentials map[string]string
	Config      map[string]any
}

// StackProvisioner is an OPTIONAL provider capability for running arbitrary
// OpenTofu root modules (provision any resource the provider's cloud supports).
type StackProvisioner interface {
	// PreflightStack validates the module offline (tofu validate, no backend).
	PreflightStack(ctx context.Context, req StackRequest) error
	// ProvisionStack runs tofu apply on the module and returns its outputs.
	ProvisionStack(ctx context.Context, req StackRequest) (*StackResult, error)
	// DestroyStack runs tofu destroy for the request's workspace.
	DestroyStack(ctx context.Context, req StackRequest) error
}

// StackResult is the output of a provisioned stack (its tofu outputs).
type StackResult struct {
	Outputs map[string]any `json:"outputs"`
}

// SnapshotRequest carries everything a provider needs to snapshot a database.
type SnapshotRequest struct {
	Workspace    string
	DBIdentifier string // source DB instance identifier
	SnapshotName string
	Credentials  map[string]string
	Config       map[string]any
}

// DatabaseSnapshotter is an OPTIONAL provider capability: take a point-in-time
// snapshot of a managed database (e.g. AWS RDS).
type DatabaseSnapshotter interface {
	SnapshotDB(ctx context.Context, req SnapshotRequest) (*SnapshotResult, error)
}

// SnapshotResult is the output of a database snapshot.
type SnapshotResult struct {
	SnapshotID string         `json:"snapshot_id"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// ── Expose-layer capabilities (ADR-0016): DNS / TLS / LB / API Gateway / CDN ──

// DNSRequest carries what a provider needs to provision a DNS zone (Route53).
type DNSRequest struct {
	Workspace   string
	Name        string
	Spec        models.DNSSpec
	Credentials map[string]string
	Config      map[string]any
}

// DNSProvisioner is an OPTIONAL provider capability for DNS zones (Route53).
type DNSProvisioner interface {
	PreflightDNS(ctx context.Context, req DNSRequest) error
	ProvisionDNS(ctx context.Context, req DNSRequest) (*DNSResult, error)
	DestroyDNS(ctx context.Context, req DNSRequest) error
}

// DNSResult is the output of provisioning a DNS zone.
type DNSResult struct {
	ZoneID      string         `json:"zone_id"`
	ZoneName    string         `json:"zone_name"`
	NameServers []string       `json:"name_servers"`
	RawOutputs  map[string]any `json:"raw_outputs"`
}

// CertRequest carries what a provider needs to provision a TLS certificate (ACM).
type CertRequest struct {
	Workspace   string
	Name        string
	Spec        models.CertSpec
	Credentials map[string]string
	Config      map[string]any
}

// CertProvisioner is an OPTIONAL provider capability for TLS certificates (ACM).
type CertProvisioner interface {
	PreflightCert(ctx context.Context, req CertRequest) error
	ProvisionCert(ctx context.Context, req CertRequest) (*CertResult, error)
	DestroyCert(ctx context.Context, req CertRequest) error
}

// CertResult is the output of provisioning a TLS certificate.
type CertResult struct {
	ARN        string         `json:"arn"`
	Domain     string         `json:"domain"`
	Status     string         `json:"status"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// LoadBalancerRequest carries what a provider needs to provision a load balancer.
type LoadBalancerRequest struct {
	Workspace   string
	Name        string
	Spec        models.LoadBalancerSpec
	Credentials map[string]string
	Config      map[string]any
}

// LoadBalancerProvisioner is an OPTIONAL provider capability for load balancers (ALB).
type LoadBalancerProvisioner interface {
	PreflightLoadBalancer(ctx context.Context, req LoadBalancerRequest) error
	ProvisionLoadBalancer(ctx context.Context, req LoadBalancerRequest) (*LoadBalancerResult, error)
	DestroyLoadBalancer(ctx context.Context, req LoadBalancerRequest) error
}

// LoadBalancerResult is the output of provisioning a load balancer.
type LoadBalancerResult struct {
	DNSName        string         `json:"dns_name"`
	ARN            string         `json:"arn"`
	ZoneID         string         `json:"zone_id"`
	TargetGroupARN string         `json:"target_group_arn"`
	RawOutputs     map[string]any `json:"raw_outputs"`
}

// APIGatewayRequest carries what a provider needs to provision an API Gateway.
type APIGatewayRequest struct {
	Workspace   string
	Name        string
	Spec        models.APIGatewaySpec
	Credentials map[string]string
	Config      map[string]any
}

// APIGatewayProvisioner is an OPTIONAL provider capability for API gateways.
type APIGatewayProvisioner interface {
	PreflightAPIGateway(ctx context.Context, req APIGatewayRequest) error
	ProvisionAPIGateway(ctx context.Context, req APIGatewayRequest) (*APIGatewayResult, error)
	DestroyAPIGateway(ctx context.Context, req APIGatewayRequest) error
}

// APIGatewayResult is the output of provisioning an API gateway.
type APIGatewayResult struct {
	Endpoint   string         `json:"endpoint"`
	APIID      string         `json:"api_id"`
	ARN        string         `json:"arn"`
	RawOutputs map[string]any `json:"raw_outputs"`
}

// CDNRequest carries what a provider needs to provision a CDN (CloudFront).
type CDNRequest struct {
	Workspace   string
	Name        string
	Spec        models.CDNSpec
	Credentials map[string]string
	Config      map[string]any
}

// CDNProvisioner is an OPTIONAL provider capability for CDNs (CloudFront).
type CDNProvisioner interface {
	PreflightCDN(ctx context.Context, req CDNRequest) error
	ProvisionCDN(ctx context.Context, req CDNRequest) (*CDNResult, error)
	DestroyCDN(ctx context.Context, req CDNRequest) error
}

// CDNResult is the output of provisioning a CDN.
type CDNResult struct {
	DomainName     string         `json:"domain_name"`
	DistributionID string         `json:"distribution_id"`
	ARN            string         `json:"arn"`
	HostedZoneID   string         `json:"hosted_zone_id"`
	RawOutputs     map[string]any `json:"raw_outputs"`
}

// ── FinOps: actual cloud spend ──

// CostQuery carries what a provider needs to report ACTUAL billed spend from the
// cloud's cost API. Read-only; never mutates anything.
type CostQuery struct {
	Days        int               // trailing window in days (provider clamps; default 30)
	Account     string            // filter to one linked account/project/subscription id ("" = all)
	Credentials map[string]string // resolved creds (need the cloud's cost-read permission)
	Config      map[string]any
}

// CostReporter is an OPTIONAL provider capability: report ACTUAL billed spend
// from the cloud's cost API (AWS Cost Explorer today) so FinOps shows real money
// - by account and service, a daily trend, a run-rate forecast, and anomalies -
// next to OPORD's own estimates. Providers without a cost API don't implement it;
// callers type-assert and fall back to estimates.
type CostReporter interface {
	ReportCost(ctx context.Context, q CostQuery) (*CostActuals, error)
}

// CostBucket is spend grouped by one key (a linked account, or a service).
type CostBucket struct {
	Key  string  `json:"key"`            // account id / service name
	Name string  `json:"name,omitempty"` // human label (e.g. the account name)
	USD  float64 `json:"usd"`
}

// CostPoint is one day of spend (for the trend chart).
type CostPoint struct {
	Date string  `json:"date"` // YYYY-MM-DD
	USD  float64 `json:"usd"`
}

// CostAnomaly flags a day whose spend is well above the trailing baseline.
type CostAnomaly struct {
	Date        string  `json:"date"`
	USD         float64 `json:"usd"`
	BaselineUSD float64 `json:"baseline_usd"`
	Factor      float64 `json:"factor"` // usd / baseline
}

// CostAccountRef is a distinct billed account seen in the window (for the filter).
type CostAccountRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CostActuals is the actual-spend report from a cloud cost API.
type CostActuals struct {
	Currency     string           `json:"currency"`
	WindowDays   int              `json:"window_days"`
	TotalUSD     float64          `json:"total_usd"`      // total over the window
	MTDUSD       float64          `json:"mtd_usd"`        // month-to-date actual
	ForecastUSD  float64          `json:"forecast_usd"`   // run-rate end-of-month projection
	DailyRunRate float64          `json:"daily_run_rate"` // recent avg daily spend
	// ByCloud is spend per provider/cloud. A single provider's ReportCost leaves it
	// nil; the orchestrator fills it when it merges actuals from several providers
	// (AWS + GCP + Azure) into one multi-cloud report.
	ByCloud   []CostBucket     `json:"by_cloud,omitempty"`
	ByAccount []CostBucket     `json:"by_account"`
	ByService []CostBucket     `json:"by_service"`
	Daily        []CostPoint      `json:"daily"`
	Anomalies    []CostAnomaly    `json:"anomalies"`
	Accounts     []CostAccountRef `json:"accounts"`
}

// Factory builds a Provider instance.
type Factory func() Provider

// Registry maps provider types to their factories.
type Registry struct {
	factories map[models.ProviderType]Factory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[models.ProviderType]Factory)}
}

// Register associates a provider type with a factory. It panics on duplicate
// registration, which can only happen via programmer error at startup.
func (r *Registry) Register(t models.ProviderType, f Factory) {
	if _, exists := r.factories[t]; exists {
		panic(fmt.Sprintf("providers: type %q already registered", t))
	}
	r.factories[t] = f
}

// Get instantiates the provider for the given type.
func (r *Registry) Get(t models.ProviderType) (Provider, error) {
	f, ok := r.factories[t]
	if !ok {
		return nil, fmt.Errorf("providers: no provider registered for type %q", t)
	}
	return f(), nil
}

// Types returns the registered provider types in sorted order.
func (r *Registry) Types() []models.ProviderType {
	types := make([]models.ProviderType, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}
