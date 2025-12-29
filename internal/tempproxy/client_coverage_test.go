package tempproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type writeErrConn struct {
	net.Conn
}

func (c writeErrConn) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func startUnixServer(t *testing.T, handler func(net.Conn)) (string, func()) {
	t.Helper()

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "proxy.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()

	cleanup := func() {
		ln.Close()
		<-done
	}
	return socketPath, cleanup
}

func startRPCServer(t *testing.T, handler func(RPCRequest) RPCResponse) (string, func()) {
	t.Helper()
	return startUnixServer(t, func(conn net.Conn) {
		defer conn.Close()
		decoder := json.NewDecoder(conn)
		var req RPCRequest
		if err := decoder.Decode(&req); err != nil {
			return
		}
		resp := handler(req)
		encoder := json.NewEncoder(conn)
		_ = encoder.Encode(resp)
	})
}

func TestNewClientSocketSelection(t *testing.T) {
	origStat := statFn
	t.Cleanup(func() { statFn = origStat })

	t.Run("EnvOverride", func(t *testing.T) {
		origEnv := os.Getenv("PULSE_SENSOR_PROXY_SOCKET")
		t.Cleanup(func() {
			if origEnv == "" {
				os.Unsetenv("PULSE_SENSOR_PROXY_SOCKET")
			} else {
				os.Setenv("PULSE_SENSOR_PROXY_SOCKET", origEnv)
			}
		})
		os.Setenv("PULSE_SENSOR_PROXY_SOCKET", "/tmp/custom.sock")
		statFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

		client := NewClient()
		if client.socketPath != "/tmp/custom.sock" {
			t.Fatalf("expected env socket, got %q", client.socketPath)
		}
	})

	t.Run("DefaultExists", func(t *testing.T) {
		statFn = func(path string) (os.FileInfo, error) {
			if path == defaultSocketPath {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		client := NewClient()
		if client.socketPath != defaultSocketPath {
			t.Fatalf("expected default socket, got %q", client.socketPath)
		}
	})

	t.Run("ContainerExists", func(t *testing.T) {
		statFn = func(path string) (os.FileInfo, error) {
			if path == containerSocketPath {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}

		client := NewClient()
		if client.socketPath != containerSocketPath {
			t.Fatalf("expected container socket, got %q", client.socketPath)
		}
	})

	t.Run("FallbackDefault", func(t *testing.T) {
		statFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		client := NewClient()
		if client.socketPath != defaultSocketPath {
			t.Fatalf("expected fallback socket, got %q", client.socketPath)
		}
	})
}

func TestClientIsAvailable(t *testing.T) {
	origStat := statFn
	t.Cleanup(func() { statFn = origStat })

	client := &Client{socketPath: "/tmp/socket"}
	statFn = func(string) (os.FileInfo, error) { return nil, nil }
	if !client.IsAvailable() {
		t.Fatalf("expected available")
	}

	statFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	if client.IsAvailable() {
		t.Fatalf("expected unavailable")
	}
}

func TestCallOnceSuccessAndDeadline(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		if req.Method != "ping" {
			t.Fatalf("unexpected method: %s", req.Method)
		}
		return RPCResponse{Success: true}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}

	resp, err := client.callOnce(context.Background(), "ping", nil)
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("expected success: resp=%v err=%v", resp, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	resp, err = client.callOnce(ctx, "ping", nil)
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("expected success with deadline: resp=%v err=%v", resp, err)
	}
}

func TestCallOnceErrors(t *testing.T) {
	t.Run("ConnectError", func(t *testing.T) {
		client := &Client{socketPath: "/tmp/missing.sock", timeout: 10 * time.Millisecond}
		if _, err := client.callOnce(context.Background(), "ping", nil); err == nil {
			t.Fatalf("expected connect error")
		}
	})

	t.Run("EncodeError", func(t *testing.T) {
		socketPath, cleanup := startUnixServer(t, func(conn net.Conn) {
			conn.Close()
		})
		t.Cleanup(cleanup)

		client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
		if _, err := client.callOnce(context.Background(), "ping", nil); err == nil {
			t.Fatalf("expected encode error")
		}
	})

	t.Run("EncodeErrorDial", func(t *testing.T) {
		origDial := dialContextFn
		t.Cleanup(func() { dialContextFn = origDial })

		clientConn, serverConn := net.Pipe()
		t.Cleanup(func() { _ = serverConn.Close() })

		dialContextFn = func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
			return writeErrConn{Conn: clientConn}, nil
		}

		client := &Client{socketPath: "/tmp/unused.sock", timeout: 50 * time.Millisecond}
		if _, err := client.callOnce(context.Background(), "ping", nil); err == nil {
			t.Fatalf("expected encode error")
		}
	})

	t.Run("DecodeError", func(t *testing.T) {
		socketPath, cleanup := startUnixServer(t, func(conn net.Conn) {
			defer conn.Close()
			_, _ = conn.Write([]byte("{"))
		})
		t.Cleanup(cleanup)

		client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
		if _, err := client.callOnce(context.Background(), "ping", nil); err == nil {
			t.Fatalf("expected decode error")
		}
	})
}

func TestCallWithContextBehavior(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
			return RPCResponse{Success: true}
		})
		t.Cleanup(cleanup)

		client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
		resp, err := client.callWithContext(context.Background(), "ping", nil)
		if err != nil || resp == nil || !resp.Success {
			t.Fatalf("expected success: resp=%v err=%v", resp, err)
		}
	})

	t.Run("NonRetryable", func(t *testing.T) {
		socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
			return RPCResponse{Success: false, Error: "unauthorized"}
		})
		t.Cleanup(cleanup)

		client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
		if _, err := client.callWithContext(context.Background(), "ping", nil); err == nil {
			t.Fatalf("expected auth error")
		}
	})

	t.Run("CancelledBeforeRetry", func(t *testing.T) {
		client := &Client{socketPath: "/tmp/missing.sock", timeout: 10 * time.Millisecond}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if _, err := client.callWithContext(ctx, "ping", nil); err == nil {
			t.Fatalf("expected cancelled error")
		}
	})

	t.Run("CancelledDuringBackoff", func(t *testing.T) {
		client := &Client{socketPath: "/tmp/missing.sock", timeout: 10 * time.Millisecond}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		if _, err := client.callWithContext(ctx, "ping", nil); err == nil {
			t.Fatalf("expected backoff cancel error")
		}
	})

	t.Run("MaxRetriesExhausted", func(t *testing.T) {
		client := &Client{socketPath: "/tmp/missing.sock", timeout: 10 * time.Millisecond}
		if _, err := client.callWithContext(context.Background(), "ping", nil); err == nil {
			t.Fatalf("expected retry error")
		}
	})
}

