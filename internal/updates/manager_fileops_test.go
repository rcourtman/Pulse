package updates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("write content: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write tarball: %v", err)
	}
}

func TestManagerExtractTarball(t *testing.T) {
	manager := &Manager{}
	src := filepath.Join(t.TempDir(), "update.tar.gz")
	dest := filepath.Join(t.TempDir(), "extract")

	writeTarGz(t, src, map[string]string{
		"bin/pulse": "binary",
	})

	if err := manager.extractTarball(src, dest); err != nil {
		t.Fatalf("extractTarball error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "bin", "pulse"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != "binary" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestManagerExtractTarballRejectsUnsafePaths(t *testing.T) {
	manager := &Manager{}
	src := filepath.Join(t.TempDir(), "bad.tar.gz")
	dest := filepath.Join(t.TempDir(), "extract")

	writeTarGz(t, src, map[string]string{
		"../evil": "nope",
	})

	if err := manager.extractTarball(src, dest); err == nil {
		t.Fatal("expected error for unsafe path")
	}
}

func TestManagerCopyFileSafe(t *testing.T) {
	manager := &Manager{}
	dir := t.TempDir()

	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(src, []byte("payload"), 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := manager.copyFileSafe(src, dest); err != nil {
		t.Fatalf("copyFileSafe error: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected dest content: %q", string(data))
	}

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(src, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	skipDest := filepath.Join(dir, "skip.txt")
	if err := manager.copyFileSafe(link, skipDest); err != nil {
		t.Fatalf("copyFileSafe symlink error: %v", err)
	}
	if _, err := os.Stat(skipDest); err == nil {
		t.Fatal("expected symlink copy to be skipped")
	}
}

func TestManagerCopyDirSafe(t *testing.T) {
	manager := &Manager{}
	srcDir := filepath.Join(t.TempDir(), "src")
	destDir := filepath.Join(t.TempDir(), "dest")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ok.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("write ok: %v", err)
	}
	if err := os.Symlink(filepath.Join(srcDir, "ok.txt"), filepath.Join(srcDir, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := manager.copyDirSafe(srcDir, destDir); err != nil {
		t.Fatalf("copyDirSafe error: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(destDir, "ok.txt")); err != nil {
		t.Fatalf("expected ok.txt copied: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(destDir, "link.txt")); err == nil {
		t.Fatal("expected symlink to be skipped")
	}
}
