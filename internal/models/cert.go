package models

// CertSpec is the declarative desired state for a TLS certificate (kind="cert")
// - an AWS ACM certificate, DNS-validated. Part of the expose-layer (ADR-0016).
// Validation completes automatically when ValidationZoneID points at a Route53
// zone whose domain is delegated; otherwise the cert stays PENDING.
type CertSpec struct {
	Domain                  string   `json:"domain"`                              // primary domain, e.g. app.example.com
	SubjectAlternativeNames []string `json:"subject_alternative_names,omitempty"` // extra SANs
	ValidationZoneID        string   `json:"validation_zone_id,omitempty"`        // Route53 zone for automatic DNS validation
	// Region is the cert region. ACM certs are regional; a cert used by an ALB or
	// API Gateway lives in the resource's region, but a cert used by CloudFront
	// MUST be in us-east-1 (see ForCloudFront).
	Region string `json:"region,omitempty"`
	// ForCloudFront forces the cert into us-east-1 regardless of Region - required
	// for CloudFront viewer certificates.
	ForCloudFront bool `json:"for_cloudfront,omitempty"`

	TargetAccount string `json:"target_account,omitempty"`
}
