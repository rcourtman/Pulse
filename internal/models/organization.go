package models

import (
	"strings"
	"time"
)

// OrganizationRole represents a user's role within an organization.
type OrganizationRole string

const (
	// OrgRoleOwner has full access and can manage all aspects of the organization.
	OrgRoleOwner OrganizationRole = "owner"
	// OrgRoleAdmin can manage resources but cannot delete the organization.
	OrgRoleAdmin OrganizationRole = "admin"
	// OrgRoleEditor can update organization resources but cannot manage members.
	OrgRoleEditor OrganizationRole = "editor"
	// OrgRoleViewer has read-only access to organization resources.
	OrgRoleViewer OrganizationRole = "viewer"
	// OrgRoleMember is a legacy alias kept for backward compatibility.
	OrgRoleMember OrganizationRole = OrgRoleViewer
)

// NormalizeOrganizationRole canonicalizes role values and maps legacy aliases.
func NormalizeOrganizationRole(role OrganizationRole) OrganizationRole {
	switch strings.ToLower(strings.TrimSpace(string(role))) {
	case string(OrgRoleOwner):
		return OrgRoleOwner
	case string(OrgRoleAdmin):
		return OrgRoleAdmin
	case string(OrgRoleEditor):
		return OrgRoleEditor
	case string(OrgRoleViewer), "member":
		return OrgRoleViewer
	default:
		return OrganizationRole(strings.ToLower(strings.TrimSpace(string(role))))
	}
}

// IsValidOrganizationRole reports whether the role is a known organization role.
func IsValidOrganizationRole(role OrganizationRole) bool {
	switch NormalizeOrganizationRole(role) {
	case OrgRoleOwner, OrgRoleAdmin, OrgRoleEditor, OrgRoleViewer:
		return true
	default:
		return false
	}
}

// OrganizationMember represents a user's membership in an organization.
type OrganizationMember struct {
	// UserID is the unique identifier of the member.
	UserID string `json:"userId"`

	// Role is the member's role within the organization.
	Role OrganizationRole `json:"role"`

	// AddedAt is when the member was added to the organization.
	AddedAt time.Time `json:"addedAt"`

	// AddedBy is the user ID of who added this member (empty for owner).
	AddedBy string `json:"addedBy,omitempty"`
}

// OrganizationShare represents a cross-organization resource/view share.
type OrganizationShare struct {
	// ID is a unique identifier for the share record.
	ID string `json:"id"`

	// TargetOrgID is the organization receiving access.
	TargetOrgID string `json:"targetOrgId"`

	// ResourceType identifies what is being shared (e.g. "resource", "view").
	ResourceType string `json:"resourceType"`

	// ResourceID is the stable identifier of the shared resource/view.
	ResourceID string `json:"resourceId"`

	// ResourceName is an optional display label used by the UI.
	ResourceName string `json:"resourceName,omitempty"`

	// AccessRole defines the access level granted to the target organization.
	AccessRole OrganizationRole `json:"accessRole"`

	// CreatedAt is when the share was created.
	CreatedAt time.Time `json:"createdAt"`

	// CreatedBy is the user ID that created the share.
	CreatedBy string `json:"createdBy"`
}

// Organization represents a distinct tenant in the system.
type Organization struct {
	// ID is the unique identifier for the organization (e.g., "customer-a").
	// It is used as the directory name for data isolation.
	ID string `json:"id"`

	// DisplayName is the human-readable name of the organization.
	DisplayName string `json:"displayName"`

	// CreatedAt is when the organization was registered.
	CreatedAt time.Time `json:"createdAt"`

	// OwnerUserID is the primary owner of this organization.
	// The owner has full administrative rights and cannot be removed.
	OwnerUserID string `json:"ownerUserId,omitempty"`

	// Members is the list of users who have access to this organization.
	// This includes the owner (with OrgRoleOwner) and any additional members.
	Members []OrganizationMember `json:"members,omitempty"`

	// SharedResources contains outgoing cross-organization shares.
	SharedResources []OrganizationShare `json:"sharedResources,omitempty"`

	// EncryptionKeyID refers to the specific encryption key used for this org's data
	// (Future proofing for per-tenant encryption keys)
	EncryptionKeyID string `json:"encryptionKeyId,omitempty"`
}

// HasMember checks if a user is a member of the organization.
func (o *Organization) HasMember(userID string) bool {
	for _, member := range o.Members {
		if member.UserID == userID {
			return true
		}
	}
	return false
}

// GetMemberRole returns the role of a user in the organization.
// Returns empty string if the user is not a member.
func (o *Organization) GetMemberRole(userID string) OrganizationRole {
	for _, member := range o.Members {
		if member.UserID == userID {
			return NormalizeOrganizationRole(member.Role)
		}
	}
	return ""
}

// IsOwner checks if a user is the owner of the organization.
func (o *Organization) IsOwner(userID string) bool {
	return o.OwnerUserID == userID
}

// CanUserAccess checks if a user has any level of access to the organization.
func (o *Organization) CanUserAccess(userID string) bool {
	if o.OwnerUserID == userID {
		return true
	}
	return o.HasMember(userID)
}

// CanUserManage checks if a user can manage the organization (owner or admin).
func (o *Organization) CanUserManage(userID string) bool {
	if o.OwnerUserID == userID {
		return true
	}
	role := o.GetMemberRole(userID)
	return role == OrgRoleOwner || role == OrgRoleAdmin
}
