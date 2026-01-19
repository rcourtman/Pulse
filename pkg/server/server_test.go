package server

import (
	"context"
	"os"
	"path/filepath"
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

// Minimal test for Server startup context cancellation
func TestServerRun_Shutdown(t *testing.T) {
	// Setup minimal environment
	tmpDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tmpDir)
	t.Setenv("PULSE_CONFIG_PATH", tmpDir)

	// Create a dummy config.yaml
	configFile := filepath.Join(tmpDir, "config.yaml")
	// Use 0 port to try to avoid conflicts, though Run() might default it.
	if err := os.WriteFile(configFile, []byte("backendHost: 127.0.0.1\nfrontendPort: 0"), 0644); err != nil {
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
