//go:build !linux && !darwin && !freebsd

package api

import "os"

// No reliable change clock is available through this platform's os.FileInfo.
// Descriptor-bound admission still works; only metadata cache reuse is disabled.
func agentArtifactChangeTime(os.FileInfo) (agentArtifactChangeStamp, bool) {
	return agentArtifactChangeStamp{}, false
}
