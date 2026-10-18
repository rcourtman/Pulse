package fsfilters

import (
	"strings"
	"testing"
)

func TestSystemMountFiltersRespectDirectoryBoundaries(t *testing.T) {
	// Include the roots as well as descendants. Paths that merely share a
	// directory's spelling must remain eligible, even when they are full.
	for _, directory := range []string{
		"/dev", "/proc", "/sys", "/run", "/var/run", "/var/lib/docker",
		"/var/lib/containers", "/snap", "/System/Volumes",
		"/Library/Developer/CoreSimulator/Volumes",
	} {
		t.Run(directory, func(t *testing.T) {
			for _, path := range []string{directory, directory + "/", directory + "/child/mount"} {
				skip, reasons := ShouldSkipFilesystem("ext4", path, 1000, 1000)
				if !skip || !strings.Contains(strings.Join(reasons, ","), "special-mountpoint") {
					t.Errorf("system directory %q escaped the filter: %v", path, reasons)
				}
			}
			for _, suffix := range []string{"s", "-data", ".backup"} {
				for _, path := range []string{directory + suffix, directory + suffix + "/child/mount"} {
					if skip, reasons := ShouldSkipFilesystem("ext4", path, 1000, 1000); skip {
						t.Errorf("independent full volume %q was hidden: %v", path, reasons)
					}
				}
			}
		})
	}
	for _, fsType := range []string{"tmpfs", "squashfs", "nfs4", "cifs", "overlay"} {
		if skip, _ := ShouldSkipFilesystem(fsType, "/snapshots", 1000, 1000); !skip {
			t.Errorf("directory-boundary repair bypassed %q type filtering", fsType)
		}
	}
	// Operator patterns deliberately support textual prefixes; this repair
	// must not narrow a user's explicit wildcard or turn an exact path broad.
	if !MatchesDiskExclude("/dev/vdb", "/snapshots", []string{"/snap*"}) ||
		MatchesDiskExclude("/dev/vdb", "/snapshots", []string{"/snap"}) {
		t.Fatal("explicit operator pattern semantics changed")
	}
}
