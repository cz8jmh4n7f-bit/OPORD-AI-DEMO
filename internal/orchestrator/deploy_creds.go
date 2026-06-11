package orchestrator

import (
	"context"
	"encoding/json"

	credspkg "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// targetAccountOf returns spec.target_account from a resource's raw spec JSON, or
// "" when absent - provider-neutral (works for any kind).
func targetAccountOf(r db.Resource) string {
	var m map[string]any
	if len(r.Spec) == 0 {
		return ""
	}
	if err := json.Unmarshal(r.Spec, &m); err != nil {
		return ""
	}
	s, _ := m["target_account"].(string)
	return s
}

// resolveDeployCreds resolves provider creds, switching to the assumed_role factory
// path when deploying into an AWS member account (the cross-account AssumeRole needs
// creds that can chain it - federation_token can't). No-op for non-AWS / no target.
func (s *Service) resolveDeployCreds(ctx context.Context, p db.Provider, targetAccount string) (map[string]string, error) {
	if p.Type == "aws" && targetAccount != "" {
		ctx = credspkg.WithFactoryCreds(ctx)
	}
	return s.creds.Resolve(ctx, p)
}

// resolveClusterCreds resolves credentials for a k8s cluster. An AWS cluster (EKS)
// ALWAYS uses the assumed_role factory creds - EKS creates IAM roles (cluster + node
// group), and federation_token credentials CANNOT call ANY IAM API operation (AWS
// rejects them with InvalidClientTokenId), so the plain catalog creds can never
// provision EKS. The assumed_role path (which the account factory + deploy-into-member
// use) can call IAM. Non-AWS providers resolve normally.
func (s *Service) resolveClusterCreds(ctx context.Context, p db.Provider, targetAccount string) (map[string]string, error) {
	if p.Type == "aws" {
		ctx = credspkg.WithFactoryCreds(ctx)
	}
	return s.creds.Resolve(ctx, p)
}

// resolveFunctionCreds resolves credentials for a serverless function (AWS Lambda).
// Like an EKS cluster, a Lambda creates an IAM execution role, and federation_token
// credentials CANNOT call ANY IAM API operation (AWS rejects them with
// InvalidClientTokenId) - so an AWS function fails in the provider's OWN account on
// the catalog's least-privilege federation_token creds (Finding D). It ALWAYS uses
// the assumed_role factory creds (which can call IAM), whether deploying into the
// provider's own account or a member account (the provider then chains AssumeRole
// for a deploy target). Non-AWS providers resolve normally.
func (s *Service) resolveFunctionCreds(ctx context.Context, p db.Provider) (map[string]string, error) {
	if p.Type == "aws" {
		ctx = credspkg.WithFactoryCreds(ctx)
	}
	return s.creds.Resolve(ctx, p)
}

// resolveProjectCreds resolves credentials for an access-vending project. On AWS
// (IAM Identity Center) provisioning calls SSO-Admin + Identity Store APIs, which
// the catalog's least-privilege federation_token creds aren't granted (Finding F:
// "sso:ListInstances AccessDenied"). It uses the assumed_role factory creds (which
// carry the governance perms). Non-AWS providers resolve normally.
func (s *Service) resolveProjectCreds(ctx context.Context, p db.Provider) (map[string]string, error) {
	if p.Type == "aws" {
		ctx = credspkg.WithFactoryCreds(ctx)
	}
	return s.creds.Resolve(ctx, p)
}
