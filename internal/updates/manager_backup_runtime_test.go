package updates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	godisk "github.com/shirou/gopsutil/v4/disk"
)

func TestPruneRetainedUpdateBackupsKeepsNewestAndClearsHistory(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)

	history, err := NewUpdateHistory(t.TempDir())
	if err != nil {
		t.Fatalf("NewUpdateHistory error: %v", err)
	}
	manager := &Manager{history: history}

	base := time.Date(2026, time.April, 22, 8, 0, 0, 0, time.UTC)
	backupPaths := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		backupDir := filepath.Join(dataDir, "backup-"+base.Add(time.Duration(i)*time.Minute).Format("20060102-150405"))
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			t.Fatalf("mkdir backup %d: %v", i, err)
		}
		modTime := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(backupDir, modTime, modTime); err != nil {
			t.Fatalf("chtimes backup %d: %v", i, err)
		}
		backupPaths = append(backupPaths, backupDir)
	}

	oldEventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
		Action:      "update",
		Status:      StatusSuccess,
		VersionFrom: "v6.0.0-rc.1",
		VersionTo:   "v6.0.0-rc.2",
		BackupPath:  backupPaths[0],
	})
	if err != nil {
		t.Fatalf("CreateEntry old error: %v", err)
	}
	newEventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
		Action:      "update",
		Status:      StatusSuccess,
		VersionFrom: "v6.0.0-rc.2",
		VersionTo:   "v6.0.0-rc.3",
		BackupPath:  backupPaths[3],
	})
	if err != nil {
		t.Fatalf("CreateEntry new error: %v", err)
	}

	if err := manager.pruneRetainedUpdateBackups(context.Background()); err != nil {
		t.Fatalf("pruneRetainedUpdateBackups error: %v", err)
	}

	if _, err := os.Stat(backupPaths[0]); !os.IsNotExist(err) {
		t.Fatalf("expected oldest backup to be pruned, stat err: %v", err)
	}
	for _, path := range backupPaths[1:] {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected retained backup %q, got %v", path, err)
		}
	}

	oldEntry, err := history.GetEntry(oldEventID)
	if err != nil {
		t.Fatalf("GetEntry old error: %v", err)
	}
	if oldEntry.BackupPath != "" {
		t.Fatalf("expected pruned backup path to be cleared, got %q", oldEntry.BackupPath)
	}
	if !strings.Contains(oldEntry.Notes, "Rollback backup pruned by retention") {
		t.Fatalf("expected retention note on old entry, got %q", oldEntry.Notes)
	}

	newEntry, err := history.GetEntry(newEventID)
	if err != nil {
		t.Fatalf("GetEntry new error: %v", err)
	}
	if filepath.Clean(newEntry.BackupPath) != filepath.Clean(backupPaths[3]) {
		t.Fatalf("expected newest backup path to remain, got %q", newEntry.BackupPath)
	}
}

func TestCreateBackupFallsBackToTmpWhenRuntimeDataDirIsTooFull(t *testing.T) {
	dataDir := t.TempDir()
	installDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	t.Setenv("PULSE_INSTALL_DIR", installDir)

	if err := os.MkdirAll(filepath.Join(installDir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "config"), 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "data", "payload.bin"), []byte(strings.Repeat("a", 1024)), 0600); err != nil {
		t.Fatalf("write data payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "config", "config.json"), []byte(`{"ok":true}`), 0600); err != nil {
		t.Fatalf("write config payload: %v", err)
	}

	originalDiskUsage := updateDiskUsage
	updateDiskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		cleaned := filepath.Clean(path)
		switch cleaned {
		case filepath.Clean(dataDir):
			return &godisk.UsageStat{Free: 1}, nil
		case "/tmp":
			return &godisk.UsageStat{Free: 1 << 30}, nil
		default:
			return &godisk.UsageStat{Free: 1 << 30}, nil
		}
	}
	t.Cleanup(func() { updateDiskUsage = originalDiskUsage })

	manager := &Manager{}
	backupDir, err := manager.createBackup(context.Background())
	if err != nil {
		t.Fatalf("createBackup error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(backupDir) })

	if !strings.HasPrefix(filepath.Clean(backupDir), filepath.Clean("/tmp/pulse-backup-")) {
		t.Fatalf("expected /tmp fallback backup path, got %q", backupDir)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "data", "payload.bin")); err != nil {
		t.Fatalf("expected backup data payload: %v", err)
	}
}

func TestCreateBackupFailsWhenNoManagedLocationHasSpace(t *testing.T) {
	dataDir := t.TempDir()
	installDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	t.Setenv("PULSE_INSTALL_DIR", installDir)

	if err := os.MkdirAll(filepath.Join(installDir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "data", "payload.bin"), []byte(strings.Repeat("a", 2048)), 0600); err != nil {
		t.Fatalf("write data payload: %v", err)
	}

	originalDiskUsage := updateDiskUsage
	updateDiskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{Free: 1}, nil
	}
	t.Cleanup(func() { updateDiskUsage = originalDiskUsage })

	manager := &Manager{}
	_, err := manager.createBackup(context.Background())
	if err == nil || !strings.Contains(err.Error(), "need") {
		t.Fatalf("expected insufficient space error, got %v", err)
	}
}

func TestEstimateTarballExtractBytesSumsRegularFiles(t *testing.T) {
	tarballPath := filepath.Join(t.TempDir(), "update.tar.gz")
	file, err := os.Create(tarballPath)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}

	gzWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzWriter)
	entries := []struct {
		Name string
		Body string
	}{
		{Name: "bin/pulse", Body: "abcdef"},
		{Name: "VERSION", Body: "v6.0.0-rc.3"},
	}

	for _, entry := range entries {
		if err := tarWriter.WriteHeader(&tar.Header{
			Name: entry.Name,
			Mode: 0600,
			Size: int64(len(entry.Body)),
		}); err != nil {
			t.Fatalf("write header %s: %v", entry.Name, err)
		}
		if _, err := tarWriter.Write([]byte(entry.Body)); err != nil {
			t.Fatalf("write body %s: %v", entry.Name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tarball file: %v", err)
	}

	extractBytes, err := estimateTarballExtractBytes(tarballPath)
	if err != nil {
		t.Fatalf("estimateTarballExtractBytes error: %v", err)
	}
	expected := int64(len(entries[0].Body) + len(entries[1].Body))
	if extractBytes != expected {
		t.Fatalf("estimateTarballExtractBytes = %d, want %d", extractBytes, expected)
	}
}
