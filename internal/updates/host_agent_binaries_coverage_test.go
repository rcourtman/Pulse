package updates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type hostAgentRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f hostAgentRoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type tarEntry struct {
	name     string
	body     []byte
	mode     int64
	typeflag byte
	linkname string
}

type errorReader struct{}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read fail")
}

func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		size := int64(len(entry.body))
		if typeflag != tar.TypeReg {
			size = 0
		}
		hdr := &tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Size:     size,
			Typeflag: typeflag,
			Linkname: entry.linkname,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if typeflag == tar.TypeReg {
			if _, err := tw.Write(entry.body); err != nil {
				t.Fatalf("Write: %v", err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func saveHostAgentHooks() func() {
	origRequired := requiredHostAgentBinaries
	origDownloadFn := downloadAndInstallHostAgentBinariesFn
	origFindMissing := findMissingHostAgentBinariesFn
	origURL := downloadURLForVersion
	origChecksumURL := checksumURLForVersion
	origClient := httpClient
	origMkdirAll := mkdirAllFn
	origCreateTemp := createTempFn
	origRemove := removeFn
	origOpen := openFileFn
	origOpenMode := openFileModeFn
	origRename := renameFn
	origSymlink := symlinkFn
	origCopy := copyFn
	origChmod := chmodFileFn
	origClose := closeFileFn

	return func() {
		requiredHostAgentBinaries = origRequired
		downloadAndInstallHostAgentBinariesFn = origDownloadFn
		findMissingHostAgentBinariesFn = origFindMissing
		downloadURLForVersion = origURL
		checksumURLForVersion = origChecksumURL
		httpClient = origClient
		mkdirAllFn = origMkdirAll
		createTempFn = origCreateTemp
		removeFn = origRemove
		openFileFn = origOpen
		openFileModeFn = origOpenMode
		renameFn = origRename
		symlinkFn = origSymlink
		copyFn = origCopy
		chmodFileFn = origChmod
		closeFileFn = origClose
	}
}

func TestEnsureHostAgentBinaries_NoMissing(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	requiredHostAgentBinaries = []HostAgentBinary{
		{Platform: "linux", Arch: "amd64", Filenames: []string{"pulse-host-agent-linux-amd64"}},
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "pulse-host-agent-linux-amd64"), []byte("bin"), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}

	origEnv := os.Getenv("PULSE_BIN_DIR")
	t.Cleanup(func() {
		if origEnv == "" {
			os.Unsetenv("PULSE_BIN_DIR")
		} else {
			os.Setenv("PULSE_BIN_DIR", origEnv)
		}
	})
	os.Setenv("PULSE_BIN_DIR", tmpDir)

	if missing := EnsureHostAgentBinaries("v1.0.0"); missing != nil {
		t.Fatalf("expected no missing binaries, got %v", missing)
	}
}

func TestEnsureHostAgentBinaries_DownloadError(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	requiredHostAgentBinaries = []HostAgentBinary{
		{Platform: "linux", Arch: "amd64", Filenames: []string{"pulse-host-agent-linux-amd64"}},
	}
	downloadAndInstallHostAgentBinariesFn = func(string, string) error {
		return errors.New("download failed")
	}

	origEnv := os.Getenv("PULSE_BIN_DIR")
	t.Cleanup(func() {
		if origEnv == "" {
			os.Unsetenv("PULSE_BIN_DIR")
		} else {
			os.Setenv("PULSE_BIN_DIR", origEnv)
		}
	})
	os.Setenv("PULSE_BIN_DIR", tmpDir)

	missing := EnsureHostAgentBinaries("v1.0.0")
	if len(missing) != 1 {
		t.Fatalf("expected missing map, got %v", missing)
	}
}

func TestEnsureHostAgentBinaries_StillMissing(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	requiredHostAgentBinaries = []HostAgentBinary{
		{Platform: "linux", Arch: "amd64", Filenames: []string{"pulse-host-agent-linux-amd64"}},
	}
	downloadAndInstallHostAgentBinariesFn = func(string, string) error { return nil }

	origEnv := os.Getenv("PULSE_BIN_DIR")
	t.Cleanup(func() {
		if origEnv == "" {
			os.Unsetenv("PULSE_BIN_DIR")
		} else {
			os.Setenv("PULSE_BIN_DIR", origEnv)
		}
	})
	os.Setenv("PULSE_BIN_DIR", tmpDir)

	missing := EnsureHostAgentBinaries("v1.0.0")
	if len(missing) != 1 {
		t.Fatalf("expected still missing")
	}
}

func TestEnsureHostAgentBinaries_RecheckAfterLock(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	requiredHostAgentBinaries = []HostAgentBinary{
		{Platform: "linux", Arch: "amd64", Filenames: []string{"pulse-host-agent-linux-amd64"}},
	}

	calls := 0
	findMissingHostAgentBinariesFn = func([]string) map[string]HostAgentBinary {
		calls++
		if calls == 1 {
			return map[string]HostAgentBinary{
				"linux-amd64": requiredHostAgentBinaries[0],
			}
		}
		return nil
	}

	origEnv := os.Getenv("PULSE_BIN_DIR")
	t.Cleanup(func() {
		if origEnv == "" {
			os.Unsetenv("PULSE_BIN_DIR")
		} else {
			os.Setenv("PULSE_BIN_DIR", origEnv)
		}
	})
	os.Setenv("PULSE_BIN_DIR", t.TempDir())

	if result := EnsureHostAgentBinaries("v1.0.0"); result != nil {
		t.Fatalf("expected nil after recheck, got %v", result)
	}
}

func TestEnsureHostAgentBinaries_RestoreSuccess(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	requiredHostAgentBinaries = []HostAgentBinary{
		{Platform: "linux", Arch: "amd64", Filenames: []string{"pulse-host-agent-linux-amd64"}},
	}
	downloadAndInstallHostAgentBinariesFn = func(string, string) error {
		return os.WriteFile(filepath.Join(tmpDir, "pulse-host-agent-linux-amd64"), []byte("bin"), 0o755)
	}

	origEnv := os.Getenv("PULSE_BIN_DIR")
	t.Cleanup(func() {
		if origEnv == "" {
			os.Unsetenv("PULSE_BIN_DIR")
		} else {
			os.Setenv("PULSE_BIN_DIR", origEnv)
		}
	})
	os.Setenv("PULSE_BIN_DIR", tmpDir)

	if missing := EnsureHostAgentBinaries("v1.0.0"); missing != nil {
		t.Fatalf("expected restore success")
	}
}

func TestDownloadAndInstallHostAgentBinariesErrors(t *testing.T) {
	t.Run("MkdirAllError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		mkdirAllFn = func(string, os.FileMode) error { return errors.New("mkdir fail") }
		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected mkdir error")
		}
	})

	t.Run("CreateTempError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		createTempFn = func(string, string) (*os.File, error) { return nil, errors.New("temp fail") }
		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected temp error")
		}
	})

	t.Run("DownloadError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		httpClient = &http.Client{
			Transport: hostAgentRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network")
			}),
		}
		downloadURLForVersion = func(string) string { return "http://example/bundle.tar.gz" }
		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected download error")
		}
	})

	t.Run("StatusError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		downloadURLForVersion = func(string) string { return server.URL + "/bundle.tar.gz" }

		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected status error")
		}
	})

	t.Run("CopyError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("data"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		downloadURLForVersion = func(string) string { return server.URL + "/bundle.tar.gz" }
		copyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy fail") }

		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected copy error")
		}
	})

	t.Run("CopyReadError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		httpClient = &http.Client{
			Transport: hostAgentRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				body := io.NopCloser(&errorReader{})
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       body,
				}, nil
			}),
		}
		downloadURLForVersion = func(string) string { return "http://example/bundle.tar.gz" }

		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected copy read error")
		}
	})

	t.Run("CloseError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("data"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		downloadURLForVersion = func(string) string { return server.URL + "/bundle.tar.gz" }
		closeFileFn = func(*os.File) error { return errors.New("close fail") }

		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected close error")
		}
	})

	t.Run("ExtractError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		payload := []byte("not a tarball")
		checksum := sha256.Sum256(payload)
		checksumLine := fmt.Sprintf("%x  bundle.tar.gz\n", checksum)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/bundle.tar.gz":
				_, _ = w.Write(payload)
			case "/bundle.tar.gz.sha256":
				_, _ = w.Write([]byte(checksumLine))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		downloadURLForVersion = func(string) string { return server.URL + "/bundle.tar.gz" }

		if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
			t.Fatalf("expected extract error")
		}
	})
}

