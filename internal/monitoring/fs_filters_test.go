package monitoring

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
			reason, skip := readOnlyFilesystemReason(tc.fsType, tc.totalBytes, tc.usedBytes)
			if reason != tc.reason || skip != tc.skip {
				t.Errorf("expected (%q, %t), got (%q, %t)", tc.reason, tc.skip, reason, skip)
			}
		})
	}
}

func TestShouldIgnoreReadOnlyFilesystem(t *testing.T) {
	if !shouldIgnoreReadOnlyFilesystem("erofs", 100, 10) {
		t.Fatalf("expected erofs to be ignored")
	}

	if shouldIgnoreReadOnlyFilesystem("ext4", 100, 10) {
		t.Fatalf("expected ext4 to be included")
	}
}
