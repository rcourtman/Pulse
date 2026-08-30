//go:build linux

package dockeragent

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCollectorOwnsRootlessEndpointRequiresOwnedRuntimeSocket(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
	})

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(runtimeDir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if !collectorOwnsRootlessEndpoint("unix://" + socketPath) {
		t.Fatal("collector-owned rootless Unix socket was rejected")
	}
	if collectorOwnsRootlessEndpoint("unix:///var/run/docker.sock") {
		t.Fatal("rootful system socket was accepted")
	}
	if collectorOwnsRootlessEndpoint("tcp://127.0.0.1:2375") {
		t.Fatal("non-Unix runtime endpoint was accepted")
	}

	symlink := filepath.Join(runtimeDir, "linked.sock")
	if err := os.Symlink(socketPath, symlink); err != nil {
		t.Fatal(err)
	}
	if collectorOwnsRootlessEndpoint("unix://" + symlink) {
		t.Fatal("symlinked runtime endpoint was accepted")
	}
}