func TestDownloadAndInstallHostAgentBinariesSuccess(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	payload := buildTarGz(t, []tarEntry{
		{name: "README.md", body: []byte("skip"), mode: 0o644},
		{name: "bin/", typeflag: tar.TypeDir, mode: 0o755},
		{name: "bin/not-agent.txt", body: []byte("skip"), mode: 0o644},
		{name: "bin/pulse-host-agent-linux-amd64", body: []byte("binary"), mode: 0o644},
		{name: "bin/pulse-host-agent-linux-amd64.exe", typeflag: tar.TypeSymlink, linkname: "pulse-host-agent-linux-amd64"},
	})
	checksum := sha256.Sum256(payload)
	checksumLine := fmt.Sprintf("%x  bundle.tar.gz\n", checksum)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundle.tar.gz":
			_, _ = w.Write(payload)
		case "/bundle.tar.gz.sha256":
			_, _ = w.Write([]byte(checksumLine))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	httpClient = server.Client()
	downloadURLForVersion = func(string) string { return server.URL + "/bundle.tar.gz" }
	symlinkFn = func(string, string) error { return errors.New("no symlink") }

	if err := DownloadAndInstallHostAgentBinaries("v1.0.0", tmpDir); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "pulse-host-agent-linux-amd64")); err != nil {
		t.Fatalf("expected binary installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "pulse-host-agent-linux-amd64.exe")); err != nil {
		t.Fatalf("expected symlink fallback copy: %v", err)
	}
}

