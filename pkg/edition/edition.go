// Package edition records which compiled Pulse binary is running: the public
// community build or the separately compiled Pulse Pro build (Audit, RBAC,
// Reporting, and SSO compiled in).
//
// It is a small public registration seam that mirrors the existing
// coreaudit.SetLogger / server.SetBusinessHooks pattern: the community binary
// leaves the default (Community) in place, and only the Pro binary flips it to
// Pro during enterpriseruntime.Initialize. The marker keys off the compiled
// binary, never off license-active state — a community binary with an active
// license is still community and must keep its normal self-update behaviour.
//
// This lives in pkg/ rather than internal/ because pulse-enterprise is a
// separate Go module and cannot import internal/ packages. Keeping it
// dependency-free also lets internal/updates consult it without any import
// cycle against pkg/server (which transitively imports internal/updates).
package edition

import (
	"strings"
	"sync/atomic"
)

const (
	// Community is the public, open-source Pulse build.
	Community = "community"
	// Pro is the separately compiled Pulse Pro build.
	Pro = "pro"
)

// current holds the running binary's edition. It always stores a string, so
// atomic.Value is safe here and lets read paths (HTTP handlers) run lock-free.
var current atomic.Value

func init() {
	current.Store(Community)
}

// SetEdition records the running binary's edition. Any value other than Pro
// normalizes to Community so a mis-wired caller can never silently claim Pro.
// Call once during startup registration.
func SetEdition(name string) {
	current.Store(normalize(name))
}

// Current returns the running binary's edition (Community or Pro).
func Current() string {
	if v, ok := current.Load().(string); ok && v != "" {
		return v
	}
	return Community
}

// IsPro reports whether the running binary is the compiled Pulse Pro edition.
func IsPro() bool {
	return Current() == Pro
}

func normalize(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), Pro) {
		return Pro
	}
	return Community
}
