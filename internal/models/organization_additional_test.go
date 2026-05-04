package models

import "testing"

func TestOrganizationAccessors(t *testing.T) {
	org := &Organization{
		ID:          "org-1",
		OwnerUserID: "owner",
		Members: []OrganizationMember{
			{UserID: "admin", Role: OrgRoleAdmin},
			{UserID: "member", Role: OrgRoleViewer},
		},
	}

	if !org.HasMember("admin") || org.HasMember("missing") {
		t.Fatalf("HasMember results unexpected")
	}
	if role := org.GetMemberRole("admin"); role != OrgRoleAdmin {
		t.Fatalf("GetMemberRole = %q, want admin", role)
	}
	if role := org.GetMemberRole("missing"); role != "" {
		t.Fatalf("GetMemberRole for missing = %q, want empty", role)
	}
	if !org.IsOwner("owner") || org.IsOwner("admin") {
		t.Fatalf("IsOwner results unexpected")
	}
	if !org.CanUserAccess("owner") || !org.CanUserAccess("member") || org.CanUserAccess("missing") {
		t.Fatalf("CanUserAccess results unexpected")
	}
	if !org.CanUserManage("owner") || !org.CanUserManage("admin") || org.CanUserManage("member") {
		t.Fatalf("CanUserManage results unexpected")
	}
}

func TestOrganizationPrincipalIdentityCanonicalization(t *testing.T) {
	org := &Organization{
		ID:          "org-1",
		OwnerUserID: "owner@example.com",
		Members: []OrganizationMember{
			{UserID: "owner@example.com", Role: OrgRoleOwner, AddedBy: "owner@example.com"},
			{UserID: "admin@example.com", Role: OrgRoleAdmin, AddedBy: "owner@example.com"},
		},
	}

	if role := org.GetMemberRoleForPrincipal("u_owner", "OWNER@example.com"); role != OrgRoleOwner {
		t.Fatalf("owner role = %q, want %q", role, OrgRoleOwner)
	}
	if !org.CanonicalizePrincipalIdentity("u_owner", "OWNER@example.com") {
		t.Fatal("expected owner identity to be canonicalized")
	}
	if org.OwnerUserID != "u_owner" || org.OwnerEmail != "owner@example.com" {
		t.Fatalf("owner identity = (%q, %q), want stable id + email", org.OwnerUserID, org.OwnerEmail)
	}
	if got := org.GetMemberRole("u_owner"); got != OrgRoleOwner {
		t.Fatalf("stable owner role = %q, want %q", got, OrgRoleOwner)
	}
	if got := org.GetMemberRole("owner@example.com"); got != OrgRoleOwner {
		t.Fatalf("legacy email owner role = %q, want %q", got, OrgRoleOwner)
	}
	if got := org.Members[0].Email; got != "owner@example.com" {
		t.Fatalf("member email = %q, want %q", got, "owner@example.com")
	}
}

func TestOrganizationStrictUserIDAccessRejectsContactEmail(t *testing.T) {
	org := &Organization{
		ID:          "org-1",
		OwnerUserID: "u_owner",
		OwnerEmail:  "owner@example.com",
		Members: []OrganizationMember{
			{UserID: "u_owner", Email: "owner@example.com", Role: OrgRoleOwner},
			{UserID: "u_admin", Email: "admin@example.com", Role: OrgRoleAdmin},
			{UserID: "u_viewer", Email: "viewer@example.com", Role: OrgRoleViewer},
		},
	}

	if !org.CanUserIDAccess("u_owner") || !org.CanUserIDManage("u_admin") {
		t.Fatal("strict user ID access should accept stored owner/admin principals")
	}
	if org.CanUserIDAccess("owner@example.com") || org.CanUserIDManage("admin@example.com") {
		t.Fatal("strict user ID access must not authorize contact email")
	}
	if got := org.GetMemberRoleByUserID("admin@example.com"); got != "" {
		t.Fatalf("strict member role for contact email = %q, want empty", got)
	}
	if got := org.GetMemberRole("admin@example.com"); got != OrgRoleAdmin {
		t.Fatalf("legacy member role for contact email = %q, want %q", got, OrgRoleAdmin)
	}
}

func TestOrganizationResolvePrincipalByEmail(t *testing.T) {
	org := &Organization{
		ID:          "org-1",
		OwnerUserID: "u_owner",
		OwnerEmail:  "owner@example.com",
		Members: []OrganizationMember{
			{UserID: "u_owner", Email: "owner@example.com", Role: OrgRoleOwner},
			{UserID: "u_admin", Email: "admin@example.com", Role: OrgRoleAdmin},
			{UserID: "legacy@example.com", Role: OrgRoleViewer},
		},
	}

	userID, role, ok := org.ResolvePrincipalByEmail("OWNER@example.com")
	if !ok || userID != "u_owner" || role != OrgRoleOwner {
		t.Fatalf("owner principal = (%q, %q, %v), want stable owner", userID, role, ok)
	}

	userID, role, ok = org.ResolvePrincipalByEmail("admin@example.com")
	if !ok || userID != "u_admin" || role != OrgRoleAdmin {
		t.Fatalf("member principal = (%q, %q, %v), want stable admin", userID, role, ok)
	}

	userID, role, ok = org.ResolvePrincipalByEmail("legacy@example.com")
	if !ok || userID != "legacy@example.com" || role != OrgRoleViewer {
		t.Fatalf("legacy principal = (%q, %q, %v), want legacy email fallback", userID, role, ok)
	}
}

