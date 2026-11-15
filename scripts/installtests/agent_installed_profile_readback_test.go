//go:build linux

package installtests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
)

func TestAgentInstalledProfileRestoreReadback(t *testing.T) {
	for _, name := range []string{"unchanged", "bytes", "permissions", "owner", "group", "special-mode", "required-absent", "lifecycle-added", "type", "root-mode", "multiple"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			file, optional := filepath.Join(root, "token"), filepath.Join(root, "connection.env")
			const secret = "fixture-protected-bytes-not-for-receipts"
			if err := os.WriteFile(file, []byte(secret), 0o600); err != nil {
				t.Fatal(err)
			}
			before := nativeProfileObserveProtected(file, optional, root)
			original := nativeProfileFiles(t, file)[file]
			if got := before[file]; got.Type != "regular" || !got.Present || got.Error != "" ||
				got.Hash != original.Hash || got.Mode != original.Mode || got.UID != original.UID || got.GID != original.GID {
				t.Fatalf("diagnostic lost the strict file identity: %+v", got)
			}
			if before[optional] != (nativeProfileProtectedIdentity{Type: "absent"}) ||
				before[root].Type != "directory" || before[root].Mode&os.ModeDir == 0 {
				t.Fatal("diagnostic lost absence or directory type")
			}
			want := []string{}
			change := func(path string, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, path)
			}
			switch name {
			case "bytes":
				change(file, os.WriteFile(file, []byte("changed"), 0o600))
			case "permissions":
				change(file, os.Chmod(file, 0o640))
			case "owner", "group", "special-mode":
				// Witness each comparison without requiring privileged chown.
				identity := before[file]
				switch name {
				case "owner":
					identity.UID++
				case "group":
					identity.GID++
				case "special-mode":
					identity.Mode |= os.ModeSetuid
				}
				before[file] = identity
				want = append(want, file)
			case "required-absent":
				change(file, os.Remove(file))
			case "lifecycle-added":
				change(optional, os.WriteFile(optional, []byte("unexpected"), 0o600))
			case "type":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				change(file, os.Mkdir(file, 0o600))
			case "root-mode":
				change(root, os.Chmod(root, 0o750))
			case "multiple":
				change(file, os.WriteFile(file, []byte("changed"), 0o600))
				change(optional, os.WriteFile(optional, []byte("unexpected"), 0o600))
				change(root, os.Chmod(root, 0o750))
			}
			sort.Strings(want)
			evidence := nativeProfileRestoreReadback(before)
			if !reflect.DeepEqual(evidence.ChangedPaths, want) || !reflect.DeepEqual(evidence.Before, before) || len(evidence.After) != 3 {
				t.Fatalf("incomplete before/after difference: %+v; want changed %v", evidence, want)
			}
			receipt := nativeProfileReceipt{Phases: []nativeProfilePhase{{Name: "reverse-protected-state-readback", Evidence: evidence}}, Passed: false}
			data, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), secret) || !strings.Contains(string(data), original.Hash) || !strings.Contains(string(data), `"passed":false`) {
				t.Fatal("failed-journey receipt lost digest evidence or exposed protected contents")
			}
		})
	}
}

func TestAgentInstalledProfileProtectedReadbackBounds(t *testing.T) {
	root := t.TempDir()
	secretTarget := filepath.Join(t.TempDir(), "target-must-not-appear")
	if err := os.WriteFile(secretTarget, []byte("unread-target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link, dangling, fifo := filepath.Join(root, "link"), filepath.Join(root, "dangling"), filepath.Join(root, "fifo")
	if err := os.Symlink(secretTarget, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretTarget+"-absent", dangling); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(root, "oversized")
	if err := os.WriteFile(large, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(large, nativeProfileReadbackMaxBytes+1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, kind, diagnostic string
	}{
		{"symlink", link, "symlink", ""},
		{"dangling-link", dangling, "symlink", ""},
		{"fifo-no-open", fifo, "unsupported", ""},
		{"oversized-no-partial-hash", large, "regular", "file-too-large"},
		{"lstat-error-not-absence", filepath.Join(secretTarget, "child"), "unknown", "lstat-failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := nativeProfileObserveProtectedPath(tc.path)
			if got.Type != tc.kind || got.Error != tc.diagnostic || got.Hash != "" {
				t.Fatalf("unsafe or incomplete readback: %+v", got)
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), secretTarget) || strings.Contains(string(data), "unread-target") {
				t.Fatal("diagnostic exposed a symlink target, raw error or contents")
			}
		})
	}
	boundary := filepath.Join(root, "exact-limit")
	if err := os.WriteFile(boundary, []byte(strings.Repeat("x", nativeProfileReadbackMaxBytes)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := nativeProfileObserveProtectedPath(boundary); got.Error != "" || got.Hash != secureRuntimeHash([]byte(strings.Repeat("x", nativeProfileReadbackMaxBytes))) {
		t.Fatal("exact bounded file was not completely hashed")
	}
}
