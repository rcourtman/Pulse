// Package safety provides shared safety utilities for AI operations
package safety

import "github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"

// ReadOnlyPatterns delegates to the canonical list in pkg/aicontracts.
var ReadOnlyPatterns = aicontracts.ReadOnlyPatterns

// IsReadOnlyCommand delegates to pkg/aicontracts.
func IsReadOnlyCommand(command string) bool {
	return aicontracts.IsReadOnlyCommand(command)
}