func TestOrganizationResolvePrincipalByEmailRejectsBlankStoredPrincipal(t *testing.T) {
	ownerOnlyEmail := &Organization{
		ID:         "org-blank-owner",
		OwnerEmail: "owner@example.com",
	}
	userID, role, ok := ownerOnlyEmail.ResolvePrincipalByEmail("owner@example.com")
	if ok || userID != "" || role != "" {
		t.Fatalf("blank owner principal = (%q, %q, %v), want rejection", userID, role, ok)
	}

	memberOnlyEmail := &Organization{
		ID: "org-blank-member",
		Members: []OrganizationMember{
			{Email: "member@example.com", Role: OrgRoleViewer},
		},
	}
	userID, role, ok = memberOnlyEmail.ResolvePrincipalByEmail("member@example.com")
	if ok || userID != "" || role != "" {
		t.Fatalf("blank member principal = (%q, %q, %v), want rejection", userID, role, ok)
	}
}

func TestOrganizationRoleNormalization(t *testing.T) {
	if got := NormalizeOrganizationRole("member"); got != OrganizationRole("member") {
		t.Fatalf("expected member to remain unchanged, got %q", got)
	}
	if got := NormalizeOrganizationRole("EDITOR"); got != OrgRoleEditor {
		t.Fatalf("expected EDITOR to normalize to editor, got %q", got)
	}
	if !IsValidOrganizationRole(OrgRoleOwner) {
		t.Fatalf("expected owner role to be valid")
	}
	if IsValidOrganizationRole("member") {
		t.Fatalf("expected member to be invalid in v6")
	}
	if IsValidOrganizationRole("unknown") {
		t.Fatalf("expected unknown role to be invalid")
	}
}

func TestOrganizationRoleAtLeast(t *testing.T) {
	testCases := []struct {
		name     string
		actual   OrganizationRole
		required OrganizationRole
		want     bool
	}{
		{name: "viewer satisfies viewer", actual: OrgRoleViewer, required: OrgRoleViewer, want: true},
		{name: "viewer does not satisfy editor", actual: OrgRoleViewer, required: OrgRoleEditor, want: false},
		{name: "editor satisfies viewer", actual: OrgRoleEditor, required: OrgRoleViewer, want: true},
		{name: "editor satisfies editor", actual: OrgRoleEditor, required: OrgRoleEditor, want: true},
		{name: "editor does not satisfy admin", actual: OrgRoleEditor, required: OrgRoleAdmin, want: false},
		{name: "admin satisfies editor", actual: OrgRoleAdmin, required: OrgRoleEditor, want: true},
		{name: "owner satisfies admin", actual: OrgRoleOwner, required: OrgRoleAdmin, want: true},
		{name: "invalid actual is denied", actual: "member", required: OrgRoleViewer, want: false},
		{name: "invalid required is denied", actual: OrgRoleOwner, required: "member", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OrganizationRoleAtLeast(tc.actual, tc.required); got != tc.want {
				t.Fatalf("OrganizationRoleAtLeast(%q, %q) = %v, want %v", tc.actual, tc.required, got, tc.want)
			}
		})
	}
}

func TestNormalizeOrgStatus(t *testing.T) {
	testCases := []struct {
		name   string
		input  OrgStatus
		output OrgStatus
	}{
		{
			name:   "empty defaults to active",
			input:  "",
			output: OrgStatusActive,
		},
		{
			name:   "whitespace defaults to active",
			input:  "   ",
			output: OrgStatusActive,
		},
		{
			name:   "active case-insensitive",
			input:  "ACTIVE",
			output: OrgStatusActive,
		},
		{
			name:   "suspended case-insensitive",
			input:  "SUSPENDED",
			output: OrgStatusSuspended,
		},
		{
			name:   "pending deletion case-insensitive",
			input:  "Pending_Deletion",
			output: OrgStatusPendingDeletion,
		},
		{
			name:   "unknown trimmed and lowered",
			input:  "  CuStOm  ",
			output: "custom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeOrgStatus(tc.input)
			if got != tc.output {
				t.Fatalf("NormalizeOrgStatus(%q) = %q, want %q", tc.input, got, tc.output)
			}
		})
	}
}

func TestNormalizeOrganizationShareStatus(t *testing.T) {
	testCases := []struct {
		name   string
		input  OrganizationShareStatus
		output OrganizationShareStatus
		valid  bool
	}{
		{
			name:   "empty defaults to accepted",
			input:  "",
			output: OrganizationShareStatusAccepted,
			valid:  true,
		},
		{
			name:   "pending case-insensitive",
			input:  "PENDING",
			output: OrganizationShareStatusPending,
			valid:  true,
		},
		{
			name:   "accepted case-insensitive",
			input:  "Accepted",
			output: OrganizationShareStatusAccepted,
			valid:  true,
		},
		{
			name:   "unknown trimmed and lowered",
			input:  "  custom  ",
			output: "custom",
			valid:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeOrganizationShareStatus(tc.input)
			if got != tc.output {
				t.Fatalf("NormalizeOrganizationShareStatus(%q) = %q, want %q", tc.input, got, tc.output)
			}
			if valid := IsValidOrganizationShareStatus(tc.input); valid != tc.valid {
				t.Fatalf("IsValidOrganizationShareStatus(%q) = %v, want %v", tc.input, valid, tc.valid)
			}
		})
	}
}
