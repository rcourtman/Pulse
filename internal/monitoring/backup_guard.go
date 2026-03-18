package monitoring

import "strings"

// shouldPreservePBSBackupsWithTerminal preserves stale PBS backups only when all
// datastore fetches failed and at least one failure was non-terminal.
func shouldPreservePBSBackupsWithTerminal(datastoreCount, datastoreFetches, datastoreTerminalFailure int) bool {
	if datastoreCount > 0 && datastoreFetches == 0 && datastoreTerminalFailure < datastoreCount {
		return true
	}
	return false
}

// shouldReuseCachedPBSBackups keeps cached data for transient fetch errors and
// avoids reuse for terminal API 4xx errors.
func shouldReuseCachedPBSBackups(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(strings.ToLower(err.Error()), "api error 4") {
		return false
	}
	return true
}
