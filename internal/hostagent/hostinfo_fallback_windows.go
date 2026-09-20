//go:build windows

package hostagent

import (
	"context"
	"os"
	"runtime"

	gohost "github.com/shirou/gopsutil/v4/host"
	"golang.org/x/sys/windows/registry"
)

const windowsMachineGUIDPath = `SOFTWARE\Microsoft\Cryptography`

// recoverWindowsHostInfo rebuilds host information when gopsutil's combined
// InfoWithContext call fails specifically because the Windows MachineGuid host
// ID cannot be read.
//
// gopsutil reads MachineGuid through a fixed-size registry buffer and rejects
// any value whose length is not exactly 36 characters. A braced GUID (38
// characters) therefore makes HostIDWithContext return an error, which
// InfoWithContext treats as fatal even though every other host field is
// available. Pulse propagated that error and the agent exited at startup
// (issue #2125). This recovery reads MachineGuid directly with a correctly
// sized buffer, strips the optional braces, and leaves the remaining fields to
// gopsutil's individual accessors, so a malformed GUID no longer aborts the
// agent.
func recoverWindowsHostInfo(ctx context.Context) *gohost.InfoStat {
	if _, err := gohost.HostIDWithContext(ctx); err == nil {
		// The host ID is readable, so the combined failure came from another
		// field and must stay fatal so real problems are not hidden.
		return nil
	}

	info := &gohost.InfoStat{
		OS:     runtime.GOOS,
		HostID: readWindowsMachineGUID(),
	}
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}
	if platform, family, version, err := gohost.PlatformInformationWithContext(ctx); err == nil {
		info.Platform = platform
		info.PlatformFamily = family
		info.PlatformVersion = version
	}
	if kernelVersion, err := gohost.KernelVersionWithContext(ctx); err == nil {
		info.KernelVersion = kernelVersion
	}
	if arch, err := gohost.KernelArch(); err == nil {
		info.KernelArch = arch
	}
	return info
}

func readWindowsMachineGUID() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, windowsMachineGUIDPath, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer key.Close()

	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return normalizeMachineGUID(value)
}
