//go:build !linux

package main

import (
	"net"
	"os"

	"github.com/rs/zerolog/log"
)

// defaultExtractPeerCredentials is a stub for non-Linux systems
func defaultExtractPeerCredentials(conn net.Conn) (*peerCredentials, error) {
	// On non-Linux systems (like macOS dev), we can't easily get the peer credentials
	// from the socket. For development purposes, we'll assume the connection
	// comes from the current user.

	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())
	pid := uint32(os.Getpid())

	log.Debug().
		Uint32("uid", uid).
		Uint32("gid", gid).
		Msg("Peer credentials (STUB: using current process credentials)")

	return &peerCredentials{
		uid: uid,
		pid: pid,
		gid: gid,
	}, nil
}
