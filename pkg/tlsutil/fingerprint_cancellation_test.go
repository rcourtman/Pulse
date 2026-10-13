package tlsutil

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func testFetchFingerprintCancellation(t *testing.T) {
	t.Run("already-cancelled-does-not-dial", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fingerprint, err := FetchFingerprintContext(ctx, "https://"+listener.Addr().String())
		if fingerprint != "" || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled capture = %q, %v", fingerprint, err)
		}
		if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		connection, err := listener.Accept()
		if connection != nil {
			connection.Close()
			t.Fatal("already-cancelled capture connected to the peer")
		}
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("listener did not establish an empty accept queue: %v", err)
		}
	})
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode+"-closes-stalled-handshake", func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			entered := make(chan net.Conn, 1)
			peerDone := make(chan error, 1)
			go func() {
				connection, err := listener.Accept()
				if err != nil {
					peerDone <- err
					return
				}
				defer connection.Close()
				var first [1]byte
				if _, err := io.ReadFull(connection, first[:]); err != nil {
					peerDone <- err
					return
				}
				if first[0] != 0x16 {
					peerDone <- errors.New("probe did not start with a TLS handshake record")
					return
				}
				entered <- connection
				// Never send a ServerHello. EOF must come from the cancelled
				// client, not a fixture timeout or the listener being closed.
				_, err = io.Copy(io.Discard, connection)
				peerDone <- err
			}()
			ctx, cancel := context.WithCancel(context.Background())
			if mode == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
			}
			defer cancel()
			type result struct {
				fingerprint string
				err         error
			}
			done := make(chan result, 1)
			go func() {
				fingerprint, err := FetchFingerprintContext(ctx, "https://"+listener.Addr().String())
				done <- result{fingerprint, err}
			}()
			var connection net.Conn
			select {
			case connection = <-entered:
				defer connection.Close()
			case err := <-peerDone:
				t.Fatalf("peer did not reach the stalled TLS handshake: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("peer did not reach the stalled TLS handshake")
			}
			if mode == "cancel" {
				cancel()
			}
			<-ctx.Done()
			select {
			case got := <-done:
				if got.fingerprint != "" || !errors.Is(got.err, ctx.Err()) {
					t.Errorf("cancelled capture = %q, %v; want no certificate and %v", got.fingerprint, got.err, ctx.Err())
				}
			case <-time.After(time.Second):
				t.Error("cancelled capture did not return while the peer remained stalled")
				connection.Close()
				<-done
			}
			select {
			case err := <-peerDone:
				if err != nil {
					t.Errorf("cancelled client did not cleanly close its owned socket: %v", err)
				}
			case <-time.After(time.Second):
				connection.Close()
				<-peerDone
				t.Error("cancelled capture returned without releasing its socket")
			}
		})
	}
	t.Run("ordinary-capture-keeps-owned-leaf-and-legacy-helper", func(t *testing.T) {
		server, expected, requests := fingerprintAuthorityFixture(t, "tcp4", "127.0.0.1:0")
		for _, capture := range []func(string) (string, error){
			FetchFingerprint,
			func(host string) (string, error) { return FetchFingerprintContext(context.Background(), host) },
		} {
			fingerprint, err := capture(server.URL + "/proxy/base/")
			if err != nil || fingerprint != expected || requests.Load() != 0 {
				t.Fatalf("ordinary capture = %q, %v; HTTP requests=%d", fingerprint, err, requests.Load())
			}
		}
	})
}
