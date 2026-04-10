//go:build release

package mockmode

import "github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"

// IsEnabled reports the canonical in-process mock runtime state. Release builds
// still fail closed because only entitled demo runtimes may enable it.
func IsEnabled() bool { return mockruntime.IsEnabled() }
