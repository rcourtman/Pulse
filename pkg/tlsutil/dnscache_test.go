package tlsutil

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestDialContextWithCache_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := DialContextWithCache(ctx, "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContextWithCache: %v", err)
	}
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not accept connection")
	}
}

func TestDialContextWithCache_BadAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := DialContextWithCache(ctx, "tcp", "missing-port")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDialContextWithCache_UnresolvableHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := DialContextWithCache(ctx, "tcp", "this-domain-should-not-exist.invalid:80")
	if err == nil {
		t.Fatalf("expected error")
	}
}