func TestExtractHostAgentBinariesErrors(t *testing.T) {
	t.Run("OpenError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		openFileFn = func(string) (*os.File, error) { return nil, errors.New("open fail") }
		if err := extractHostAgentBinaries("missing.tar.gz", t.TempDir()); err == nil {
			t.Fatalf("expected open error")
		}
	})

	t.Run("GzipError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		tmp := filepath.Join(t.TempDir(), "bad.gz")
		if err := os.WriteFile(tmp, []byte("bad"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := extractHostAgentBinaries(tmp, t.TempDir()); err == nil {
			t.Fatalf("expected gzip error")
		}
	})

	t.Run("TarReadError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		_, _ = gzw.Write([]byte("not a tar"))
		_ = gzw.Close()

		tmp := filepath.Join(t.TempDir(), "bad.tar.gz")
		if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := extractHostAgentBinaries(tmp, t.TempDir()); err == nil {
			t.Fatalf("expected tar read error")
		}
	})
}

func TestExtractHostAgentBinariesRemoveError(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	payload := buildTarGz(t, []tarEntry{
		{name: "bin/pulse-host-agent-linux-amd64", body: []byte("binary"), mode: 0o644},
		{name: "bin/pulse-host-agent-linux-amd64.exe", typeflag: tar.TypeSymlink, linkname: "pulse-host-agent-linux-amd64"},
	})

	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(archive, payload, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	removeFn = func(string) error { return errors.New("remove fail") }
	if err := extractHostAgentBinaries(archive, tmpDir); err == nil {
		t.Fatalf("expected remove error")
	}
}

func TestExtractHostAgentBinariesSymlinkCopyError(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	payload := buildTarGz(t, []tarEntry{
		{name: "bin/pulse-host-agent-linux-amd64", body: []byte("binary"), mode: 0o644},
		{name: "bin/pulse-host-agent-linux-amd64.exe", typeflag: tar.TypeSymlink, linkname: "pulse-host-agent-linux-amd64"},
	})
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(archive, payload, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	symlinkFn = func(string, string) error { return errors.New("no symlink") }
	openFileFn = func(path string) (*os.File, error) {
		if path == archive {
			return os.Open(path)
		}
		return nil, errors.New("open fail")
	}

	if err := extractHostAgentBinaries(archive, tmpDir); err == nil {
		t.Fatalf("expected symlink fallback error")
	}
}

func TestWriteHostAgentFileErrors(t *testing.T) {
	t.Run("MkdirAllError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		mkdirAllFn = func(string, os.FileMode) error { return errors.New("mkdir fail") }
		if err := writeHostAgentFile("dest", strings.NewReader("data"), 0o644); err == nil {
			t.Fatalf("expected mkdir error")
		}
	})

	t.Run("CreateTempError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		createTempFn = func(string, string) (*os.File, error) { return nil, errors.New("temp fail") }
		if err := writeHostAgentFile(filepath.Join(t.TempDir(), "dest"), strings.NewReader("data"), 0o644); err == nil {
			t.Fatalf("expected temp error")
		}
	})

	t.Run("CopyError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		copyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy fail") }
		if err := writeHostAgentFile(filepath.Join(t.TempDir(), "dest"), strings.NewReader("data"), 0o644); err == nil {
			t.Fatalf("expected copy error")
		}
	})

	t.Run("ChmodError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		chmodFileFn = func(*os.File, os.FileMode) error { return errors.New("chmod fail") }
		if err := writeHostAgentFile(filepath.Join(t.TempDir(), "dest"), strings.NewReader("data"), 0o644); err == nil {
			t.Fatalf("expected chmod error")
		}
	})

	t.Run("CloseError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		closeFileFn = func(*os.File) error { return errors.New("close fail") }
		if err := writeHostAgentFile(filepath.Join(t.TempDir(), "dest"), strings.NewReader("data"), 0o644); err == nil {
			t.Fatalf("expected close error")
		}
	})

	t.Run("RenameError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		renameFn = func(string, string) error { return errors.New("rename fail") }
		if err := writeHostAgentFile(filepath.Join(t.TempDir(), "dest"), strings.NewReader("data"), 0o644); err == nil {
			t.Fatalf("expected rename error")
		}
	})
}

