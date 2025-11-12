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
