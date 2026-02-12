package updates

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractHostAgentBinaries_ExtractsAndSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "bundle.tar.gz")
	targetDir := filepath.Join(tmpDir, "out")

	content := []byte("binary-content")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	writeFile := func(name string, mode int64, data []byte) {
		hdr := &tar.Header{
			Name: name,
			Mode: mode,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%s): %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("Write(%s): %v", name, err)
		}
	}

	writeSymlink := func(name, target string) {
		hdr := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeSymlink,
			Linkname: target,
			Mode:     0o777,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%s symlink): %v", name, err)
		}
	}

	// Relevant binary under bin/
	writeFile("bin/pulse-host-agent-linux-amd64", 0o644, content)
	// Irrelevant entries should be ignored
	writeFile("bin/not-a-host-agent", 0o644, []byte("ignore"))
	writeFile("docs/readme.txt", 0o644, []byte("ignore"))
	// Symlink entry should be created as-is
	writeSymlink("bin/pulse-host-agent-linux-amd64-link", "pulse-host-agent-linux-amd64")

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}

	if err := extractHostAgentBinaries(archivePath, targetDir); err != nil {
		t.Fatalf("extractHostAgentBinaries: %v", err)
	}

	extracted := filepath.Join(targetDir, "pulse-host-agent-linux-amd64")
	info, err := os.Stat(extracted)
	if err != nil {
		t.Fatalf("stat extracted: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable permissions, got %v", info.Mode())
	}

	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch")
	}

	linkPath := filepath.Join(targetDir, "pulse-host-agent-linux-amd64-link")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "pulse-host-agent-linux-amd64" {
		t.Fatalf("link target = %q", target)
	}
}
