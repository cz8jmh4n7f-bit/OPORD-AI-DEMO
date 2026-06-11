package orchestrator

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// ProviderReadiness is a static, fast guardrail check for the provider row. It
// does not create cloud resources; it tells the UI whether OPORD has enough
// config to provision safely and ingest FinOps data.
type ProviderReadiness struct {
	Provider    string
	Type        string
	Status      string
	Checks      []ProviderReadinessCheck
	NextActions []string
}

type ProviderReadinessCheck struct {
	ID      string
	Label   string
	Status  string // "ok" | "warn" | "failed"
	Message string
}

func (s *Service) ProviderReadiness(ctx context.Context, name string) (*ProviderReadiness, error) {
	p, err := s.q.GetProviderByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found: %w", name, err)
	}
	cfg := s.providerCfg(ctx, p)
	creds := map[string]string{}
	var credErr error
	if s.creds != nil {
		creds, credErr = s.creds.Resolve(ctx, p)
	}

	out := &ProviderReadiness{Provider: p.Name, Type: p.Type}
	switch p.Type {
	case "aws":
		out.Checks = append(out.Checks, awsReadinessChecks(p, cfg, creds, credErr)...)
	case "azure":
		out.Checks = append(out.Checks, azureReadinessChecks(p, cfg, creds, credErr)...)
	case "gcp":
		out.Checks = append(out.Checks, gcpReadinessChecks(p, cfg, creds, credErr)...)
	default:
		out.Checks = append(out.Checks,
			readinessCheck("identity", "Provider registration", "ok", "Provider is registered in OPORD."),
			readinessCheck("health", "Connection test", healthStatus(p), healthMessage(p)),
		)
	}
	out.Status = rollupReadiness(out.Checks)
	out.NextActions = readinessNextActions(out.Checks)
	return out, nil
}

func gcpReadinessChecks(p db.Provider, cfg map[string]any, creds map[string]string, credErr error) []ProviderReadinessCheck {
	finops := readMap(cfg, "finops")
	credStatus := credentialStatus(creds, credErr, "gcp_credentials")
	credMessage := credentialMessage(p, creds, credErr, "gcp_credentials")
	if credStatus != "ok" && p.LastCheckStatus == "ok" {
		credStatus = "ok"
		credMessage = "Last connection test succeeded; credentials resolve via OpenBao dynamic token, static key, or ADC."
	}
	return []ProviderReadinessCheck{
		readinessCheck("identity", "Provider registration", "ok", "Google Cloud provider is registered in OPORD."),
		readinessCheck("project", "Project ID", presentStatus(readString(cfg, "project_id")), presentMessage(readString(cfg, "project_id"), "GCP project_id is configured.", "Set project_id in provider config.")),
		readinessCheck("region", "Region", presentStatus(readString(cfg, "region")), presentMessage(readString(cfg, "region"), "Default GCP region is configured.", "Set a default GCP region such as europe-west1.")),
		readinessCheck("zone", "Zone", presentStatus(readString(cfg, "zone")), presentMessage(readString(cfg, "zone"), "Default GCP zone is configured.", "Set a default GCP zone such as europe-west1-b.")),
		readinessCheck("credentials", "Credentials", credStatus, credMessage),
		readinessCheck("health", "Connection test", healthStatus(p), healthMessage(p)),
		readinessCheck("safety", "Safety profile", safetyStatus(cfg), safetyMessage(cfg)),
		readinessCheck("finops", "FOCUS export", gcpFinopsStatus(finops), gcpFinopsMessage(finops)),
	}
}

func awsReadinessChecks(p db.Provider, cfg map[string]any, creds map[string]string, credErr error) []ProviderReadinessCheck {
	finops := readMap(cfg, "finops")
	return []ProviderReadinessCheck{
		readinessCheck("identity", "Provider registration", "ok", "AWS provider is registered in OPORD."),
		readinessCheck("region", "Region", presentStatus(readString(cfg, "region")), presentMessage(readString(cfg, "region"), "Region is configured.", "Set a default AWS region.")),
		readinessCheck("credentials", "Credentials", credentialStatus(creds, credErr, "access_key_id", "secret_access_key"), credentialMessage(p, creds, credErr, "access_key_id", "secret_access_key")),
		readinessCheck("health", "Connection test", healthStatus(p), healthMessage(p)),
		readinessCheck("safety", "Safety profile", safetyStatus(cfg), safetyMessage(cfg)),
		readinessCheck("finops", "FOCUS export", awsFinopsStatus(finops), awsFinopsMessage(finops)),
	}
}

