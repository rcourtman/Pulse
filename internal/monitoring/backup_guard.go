package monitoring

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// shouldReuseCachedPBSBackups keeps cached data for transient fetch errors and
// avoids reuse for terminal API 4xx errors. A timed-out or rate-limited read
// cannot establish absence or revoked access; retain it without renewing age.
func shouldReuseCachedPBSBackups(err error) bool {
	if err == nil {
		return false
	}
	if status, ok := pbs.HTTPStatus(err); ok {
		return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status < 400 || status >= 500
	}
	// Retain compatibility with untyped errors from older callers.
	if message := strings.ToLower(err.Error()); strings.Contains(message, "api error 4") {
		// Only the first status marker owns the failure. A later marker in a
		// legacy provider body cannot turn a permission denial into a retry.
		message = message[strings.Index(message, "api error 4"):]
		return strings.HasPrefix(message, "api error 408:") || strings.HasPrefix(message, "api error 429:")
	}
	return true
}
