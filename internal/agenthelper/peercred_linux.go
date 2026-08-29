//go:build linux

package agenthelper

import (
	"errors"
	"fmt"
	"net"
	"syscall"
)

type PlatformPeerResolver struct{}

func (PlatformPeerResolver) Resolve(conn net.Conn) (Peer, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return Peer{}, errors.New("peer credential resolution requires a Unix connection")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return Peer{}, fmt.Errorf("access Unix connection: %w", err)
	}

	var (
		credentials *syscall.Ucred
		controlErr  error
	)
	if err := raw.Control(func(fd uintptr) {
		credentials, controlErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return Peer{}, fmt.Errorf("inspect Unix connection: %w", err)
	}
	if controlErr != nil {
		return Peer{}, fmt.Errorf("resolve SO_PEERCRED: %w", controlErr)
	}
	if credentials == nil {
		return Peer{}, errors.New("SO_PEERCRED returned no credentials")
	}
	return Peer{UID: credentials.Uid, GID: credentials.Gid, PID: credentials.Pid}, nil
}
