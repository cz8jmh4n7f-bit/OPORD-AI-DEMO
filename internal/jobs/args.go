// Package jobs defines OPORD's durable background work on top of River
// (Postgres-backed queue). Job args carry only identifiers; the worker reloads
// everything from the database so jobs survive process restarts.
package jobs

import "github.com/google/uuid"

// ProvisionVMArgs runs the Tofu apply for a standalone-VM resource.
type ProvisionVMArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionVMArgs) Kind() string { return "provision_vm" }

// DestroyVMArgs runs the Tofu destroy for a standalone-VM resource.
type DestroyVMArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyVMArgs) Kind() string { return "destroy_vm" }

// ProvisionClusterArgs runs the full cluster delivery (Tofu apply + Ansible).
type ProvisionClusterArgs struct {
	ClusterID uuid.UUID `json:"cluster_id"`
	JobID     uuid.UUID `json:"job_id"`
}

func (ProvisionClusterArgs) Kind() string { return "provision_cluster" }

// ScaleClusterArgs re-applies a cluster after its worker count changed. The work is
// identical to ProvisionClusterArgs (an idempotent tofu apply via ProvisionClusterByID),
// but a distinct kind so a SCALE shows as "scale_cluster" in the job queue instead of
// looking like a fresh provision.
type ScaleClusterArgs struct {
	ClusterID uuid.UUID `json:"cluster_id"`
	JobID     uuid.UUID `json:"job_id"`
}

func (ScaleClusterArgs) Kind() string { return "scale_cluster" }

// DestroyClusterArgs runs the Tofu destroy for a cluster.
type DestroyClusterArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyClusterArgs) Kind() string { return "destroy_cluster" }

// ProvisionDatabaseArgs runs the Tofu apply for a managed database (RDS).
type ProvisionDatabaseArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionDatabaseArgs) Kind() string { return "provision_database" }

// DestroyDatabaseArgs runs the Tofu destroy for a managed database.
type DestroyDatabaseArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyDatabaseArgs) Kind() string { return "destroy_database" }

// ProvisionStackArgs runs the Tofu apply for a generic stack (arbitrary module).
type ProvisionStackArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionStackArgs) Kind() string { return "provision_stack" }

// DestroyStackArgs runs the Tofu destroy for a generic stack.
type DestroyStackArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyStackArgs) Kind() string { return "destroy_stack" }

// ProvisionTableArgs runs the Tofu apply for a managed table (DynamoDB).
type ProvisionTableArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionTableArgs) Kind() string { return "provision_table" }

// DestroyTableArgs runs the Tofu destroy for a managed table.
type DestroyTableArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyTableArgs) Kind() string { return "destroy_table" }

// ProvisionFunctionArgs runs the Tofu apply for a serverless function (Lambda).
type ProvisionFunctionArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionFunctionArgs) Kind() string { return "provision_function" }

// DestroyFunctionArgs runs the Tofu destroy for a serverless function.
type DestroyFunctionArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyFunctionArgs) Kind() string { return "destroy_function" }

// ProvisionS3Args runs the Tofu apply for an object storage bucket.
type ProvisionS3Args struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionS3Args) Kind() string { return "provision_s3" }

// DestroyS3Args runs the Tofu destroy for an object storage bucket.
type DestroyS3Args struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyS3Args) Kind() string { return "destroy_s3" }

// ProvisionSecretArgs runs the Tofu apply for a managed secret.
type ProvisionSecretArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionSecretArgs) Kind() string { return "provision_secret" }

// DestroySecretArgs runs the Tofu destroy for a managed secret.
type DestroySecretArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroySecretArgs) Kind() string { return "destroy_secret" }

// ProvisionQueueArgs runs the Tofu apply for a message queue.
type ProvisionQueueArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionQueueArgs) Kind() string { return "provision_queue" }

// DestroyQueueArgs runs the Tofu destroy for a message queue.
type DestroyQueueArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyQueueArgs) Kind() string { return "destroy_queue" }

// ProvisionCacheArgs runs the Tofu apply for an in-memory cache.
type ProvisionCacheArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionCacheArgs) Kind() string { return "provision_cache" }

// DestroyCacheArgs runs the Tofu destroy for an in-memory cache.
type DestroyCacheArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyCacheArgs) Kind() string { return "destroy_cache" }

// ProvisionProjectArgs runs the Tofu apply for an access-vending project.
type ProvisionProjectArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionProjectArgs) Kind() string { return "provision_project" }

// DestroyProjectArgs runs the Tofu destroy for an access-vending project.
type DestroyProjectArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyProjectArgs) Kind() string { return "destroy_project" }

// ProvisionAccountArgs runs the layered Tofu apply for a member AWS account.
type ProvisionAccountArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionAccountArgs) Kind() string { return "provision_account" }

// DestroyAccountArgs runs the layered Tofu destroy for a member AWS account.
type DestroyAccountArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyAccountArgs) Kind() string { return "destroy_account" }

// ── Expose-layer jobs (ADR-0016): DNS / cert / loadbalancer / apigateway / cdn ──

// ProvisionDNSArgs runs the Tofu apply for a DNS zone (Route53).
type ProvisionDNSArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionDNSArgs) Kind() string { return "provision_dns" }

// DestroyDNSArgs runs the Tofu destroy for a DNS zone.
type DestroyDNSArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyDNSArgs) Kind() string { return "destroy_dns" }

// ProvisionCertArgs runs the Tofu apply for a TLS certificate (ACM).
type ProvisionCertArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionCertArgs) Kind() string { return "provision_cert" }

// DestroyCertArgs runs the Tofu destroy for a TLS certificate.
type DestroyCertArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyCertArgs) Kind() string { return "destroy_cert" }

// ProvisionLoadBalancerArgs runs the Tofu apply for a load balancer (ALB).
type ProvisionLoadBalancerArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionLoadBalancerArgs) Kind() string { return "provision_loadbalancer" }

// DestroyLoadBalancerArgs runs the Tofu destroy for a load balancer.
type DestroyLoadBalancerArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyLoadBalancerArgs) Kind() string { return "destroy_loadbalancer" }

// ProvisionAPIGatewayArgs runs the Tofu apply for an API gateway.
type ProvisionAPIGatewayArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionAPIGatewayArgs) Kind() string { return "provision_apigateway" }

// DestroyAPIGatewayArgs runs the Tofu destroy for an API gateway.
type DestroyAPIGatewayArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyAPIGatewayArgs) Kind() string { return "destroy_apigateway" }

// ProvisionCDNArgs runs the Tofu apply for a CDN (CloudFront).
type ProvisionCDNArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

func (ProvisionCDNArgs) Kind() string { return "provision_cdn" }

// DestroyCDNArgs runs the Tofu destroy for a CDN.
type DestroyCDNArgs struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

func (DestroyCDNArgs) Kind() string { return "destroy_cdn" }
