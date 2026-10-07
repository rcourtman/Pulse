package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// windowsVMFilesystemCapacityKey identifies a filesystem, not its backing
// physical disk. Windows QGA reports disk.dev as PhysicalDriveN for every
// partition on that drive. A volume GUID is stronger than its mount path;
// without one, keep distinct drive/mount paths rather than guess equivalence.
// This key is local to capacity aggregation, never hardware or History identity.
func windowsVMFilesystemCapacityKey(fs proxmox.VMFileSystem) string {
	name := strings.ToLower(strings.TrimSpace(fs.Name))
	if strings.HasPrefix(name, `\\?\volume{`) && strings.HasSuffix(strings.TrimRight(name, `\/`), "}") {
		return "volume:" + strings.TrimRight(name, `\/`)
	}
	mount := strings.TrimSpace(fs.Mountpoint)
	if len(mount) >= 2 && mount[1] == ':' &&
		((mount[0] >= 'a' && mount[0] <= 'z') || (mount[0] >= 'A' && mount[0] <= 'Z')) &&
		(len(mount) == 2 || mount[2] == '\\' || mount[2] == '/') {
		return "mount:" + strings.TrimRight(strings.ToLower(strings.ReplaceAll(mount, "/", `\`)), `\`)
	}
	return ""
}
