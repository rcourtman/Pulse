package fsfilters

import (
	"strings"
	"testing"
)

func TestEligibleFilesystemFilteringDoesNotAllocate(t *testing.T) {
	paths := []string{"", "/", "/data", "C:\\", " /runtime "}
	for _, prefix := range specialMountPrefixes {
		directory := strings.TrimSuffix(prefix, "/")
		// Exercise short and long mismatches, including the macOS prefixes
		// whose slash-suffixed strings cannot fit a Go stack concat buffer.
		for _, suffix := range []string{"s", "-data", ".backup"} {
			paths = append(paths, directory+suffix, directory+suffix+"/child/mount")
		}
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var skip bool
			var reasons []string
			allocs := testing.AllocsPerRun(1000, func() {
				skip, reasons = ShouldSkipFilesystem("ext4", path, 1000, 1000)
			})
			if skip || len(reasons) != 0 {
				t.Fatalf("independent local volume was hidden: skip=%t, reasons=%v", skip, reasons)
			}
			if allocs != 0 {
				t.Fatalf("eligible filesystem filtering allocated %g objects per call, want zero", allocs)
			}
		})
	}
}
