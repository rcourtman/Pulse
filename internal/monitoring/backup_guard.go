package monitoring

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// shouldPreservePBSBackupsWithTerminal preserves stale PBS backups only when all
// datastore fetches failed and at least one failure was non-terminal.
func shouldPreservePBSBackupsWithTerminal(datastoreCount, datastoreFetches, datastoreTerminalFailure int) bool {
	if datastoreCount > 0 && datastoreFetches == 0 && datastoreTerminalFailure < datastoreCount {
		return true
	}
	return false
}

// shouldReuseCachedPBSBackups keeps cached data for transient fetch errors and
// avoids reuse for terminal API 4xx errors. A response-level timeout or rate
// limit is not a rejected scope or an accepted empty inventory. Retaining it
// neither retries a request nor renews the cached artifact's evidence.
func shouldReuseCachedPBSBackups(err error) bool {
	if err == nil {
		return false
	}
	if status, ok := pbs.HTTPStatus(err); ok {
		return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests ||
			status < 400 || status >= 500
	}
	// Retain compatibility with untyped errors from older callers.
	if strings.Contains(strings.ToLower(err.Error()), "api error 4") {
		return false
	}
	return true
}
