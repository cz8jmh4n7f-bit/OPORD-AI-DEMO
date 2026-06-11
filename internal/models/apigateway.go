package models

// APIGatewaySpec is the declarative desired state for an API Gateway
// (kind="apigateway") - an AWS API Gateway v2 HTTP API + integration + optional
// custom domain. Part of the expose-layer (ADR-0016). No VPC dependency, so it's
// the simplest "public HTTPS endpoint for a Lambda" primitive.
type APIGatewaySpec struct {
	Name              string `json:"name"`
	IntegrationType   string `json:"integration_type,omitempty"`   // lambda | http
	IntegrationTarget string `json:"integration_target,omitempty"` // Lambda ARN or upstream HTTP URL
	RouteKey          string `json:"route_key,omitempty"`          // default "$default" (catch-all proxy)
	// Optional custom domain: needs a cert (same region) + a hosted zone to alias.
	DomainName     string `json:"domain_name,omitempty"`
	CertificateARN string `json:"certificate_arn,omitempty"`
	HostedZoneID   string `json:"hosted_zone_id,omitempty"`
	Region         string `json:"region,omitempty"`

	TargetAccount string `json:"target_account,omitempty"`
}
