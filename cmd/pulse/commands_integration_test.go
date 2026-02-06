//go:build integration

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunServer(t *testing.T) {
	oldPort := metricsPort
	metricsPort = 0
	defer func() { metricsPort = oldPort }()

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("PULSE_FRONTEND_PORT", "0")

	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	// Test case: AllowedOrigins = "*"
	t.Setenv("PULSE_ALLOWED_ORIGINS", "*")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	captureOutput(func() {
		_ = runServer(ctx)
	})

	// Test case: Specific AllowedOrigins
	t.Setenv("PULSE_ALLOWED_ORIGINS", "http://localhost:3000")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	captureOutput(func() {
		_ = runServer(ctx2)
	})
}

func TestSIGHUP(t *testing.T) {
	oldPort := metricsPort
	metricsPort = 0
	defer func() { metricsPort = oldPort }()

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("PULSE_FRONTEND_PORT", "0")
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	captureOutput(func() {
		_ = runServer(ctx)
	})
}

func TestMainActual(t *testing.T) {
	oldPort := metricsPort
	metricsPort = 0
	defer func() { metricsPort = oldPort }()

	rootCmd.SetArgs([]string{"version"})
	main()

	oldExit := osExit
	defer func() { osExit = oldExit }()
	exitCode := 0
	osExit = func(code int) { exitCode = code }

	rootCmd.SetArgs([]string{"--invalid-flag"})
	captureOutput(func() {
		main()
	})
	assert.Equal(t, 1, exitCode)
}

func TestRunServer_HTTPS(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	t.Setenv("PULSE_HTTPS_ENABLED", "true")
	t.Setenv("PULSE_TLS_CERT_FILE", "nonexistent.crt")
	t.Setenv("PULSE_TLS_KEY_FILE", "nonexistent.key")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	captureOutput(func() {
		_ = runServer(ctx)
	})
}

func TestRunServer_ConfigReload(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("PULSE_FRONTEND_PORT", "0")
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	oldMetricsPort := metricsPort
	metricsPort = 0 // Use random port for metrics
	defer func() { metricsPort = oldMetricsPort }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- runServer(ctx)
	}()

	// Wait for server to start.
	time.Sleep(500 * time.Millisecond)

	// Trigger reload via SIGHUP.
	_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(200 * time.Millisecond)

	// Trigger mock reload if possible.
	mockEnv := filepath.Join(tempDir, "mock.env")
	require.NoError(t, os.WriteFile(mockEnv, []byte("PULSE_MOCK_MODE=true\n"), 0644))
	time.Sleep(200 * time.Millisecond)

	cancel()
	err := <-errChan
	assert.NoError(t, err)

	// Give time for pending file watcher events to complete before cleanup.
	time.Sleep(100 * time.Millisecond)
}

func TestMainCmd(t *testing.T) {
	// Root command without args should run runServer.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	oldRunE := rootCmd.RunE
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runServer(ctx)
	}
	defer func() { rootCmd.RunE = oldRunE }()

	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

func TestRunServer_AutoImportFail(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	// Setup auto-import env vars with invalid data that causes normalize error.
	t.Setenv("PULSE_INIT_CONFIG_DATA", "   ")
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	output := captureOutput(func() {
		_ = runServer(ctx)
	})
	assert.NotEmpty(t, output)
}

func TestRunServer_WebSocket(t *testing.T) {
	resetFlags()

	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	t.Setenv("FRONTEND_PORT", fmt.Sprintf("%d", port))
	t.Setenv("PULSE_AUTH_USER", "testuser")
	t.Setenv("PULSE_AUTH_PASS", "testpass")

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	sysConfig := map[string]any{
		"allowedOrigins": "*",
	}
	sysData, _ := json.Marshal(sysConfig)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "system.json"), sysData, 0644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = runServer(ctx)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start listening on port %d", port)
		}
		time.Sleep(50 * time.Millisecond)
	}

	url := fmt.Sprintf("ws://localhost:%d/ws", port)

	dialer := websocket.Dialer{}
	auth := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
	header := http.Header{}
	header.Add("Authorization", "Basic "+auth)

	conn, _, err := dialer.Dial(url, header)
	require.NoError(t, err)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected first websocket read to succeed, got: %v", err)
	}
}

func TestRunServer_AllowedOrigins(t *testing.T) {
	resetFlags()
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	sysConfig := map[string]any{
		"allowedOrigins": "example.com,foo.com",
	}
	sysData, _ := json.Marshal(sysConfig)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "system.json"), sysData, 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	captureOutput(func() {
		_ = runServer(ctx)
	})
}

func TestRunServer_FrontendFail(t *testing.T) {
	resetFlags()

	oldMetricsPort := metricsPort
	metricsPort = 0
	defer func() { metricsPort = oldMetricsPort }()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	defer l.Close()

	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("FRONTEND_PORT", fmt.Sprintf("%d", port))

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	createTestEncryptionKey(t, tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	output := captureOutput(func() {
		_ = runServer(ctx)
	})
	assert.Contains(t, output, "Failed to start HTTP server")
}
