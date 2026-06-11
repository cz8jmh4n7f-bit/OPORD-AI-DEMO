package aiproviders

import "testing"

// TestRoleComboAllowed locks in the org×workspace compatibility matrix established
// by live recon on 2026-06-12 (see ADR-0022). If Anthropic changes the rule this
// test should be revisited against fresh recon - it is the empirical contract.
func TestRoleComboAllowed(t *testing.T) {
	assignable := []WorkspaceRole{WSRoleUser, WSRoleDeveloper, WSRoleRestrictedDeveloper, WSRoleAdmin}

	// user / claude_code_user / developer accept every assignable workspace role.
	for _, org := range []OrgRole{OrgRoleUser, OrgRoleClaudeCodeUser, OrgRoleDeveloper} {
		for _, ws := range assignable {
			if !RoleComboAllowed(org, ws) {
				t.Errorf("RoleComboAllowed(%s, %s) = false, want true", org, ws)
			}
		}
	}

	// billing accepts ONLY workspace_admin (its workspace_billing is inherited).
	for _, ws := range assignable {
		want := ws == WSRoleAdmin
		if got := RoleComboAllowed(OrgRoleBilling, ws); got != want {
			t.Errorf("RoleComboAllowed(billing, %s) = %v, want %v", ws, got, want)
		}
	}

	// workspace_billing is inherited-only - never assignable to anyone.
	for _, org := range []OrgRole{OrgRoleUser, OrgRoleClaudeCodeUser, OrgRoleDeveloper, OrgRoleBilling} {
		if RoleComboAllowed(org, WSRoleBilling) {
			t.Errorf("RoleComboAllowed(%s, workspace_billing) = true, want false (inherited-only)", org)
		}
	}
}

func TestInheritedWorkspaceRole(t *testing.T) {
	cases := map[OrgRole]WorkspaceRole{
		OrgRoleAdmin:          WSRoleAdmin,   // admins inherit workspace_admin everywhere
		OrgRoleBilling:        WSRoleBilling, // billing inherits workspace_billing
		OrgRoleUser:           "",
		OrgRoleClaudeCodeUser: "",
		OrgRoleDeveloper:      "",
	}
	for org, want := range cases {
		if got := InheritedWorkspaceRole(org); got != want {
			t.Errorf("InheritedWorkspaceRole(%s) = %q, want %q", org, got, want)
		}
	}
}
