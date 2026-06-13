package checks

import (
	"strconv"
	"strings"
)

// --- small, self-contained helpers for reading the parsed spec/observed maps ---

func mapStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func mapNum(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

func mapBool(m map[string]any, key string) (val, present bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok {
		return false, false
	}
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		return strings.EqualFold(b, "true"), true
	}
	return false, true
}

func subMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if sm, ok := m[key].(map[string]any); ok {
		return sm
	}
	return nil
}

// tagValue looks up a tagging key directly on the spec and inside common nested
// tag/label maps (tags, labels) - providers spread tags differently.
func tagValue(spec map[string]any, key string) string {
	if v := mapStr(spec, key); v != "" {
		return v
	}
	for _, nest := range []string{"tags", "labels"} {
		if v := mapStr(subMap(spec, nest), key); v != "" {
			return v
		}
	}
	return ""
}

// minorVersion parses a kubernetes version like "1.33" / "1.33.2-gke" to 133-ish
// comparable minor (major*100+minor). Returns ok=false when unparseable/empty.
func minorVersion(v string) (int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, false
	}
	maj, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	// the minor may carry a suffix (e.g. "33-gke") - take the leading digits.
	minDigits := ""
	for _, r := range strings.TrimSpace(parts[1]) {
		if r < '0' || r > '9' {
			break
		}
		minDigits += string(r)
	}
	min, err2 := strconv.Atoi(minDigits)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return maj*100 + min, true
}

// k8sFloor is the lowest kubernetes minor OPORD considers supported (1.31).
const k8sFloor = 1*100 + 31

// ttlKinds are resource kinds that accrue cost while running and therefore
// benefit from a TTL (ephemeral-by-default hygiene).
var ttlKinds = []string{"vm", "database", "table", "cache", "stack", "function"}

