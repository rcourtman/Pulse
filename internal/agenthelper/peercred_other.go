//go:build !linux

package agenthelper

import (
	"errors"
	"net"
)

type PlatformPeerResolver struct{}

func (PlatformPeerResolver) Resolve(net.Conn) (Peer, error) {
	return Peer{}, errors.New("peer credentials are supported only on Linux")
}
