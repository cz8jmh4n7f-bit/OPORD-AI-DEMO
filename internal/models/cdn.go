package models

// CDNSpec is the declarative desired state for a CDN (kind="cdn") - an AWS
// CloudFront distribution. Part of the expose-layer (ADR-0016). Composes with
// the s3 primitive (static sites) or an ALB/API Gateway origin. The viewer
// certificate, when set, MUST be an ACM cert in us-east-1.
type CDNSpec struct {
	Name              string   `json:"name"`
	OriginType        string   `json:"origin_type,omitempty"`   // s3 | alb | apigw | custom
	OriginDomain      string   `json:"origin_domain,omitempty"` // origin domain name (bucket regional domain / ALB DNS / ...)
	Aliases           []string `json:"aliases,omitempty"`       // CNAMEs served (need the us-east-1 cert)
	CertificateARN    string   `json:"certificate_arn,omitempty"`
	DefaultRootObject string   `json:"default_root_object,omitempty"` // e.g. index.html
	PriceClass        string   `json:"price_class,omitempty"`         // PriceClass_100 | _200 | _All

	TargetAccount string `json:"target_account,omitempty"`
}
