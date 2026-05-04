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
)

// OrgStatus represents lifecycle status for an organization.
type OrgStatus string

const (
	OrgStatusActive          OrgStatus = "active"
	OrgStatusSuspended       OrgStatus = "suspended"
	OrgStatusPendingDeletion OrgStatus = "pending_deletion"
)

// OrganizationShareStatus represents the lifecycle state of a cross-org share.
type OrganizationShareStatus string

const (
	// OrganizationShareStatusPending requires explicit target-org acceptance
	// before the share becomes active.
	OrganizationShareStatusPending OrganizationShareStatus = "pending"
	// OrganizationShareStatusAccepted is an active share that target-org admins
	// have already accepted. Empty legacy status values normalize here.
	OrganizationShareStatusAccepted OrganizationShareStatus = "accepted"
)

// NormalizeOrgStatus canonicalizes lifecycle status values.
// Empty status is treated as active for backward compatibility.
func NormalizeOrgStatus(status OrgStatus) OrgStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case "", string(OrgStatusActive):
		return OrgStatusActive
	case string(OrgStatusSuspended):
		return OrgStatusSuspended
	case string(OrgStatusPendingDeletion):
		return OrgStatusPendingDeletion
	default:
		return OrgStatus(strings.ToLower(strings.TrimSpace(string(status))))
	}
}

// NormalizeOrganizationShareStatus canonicalizes share lifecycle values.
// Empty status is treated as accepted for backward compatibility.
func NormalizeOrganizationShareStatus(status OrganizationShareStatus) OrganizationShareStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case "", string(OrganizationShareStatusAccepted):
		return OrganizationShareStatusAccepted
	case string(OrganizationShareStatusPending):
		return OrganizationShareStatusPending
	default:
		return OrganizationShareStatus(strings.ToLower(strings.TrimSpace(string(status))))
	}
}

// IsValidOrganizationShareStatus reports whether the share status is known.
func IsValidOrganizationShareStatus(status OrganizationShareStatus) bool {
	switch NormalizeOrganizationShareStatus(status) {
	case OrganizationShareStatusPending, OrganizationShareStatusAccepted:
		return true
	default:
		return false
	}
}

