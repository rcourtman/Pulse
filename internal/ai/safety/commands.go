package safety

import "github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"

// BlockedCommands delegates to the canonical list in pkg/aicontracts.
var BlockedCommands = aicontracts.BlockedCommands

// DestructivePatterns is an alias for BlockedCommands for backward compatibility.
var DestructivePatterns = BlockedCommands

// IsBlockedCommand delegates to pkg/aicontracts.
func IsBlockedCommand(command string) bool {
	return aicontracts.IsBlockedCommand(command)
}

// IsDestructiveCommand delegates to pkg/aicontracts.
func IsDestructiveCommand(command string) bool {
	return aicontracts.IsDestructiveCommand(command)
}
