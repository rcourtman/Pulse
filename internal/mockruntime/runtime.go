package mockruntime

import (
	"errors"
	"sync/atomic"
)

// ErrReleaseFixturesUnauthorized is returned when a release build attempts to
// enable mock fixtures without explicit runtime authorization.
var ErrReleaseFixturesUnauthorized = errors.New("mock fixtures require demo_fixtures entitlement in demo mode")

var (
	enabled                   atomic.Bool
	releaseFixturesAuthorized atomic.Bool
)

func init() {
	enabled.Store(startupEnabledFromEnv())
}

// IsEnabled reports whether mock fixtures are currently enabled in-process.
func IsEnabled() bool {
	return enabled.Load()
}

// SetEnabled synchronizes the canonical in-process mock mode flag.
func SetEnabled(enable bool) {
	enabled.Store(enable)
}

// SetReleaseFixturesAuthorized records whether the current release runtime is
// allowed to render mock fixtures.
func SetReleaseFixturesAuthorized(authorized bool) {
	releaseFixturesAuthorized.Store(authorized)
}