// NormalizeOrganizationRole canonicalizes role values.
func NormalizeOrganizationRole(role OrganizationRole) OrganizationRole {
	switch strings.ToLower(strings.TrimSpace(string(role))) {
	case string(OrgRoleOwner):
		return OrgRoleOwner
	case string(OrgRoleAdmin):
		return OrgRoleAdmin
	case string(OrgRoleEditor):
		return OrgRoleEditor
	case string(OrgRoleViewer):
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

func organizationRoleRank(role OrganizationRole) int {
	switch NormalizeOrganizationRole(role) {
	case OrgRoleViewer:
		return 1
	case OrgRoleEditor:
		return 2
	case OrgRoleAdmin:
		return 3
	case OrgRoleOwner:
		return 4
	default:
		return 0
	}
}

// OrganizationRoleAtLeast reports whether actualRole satisfies requiredRole.
func OrganizationRoleAtLeast(actualRole, requiredRole OrganizationRole) bool {
	actualRole = NormalizeOrganizationRole(actualRole)
	requiredRole = NormalizeOrganizationRole(requiredRole)
	if !IsValidOrganizationRole(actualRole) || !IsValidOrganizationRole(requiredRole) {
		return false
	}
	return organizationRoleRank(actualRole) >= organizationRoleRank(requiredRole)
}

// OrganizationMember represents a user's membership in an organization.
type OrganizationMember struct {
	// UserID is the unique identifier of the member.
	UserID string `json:"userId"`

	// Email is the member's contact/display email. It is not the durable
	// identity key; legacy records may still have email-shaped UserID values.
	Email string `json:"email,omitempty"`

	// Role is the member's role within the organization.
	Role OrganizationRole `json:"role"`

	// AddedAt is when the member was added to the organization.
	AddedAt time.Time `json:"addedAt"`

	// AddedBy is the user ID of who added this member (empty for owner).
	AddedBy string `json:"addedBy,omitempty"`
}

// OrganizationInvitation represents pending organization access awaiting
// explicit acceptance by the invited user.
type OrganizationInvitation struct {
	// UserID is the invited username.
	UserID string `json:"userId"`

	// Role is the role that will be granted on acceptance.
	Role OrganizationRole `json:"role"`

	// InvitedAt is when the invitation was created.
	InvitedAt time.Time `json:"invitedAt"`

	// InvitedBy is the user ID that created the invitation.
	InvitedBy string `json:"invitedBy"`
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

	// Status tracks whether the target organization has accepted the share.
	Status OrganizationShareStatus `json:"status"`

	// CreatedAt is when the share was created.
	CreatedAt time.Time `json:"createdAt"`

	// CreatedBy is the user ID that created the share.
	CreatedBy string `json:"createdBy"`

	// AcceptedAt is when the target organization accepted the share.
	AcceptedAt time.Time `json:"acceptedAt,omitempty"`

	// AcceptedBy is the target-org admin who accepted the share.
	AcceptedBy string `json:"acceptedBy,omitempty"`
}

// Organization represents a distinct tenant in the system.
type Organization struct {
	// ID is the unique identifier for the organization (e.g., "customer-a").
	// It is used as the directory name for data isolation.
	ID string `json:"id"`

	// DisplayName is the human-readable name of the organization.
	DisplayName string `json:"displayName"`

	// Status is the current lifecycle status for the organization.
	// Empty status is treated as active for backward compatibility.
	Status OrgStatus `json:"status,omitempty"`

	// CreatedAt is when the organization was registered.
	CreatedAt time.Time `json:"createdAt"`

	// OwnerUserID is the primary owner of this organization.
	// The owner has full administrative rights and cannot be removed.
	OwnerUserID string `json:"ownerUserId,omitempty"`

	// OwnerEmail is the owner's contact/display email. It is not the durable
	// identity key; legacy records may still have email-shaped OwnerUserID values.
	OwnerEmail string `json:"ownerEmail,omitempty"`

	// Members is the list of users who have access to this organization.
	// This includes the owner (with OrgRoleOwner) and any additional members.
	Members []OrganizationMember `json:"members,omitempty"`

	// PendingInvitations contains invited users who must explicitly accept
	// organization access before membership becomes active.
	PendingInvitations []OrganizationInvitation `json:"pendingInvitations,omitempty"`

	// SharedResources contains outgoing cross-organization shares.
	SharedResources []OrganizationShare `json:"sharedResources,omitempty"`

	// SuspendedAt records when the organization was suspended.
	SuspendedAt *time.Time `json:"suspendedAt,omitempty"`

	// SuspendReason stores the reason for suspension, if provided.
	SuspendReason string `json:"suspendReason,omitempty"`

	// DeletionRequestedAt records when soft-deletion was requested.
	DeletionRequestedAt *time.Time `json:"deletionRequestedAt,omitempty"`

	// RetentionDays stores the soft-delete retention period in days.
	RetentionDays int `json:"retentionDays,omitempty"`

	// EncryptionKeyID refers to the specific encryption key used for this org's data
	// (Future proofing for per-tenant encryption keys)
	EncryptionKeyID string `json:"encryptionKeyId,omitempty"`
}

func normalizeOrganizationIdentityValue(value string) string {
	return strings.TrimSpace(value)
}

func normalizeOrganizationEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func identityLooksLikeEmail(value string) bool {
	return strings.Contains(strings.TrimSpace(value), "@")
}

func memberEmail(member OrganizationMember) string {
	email := normalizeOrganizationEmail(member.Email)
	if email != "" {
		return email
	}
	if identityLooksLikeEmail(member.UserID) {
		return normalizeOrganizationEmail(member.UserID)
	}
	return ""
}

func memberMatchesUserID(member OrganizationMember, userID string) bool {
	userID = normalizeOrganizationIdentityValue(userID)
	return userID != "" && normalizeOrganizationIdentityValue(member.UserID) == userID
}

func memberMatchesEmail(member OrganizationMember, email string) bool {
	email = normalizeOrganizationEmail(email)
	return email != "" && memberEmail(member) == email
}

func ownerMatchesUserID(org *Organization, userID string) bool {
	if org == nil {
		return false
	}
	userID = normalizeOrganizationIdentityValue(userID)
	return userID != "" && normalizeOrganizationIdentityValue(org.OwnerUserID) == userID
}

func ownerMatchesEmail(org *Organization, email string) bool {
	if org == nil {
		return false
	}
	email = normalizeOrganizationEmail(email)
	if email == "" {
		return false
	}
	ownerEmail := normalizeOrganizationEmail(org.OwnerEmail)
	if ownerEmail != "" {
		return ownerEmail == email
	}
	if identityLooksLikeEmail(org.OwnerUserID) {
		return normalizeOrganizationEmail(org.OwnerUserID) == email
	}
	return false
}

// HasMember checks if a user is a member of the organization.
func (o *Organization) HasMember(userID string) bool {
	for _, member := range o.Members {
		if memberMatchesUserID(member, userID) || memberMatchesEmail(member, userID) {
			return true
		}
	}
	return false
}

// HasMemberUserID checks membership using only the stored durable user ID.
func (o *Organization) HasMemberUserID(userID string) bool {
	for _, member := range o.Members {
		if memberMatchesUserID(member, userID) {
			return true
		}
	}
	return false
}

// GetMemberRole returns the role of a user in the organization.
// Returns empty string if the user is not a member.
func (o *Organization) GetMemberRole(userID string) OrganizationRole {
	for _, member := range o.Members {
		if memberMatchesUserID(member, userID) || memberMatchesEmail(member, userID) {
			return NormalizeOrganizationRole(member.Role)
		}
	}
	return ""
}

// GetMemberRoleByUserID returns a role using only the stored durable user ID.
func (o *Organization) GetMemberRoleByUserID(userID string) OrganizationRole {
	for _, member := range o.Members {
		if memberMatchesUserID(member, userID) {
			return NormalizeOrganizationRole(member.Role)
		}
	}
	return ""
}

// IsOwner checks if a user is the owner of the organization.
func (o *Organization) IsOwner(userID string) bool {
	return ownerMatchesUserID(o, userID) || ownerMatchesEmail(o, userID)
}

// IsOwnerUserID checks ownership using only the stored durable owner user ID.
func (o *Organization) IsOwnerUserID(userID string) bool {
	return ownerMatchesUserID(o, userID)
}

// CanUserAccess checks if a user has any level of access to the organization.
func (o *Organization) CanUserAccess(userID string) bool {
	if o.IsOwner(userID) {
		return true
	}
	return o.HasMember(userID)
}

// CanUserIDAccess checks access using only durable owner/member user IDs.
func (o *Organization) CanUserIDAccess(userID string) bool {
	if o.IsOwnerUserID(userID) {
		return true
	}
	return o.HasMemberUserID(userID)
}

// CanUserManage checks if a user can manage the organization (owner or admin).
func (o *Organization) CanUserManage(userID string) bool {
	if o.IsOwner(userID) {
		return true
	}
	return OrganizationRoleAtLeast(o.GetMemberRole(userID), OrgRoleAdmin)
}

// CanUserIDManage checks management access using only durable user IDs.
func (o *Organization) CanUserIDManage(userID string) bool {
	if o.IsOwnerUserID(userID) {
		return true
	}
	return OrganizationRoleAtLeast(o.GetMemberRoleByUserID(userID), OrgRoleAdmin)
}

// GetMemberRoleForPrincipal resolves a role using the durable user ID first,
// then email only as a legacy/contact fallback.
func (o *Organization) GetMemberRoleForPrincipal(userID, email string) OrganizationRole {
	if o == nil {
		return ""
	}
	if ownerMatchesUserID(o, userID) || ownerMatchesEmail(o, email) {
		return OrgRoleOwner
	}
	userID = normalizeOrganizationIdentityValue(userID)
	email = normalizeOrganizationEmail(email)
	for _, member := range o.Members {
		if memberMatchesUserID(member, userID) || memberMatchesEmail(member, email) {
			return NormalizeOrganizationRole(member.Role)
		}
	}
	return ""
}

// ResolvePrincipalByEmail returns the durable principal for a contact email.
// It is intended for email-delivery flows such as magic links where the email
// proves delivery but the session should still bind to the stored user ID.
func (o *Organization) ResolvePrincipalByEmail(email string) (string, OrganizationRole, bool) {
	if o == nil {
		return "", "", false
	}
	email = normalizeOrganizationEmail(email)
	if email == "" {
		return "", "", false
	}
	if ownerMatchesEmail(o, email) {
		userID := normalizeOrganizationIdentityValue(o.OwnerUserID)
		if userID == "" {
			return "", "", false
		}
		return userID, OrgRoleOwner, true
	}
	for _, member := range o.Members {
		if !memberMatchesEmail(member, email) {
			continue
		}
		userID := normalizeOrganizationIdentityValue(member.UserID)
		if userID == "" {
			return "", "", false
		}
		return userID, NormalizeOrganizationRole(member.Role), true
	}
	return "", "", false
}

// CanonicalizePrincipalIdentity upgrades legacy email-keyed membership records
// to a stable user ID while preserving email as contact metadata.
func (o *Organization) CanonicalizePrincipalIdentity(userID, email string) bool {
	if o == nil {
		return false
	}
	userID = normalizeOrganizationIdentityValue(userID)
	email = normalizeOrganizationEmail(email)
	if userID == "" || email == "" {
		return false
	}

	changed := false
	if ownerMatchesUserID(o, userID) || ownerMatchesEmail(o, email) {
		if normalizeOrganizationIdentityValue(o.OwnerUserID) != userID {
			o.OwnerUserID = userID
			changed = true
		}
		if normalizeOrganizationEmail(o.OwnerEmail) != email {
			o.OwnerEmail = email
			changed = true
		}
	}

	for i := range o.Members {
		member := o.Members[i]
		if !memberMatchesUserID(member, userID) && !memberMatchesEmail(member, email) {
			continue
		}
		oldUserID := normalizeOrganizationIdentityValue(member.UserID)
		if oldUserID != userID {
			o.Members[i].UserID = userID
			changed = true
		}
		if normalizeOrganizationEmail(member.Email) != email {
			o.Members[i].Email = email
			changed = true
		}
		if addedBy := normalizeOrganizationIdentityValue(member.AddedBy); addedBy != "" && (addedBy == oldUserID || normalizeOrganizationEmail(addedBy) == email) && addedBy != userID {
			o.Members[i].AddedBy = userID
			changed = true
		}
	}

	return changed
}
