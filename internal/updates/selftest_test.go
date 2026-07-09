package updates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeVersionScript writes an executable script that mimics the pulse
// binary's --version output. It is deliberately NOT named "pulse" so local
// dev tooling that kills processes by that exact name can never race it.
func writeFakeVersionScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-pulse")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func TestSelfTestNewBinaryAcceptsMatchingVersion(t *testing.T) {
	path := writeFakeVersionScript(t, `echo "Pulse v9.9.9"`)
	for _, expected := range []string{"v9.9.9", "9.9.9"} {
		if err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), expected); err != nil {
			t.Fatalf("expected version %s to pass, got: %v", expected, err)
		}
	}
}

func TestSelfTestNewBinaryAcceptsPrereleaseVersion(t *testing.T) {
	path := writeFakeVersionScript(t, `echo "Pulse v9.9.9-rc.2"`)
	if err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), "v9.9.9-rc.2"); err != nil {
		t.Fatalf("expected prerelease version to pass, got: %v", err)
	}
	if err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), "v9.9.9"); err == nil {
		t.Fatal("expected v9.9.9 to be rejected when the binary reports v9.9.9-rc.2")
	}
}

func TestSelfTestNewBinaryRejectsVersionMismatch(t *testing.T) {
	path := writeFakeVersionScript(t, `echo "Pulse v9.9.9"`)
	err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "expected version v1.2.3") {
		t.Fatalf("expected version mismatch error, got: %v", err)
	}
}

func TestSelfTestNewBinaryRejectsUnstampedBuild(t *testing.T) {
	path := writeFakeVersionScript(t, `echo "Pulse dev"`)
	err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), "v1.2.3")
	if err == nil {
		t.Fatal("expected an unstamped build to be rejected when a version is expected")
	}
}

func TestSelfTestNewBinarySkipsVersionCheckWithoutExpectation(t *testing.T) {
	path := writeFakeVersionScript(t, `echo "Pulse dev"`)
	if err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), ""); err != nil {
		t.Fatalf("expected exec-only self-test to pass, got: %v", err)
	}
}

func TestSelfTestNewBinaryRejectsFailingBinary(t *testing.T) {
	path := writeFakeVersionScript(t, `exit 3`)
	err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "self-test") {
		t.Fatalf("expected self-test failure, got: %v", err)
	}
}

func TestSelfTestNewBinaryRejectsUnexecutableArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake-pulse")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write junk binary: %v", err)
	}
	err := selfTestNewBinary(context.Background(), path, filepath.Dir(path), "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "self-test") {
		t.Fatalf("expected exec-format self-test failure, got: %v", err)
	}
}

func TestLocateExtractedPulseBinary(t *testing.T) {
	t.Run("root layout", func(t *testing.T) {
		dir := t.TempDir()
		want := filepath.Join(dir, "pulse")
		if err := os.WriteFile(want, []byte("stub"), 0o755); err != nil {
			t.Fatalf("write stub: %v", err)
		}
		got, err := locateExtractedPulseBinary(dir)
		if err != nil || got != want {
			t.Fatalf("locate root layout: got %q, %v", got, err)
		}
	})

	t.Run("bin layout", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}
		want := filepath.Join(dir, "bin", "pulse")
		if err := os.WriteFile(want, []byte("stub"), 0o755); err != nil {
			t.Fatalf("write stub: %v", err)
		}
		got, err := locateExtractedPulseBinary(dir)
		if err != nil || got != want {
			t.Fatalf("locate bin layout: got %q, %v", got, err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := locateExtractedPulseBinary(t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "pulse binary not found") {
			t.Fatalf("expected not-found error, got: %v", err)
		}
	})
}

// buildTarballWithPulseBinary builds a gzipped tarball whose bin/pulse member
// holds the given bytes, for exercising the pre-install self-test through the
// full apply pipeline.
func buildTarballWithPulseBinary(t *testing.T, binary []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "bin/pulse",
		Mode: 0o755,
		Size: int64(len(binary)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatalf("write tar content: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}
