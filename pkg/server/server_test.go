package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestBusinessHooks(t *testing.T) {
	called := false
	hook := func(store *metrics.Store) {
		called = true
	}

	SetBusinessHooks(BusinessHooks{
		OnMetricsStoreReady: hook,
	})

	globalHooksMu.Lock()
	defer globalHooksMu.Unlock()

	if globalHooks.OnMetricsStoreReady == nil {
		t.Error("expected OnMetricsStoreReady to be set")
	}

	// Manually trigger to verify it works
	globalHooks.OnMetricsStoreReady(nil)
	if !called {
		t.Error("expected hook to be called")
	}

}

func TestPerformAutoImport_Success(t *testing.T) {
	capture := setCaptureAuditLogger(t)

	// Setup temp directory
	tmpDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tmpDir)

	// Create a persistence instance to generate valid encrypted payload
	sourceDir := t.TempDir()
	sourcePersistence := config.NewConfigPersistence(sourceDir)

	passphrase := "test-pass"
	encryptedData, err := sourcePersistence.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("failed to generate export data: %v", err)
	}

	t.Setenv("PULSE_INIT_CONFIG_DATA", encryptedData)
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", passphrase)

	// Run PerformAutoImport
	if err := PerformAutoImport(); err != nil {
		t.Fatalf("PerformAutoImport failed: %v", err)
	}

	if len(capture.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(capture.events))
	}

	event := capture.events[0]
	if event.EventType != "config_auto_import" {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	if !event.Success {
		t.Fatal("expected success audit event")
	}
	if event.User != "system" {
		t.Fatalf("unexpected audit user: %q", event.User)
	}
	if event.Path != "/startup/auto-import" {
		t.Fatalf("unexpected audit path: %q", event.Path)
	}
	if !strings.Contains(event.Details, "source=env_data") {
		t.Fatalf("expected source in details, got %q", event.Details)
	}

	// Verify persistence file created (nodes.enc is a good indicator)
	_, err = os.Stat(filepath.Join(tmpDir, "nodes.enc"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Error("expected nodes.enc to be created")
		} else {
			t.Error(err)
		}
	}
}

func availableTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func waitForHTTPStatus(t *testing.T, url string, want int) {
	t.Helper()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	var lastStatus int
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			lastStatus = resp.StatusCode
			resp.Body.Close()
			if lastStatus == want {
				return
			}
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("timed out waiting for %s: last error: %v", url, lastErr)
	}
	t.Fatalf("timed out waiting for %s: last status %d, want %d", url, lastStatus, want)
}

// Minimal test for Server startup context cancellation
func TestServerRun_Shutdown(t *testing.T) {
	// Setup minimal environment
	tmpDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tmpDir)
	t.Setenv("PULSE_CONFIG_PATH", tmpDir)
	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("FRONTEND_PORT", fmt.Sprintf("%d", availableTCPPort(t)))

	oldMetricsPort := MetricsPort
	MetricsPort = 0
	defer func() { MetricsPort = oldMetricsPort }()

	// Create a minimal config; environment variables own the listener ports for this test.
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("bindAddress: 127.0.0.1\nfrontendPort: 0"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately/shortly to trigger shutdown path
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := Run(ctx, "test-version")

	if err != nil && err != context.Canceled {
		t.Logf("Run returned: %v", err)
	}
}

func TestServerRunFailsFastWhenFrontendPortIsAlreadyBound(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	tmpDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tmpDir)
	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("FRONTEND_PORT", fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port))

	oldMetricsPort := MetricsPort
	MetricsPort = 0
	defer func() { MetricsPort = oldMetricsPort }()

	err = Run(context.Background(), "test-version")
	if err == nil || !strings.Contains(err.Error(), "failed to bind UI/API server") {
		t.Fatalf("expected frontend bind failure, got %v", err)
	}
}

func TestServerRunKeepsFrontendWhenMetricsPortConflicts(t *testing.T) {
	port := availableTCPPort(t)

	tmpDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tmpDir)
	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("FRONTEND_PORT", fmt.Sprintf("%d", port))

	oldMetricsPort := MetricsPort
	MetricsPort = port
	defer func() { MetricsPort = oldMetricsPort }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, "test-version")
	}()

	waitForHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/api/health", port), http.StatusOK)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run to shut down")
	}
}

func TestServerRun_RejectsWildcardTrustedProxyCIDR(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tmpDir)
	t.Setenv("PULSE_CONFIG_PATH", tmpDir)
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "0.0.0.0/0")

	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("bindAddress: 127.0.0.1\nfrontendPort: 0"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), "test-version")
	if err == nil || !strings.Contains(err.Error(), "wildcard trust range") {
		t.Fatalf("expected wildcard trusted proxy configuration to be rejected, got %v", err)
	}
}
