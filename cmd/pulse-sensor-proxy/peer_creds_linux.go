//go:build linux

package main

import (
	"fmt"
	"net"
	"syscall"

	"github.com/rs/zerolog/log"
)

// defaultExtractPeerCredentials extracts peer credentials via SO_PEERCRED
func defaultExtractPeerCredentials(conn net.Conn) (*peerCredentials, error) {
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

	cred, err := syscall.GetsockoptUcred(fd, syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer credentials: %w", err)
	}

	log.Debug().
		Int32("pid", cred.Pid).
		Uint32("uid", cred.Uid).
		Uint32("gid", cred.Gid).
		Msg("Peer credentials")

	return &peerCredentials{
		uid: cred.Uid,
		pid: uint32(cred.Pid),
		gid: cred.Gid,
	}, nil
}