func TestWriteHostAgentFileSuccess(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	dest := filepath.Join(t.TempDir(), "pulse-host-agent-linux-amd64")
	if err := writeHostAgentFile(dest, strings.NewReader("data"), 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected file written: %v", err)
	}
}

func TestCopyHostAgentFileErrors(t *testing.T) {
	t.Run("OpenError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		openFileFn = func(string) (*os.File, error) { return nil, errors.New("open fail") }
		if err := copyHostAgentFile("missing", filepath.Join(t.TempDir(), "dest")); err == nil {
			t.Fatalf("expected open error")
		}
	})

	t.Run("MkdirAllError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		src := filepath.Join(t.TempDir(), "src")
		if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		mkdirAllFn = func(string, os.FileMode) error { return errors.New("mkdir fail") }
		if err := copyHostAgentFile(src, filepath.Join(t.TempDir(), "dest")); err == nil {
			t.Fatalf("expected mkdir error")
		}
	})

	t.Run("OpenFileError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		src := filepath.Join(t.TempDir(), "src")
		if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		openFileModeFn = func(string, int, os.FileMode) (*os.File, error) {
			return nil, errors.New("create fail")
		}
		if err := copyHostAgentFile(src, filepath.Join(t.TempDir(), "dest")); err == nil {
			t.Fatalf("expected open file error")
		}
	})

	t.Run("CopyError", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		src := filepath.Join(t.TempDir(), "src")
		if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		copyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy fail") }
		if err := copyHostAgentFile(src, filepath.Join(t.TempDir(), "dest")); err == nil {
			t.Fatalf("expected copy error")
		}
	})
}

func TestCopyHostAgentFileSuccess(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest")
	if err := copyHostAgentFile(src, dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected dest file: %v", err)
	}
}

func TestExtractHostAgentBinariesWriteError(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	tmpDir := t.TempDir()
	payload := buildTarGz(t, []tarEntry{
		{name: "bin/pulse-host-agent-linux-amd64", body: []byte("binary"), mode: 0o644},
	})
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(archive, payload, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mkdirAllFn = func(string, os.FileMode) error { return errors.New("mkdir fail") }
	if err := extractHostAgentBinaries(archive, tmpDir); err == nil {
		t.Fatalf("expected write error")
	}
}

func TestDownloadAndInstallHostAgentBinaries_Context(t *testing.T) {
	restore := saveHostAgentHooks()
	t.Cleanup(restore)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Err() != nil {
			t.Fatalf("unexpected context error")
		}
		_, _ = w.Write([]byte("data"))
	}))
	t.Cleanup(server.Close)

	httpClient = server.Client()
	downloadURLForVersion = func(string) string { return server.URL + "/bundle.tar.gz" }
	copyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy fail") }

	if err := DownloadAndInstallHostAgentBinaries("v1.0.0", t.TempDir()); err == nil {
		t.Fatalf("expected copy error")
	}
}
