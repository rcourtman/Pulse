package fsfilters

import "testing"

func TestReadOnlyFilesystemReason(t *testing.T) {
	tests := []struct {
		name       string
		fsType     string
		totalBytes uint64
		usedBytes  uint64
		reason     string
		skip       bool
	}{
		{
			name:       "erofs filesystem",
			fsType:     "erofs",
			totalBytes: 10,
			usedBytes:  5,
			reason:     "erofs",
			skip:       true,
		},
		{
			name:       "fuse.erofs filesystem",
			fsType:     "fuse.erofs",
			totalBytes: 10,
			usedBytes:  8,
			reason:     "erofs",
			skip:       true,
		},
		{
			name:       "overlay at capacity",
			fsType:     "overlay",
			totalBytes: 100,
			usedBytes:  100,
			reason:     "overlay",
			skip:       true,
		},
		{
			name:       "squashfs filesystem",
			fsType:     "squashfs",
			totalBytes: 4096,
			usedBytes:  4096,
			reason:     "squashfs",
			skip:       true,
		},
		{
			name:       "squash-fs alias",
			fsType:     "Squash-FS",
			totalBytes: 2048,
			usedBytes:  2048,
			reason:     "squashfs",
			skip:       true,
		},
		{
			name:       "iso9660 filesystem",
			fsType:     "iso9660",
			totalBytes: 700 * 1024 * 1024,
			usedBytes:  700 * 1024 * 1024,
			reason:     "iso9660",
			skip:       true,
		},
		{
			name:       "cdfs filesystem",
			fsType:     "CDFS",
			totalBytes: 600 * 1024 * 1024,
			usedBytes:  600 * 1024 * 1024,
			reason:     "cdfs",
			skip:       true,
		},
		{
			name:       "udf optical media",
			fsType:     "udf",
			totalBytes: 4 * 1024 * 1024 * 1024,
			usedBytes:  4 * 1024 * 1024 * 1024,
			reason:     "udf",
			skip:       true,
		},
		{
			name:       "cramfs image",
			fsType:     "cramfs",
			totalBytes: 128 * 1024 * 1024,
			usedBytes:  128 * 1024 * 1024,
			reason:     "cramfs",
			skip:       true,
		},
		{
			name:       "romfs firmware partition",
			fsType:     "romfs",
			totalBytes: 16 * 1024 * 1024,
			usedBytes:  16 * 1024 * 1024,
			reason:     "romfs",
			skip:       true,
		},
		{
			name:       "fuse iso image",
			fsType:     "fuse.cdfs",
			totalBytes: 700 * 1024 * 1024,
			usedBytes:  700 * 1024 * 1024,
			reason:     "cdfs",
			skip:       true,
		},
		{
			name:       "overlay below capacity",
			fsType:     "overlay",
			totalBytes: 100,
			usedBytes:  50,
			reason:     "",
			skip:       false,
		},
		{
			name:       "regular ext4",
			fsType:     "ext4",
			totalBytes: 100,
			usedBytes:  50,
			reason:     "",
			skip:       false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reason, skip := ReadOnlyFilesystemReason(tc.fsType, tc.totalBytes, tc.usedBytes)
			if reason != tc.reason || skip != tc.skip {
				t.Errorf("expected (%q, %t), got (%q, %t)", tc.reason, tc.skip, reason, skip)
			}
		})
	}
}

func TestShouldIgnoreReadOnlyFilesystem(t *testing.T) {
	if !ShouldIgnoreReadOnlyFilesystem("erofs", 100, 10) {
		t.Fatalf("expected erofs to be ignored")
	}

	if !ShouldIgnoreReadOnlyFilesystem("squashfs", 100, 100) {
		t.Fatalf("expected squashfs to be ignored")
	}

	if ShouldIgnoreReadOnlyFilesystem("ext4", 100, 10) {
		t.Fatalf("expected ext4 to be included")
	}
}

