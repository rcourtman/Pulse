package main

import (
	"fmt"
	"net"
	"syscall"

	"github.com/rs/zerolog/log"
)

// verifyPeerCredentials checks if the connecting process is authorized
// Returns nil if authorized, error otherwise
func verifyPeerCredentials(conn net.Conn) error {
	// Get the underlying file descriptor
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a unix connection")
	}

	file, err := unixConn.File()
	if err != nil {
		return fmt.Errorf("failed to get file descriptor: %w", err)
	}
	defer file.Close()

	fd := int(file.Fd())

	// Get peer credentials using SO_PEERCRED
	cred, err := syscall.GetsockoptUcred(fd, syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return fmt.Errorf("failed to get peer credentials: %w", err)
	}

	log.Debug().
		Int32("pid", cred.Pid).
		Uint32("uid", cred.Uid).
		Uint32("gid", cred.Gid).
		Msg("Peer credentials")

	// Allow root (UID 0) - this covers most service scenarios
	if cred.Uid == 0 {
		return nil
	}

	// Allow the proxy's own user (for testing/debugging)
	if cred.Uid == uint32(syscall.Getuid()) {
		return nil
	}

	// Reject all other users
	return fmt.Errorf("unauthorized: uid=%d gid=%d", cred.Uid, cred.Gid)
}
