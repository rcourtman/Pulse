package fsfilters

import "testing"

func TestRuntimeDataMountsRemainEligible(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs", "btrfs", "zfs", ""} {
		for _, mount := range []string{
			"/var/lib/containers", "/var/lib/containers/", "/var/lib/containers/storage",
			"/var/lib/containers/storage/volumes/data", "/var/lib/docker", "/var/lib/docker/",
			"/var/lib/docker/volumes/data/_data", "/var/lib/docker-data",
			"/var/lib/containers/storage/overlay-backups", "/var/lib/docker/overlay2-data",
		} {
			t.Run(fsType+mount, func(t *testing.T) {
				for _, used := range []uint64{0, 980, 1000} {
					if skip, reasons := ShouldSkipFilesystem(fsType, mount, 1000, used); skip {
						t.Fatalf("real runtime data disk hidden at %d/1000: %v", used, reasons)
					}
				}
			})
		}
	}
}

func TestRuntimeDataMountsStillExcludeLayersAndSpecialFilesystems(t *testing.T) {
	for _, mount := range []string{"/var/lib/containers", "/var/lib/docker"} {
		for _, fsType := range []string{"overlay", "overlayfs", "tmpfs", "squashfs", "nfs4", "cifs"} {
			if skip, _ := ShouldSkipFilesystem(fsType, mount, 1000, 980); !skip {
				t.Errorf("runtime directory bypassed %q filtering at %q", fsType, mount)
			}
		}
	}
	for _, directory := range containerLayerMountPrefixes {
		for _, fsType := range []string{"ext4", "xfs", "btrfs"} {
			for _, mount := range []string{directory, directory + "/", directory + "/layer/merged"} {
				if skip, _ := ShouldSkipFilesystem(fsType, mount, 1000, 980); !skip {
					t.Errorf("runtime layer admitted at %q (%s)", mount, fsType)
				}
			}
			if skip, reasons := ShouldSkipFilesystem(fsType, directory+"-data", 1000, 980); skip {
				t.Errorf("layer directory swallowed sibling data mount %q: %v", directory+"-data", reasons)
			}
		}
	}
}
