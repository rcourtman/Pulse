//go:build !linux

package dockeragent

func collectorOwnsRootlessEndpoint(string) bool { return false }
