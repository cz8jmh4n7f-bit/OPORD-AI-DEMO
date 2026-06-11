package jobs

import (
	"strings"

	"github.com/riverqueue/river"
)

// River retries a failed job up to its default max attempts (25). That's right
// for TRANSIENT failures (tofu pg-backend state-lock races, throttling - VM/db
// provisioning relies on this) but wasteful for PERMANENT ones (a bad config, a
// failed precondition, invalid creds) that will never succeed. cancelIfPermanent
// classifies the error and, for clearly-permanent failures, returns
// river.JobCancel so the job stops immediately instead of burning 25 attempts.

// hardPermanentHints win over EVERYTHING, including transient hints. Some
// Azure errors embed transient-looking phrasing in a fundamentally permanent
// failure - e.g. LocationIsOfferRestricted says "Try again in a different
// location", which would otherwise match the "try again" transient hint and
// loop 25× uselessly. These are checked before the transient list.
var hardPermanentHints = []string{
	"locationisofferrestricted",
	"subscriptions are restricted from provisioning",
	"skunotavailable",
	"capacity restrictions",
	"is not allowed in your subscription",
	"k8sversionnotsupported",
	// PIM-eligible assignment without the required Microsoft Entra ID P2 license.
	"aadpremiumlicenserequired",
	"premium license",
	"p2 license",
}

// transientHints mark errors worth retrying. Checked after hardPermanentHints
// so a "tofu plan failed … Error acquiring the state lock" is treated as
// transient even though the outer message looks like a plan failure.
var transientHints = []string{
	"error acquiring the state lock",
	"conditionalcheckfailed",
	"throttl", // Throttling / ThrottlingException
	"requestlimitexceeded",
	"toomanyrequests",
	"serviceunavailable",
	"timeout",
	"i/o timeout",
	"connection refused",
	"try again",
	"deadline exceeded",
	// Azure AD app-secret propagation: a freshly-minted dynamic SP secret
	// (OpenBao Azure engine, ADR-0010) is eventually consistent across AAD
	// replicas, so the first tofu apply can hit a replica that doesn't have it
	// yet (AADSTS7000215 "Invalid client secret"). Transient - a retry re-mints +
	// waits, by which point it has propagated. Checked before the generic
	// "invalid" permanent hint so it retries instead of cancelling.
	"aadsts7000215",
}

// permanentHints mark errors that re-running won't fix (config / precondition /
// auth / quota / capability). NOTE: AccessDenied is deliberately NOT here - IAM
// is eventually consistent (e.g. AssumeRole right after CreateAccount), so it
// stays retryable.
var permanentHints = []string{
	"precondition failed",
	"invalid value for",
	"invalid function argument",
	"invalid index",
	"malformedpolicydocument",
	"validationexception",
	"validationerror",
	"entityalreadyexists",
	"already exists",
	"invalidclienttokenid",
	"signaturedoesnotmatch",
	"unrecognizedclientexception",
	"no iam identity center instance",
	"account_limit_exceeded",
	"constraintviolation",
	// RDS manage_master_user_password needs kms: perms (CreateGrant/Decrypt/
	// GenerateDataKey); without them the apply fails KMSKeyNotAccessibleFault and
	// won't recover on retry (the creds simply lack the permission) - cancel
	// instead of burning 25× churn (Finding B).
	"kmskeynotaccessiblefault",
	// Azure-specific permanent auth failures (azurerm provider).
	"unable to build authorizer",
	"executable file not found in $path",
	// Azure capacity restrictions: a SKU is unavailable in the chosen region.
	// Won't recover on retry; operator must pick a different size or region.
	"skunotavailable",
	"capacity restrictions",
	// Azure subscription-level restriction: this offer (e.g. Postgres Flexible
	// Server) is not enabled for this subscription in the chosen region.
	"locationisofferrestricted",
	"subscriptions are restricted from provisioning",
	"is not allowed in your subscription",
	// AKS k8s version retired or unavailable (use `az aks get-versions`).
	"k8sversionnotsupported",
	// GKE: the requested k8s version doesn't exist (too new / wrong) - won't recover
	// on retry. Check `gcloud container get-server-config`.
	"no valid versions with the prefix",
	// GCP (google provider) permanent config/auth failures: missing creds, an
	// API not enabled in the project, or billing not enabled. None recover on retry.
	"could not find default credentials",
	"accessnotconfigured",
	"has not been used in project",
	"billing account",
	"billingnotenabled",
	// GCP Cloud Functions/Build: the build service account lacks a required role
	// (common under a secure org policy). A retry can't grant IAM, so cancel.
	"missing permission on the build service account",
	// GKE into a governed project with no "default" network (OPORD now passes the
	// factory VPC explicitly; this guards misconfigured/old runs from churning).
	"has no network named",
	// Resource record purged from DB while a job was still queued: the job can
	// never load its target row, so retrying is pointless.
	"no rows in result set",
	"does not support", // provider capability error
	"is required",      // spec validation
	"invalid",          // generic tofu/var validation (after transient check)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// isPermanent reports whether an error is a clearly-permanent provisioning
// failure. Order: hard-permanent wins over everything; then transient signals
// win over soft-permanent; anything unrecognized is treated as retryable
// (preserving the prior default behavior).
func isPermanent(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if containsAny(msg, hardPermanentHints) {
		return true
	}
	if containsAny(msg, transientHints) {
		return false
	}
	return containsAny(msg, permanentHints)
}

// cancelIfPermanent wraps a worker result: permanent failures become
// river.JobCancel (no retries); transient/unknown errors pass through (retried).
func cancelIfPermanent(err error) error {
	if isPermanent(err) {
		return river.JobCancel(err)
	}
	return err
}
