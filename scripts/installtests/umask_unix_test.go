//go:build unix

package installtests

import "syscall"

// The installer lifecycle trust checks refuse state files whose parent
// directory is group- or world-writable. Test fixtures build those parents with
// t.TempDir, which inherits the process umask, so a worker with umask 002 (the
// Ubuntu default for a user's private group) produced 775 fixture directories
// and failed every trusted-state test that passes under umask 022 on GitHub
// runners and macOS. Pin the umask the fixtures were written against so the
// package proves the installer contract instead of the host's umask.
func init() {
	syscall.Umask(0o022)
}
