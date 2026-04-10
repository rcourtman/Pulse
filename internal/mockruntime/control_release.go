//go:build release

package mockruntime

func startupEnabledFromEnv() bool {
	return false
}

// ValidateEnablement fail-closes release builds unless runtime authorization
// has been granted by the active entitlement.
func ValidateEnablement(enable bool) error {
	if !enable {
		return nil
	}
	if !releaseFixturesAuthorized.Load() {
		return ErrReleaseFixturesUnauthorized
	}
	return nil
}
