package models

// LoadBalancerSpec is the declarative desired state for a load balancer
// (kind="loadbalancer") - an AWS Application Load Balancer + listener(s) +
// target group. Part of the expose-layer (ADR-0016). VPC-bound: needs subnets
// (from the spec or, for deploy-into-member, discovered in the target account).
type LoadBalancerSpec struct {
	Name             string       `json:"name"`
	Internal         bool         `json:"internal,omitempty"`           // internal vs internet-facing
	SubnetIDs        []string     `json:"subnet_ids,omitempty"`         // ≥2 AZs; from spec or provider config
	SecurityGroupIDs []string     `json:"security_group_ids,omitempty"` // empty = auto VPC-CIDR-scoped SG
	Listeners        []LBListener `json:"listeners,omitempty"`          // empty = HTTP:80
	TargetType       string       `json:"target_type,omitempty"`        // instance | ip | lambda
	Targets          []string     `json:"targets,omitempty"`            // instance ids / ips / a lambda ARN
	HealthCheckPath  string       `json:"health_check_path,omitempty"`  // default "/"
	Region           string       `json:"region,omitempty"`

	TargetAccount string `json:"target_account,omitempty"`
}

// LBListener is a single ALB listener. A certificate_arn turns it into HTTPS.
type LBListener struct {
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"`                  // HTTP | HTTPS
	CertificateARN string `json:"certificate_arn,omitempty"` // required for HTTPS
}
