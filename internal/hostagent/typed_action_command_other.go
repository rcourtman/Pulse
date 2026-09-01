//go:build !linux

package hostagent

import "context"

func runTypedActionCommandPlatform(context.Context, []string, typedActionCatalog, string, ...string) typedActionCommandResult {
	return typedActionCommandResult{exitCode: -1, err: errTypedActionContainmentUnavailable}
}

func RunTypedActionLauncher([]string) int {
	return typedActionLauncherSupervisorFailureExit
}

func ReconcileTypedActionUnits(context.Context) error {
	return errTypedActionContainmentUnavailable
}
