//go:build !windows

package hostagent

import (
	"context"

	gohost "github.com/shirou/gopsutil/v4/host"
)

// recoverWindowsHostInfo has no non-Windows behaviour: the MachineGuid buffer
// and length defect it repairs is specific to the Windows host-ID path.
func recoverWindowsHostInfo(context.Context) *gohost.InfoStat {
	return nil
}