func azureReadinessChecks(p db.Provider, cfg map[string]any, creds map[string]string, credErr error) []ProviderReadinessCheck {
	finops := readMap(cfg, "finops")
	return []ProviderReadinessCheck{
		readinessCheck("identity", "Provider registration", "ok", "Azure provider is registered in OPORD."),
		readinessCheck("subscription", "Subscription", presentStatus(readString(cfg, "subscription_id")), presentMessage(readString(cfg, "subscription_id"), "Subscription ID is configured.", "Set subscription_id in provider config.")),
		readinessCheck("location", "Location", presentStatus(readString(cfg, "location")), presentMessage(readString(cfg, "location"), "Default Azure location is configured.", "Set a default Azure location such as westeurope.")),
		readinessCheck("credentials", "Service principal", credentialStatus(creds, credErr, "tenant_id", "client_id", "client_secret"), credentialMessage(p, creds, credErr, "tenant_id", "client_id", "client_secret")),
		readinessCheck("health", "Connection test", healthStatus(p), healthMessage(p)),
		readinessCheck("safety", "Safety profile", safetyStatus(cfg), safetyMessage(cfg)),
		readinessCheck("finops", "FOCUS export", azureFinopsStatus(finops), azureFinopsMessage(finops)),
	}
}

func readinessCheck(id, label, status, message string) ProviderReadinessCheck {
	return ProviderReadinessCheck{ID: id, Label: label, Status: status, Message: message}
}

func readString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func readMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

func presentStatus(value string) string {
	if value == "" {
		return "failed"
	}
	return "ok"
}

func presentMessage(value, okMsg, missingMsg string) string {
	if value == "" {
		return missingMsg
	}
	return okMsg
}

func credentialStatus(creds map[string]string, err error, keys ...string) string {
	if err != nil {
		return "warn"
	}
	for _, key := range keys {
		if credentialValue(creds, key) == "" {
			return "warn"
		}
	}
	return "ok"
}

func credentialMessage(p db.Provider, creds map[string]string, err error, keys ...string) string {
	if err != nil {
		return "Credential resolver returned an error; run Test to see the live failure."
	}
	if creds["azure_creds_path"] != "" && creds["azure_dynamic_error"] != "" {
		return fmt.Sprintf("Azure dynamic credential path %q failed: %s", creds["azure_creds_path"], creds["azure_dynamic_error"])
	}
	for _, key := range keys {
		if credentialValue(creds, key) == "" {
			if p.SecretRef == "" {
				return "No OpenBao secret ref is set; OPORD will fall back to process environment credentials."
			}
			return "Secret ref is configured, but one or more expected credential keys are missing."
		}
	}
	return "Expected credential keys are available to OPORD."
}

func credentialValue(creds map[string]string, key string) string {
	for _, alias := range credentialAliases(key) {
		if creds[alias] != "" {
			return creds[alias]
		}
	}
	return ""
}

func credentialAliases(key string) []string {
	switch key {
	case "access_key_id":
		return []string{"access_key_id", "aws_access_key_id"}
	case "secret_access_key":
		return []string{"secret_access_key", "aws_secret_access_key"}
	case "tenant_id":
		return []string{"tenant_id", "arm_tenant_id", "azure_tenant_id", "tenant"}
	case "client_id":
		return []string{"client_id", "arm_client_id", "azure_client_id", "app_id", "appId", "application_id"}
	case "client_secret":
		return []string{"client_secret", "arm_client_secret", "azure_client_secret", "password", "secret"}
	case "gcp_credentials":
		return []string{"access_token", "credentials", "service_account_json", "google_credentials", "key", "json"}
	default:
		return []string{key}
	}
}

