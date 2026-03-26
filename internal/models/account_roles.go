package models

import "strings"

// OrganizationRoleFromAccountRole maps control-plane commercial/account roles to
// tenant organization roles for hosted handoff and seeded tenant metadata.
func OrganizationRoleFromAccountRole(role string) OrganizationRole {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner":
		return OrgRoleOwner
	case "admin":
		return OrgRoleAdmin
	case "tech":
		return OrgRoleEditor
	case "read_only":
		return OrgRoleViewer
	default:
		return OrgRoleViewer
	}
}
