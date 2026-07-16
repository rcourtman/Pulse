package models

import "testing"

// TestBranchCovOrganizationRoleFromAccountRole exercises every switch arm of
// OrganizationRoleFromAccountRole, including the strings.ToLower /
// strings.TrimSpace normalization that gates the switch, and the default arm
// reached by empty, whitespace-only, and unknown inputs.
func TestBranchCovOrganizationRoleFromAccountRole(t *testing.T) {
	testCases := []struct {
		name string
		role string
		want OrganizationRole
	}{
		// Explicit named arms (canonical spelling).
		{name: "owner arm", role: "owner", want: OrgRoleOwner},
		{name: "admin arm", role: "admin", want: OrgRoleAdmin},
		{name: "tech arm maps to editor", role: "tech", want: OrgRoleEditor},
		{name: "read_only arm maps to viewer", role: "read_only", want: OrgRoleViewer},

		// ToLower normalization: each named arm must be reached case-insensitively.
		{name: "OWNER upper hits owner arm", role: "OWNER", want: OrgRoleOwner},
		{name: "Owner mixed hits owner arm", role: "Owner", want: OrgRoleOwner},
		{name: "ADMIN upper hits admin arm", role: "ADMIN", want: OrgRoleAdmin},
		{name: "Tech mixed hits tech arm", role: "Tech", want: OrgRoleEditor},
		{name: "READ_ONLY upper hits read_only arm", role: "READ_ONLY", want: OrgRoleViewer},

		// TrimSpace normalization: surrounding whitespace must not affect mapping.
		{name: "leading space owner", role: " owner", want: OrgRoleOwner},
		{name: "trailing space owner", role: "owner ", want: OrgRoleOwner},
		{name: "surrounding whitespace admin", role: "\tadmin\n", want: OrgRoleAdmin},
		{name: "surrounding whitespace read_only", role: "  read_only  ", want: OrgRoleViewer},

		// Combined case-folding + trimming for each arm.
		{name: "trimmed upper OWNER", role: "  OWNER  ", want: OrgRoleOwner},
		{name: "trimmed mixed Tech", role: " Tech ", want: OrgRoleEditor},

		// Default arm: empty, whitespace-only, and unknown inputs all fall
		// through to OrgRoleViewer.
		{name: "empty string default", role: "", want: OrgRoleViewer},
		{name: "whitespace-only default", role: "   \t\n", want: OrgRoleViewer},
		{name: "unknown role default", role: "billing", want: OrgRoleViewer},
		{name: "superuser unknown default", role: "superuser", want: OrgRoleViewer},

		// Boundary: "read_only" (with underscore) is a recognized arm, whereas
		// the visually similar "readonly" (no underscore) is NOT and must
		// fall through to the default arm. Both yield OrgRoleViewer, but they
		// exercise different branches.
		{name: "read_only underscore arm", role: "read_only", want: OrgRoleViewer},
		{name: "readonly no-underscore hits default", role: "readonly", want: OrgRoleViewer},

		// Sanity: the viewer-equivalent account roles are NOT OrgRoleOwner/Admin/Editor.
		{name: "read_only does not upgrade", role: "read_only", want: OrgRoleViewer},
		{name: "unknown does not upgrade", role: "root", want: OrgRoleViewer},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := OrganizationRoleFromAccountRole(tc.role)
			if got != tc.want {
				t.Fatalf("OrganizationRoleFromAccountRole(%q) = %q, want %q",
					tc.role, got, tc.want)
			}
		})
	}
}

// TestBranchCovOrganizationRoleFromAccountRole_DistinctArms verifies that the
// four non-default arms each return a distinct (or specifically viewer-bound)
// result, so coverage of the read_only -> viewer path is not conflated with
// the default -> viewer path. This guards against accidental arm removal.
func TestBranchCovOrganizationRoleFromAccountRole_DistinctArms(t *testing.T) {
	owner := OrganizationRoleFromAccountRole("owner")
	admin := OrganizationRoleFromAccountRole("admin")
	editor := OrganizationRoleFromAccountRole("tech")
	readOnly := OrganizationRoleFromAccountRole("read_only")
	def := OrganizationRoleFromAccountRole("totally-unknown")

	if owner != OrgRoleOwner {
		t.Fatalf("owner arm = %q, want %q", owner, OrgRoleOwner)
	}
	if admin != OrgRoleAdmin {
		t.Fatalf("admin arm = %q, want %q", admin, OrgRoleAdmin)
	}
	if editor != OrgRoleEditor {
		t.Fatalf("tech arm = %q, want %q", editor, OrgRoleEditor)
	}

	// Both read_only and default return OrgRoleViewer by design (read_only is
	// the least-privileged known account role; unknown roles are downgraded to
	// least privilege for safety). Assert they agree on the value but exercise
	// both arms.
	if readOnly != OrgRoleViewer {
		t.Fatalf("read_only arm = %q, want %q", readOnly, OrgRoleViewer)
	}
	if def != OrgRoleViewer {
		t.Fatalf("default arm = %q, want %q", def, OrgRoleViewer)
	}

	// owner/admin/editor must all be distinct from each other and from viewer.
	seen := map[OrganizationRole]string{}
	for label, r := range map[string]OrganizationRole{
		"owner": owner, "admin": admin, "editor": editor, "viewer(read_only)": readOnly,
	} {
		if prev, dup := seen[r]; dup {
			t.Fatalf("role collision: %q and %q both map to %q", prev, label, r)
		}
		seen[r] = label
	}
}
