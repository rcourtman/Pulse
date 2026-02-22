package api

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

const maxUsersLicenseGateKey = licensing.MaxUsersLicenseGateKey

// maxUsersLimitForContext returns the max_users limit from the license for the current context.
// Returns 0 if no limit is set (unlimited).
func maxUsersLimitForContext(ctx context.Context) int {
	service := getLicenseServiceForContext(ctx)
	if service == nil {
		return 0
	}

	lic := service.Current()
	return licensing.MaxUsersLimitFromLicense(lic)
}

// currentUserCount returns the number of members in the organization.
func currentUserCount(org *models.Organization) int {
	if org == nil {
		return 0
	}
	return len(org.Members)
}

// enforceUserLimitForMemberAdd checks if adding a new member would exceed the max_users limit.
// Returns true if the request was blocked (402 written), false if allowed.
// Only call this for NEW member additions, not for role updates of existing members.
func enforceUserLimitForMemberAdd(w http.ResponseWriter, ctx context.Context, org *models.Organization) bool {
	limit := maxUsersLimitForContext(ctx)
	if limit <= 0 {
		return false // No limit set - unlimited
	}

	current := currentUserCount(org)
	if !licensing.ExceedsUserLimit(current, 1, limit) {
		return false // Within limit
	}

	WriteLicenseRequired(
		w,
		maxUsersLicenseGateKey,
		licensing.UserLimitExceededMessage(current, limit),
	)
	return true
}
