package models

// DNSSpec is the declarative desired state for a DNS zone (kind="dns") - an AWS
// Route53 hosted zone, optionally with records. Part of the expose-layer
// (ADR-0016): the zone is the anchor that ACM DNS-validation and the public
// record point at.
type DNSSpec struct {
	Name    string      `json:"name"`              // the domain, e.g. example.com
	Private bool        `json:"private,omitempty"` // private (VPC) zone vs public
	VPCID   string      `json:"vpc_id,omitempty"`  // required when Private
	Records []DNSRecord `json:"records,omitempty"` // optional records to create

	// Deploy target (ADR-0013): a OPORD-managed account to provision into.
	TargetAccount string `json:"target_account,omitempty"`
}

// DNSRecord is a single Route53 record in a zone.
type DNSRecord struct {
	Name  string `json:"name"`            // record name (e.g. app.example.com or @)
	Type  string `json:"type"`            // A, AAAA, CNAME, TXT, ...
	Value string `json:"value"`           // record value or alias target
	Alias bool   `json:"alias,omitempty"` // ALIAS record (to an ALB/CloudFront)
	TTL   int    `json:"ttl,omitempty"`   // non-alias TTL (default 300)
}
