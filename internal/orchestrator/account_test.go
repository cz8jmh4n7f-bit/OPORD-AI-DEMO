package orchestrator

import (
	"strings"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
)

func TestValidateAccountSpec(t *testing.T) {
	base := models.AccountSpec{
		CSAID:     "alpha001",
		CloudName: "prod",
		Owner:     "alice",
		Email:     "aws+alpha001@example.com",
	}

	tests := []struct {
		name    string
		mutate  func(s *models.AccountSpec)
		wantErr bool
	}{
		{"valid", func(s *models.AccountSpec) {}, false},
		{"missing csa_id", func(s *models.AccountSpec) { s.CSAID = "" }, true},
		{"missing cloud_name", func(s *models.AccountSpec) { s.CloudName = "" }, true},
		{"missing email when creating", func(s *models.AccountSpec) { s.Email = "" }, true},
		{"bad email when creating", func(s *models.AccountSpec) { s.Email = "not-an-email" }, true},
		{
			"email not required for existing account",
			func(s *models.AccountSpec) {
				s.Email = ""
				s.AccountID = "111122223333"
				s.Skip.CreateAccount = true
			},
			false,
		},
		{
			"email not required when create skipped",
			func(s *models.AccountSpec) {
				s.Email = ""
				s.Skip.CreateAccount = true
			},
			false,
		},
		{
			"valid /22 cidr",
			func(s *models.AccountSpec) { s.CreateVPC = true; s.VPCCidr = "10.16.0.0/22" },
			false,
		},
		{
			"non-/22 cidr rejected",
			func(s *models.AccountSpec) { s.CreateVPC = true; s.VPCCidr = "10.16.0.0/24" },
			true,
		},
		{
			"empty cidr ok (IPAM allocates)",
			func(s *models.AccountSpec) { s.CreateVPC = true; s.VPCCidr = "" },
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			tc.mutate(&s)
			err := validateAccountSpec(s)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateAccountSpec() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateAccountSpec_Azure(t *testing.T) {
	azBase := func() models.AccountSpec {
		return models.AccountSpec{CSAID: "acme", CloudName: "dev"}
	}
	tests := []struct {
		name    string
		mutate  func(*models.AccountSpec)
		wantErr string // substring; "" = expect success
	}{
		{"adopt needs subscription_id",
			func(s *models.AccountSpec) { s.AzureMode = "adopt" }, "azure_subscription_id"},
		{"adopt with subscription_id ok",
			func(s *models.AccountSpec) { s.AzureMode = "adopt"; s.AzureSubscriptionID = "sub" }, ""},
		{"create needs billing_scope_id",
			func(s *models.AccountSpec) { s.AzureMode = "create" }, "azure_billing_scope_id"},
		{"create with billing scope ok",
			func(s *models.AccountSpec) {
				s.AzureMode = "create"
				s.AzureBillingScopeID = "/providers/Microsoft.Billing/…"
			}, ""},
		{"bad mode rejected",
			func(s *models.AccountSpec) { s.AzureMode = "borrow" }, "azure_mode must be"},
		{"vnet cidr must be /22",
			func(s *models.AccountSpec) {
				s.AzureMode = "adopt"
				s.AzureSubscriptionID = "sub"
				s.AzureVNetCIDR = "10.20.0.0/24"
			}, "azure_vnet_cidr must be a /22"},
		{"azure spec does NOT require AWS email",
			func(s *models.AccountSpec) { s.AzureMode = "adopt"; s.AzureSubscriptionID = "sub" }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := azBase()
			tc.mutate(&s)
			err := validateAccountSpec(s)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want success, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateAccountSpec_GCP(t *testing.T) {
	base := func() models.AccountSpec {
		return models.AccountSpec{CSAID: "acme", CloudName: "dev"}
	}
	tests := []struct {
		name    string
		mutate  func(*models.AccountSpec)
		wantErr string // substring; "" = expect success
	}{
		{"create mode ok (no email required)",
			func(s *models.AccountSpec) { s.GCPMode = "create" }, ""},
		{"adopt needs project_id",
			func(s *models.AccountSpec) { s.GCPMode = "adopt" }, "gcp_mode=adopt requires gcp_project_id"},
		{"adopt with project_id ok",
			func(s *models.AccountSpec) { s.GCPMode = "adopt"; s.GCPProjectID = "acme-dev" }, ""},
		{"bad mode rejected",
			func(s *models.AccountSpec) { s.GCPMode = "borrow" }, "gcp_mode must be"},
		{"gcp spec does NOT require AWS email",
			func(s *models.AccountSpec) { s.GCPMode = "create" }, ""},
		{"gcp vpc cidr still must be /22",
			func(s *models.AccountSpec) { s.GCPMode = "create"; s.CreateVPC = true; s.VPCCidr = "10.30.0.0/24" }, "vpc_cidr must be a /22"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.mutate(&s)
			err := validateAccountSpec(s)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want success, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}
