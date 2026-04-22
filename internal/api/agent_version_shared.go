package api

import "github.com/rcourtman/pulse-go-rewrite/internal/updates"

// currentAgentTargetVersion returns the canonical version string agents should
// compare themselves against. Development/source builds deliberately return an
// empty string so operator-facing surfaces do not raise false update warnings
// when the server reports "dev" to avoid agent update loops.
func currentAgentTargetVersion() string {
	versionInfo, err := updates.GetCurrentVersion()
	if err != nil || versionInfo.IsDevelopment {
		return ""
	}
	return versionInfo.Version
}
