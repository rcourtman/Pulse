//go:build !linux

package dockeragent

import "errors"

func collectorOwnsRootlessEndpoint(string) bool { return false }

func collectorRootlessRuntimeCandidates(RuntimeKind) ([]runtimeCandidate, error) {
	return nil, errors.New("collector-owned rootless container runtimes are supported only on Linux")
}
