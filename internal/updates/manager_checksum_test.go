package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestManagerVerifyChecksum(t *testing.T) {
	tarballPath := filepath.Join(t.TempDir(), "pulse.tar.gz")
	if err := os.WriteFile(tarballPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write tarball: %v", err)
	}
	sum := sha256.Sum256([]byte("payload"))
	checksum := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "SHA256SUMS") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(checksum + "  pulse.tar.gz\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	manager := NewManager(nil)
	tarballURL := server.URL + "/pulse.tar.gz"

	if err := manager.verifyChecksum(context.Background(), tarballURL, tarballPath); err != nil {
		t.Fatalf("verifyChecksum error: %v", err)
	}
}

func TestUpdaterPrepareUpdateInstructions(t *testing.T) {
	ctx := context.Background()

	docker := NewDockerUpdater()
	plan, err := docker.PrepareUpdate(ctx, UpdateRequest{Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("docker PrepareUpdate error: %v", err)
	}
	if plan.CanAutoUpdate || len(plan.Instructions) == 0 {
		t.Fatalf("unexpected docker plan: %+v", plan)
	}

	aur := NewAURUpdater()
	plan, err = aur.PrepareUpdate(ctx, UpdateRequest{Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("aur PrepareUpdate error: %v", err)
	}
	if plan.CanAutoUpdate || len(plan.Instructions) == 0 {
		t.Fatalf("unexpected aur plan: %+v", plan)
	}
}

func TestUpdaterExecuteRollbackErrors(t *testing.T) {
	ctx := context.Background()

	if err := NewDockerUpdater().Execute(ctx, UpdateRequest{}, nil); err == nil {
		t.Fatal("expected docker Execute error")
	}
	if err := NewDockerUpdater().Rollback(ctx, "event"); err == nil {
		t.Fatal("expected docker Rollback error")
	}

	if err := NewAURUpdater().Execute(ctx, UpdateRequest{}, nil); err == nil {
		t.Fatal("expected aur Execute error")
	}
	if err := NewAURUpdater().Rollback(ctx, "event"); err == nil {
		t.Fatal("expected aur Rollback error")
	}
}

func TestManagerCloseAndBackup(t *testing.T) {
	manager := NewManager(&config.Config{})
	manager.Close()
	manager.Close()

	// Ensure post-close status updates are no-ops for broadcast/progress fan-out.
	manager.updateStatus("completed", 100, "done")

	select {
	case _, ok := <-manager.GetProgressChannel():
		if ok {
			t.Fatal("expected progress channel to be closed")
		}
	default:
		t.Fatal("expected closed progress channel")
	}

	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	installDir := t.TempDir()
	t.Setenv("PULSE_INSTALL_DIR", installDir)

	dataDir := filepath.Join(installDir, "data")
	configDir := filepath.Join(installDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dataDir)
		_ = os.RemoveAll(configDir)
	})

	if err := os.WriteFile(filepath.Join(dataDir, "data.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write data file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	backupDir, err := manager.createBackup()
	if err != nil {
		t.Fatalf("createBackup error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(backupDir)
	})

	if _, err := os.Stat(filepath.Join(backupDir, "data", "data.txt")); err != nil {
		t.Fatalf("expected data backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "config", "config.txt")); err != nil {
		t.Fatalf("expected config backup: %v", err)
	}
}
