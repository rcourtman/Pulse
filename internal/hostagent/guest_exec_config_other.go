//go:build !linux

package hostagent

import (
	"context"
	"errors"
)

const localGuestExecQM = "/usr/sbin/qm"

func readLocalGuestExecConfig(context.Context, string) ([]byte, error) {
	return nil, errors.New("local PVE guest config unavailable on this platform")
}