func TestShouldSkipFilesystem(t *testing.T) {
	tests := []struct {
		name       string
		fsType     string
		mountpoint string
		totalBytes uint64
		usedBytes  uint64
		expectSkip bool
	}{
		// Virtual filesystem types
		{"tmpfs", "tmpfs", "/tmp", 1024, 512, true},
		{"devtmpfs", "devtmpfs", "/dev", 1024, 100, true},
		{"cgroup2", "cgroup2", "/sys/fs/cgroup", 0, 0, true},
		{"sysfs", "sysfs", "/sys", 0, 0, true},
		{"proc", "proc", "/proc", 0, 0, true},
		{"virtual fs case insensitive", "TMPFS", "/tmp", 1024, 512, true},

		// Network filesystem types
		{"nfs mount", "nfs4", "/mnt/nas", 1000000, 500000, true},
		{"cifs mount", "cifs", "/mnt/share", 1000000, 500000, true},
		{"fuse.sshfs", "fuse.sshfs", "/mnt/remote", 1000000, 500000, true},
		{"9p VM shared folder", "9p", "/mnt/host", 1000000, 500000, true},

		// Special mountpoint prefixes
		{"/dev prefix", "ext4", "/dev/shm", 1024, 100, true},
		{"/proc prefix", "ext4", "/proc/sys", 1024, 100, true},
		{"/sys prefix", "ext4", "/sys/kernel", 1024, 100, true},
		{"/run prefix", "ext4", "/run/user/1000", 1024, 100, true},
		{"/var/lib/docker", "ext4", "/var/lib/docker/overlay2", 1000000, 500000, true},
		{"/snap prefix", "ext4", "/snap/core/12345", 1000000, 500000, true},
		{"/boot/efi exact", "vfat", "/boot/efi", 512 * 1024 * 1024, 50 * 1024 * 1024, true},
		{"/var/lib/containers podman", "ext4", "/var/lib/containers/storage/overlay/abc123/merged", 1000000, 500000, true},

		// Container overlay paths in non-standard locations (issue #790)
		{"enhance containers overlay", "ext4", "/var/local/enhance/containers/abc123/overlay/merged", 1000000, 500000, true},
		{"custom containers diff", "ext4", "/opt/containers/myapp/diff/layer", 1000000, 500000, true},
		{"custom containers overlay2", "ext4", "/data/containers/xyz/overlay2/layer1", 1000000, 500000, true},

		// Windows paths
		{"Windows System Reserved", "NTFS", "System Reserved", 500 * 1024 * 1024, 100 * 1024 * 1024, true},
		{"Windows C drive - should NOT skip", "NTFS", "C:\\", 500 * 1024 * 1024 * 1024, 200 * 1024 * 1024 * 1024, false},
		{"Windows D drive - should NOT skip", "NTFS", "D:\\", 1000 * 1024 * 1024 * 1024, 500 * 1024 * 1024 * 1024, false},

		// Regular filesystems that should NOT be skipped
		{"ext4 root", "ext4", "/", 100 * 1024 * 1024 * 1024, 50 * 1024 * 1024 * 1024, false},
		{"xfs data", "xfs", "/data", 500 * 1024 * 1024 * 1024, 200 * 1024 * 1024 * 1024, false},
		{"btrfs home", "btrfs", "/home", 200 * 1024 * 1024 * 1024, 100 * 1024 * 1024 * 1024, false},
		{"zfs pool", "zfs", "/tank", 10 * 1024 * 1024 * 1024 * 1024, 5 * 1024 * 1024 * 1024 * 1024, false},

		// Edge cases
		{"empty fsType", "", "/mnt/data", 1000000, 500000, false},
		{"empty mountpoint", "ext4", "", 1000000, 500000, false},
		{"whitespace fsType", "  tmpfs  ", "/tmp", 1024, 512, true},

		// Read-only filesystems (should still work through new function)
		{"squashfs via ShouldSkipFilesystem", "squashfs", "/snap/firefox/123", 100 * 1024 * 1024, 100 * 1024 * 1024, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			skip, reasons := ShouldSkipFilesystem(tc.fsType, tc.mountpoint, tc.totalBytes, tc.usedBytes)
			if skip != tc.expectSkip {
				t.Errorf("expected skip=%t, got skip=%t (reasons: %v)", tc.expectSkip, skip, reasons)
			}
		})
	}
}
