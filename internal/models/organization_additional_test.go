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
