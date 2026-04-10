//go:build !release

package mockmode

import "github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"

// IsEnabled reports whether Pulse is running in mock mode.
func IsEnabled() bool {
	return mockruntime.IsEnabled()
}
