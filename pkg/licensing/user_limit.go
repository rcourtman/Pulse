package licensing

import "fmt"

const MaxUsersLicenseGateKey = "max_users"

func MaxUsersLimitFromLimits(limits map[string]int64) int {
	if len(limits) == 0 {
		return 0
	}

	v, ok := limits[MaxUsersLicenseGateKey]
	if !ok || v <= 0 {
		return 0
	}

	return int(v)
}

func MaxUsersLimitFromLicense(lic *License) int {
	if lic == nil {
		return 0
	}
	return MaxUsersLimitFromLimits(lic.Claims.EffectiveLimits())
}

func ExceedsUserLimit(current, additions, limit int) bool {
	if limit <= 0 || additions <= 0 {
		return false
	}
	return current+additions > limit
}

func UserLimitExceededMessage(current, limit int) string {
	return fmt.Sprintf("User limit reached (%d/%d). Remove a member or upgrade your license.", current, limit)
}
