package models

import "testing"

func TestOrganizationAccessors(t *testing.T) {
	org := &Organization{
		ID:          "org-1",
		OwnerUserID: "owner",
		Members: []OrganizationMember{
			{UserID: "admin", Role: OrgRoleAdmin},
			{UserID: "member", Role: OrgRoleMember},
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
	if got := NormalizeOrganizationRole("member"); got != OrgRoleViewer {
		t.Fatalf("expected member alias to normalize to viewer, got %q", got)
	}
	if got := NormalizeOrganizationRole("EDITOR"); got != OrgRoleEditor {
		t.Fatalf("expected EDITOR to normalize to editor, got %q", got)
	}
	if !IsValidOrganizationRole(OrgRoleOwner) || !IsValidOrganizationRole("member") {
		t.Fatalf("expected owner and member alias to be valid roles")
	}
	if IsValidOrganizationRole("unknown") {
		t.Fatalf("expected unknown role to be invalid")
	}
}
