package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// shouldReuseCachedPBSBackups keeps cached data for transient fetch errors and
// avoids reuse for terminal API 4xx errors.
func shouldReuseCachedPBSBackups(err error) bool {
	if err == nil {
		return false
	}
	if status, ok := pbs.HTTPStatus(err); ok {
		return status < 400 || status >= 500
	}
	// Retain compatibility with untyped errors from older callers.
	if strings.Contains(strings.ToLower(err.Error()), "api error 4") {
		return false
	}
	return true
}
