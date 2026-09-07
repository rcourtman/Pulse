//go:build linux

package filesystemprobe

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

func TestFilesystemCountersPreserveFullAndReservedSpace(t *testing.T) {
	stat := unix.Statfs_t{Blocks: 2048, Bsize: 4096, Frsize: 4096, Bfree: 3, Bavail: 0, Files: 10, Ffree: 0}
	u, err := filesystemUsage(stat)
	if err != nil || u.CapacityBytes != 8<<20 || u.FreeBytes != 12288 || u.AvailableBytes != 0 || u.Inodes == nil || u.Inodes.Free != 0 {
		t.Fatalf("counters: %+v %v", u, err)
	}
	stat.Files = 0
	stat.Ffree = math.MaxUint64
	u, err = filesystemUsage(stat)
	if err != nil || u.Inodes != nil {
		t.Fatalf("unsupported inode inventory invalidated capacity: %+v %v", u, err)
	}
	stat.Blocks = math.MaxUint64
	if _, err := filesystemUsage(stat); err == nil {
		t.Fatal("overflow accepted")
	}
	stat.Blocks = 1
	stat.Bfree = 2
	if _, err := filesystemUsage(stat); err == nil {
		t.Fatal("inconsistent free counters accepted")
	}
}

func TestMountResolutionStaysWithinResourceRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "inside"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/inside", filepath.Join(dir, "absolute")); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Open(dir, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	for _, mount := range []string{"/", "/inside", "/absolute"} {
		kind, u, err := observePath(fd, mount)
		if err != nil || u == nil || kind == "" {
			t.Fatalf("confined mount %s: %s %+v %v", mount, kind, u, err)
		}
	}
	if err := os.Symlink("/etc", filepath.Join(dir, "outside")); err != nil {
		t.Fatal(err)
	}
	if _, u, err := observePath(fd, "/outside"); err == nil || u != nil {
		t.Fatal("absolute symlink escaped resource root")
	}
}

func TestMountResolutionRejectsProcMagicLinks(t *testing.T) {
	root, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(root)
	if _, u, err := observePath(root, "/proc/self/fd/"+strconv.Itoa(root)); err == nil || u != nil {
		t.Fatal("proc magic link accepted")
	}
}

func TestWrongProcessCannotSupplyContainerCapacity(t *testing.T) {
	r := testRequest()
	r.PID = os.Getpid()
	if got, err := observeContainer(context.Background(), r); err == nil || got != nil {
		t.Fatalf("unrelated process supplied capacity: %+v %v", got, err)
	}
}
