package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newIPv6HTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("cannot listen on tcp6 loopback (tests require local IPv6 sockets): %v", err)
	}
	srv := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{Handler: handler},
	}
	srv.Start()
	return srv
}