// BuiltinChecks is OPORD's default guardrail set (MVP). Every check evaluates
// purely from the stored spec/observed - no cloud calls.
func BuiltinChecks() []Check {
	return []Check{
		// --- tagging / allocation ---
		{
			ID:          "owner-tag",
			Title:       "Resource has an owner",
			Description: "Every resource should record an owner for accountability and cost allocation.",
			Category:    CategoryTagging,
			Severity:    SeverityWarning,
			Remediation: "Set an owner (or tags.owner) on the resource spec.",
			Eval: func(s Subject) (Status, string) {
				if tagValue(s.Spec, "owner") != "" {
					return StatusPass, "owner set"
				}
				return StatusFail, "no owner tag"
			},
		},
		{
			ID:          "cost-center-tag",
			Title:       "Resource has a cost center",
			Description: "A cost_center tag lets spend be charged back to a team or client.",
			Category:    CategoryTagging,
			Severity:    SeverityInfo,
			Remediation: "Set cost_center (or tags.cost_center) on the resource spec.",
			Eval: func(s Subject) (Status, string) {
				if tagValue(s.Spec, "cost_center") != "" {
					return StatusPass, "cost_center set"
				}
				return StatusFail, "no cost_center tag"
			},
		},
		// --- cost hygiene ---
		{
			ID:          "ttl-set",
			Title:       "Cost-bearing resource has a TTL",
			Description: "Ephemeral resources should carry a TTL so they auto-expire instead of accruing cost indefinitely.",
			Category:    CategoryCost,
			Severity:    SeverityInfo,
			Kinds:       ttlKinds,
			Remediation: "Set ttl_hours on the spec (OPORD's reaper auto-destroys on expiry).",
			Eval: func(s Subject) (Status, string) {
				if mapNum(s.Spec, "ttl_hours") > 0 {
					return StatusPass, "TTL set"
				}
				return StatusFail, "no TTL - runs until manually destroyed"
			},
		},
		{
			ID:          "account-budget",
			Title:       "Account has a budget",
			Description: "A landing-zone account should set a monthly budget so spend is capped and alerted.",
			Category:    CategoryCost,
			Severity:    SeverityWarning,
			Kinds:       []string{"account"},
			Remediation: "Set monthly_budget_usd on the account spec.",
			Eval: func(s Subject) (Status, string) {
				if mapNum(s.Spec, "monthly_budget_usd") > 0 {
					return StatusPass, "budget set"
				}
				return StatusFail, "no monthly budget"
			},
		},
		// --- security ---
		{
			ID:          "storage-not-public",
			Title:       "Object storage blocks public access",
			Description: "Buckets must block public access unless explicitly intended (the #1 cloud data-leak cause).",
			Category:    CategorySecurity,
			Severity:    SeverityCritical,
			Kinds:       []string{"object-storage", "s3"},
			Remediation: "Set block_public_access=true (or public=false) on the storage spec.",
			Eval: func(s Subject) (Status, string) {
				if pub, ok := mapBool(s.Spec, "public"); ok && pub {
					return StatusFail, "bucket is public"
				}
				if blk, ok := mapBool(s.Spec, "block_public_access"); ok && !blk {
					return StatusFail, "public access not blocked"
				}
				return StatusPass, "public access blocked"
			},
		},
		{
			ID:          "db-no-plaintext-password",
			Title:       "Database password not stored in plaintext",
			Description: "A managed-DB master password must not be exposed in OPORD's observed state (it belongs in OpenBao, IAM, or the cloud's secret manager).",
			Category:    CategorySecurity,
			Severity:    SeverityCritical,
			Kinds:       []string{"database"},
			Remediation: "Use auth_mode=iam, the RDS-managed master password, or the OpenBao password_secret pointer (OPORD strips it from observed).",
			Eval: func(s Subject) (Status, string) {
				for _, k := range []string{"password", "admin_password", "master_password", "db_password"} {
					if mapStr(s.Observed, k) != "" {
						return StatusFail, "plaintext password in observed state (" + k + ")"
					}
				}
				return StatusPass, "no plaintext password"
			},
		},
		{
			ID:          "account-default-vpcs-removed",
			Title:       "Default VPCs removed (AWS account)",
			Description: "A secure AWS account should have its default VPCs stripped in every region.",
			Category:    CategorySecurity,
			Severity:    SeverityWarning,
			Kinds:       []string{"account"},
			Remediation: "Leave skip.delete_default_vpcs unset (false) so the factory removes default VPCs.",
			Eval: func(s Subject) (Status, string) {
				// Only meaningful for AWS accounts.
				if mapStr(s.Spec, "azure_mode") != "" || mapStr(s.Spec, "gcp_mode") != "" {
					return StatusSkip, "not an AWS account"
				}
				if skip := subMap(s.Spec, "skip"); skip != nil {
					if v, ok := mapBool(skip, "delete_default_vpcs"); ok && v {
						return StatusFail, "default VPCs not removed"
					}
				}
				return StatusPass, "default VPCs removed"
			},
		},
		// --- reliability ---
		{
			ID:          "not-failed",
			Title:       "Resource is not in a failed state",
			Description: "A resource stuck in 'failed' needs attention - it may be partially provisioned or drifted.",
			Category:    CategoryReliability,
			Severity:    SeverityWarning,
			Remediation: "Inspect the resource's job logs; re-provision or destroy + recreate.",
			Eval: func(s Subject) (Status, string) {
				if s.Status == "failed" {
					return StatusFail, "resource is in failed state"
				}
				return StatusPass, "healthy state"
			},
		},
		{
			ID:          "cluster-k8s-supported",
			Title:       "Cluster runs a supported Kubernetes version",
			Description: "Kubernetes minors below the supported floor miss security patches and lose upstream support.",
			Category:    CategoryReliability,
			Severity:    SeverityWarning,
			Kinds:       []string{"cluster"},
			Remediation: "Upgrade the cluster to a current supported minor (>= 1.31).",
			Eval: func(s Subject) (Status, string) {
				v := mapStr(s.Spec, "kubernetes_version")
				mv, ok := minorVersion(v)
				if !ok {
					return StatusSkip, "version not pinned (provider default)"
				}
				if mv < k8sFloor {
					return StatusFail, "k8s " + v + " is below the supported floor (1.31)"
				}
				return StatusPass, "k8s " + v + " supported"
			},
		},
		// --- AI access governance (kind "ai-access") ---
		{
			ID:          "ai-access-has-owner",
			Title:       "AI access grant has an owner",
			Description: "Every AI access grant must name an owner so it can be attributed, reviewed, and renewed.",
			Category:    CategoryTagging,
			Severity:    SeverityWarning,
			Kinds:       []string{"ai-access"},
			Remediation: "Set an owner on the AI access request.",
			Eval: func(s Subject) (Status, string) {
				if strings.TrimSpace(mapStr(s.Observed, "owner")) != "" {
					return StatusPass, "owner set"
				}
				return StatusFail, "AI access has no owner"
			},
		},
		{
			ID:          "ai-access-has-expiry",
			Title:       "AI access grant has an expiry",
			Description: "Standing AI access should expire so it is re-justified periodically (SOC2/ISO access review).",
			Category:    CategoryGovernance,
			Severity:    SeverityWarning,
			Kinds:       []string{"ai-access"},
			Remediation: "Set expires_at on the AI access request; OPORD auto-revokes on expiry.",
			Eval: func(s Subject) (Status, string) {
				if _, present := mapBool(s.Observed, "has_expiry"); present {
					if v, _ := mapBool(s.Observed, "has_expiry"); v {
						return StatusPass, "expiry set"
					}
				}
				return StatusFail, "AI access never expires (no expiry date)"
			},
		},
		{
			ID:          "ai-access-not-overdue",
			Title:       "Active AI access is not past its expiry",
			Description: "An active grant past its expiry means the expiry reaper hasn't revoked it - access outliving its approved window.",
			Category:    CategoryGovernance,
			Severity:    SeverityCritical,
			Kinds:       []string{"ai-access"},
			Remediation: "Run the AI expiry reaper (or revoke the grant) - access should not outlive its expiry.",
			Eval: func(s Subject) (Status, string) {
				if overdue, present := mapBool(s.Observed, "overdue"); present && overdue {
					return StatusFail, "active AI access is past its expiry"
				}
				return StatusPass, "within approved window"
			},
		},
	}
}