func healthStatus(p db.Provider) string {
	switch p.LastCheckStatus {
	case "ok":
		return "ok"
	case "failed":
		return "failed"
	case "unsupported":
		return "warn"
	default:
		return "warn"
	}
}

func healthMessage(p db.Provider) string {
	switch p.LastCheckStatus {
	case "ok":
		return "Last connection test succeeded."
	case "failed":
		if p.LastCheckMessage != "" {
			return p.LastCheckMessage
		}
		return "Last connection test failed."
	case "unsupported":
		return "Live connection test is not implemented for this provider type."
	default:
		return "Run Test once to confirm live cloud/API access."
	}
}

func safetyStatus(cfg map[string]any) string {
	if readString(cfg, "safety_profile") == "" {
		return "warn"
	}
	return "ok"
}

func safetyMessage(cfg map[string]any) string {
	switch readString(cfg, "safety_profile") {
	case "sandbox":
		return "Sandbox profile is explicit; resources stay easy to test and delete."
	case "dev":
		return "Dev profile is explicit; OPORD applies balanced defaults."
	case "prod":
		return "Prod profile is explicit; OPORD tightens public access and retention defaults."
	default:
		return "Set safety_profile to sandbox, dev, or prod."
	}
}

func awsFinopsStatus(finops map[string]any) string {
	if readString(finops, "s3_bucket") == "" || readString(finops, "s3_prefix") == "" {
		return "warn"
	}
	if readString(finops, "athena_database") == "" || readString(finops, "athena_table") == "" {
		return "warn"
	}
	return "ok"
}

func awsFinopsMessage(finops map[string]any) string {
	if readString(finops, "s3_bucket") == "" || readString(finops, "s3_prefix") == "" {
		return "Add AWS Data Export bucket and prefix for FOCUS ingestion."
	}
	if readString(finops, "athena_database") == "" || readString(finops, "athena_table") == "" {
		return "Add Athena database/table names after Glue/Athena is ready."
	}
	return "AWS FOCUS export metadata is configured."
}

func azureFinopsStatus(finops map[string]any) string {
	if readString(finops, "storage_account") == "" || readString(finops, "container") == "" || readString(finops, "directory") == "" {
		return "warn"
	}
	return "ok"
}

func azureFinopsMessage(finops map[string]any) string {
	if readString(finops, "storage_account") == "" || readString(finops, "container") == "" || readString(finops, "directory") == "" {
		return "Add Azure Cost Management FOCUS export destination: storage account, container, and directory."
	}
	return "Azure FOCUS export destination is configured."
}

func gcpFinopsStatus(finops map[string]any) string {
	if readString(finops, "bigquery_dataset") == "" || readString(finops, "focus_view") == "" {
		return "warn"
	}
	if readString(finops, "detailed_export_table") == "" || readString(finops, "pricing_export_table") == "" {
		return "warn"
	}
	return "ok"
}

func gcpFinopsMessage(finops map[string]any) string {
	if readString(finops, "bigquery_dataset") == "" || readString(finops, "focus_view") == "" {
		return "Add the BigQuery dataset and FOCUS view used by Google Cloud billing exports."
	}
	if readString(finops, "detailed_export_table") == "" && readString(finops, "pricing_export_table") == "" {
		return "Add Detailed Billing Export and Price Export table names for FOCUS mapping."
	}
	if readString(finops, "detailed_export_table") == "" {
		return "Add the Detailed Billing Export table name created in BigQuery."
	}
	if readString(finops, "pricing_export_table") == "" {
		return "Add the Price Export table name after Google Cloud creates it in BigQuery."
	}
	return "Google Cloud BigQuery FOCUS metadata is configured."
}

func rollupReadiness(checks []ProviderReadinessCheck) string {
	status := "ok"
	for _, check := range checks {
		if check.Status == "failed" {
			return "failed"
		}
		if check.Status == "warn" {
			status = "warn"
		}
	}
	return status
}

func readinessNextActions(checks []ProviderReadinessCheck) []string {
	out := []string{}
	for _, check := range checks {
		if check.Status != "ok" {
			out = append(out, check.Message)
		}
	}
	if len(out) == 0 {
		out = append(out, "Provider is ready for catalog provisioning and FinOps ingestion wiring.")
	}
	return out
}
