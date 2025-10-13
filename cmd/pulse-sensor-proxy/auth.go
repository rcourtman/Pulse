package main

import (
	"fmt"
	"net"
	"syscall"

	"github.com/rs/zerolog/log"
)

// peerCredentials holds extracted credentials from SO_PEERCRED
type peerCredentials struct {
	uid uint32
	pid uint32
	gid uint32
}

// extractPeerCredentials extracts and verifies peer credentials
// Returns credentials if authorized, error otherwise
func extractPeerCredentials(conn net.Conn) (*peerCredentials, error) {
	// Get the underlying file descriptor
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("not a unix connection")
	}

	file, err := unixConn.File()
	if err != nil {
		return nil, fmt.Errorf("failed to get file descriptor: %w", err)
	}
	defer file.Close()

	fd := int(file.Fd())

	// Get peer credentials using SO_PEERCRED
	cred, err := syscall.GetsockoptUcred(fd, syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer credentials: %w", err)
	}

	log.Debug().
		Int32("pid", cred.Pid).
		Uint32("uid", cred.Uid).
		Uint32("gid", cred.Gid).
		Msg("Peer credentials")

	// Allow root (UID 0) - this covers most service scenarios
	if cred.Uid == 0 {
		return &peerCredentials{
			uid: cred.Uid,
			pid: uint32(cred.Pid),
			gid: cred.Gid,
		}, nil
	}

	// Allow the proxy's own user (for testing/debugging)
	if cred.Uid == uint32(syscall.Getuid()) {
		return &peerCredentials{
			uid: cred.Uid,
			pid: uint32(cred.Pid),
			gid: cred.Gid,
		}, nil
	}

	// Reject all other users
	return nil, fmt.Errorf("unauthorized: uid=%d gid=%d", cred.Uid, cred.Gid)
}

// verifyPeerCredentials checks if the connecting process is authorized (legacy function)
// Returns nil if authorized, error otherwise
func verifyPeerCredentials(conn net.Conn) error {
	_, err := extractPeerCredentials(conn)
	return err
}
