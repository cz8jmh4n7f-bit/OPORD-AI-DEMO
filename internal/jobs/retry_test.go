package jobs

import (
	"errors"
	"testing"
)

func TestIsPermanent(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"state lock is transient", errors.New("Error acquiring the state lock: ConditionalCheckFailedException"), false},
		{"throttling is transient", errors.New("operation error: ThrottlingException: rate exceeded"), false},
		{"timeout is transient", errors.New("dial tcp: i/o timeout"), false},
		{"precondition is permanent", errors.New("tofu plan failed (exit 1): Resource precondition failed"), true},
		{"invalid index is permanent", errors.New("tofu plan failed: Invalid index ... arns is empty list"), true},
		{"bad creds is permanent", errors.New("operation error STS: InvalidClientTokenId"), true},
		{"already exists is permanent", errors.New("EntityAlreadyExists: role already exists"), true},
		{"capability error is permanent", errors.New(`provider "vsphere" does not support managed databases`), true},
		{"spec validation is permanent", errors.New("invalid account spec: csa_id is required"), true},
		{"unknown error is retryable", errors.New("some unexpected boom"), false},
		{
			"state lock wins even with 'invalid' present",
			errors.New("Error acquiring the state lock; invalid prior state"),
			false,
		},
		{
			// Azure PG offer-restriction: the message contains "Try again in a
			// different location" - a transient-looking phrase - but it's a hard
			// permanent failure. hardPermanentHints must win over the transient
			// "try again" hint, else it loops 25x uselessly (observed live).
			"offer-restricted wins over 'try again'",
			errors.New(`LocationIsOfferRestricted: Subscriptions are restricted from provisioning in location 'westeurope'. Try again in a different location.`),
			true,
		},
		{"sku not available is permanent", errors.New("SkuNotAvailable: Standard_B1s ... Try another size"), true},
		{"aks sku not allowed is permanent", errors.New("BadRequest: The VM size of Standard_B2s is not allowed in your subscription"), true},
		{
			// PIM-eligible role assignment without Entra ID P2 - verified live: the
			// real azurerm error is exactly this string. Must be permanent so it
			// cancels at attempt 1 instead of retrying 25x.
			"pim without P2 license is permanent",
			errors.New(`listing Role Eligiblity Schedules: unexpected status 400 with error: AadPremiumLicenseRequired: The tenant needs to have Microsoft Entra ID P2 or Microsoft Entra ID Governance license.`),
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPermanent(tc.err); got != tc.want {
				t.Fatalf("isPermanent(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestCancelIfPermanent_passthrough(t *testing.T) {
	if cancelIfPermanent(nil) != nil {
		t.Fatal("nil should pass through as nil")
	}
	transient := errors.New("Error acquiring the state lock")
	if got := cancelIfPermanent(transient); got != transient {
		t.Fatalf("transient error should pass through unchanged, got %v", got)
	}
	// Permanent errors are wrapped by river.JobCancel (a different value).
	perm := errors.New("Resource precondition failed")
	if got := cancelIfPermanent(perm); got == perm {
		t.Fatal("permanent error should be wrapped (river.JobCancel), not returned as-is")
	}
}
