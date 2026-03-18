package api_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newIPv4HTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on tcp4 loopback (tests require local sockets): %v", err)
	}
	srv := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{Handler: handler},
	}
	srv.Start()
	return srv
}
