package azure

import (
	"strings"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
)

func TestArchiveStorageAccountID(t *testing.T) {
	tests := []struct {
		name    string
		sub     string
		csa     string
		wantSA  string // the storage-account name segment we expect in the id
		wantNil bool
	}{
		{"simple", "sub-1", "acme", "opordacmelogssa", false},
		{"hyphens stripped", "sub-1", "acme-prod", "opordacmeprodlogssa", false},
		{
			// opord + a long csa + logssa, capped to 24 chars.
			"capped to 24", "sub-1", "verylongcustomername", "opordverylongcustomernam", false,
		},
		{"empty sub", "", "acme", "", true},
		{"empty csa", "sub-1", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := archiveStorageAccountID(tc.sub, tc.csa)
			if tc.wantNil {
				if got != "" {
					t.Fatalf("want empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, "/subscriptions/"+tc.sub+"/") {
				t.Errorf("id missing subscription: %q", got)
			}
			if !strings.Contains(got, "opord-"+tc.csa+"-logs-rg") {
				t.Errorf("id missing logs RG: %q", got)
			}
			// SA name segment is the last path element.
			parts := strings.Split(got, "/")
			saName := parts[len(parts)-1]
			if len(saName) > 24 {
				t.Errorf("SA name %q exceeds 24 chars", saName)
			}
			if saName != tc.wantSA {
				t.Errorf("SA name = %q, want %q", saName, tc.wantSA)
			}
		})
	}
}

func TestTagsForAzureAccount(t *testing.T) {
	tags := tagsForAzureAccount(models.AccountSpec{
		CSAID:     "acme",
		CloudName: "prod",
		Owner:     "alice",
	})
	for _, k := range []string{"Project", "CsaId", "Cloud", "Owner", "ManagedBy"} {
		if _, ok := tags[k]; !ok {
			t.Errorf("missing tag %q", k)
		}
	}
	if tags["ManagedBy"] != "opord" {
		t.Errorf("ManagedBy = %q, want opord", tags["ManagedBy"])
	}
	if tags["CsaId"] != "acme" {
		t.Errorf("CsaId = %q, want acme", tags["CsaId"])
	}
}

func TestWorkspaceName(t *testing.T) {
	if got := workspaceName("ws-uuid", "l4"); got != "ws-uuid-l4" {
		t.Fatalf("workspaceName = %q, want ws-uuid-l4", got)
	}
}
