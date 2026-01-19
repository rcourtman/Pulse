package tlsutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDialContextWithCache(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := DialContextWithCache(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("DialContextWithCache error: %v", err)
	}
	conn.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected server accept")
	}
}

func TestFetchFingerprint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cert := server.TLS.Certificates[0]
	if len(cert.Certificate) == 0 {
		t.Fatal("expected server certificate")
	}

	sum := sha256.Sum256(cert.Certificate[0])
	expected := hex.EncodeToString(sum[:])

	fingerprint, err := FetchFingerprint(server.URL)
	if err != nil {
		t.Fatalf("FetchFingerprint error: %v", err)
	}
	if fingerprint != expected {
		t.Fatalf("unexpected fingerprint: %s", fingerprint)
	}
}
