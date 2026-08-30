//go:build !windows

package collectorlifecycle

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCredentialFileOwnerAllowed(t *testing.T) {
	collectorUID := uint64(998)
	arbitraryUID := uint64(os.Geteuid()) + 1
	if arbitraryUID == collectorUID {
		arbitraryUID++
	}
	for _, test := range []struct {
		name     string
		ownerUID uint64
		allowed  *uint64
		want     bool
	}{
		{name: "root owned", ownerUID: 0, want: true},
		{name: "root owned with collector configured", ownerUID: 0, allowed: &collectorUID, want: true},
		{name: "configured collector owned", ownerUID: 998, allowed: &collectorUID, want: true},
		{name: "effective owner", ownerUID: uint64(os.Geteuid()), want: true},
		{name: "arbitrary owner without collector", ownerUID: arbitraryUID, want: false},
		{name: "arbitrary owner with collector", ownerUID: arbitraryUID, allowed: &collectorUID, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := credentialFileOwnerAllowed(test.ownerUID, test.allowed); got != test.want {
				t.Fatalf("credentialFileOwnerAllowed(%d) = %v, want %v", test.ownerUID, got, test.want)
			}
		})
	}
}

func TestReadPrivateBearerRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.token")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ownerUID := uint64(os.Geteuid())
	started := time.Now()
	if _, err := readPrivateBearer(path, &ownerUID); err == nil || !strings.Contains(err.Error(), "private regular file") {
		t.Fatalf("readPrivateBearer FIFO error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO rejection blocked for %s", elapsed)
	}
}

func TestOpenCredentialFileDescriptorCannotBeSymlinkSwapped(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "collector.token")
	if err := os.WriteFile(path, []byte("original-bearer"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := openCredentialFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	attacker := filepath.Join(directory, "attacker.token")
	if err := os.WriteFile(attacker, []byte("attacker-bearer"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(attacker, path); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original-bearer" {
		t.Fatalf("open descriptor read %q after path swap", got)
	}
}
