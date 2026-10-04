package agentexec

import (
	"fmt"
	"strings"
)

// Version 1 requires local lock admission, per-VM serialization, and an
// uncertainty cooldown. It does NOT reserve QGA against PVE backup workers.
const GuestExecGuardProtocolVersion = 1

const guestExecDeferredPrefix = "VM guest execution deferred: "

// GuestExecDeferred carries only a fixed safety reason, never guest/config data.
func GuestExecDeferred(reason string) error {
	return fmt.Errorf("%s%s", guestExecDeferredPrefix, reason)
}

func IsGuestExecDeferred(message string) bool {
	return strings.HasPrefix(message, guestExecDeferredPrefix)
}

const (
	GuestExecGuardUnavailable  = "update the host agent for backup-safety support"
	GuestExecInvalidTarget     = "the local VM identity is invalid"
	GuestExecLockUnverified    = "the local VM backup lock could not be verified"
	GuestExecVMLocked          = "the VM has an active operation lock"
	GuestExecCompletionUnknown = "guest command completion could not be verified"
	GuestExecCooldown          = "a previous guest command is still uncertain"
	GuestExecCapacity          = "guest command admission is at capacity"
	GuestExecTargetUnverified  = "the owning node agent could not be uniquely identified"
)

func requireVMGuestExecGuard(ac *agentConn, targetType string) error {
	if strings.EqualFold(strings.TrimSpace(targetType), "vm") &&
		(ac == nil || ac.guestExecGuardVersion != GuestExecGuardProtocolVersion || ac.agent.Platform != "linux") {
		return GuestExecDeferred(GuestExecGuardUnavailable)
	}
	return nil
}
