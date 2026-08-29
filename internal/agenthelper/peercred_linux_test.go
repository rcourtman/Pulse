//go:build linux

package agenthelper

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformPeerResolverReadsSOPEERCRED(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	result := make(chan struct {
		peer Peer
		err  error
	}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- struct {
				peer Peer
				err  error
			}{err: err}
			return
		}
		defer conn.Close()
		peer, err := (PlatformPeerResolver{}).Resolve(conn)
		result <- struct {
			peer Peer
			err  error
		}{peer: peer, err: err}
	}()

	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	resolved := <-result
	if resolved.err != nil {
		t.Fatalf("Resolve: %v", resolved.err)
	}
	if resolved.peer.UID != uint32(os.Geteuid()) || resolved.peer.GID != uint32(os.Getegid()) {
		t.Fatalf("peer = %#v, current uid=%d gid=%d", resolved.peer, os.Geteuid(), os.Getegid())
	}
	if resolved.peer.PID != int32(os.Getpid()) {
		t.Fatalf("peer pid = %d, want %d", resolved.peer.PID, os.Getpid())
	}
}
