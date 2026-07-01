package server

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestStartMetricsServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	if err := startMetricsServer(ctx, addr, "", false); err != nil {
		t.Fatalf("startMetricsServer returned error: %v", err)
	}

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	var status int
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/metrics")
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
			if status == http.StatusOK {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if status != http.StatusOK {
		t.Fatalf("expected metrics endpoint to respond, got status %d", status)
	}

	cancel()

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.Get("http://" + addr + "/metrics"); err != nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("expected metrics server to stop after context cancellation")
}

func TestStartMetricsServerRejectsBearerTokenOnRemotePlaintextByDefault(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := startMetricsServer(ctx, "0.0.0.0:9091", "secret-token", false)
	if err == nil {
		t.Fatal("expected remote plaintext metrics token listener to be rejected")
	}
}

func TestStartMetricsServerAllowsBearerTokenOnLoopback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	if err := startMetricsServer(ctx, addr, "secret-token", false); err != nil {
		t.Fatalf("startMetricsServer returned error: %v", err)
	}

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	var authorized bool
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/metrics", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer secret-token")
		resp, err := client.Do(req)
		if err == nil {
			authorized = resp.StatusCode == http.StatusOK
			resp.Body.Close()
			if authorized {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !authorized {
		t.Fatal("expected bearer-token metrics endpoint to allow loopback request")
	}
}
