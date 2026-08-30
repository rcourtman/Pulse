//go:build windows

package securityutil

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	localSystemSID    = "S-1-5-18"
	administratorsSID = "S-1-5-32-544"
)

// HardenPrivatePath protects a file or directory DACL from inheritance and
// grants access only to its owner identity, LocalSystem, and Administrators.
// Windows ignores Unix owner/group/other mode bits, so os.Chmod cannot provide
// this boundary.
func HardenPrivatePath(path string, _ os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect private Windows path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return fmt.Errorf("private Windows path must be a real file or directory")
	}
	allowed, currentSID, err := privateWindowsSIDs()
	if err != nil {
		return err
	}
	if err := validatePrivateWindowsOwner(path, allowed); err != nil {
		return err
	}

	inheritance := ""
	if info.IsDir() {
		inheritance = "OICI"
	}
	entries := make([]string, 0, len(allowed))
	seen := make(map[string]struct{}, len(allowed))
	for _, sid := range append([]*windows.SID{currentSID}, allowed...) {
		value := sid.String()
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		entries = append(entries, fmt.Sprintf("(A;%s;GA;;;%s)", inheritance, value))
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P" + strings.Join(entries, ""))
	if err != nil {
		return fmt.Errorf("build private Windows DACL: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read private Windows DACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		return fmt.Errorf("apply private Windows DACL: %w", err)
	}
	return ValidatePrivatePath(path, info)
}

// ValidatePrivatePath verifies that inheritance is disabled and every access
// entry belongs to the current identity, LocalSystem, or Administrators.
func ValidatePrivatePath(path string, _ os.FileInfo) error {
	allowed, currentSID, err := privateWindowsSIDs()
	if err != nil {
		return err
	}
	if err := validatePrivateWindowsOwner(path, allowed); err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read private Windows security descriptor: %w", err)
	}
	if descriptor == nil {
		return fmt.Errorf("private Windows path has no security descriptor")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return fmt.Errorf("read private Windows DACL control: %w", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("private Windows DACL still inherits access")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read private Windows DACL: %w", err)
	}
	if dacl == nil || dacl.AceCount == 0 {
		return fmt.Errorf("private Windows path has no access entries")
	}

	currentAllowed := false
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var entry *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &entry); err != nil {
			return fmt.Errorf("read private Windows DACL entry: %w", err)
		}
		if entry.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("private Windows DACL contains a non-allow entry")
		}
		sid := (*windows.SID)(unsafe.Pointer(&entry.SidStart))
		if !sid.IsValid() || !windowsSIDAllowed(sid, allowed) {
			return fmt.Errorf("private Windows DACL grants an unapproved identity")
		}
		if sid.Equals(currentSID) {
			currentAllowed = true
		}
	}
	if !currentAllowed {
		return fmt.Errorf("private Windows DACL omits the current identity")
	}
	return nil
}

func privateWindowsSIDs() ([]*windows.SID, *windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve current Windows identity: %w", err)
	}
	currentSID, err := user.User.Sid.Copy()
	if err != nil {
		return nil, nil, fmt.Errorf("copy current Windows identity: %w", err)
	}
	systemSID, err := windows.StringToSid(localSystemSID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve LocalSystem SID: %w", err)
	}
	adminSID, err := windows.StringToSid(administratorsSID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve Administrators SID: %w", err)
	}
	return []*windows.SID{currentSID, systemSID, adminSID}, currentSID, nil
}

func validatePrivateWindowsOwner(path string, allowed []*windows.SID) error {
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read private Windows path owner: %w", err)
	}
	if descriptor == nil {
		return fmt.Errorf("private Windows path has no owner descriptor")
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("read private Windows path owner: %w", err)
	}
	if owner == nil || !windowsSIDAllowed(owner, allowed) {
		return fmt.Errorf("private Windows path has an unapproved owner")
	}
	return nil
}

func windowsSIDAllowed(candidate *windows.SID, allowed []*windows.SID) bool {
	for _, sid := range allowed {
		if candidate.Equals(sid) {
			return true
		}
	}
	return false
}
