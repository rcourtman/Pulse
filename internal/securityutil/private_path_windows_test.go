//go:build windows

package securityutil

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestValidatePrivatePathRejectsInheritedOrBroadWindowsDACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broad.json")
	if err := os.WriteFile(path, []byte("state"), 0o600); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:(A;;GA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrivatePath(path, info); err == nil {
		t.Fatal("broad Windows DACL was accepted")
	}
	if err := HardenPrivatePath(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrivatePath(path, info); err != nil {
		t.Fatalf("hardened Windows DACL was rejected: %v", err)
	}
}
