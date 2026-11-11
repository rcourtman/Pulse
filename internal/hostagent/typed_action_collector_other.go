//go:build !linux

package hostagent

import "context"

// A Proxmox node is a Linux host, so there is no guest to start anywhere else.
func prepareCollectorTypedActions(context.Context, typedActionSupervisor) (string, error) {
	return "", errCollectorTypedActionsNotApplicable
}