func TestGetStatus(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{"ok": true}}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	data, err := client.GetStatus()
	if err != nil || data["ok"] != true {
		t.Fatalf("unexpected status: %v err=%v", data, err)
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: "boom"}
	})
	t.Cleanup(cleanup)

	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.GetStatus(); err == nil {
		t.Fatalf("expected status error")
	}
}

func TestRegisterNodes(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{
			"nodes": []interface{}{map[string]interface{}{"name": "node1"}},
		}}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	nodes, err := client.RegisterNodes()
	if err != nil || len(nodes) != 1 || nodes[0]["name"] != "node1" {
		t.Fatalf("unexpected nodes: %v err=%v", nodes, err)
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: "unauthorized"}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.RegisterNodes(); err == nil {
		t.Fatalf("expected proxy error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{}}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.RegisterNodes(); err == nil {
		t.Fatalf("expected missing nodes error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{"nodes": "bad"}}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.RegisterNodes(); err == nil {
		t.Fatalf("expected nodes type error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{"nodes": []interface{}{"bad"}}}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.RegisterNodes(); err == nil {
		t.Fatalf("expected node map error")
	}
}

func TestGetTemperature(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{"temperature": "42"}}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	temp, err := client.GetTemperature("node1")
	if err != nil || temp != "42" {
		t.Fatalf("unexpected temp: %q err=%v", temp, err)
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: "unauthorized"}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.GetTemperature("node1"); err == nil {
		t.Fatalf("expected proxy error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: "boom"}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.GetTemperature("node1"); err == nil {
		t.Fatalf("expected unknown error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{}}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.GetTemperature("node1"); err == nil {
		t.Fatalf("expected missing temperature error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true, Data: map[string]interface{}{"temperature": 12}}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.GetTemperature("node1"); err == nil {
		t.Fatalf("expected type error")
	}
}

func TestRequestCleanup(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		if req.Method != "request_cleanup" {
			t.Fatalf("unexpected method: %s", req.Method)
		}
		if req.Params["host"] != "node1" {
			t.Fatalf("unexpected params: %v", req.Params)
		}
		return RPCResponse{Success: true}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if err := client.RequestCleanup("node1"); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: "boom"}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if err := client.RequestCleanup(""); err == nil {
		t.Fatalf("expected proxy error")
	}

	socketPath, cleanup = startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: ""}
	})
	t.Cleanup(cleanup)
	client = &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if err := client.RequestCleanup(""); err == nil {
		t.Fatalf("expected rejected error")
	}

	client = &Client{socketPath: "/tmp/missing.sock", timeout: 10 * time.Millisecond}
	if err := client.RequestCleanup(""); err == nil {
		t.Fatalf("expected call error")
	}
}

func TestCallWrapper(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: true}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	resp, err := client.call("ping", nil)
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("expected call success: resp=%v err=%v", resp, err)
	}
}

func TestCallWithContextRetryableResponseError(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: "ssh timeout"}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.callWithContext(context.Background(), "ping", nil); err == nil {
		t.Fatalf("expected retry exhaustion")
	}
}

func TestCallWithContextSuccessOnRetry(t *testing.T) {
	count := 0
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		count++
		if count == 1 {
			return RPCResponse{Success: false, Error: "ssh timeout"}
		}
		return RPCResponse{Success: true}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	resp, err := client.callWithContext(context.Background(), "ping", nil)
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("expected retry success: resp=%v err=%v", resp, err)
	}
}

func TestCallWithContextRespErrorString(t *testing.T) {
	socketPath, cleanup := startRPCServer(t, func(req RPCRequest) RPCResponse {
		return RPCResponse{Success: false, Error: strings.Repeat("x", 1)}
	})
	t.Cleanup(cleanup)

	client := &Client{socketPath: socketPath, timeout: 50 * time.Millisecond}
	if _, err := client.callWithContext(context.Background(), "ping", nil); err == nil {
		t.Fatalf("expected error")
	}
}
