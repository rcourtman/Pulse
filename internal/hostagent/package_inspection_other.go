//go:build !linux

package hostagent

import (
	"context"
	"errors"
)

var packageInspectionResolveAPT = func() (string, error) {
	return "", errors.New("APT package inspection is unavailable on this platform")
}

func runPackageInspectionCommand(context.Context) packageUpdateCommandResult {
	return packageUpdateCommandResult{exitCode: -1, err: errors.New("APT package inspection is unavailable on this platform")}
}
