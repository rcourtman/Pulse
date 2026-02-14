package updates

import (
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

func TestFileNameFromURL(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "empty",
			rawURL: "",
			want:   "",
		},
		{
			name:   "full URL with query",
			rawURL: "https://example.com/releases/pulse-v1.2.3.tar.gz?download=1",
			want:   "pulse-v1.2.3.tar.gz",
		},
		{
			name:   "relative path",
			rawURL: "releases/pulse-v1.2.3.tar.gz",
			want:   "pulse-v1.2.3.tar.gz",
		},
		{
			name:   "malformed URL falls back and strips query",
			rawURL: "%zz?download=1",
			want:   "%zz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fileNameFromURL(tc.rawURL)
			if got != tc.want {
				t.Fatalf("fileNameFromURL(%q) = %q, want %q", tc.rawURL, got, tc.want)
			}
		})
	}
}

func TestHashFileSHA256(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		path := filepath.Join(t.TempDir(), "bundle.tar.gz")
		payload := []byte("bundle")
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		got, err := hashFileSHA256(path)
		if err != nil {
			t.Fatalf("hashFileSHA256 returned error: %v", err)
		}

		want := fmt.Sprintf("%x", sha256.Sum256(payload))
		if got != want {
			t.Fatalf("hashFileSHA256(%q) = %q, want %q", path, got, want)
		}
	})

	t.Run("open error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		openFileFn = func(string) (*os.File, error) {
			return nil, errors.New("open fail")
		}

		if _, err := hashFileSHA256("missing.tar.gz"); err == nil {
			t.Fatalf("expected error from hashFileSHA256")
		}
	})

	t.Run("read error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		closedFile, err := os.CreateTemp(t.TempDir(), "closed-*")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		if err := closedFile.Close(); err != nil {
			t.Fatalf("close temp file: %v", err)
		}

		openFileFn = func(string) (*os.File, error) {
			return closedFile, nil
		}

		if _, err := hashFileSHA256("bundle.tar.gz"); err == nil {
			t.Fatalf("expected hashFileSHA256 read error")
		}
	})
}

func TestDownloadHostAgentChecksum(t *testing.T) {
	t.Run("request error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network fail")
			}),
		}

		if _, _, err := downloadHostAgentChecksum("https://example.com/checksum.sha256"); err == nil {
			t.Fatalf("expected request error")
		}
	})

	t.Run("status error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		if _, _, err := downloadHostAgentChecksum(server.URL + "/checksum.sha256"); err == nil {
			t.Fatalf("expected status error")
		}
	})

	t.Run("read error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(&errorReader{}),
				}, nil
			}),
		}

		if _, _, err := downloadHostAgentChecksum("https://example.com/checksum.sha256"); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(" \n\t"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		if _, _, err := downloadHostAgentChecksum(server.URL + "/checksum.sha256"); err == nil {
			t.Fatalf("expected empty payload error")
		}
	})

	t.Run("invalid checksum length", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("12345  bundle.tar.gz\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		if _, _, err := downloadHostAgentChecksum(server.URL + "/checksum.sha256"); err == nil {
			t.Fatalf("expected invalid checksum error")
		}
	})

	t.Run("invalid checksum hex", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		bad := strings.Repeat("g", 64)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(bad + "  bundle.tar.gz\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		if _, _, err := downloadHostAgentChecksum(server.URL + "/checksum.sha256"); err == nil {
			t.Fatalf("expected invalid checksum error")
		}
	})

	t.Run("success with filename", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		checksum := strings.ToUpper(strings.Repeat("a", 64))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(checksum + "  dist/pulse-v1.2.3.tar.gz\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()

		gotChecksum, gotFilename, err := downloadHostAgentChecksum(server.URL + "/checksum.sha256")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotChecksum != strings.ToLower(checksum) {
			t.Fatalf("checksum = %q, want %q", gotChecksum, strings.ToLower(checksum))
		}
		if gotFilename != "pulse-v1.2.3.tar.gz" {
			t.Fatalf("filename = %q, want pulse-v1.2.3.tar.gz", gotFilename)
		}
	})
}

func TestVerifyHostAgentBundleChecksum(t *testing.T) {
	makeBundle := func(t *testing.T, content string) (string, string) {
		t.Helper()
		bundleName := "pulse-v1.2.3.tar.gz"
		bundlePath := filepath.Join(t.TempDir(), bundleName)
		if err := os.WriteFile(bundlePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write bundle: %v", err)
		}
		return bundlePath, bundleName
	}

	t.Run("success", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		bundlePath, bundleName := makeBundle(t, "bundle-content")
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte("bundle-content")))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(sum + "  " + bundleName + "\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		bundleURL := "https://example.com/releases/" + bundleName

		if err := verifyHostAgentBundleChecksum(bundlePath, bundleURL, server.URL+"/checksum.sha256"); err != nil {
			t.Fatalf("verifyHostAgentBundleChecksum returned error: %v", err)
		}
	})

	t.Run("filename mismatch", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		bundlePath, bundleName := makeBundle(t, "bundle-content")
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte("bundle-content")))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(sum + "  other-file.tar.gz\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		bundleURL := "https://example.com/releases/" + bundleName

		err := verifyHostAgentBundleChecksum(bundlePath, bundleURL, server.URL+"/checksum.sha256")
		if err == nil || !strings.Contains(err.Error(), "does not match bundle name") {
			t.Fatalf("expected filename mismatch error, got %v", err)
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		bundlePath, bundleName := makeBundle(t, "bundle-content")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("a", 64) + "  " + bundleName + "\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		bundleURL := "https://example.com/releases/" + bundleName

		err := verifyHostAgentBundleChecksum(bundlePath, bundleURL, server.URL+"/checksum.sha256")
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected checksum mismatch error, got %v", err)
		}
	})

	t.Run("checksum download error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		bundlePath, bundleName := makeBundle(t, "bundle-content")
		httpClient = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network fail")
			}),
		}

		bundleURL := "https://example.com/releases/" + bundleName
		if err := verifyHostAgentBundleChecksum(bundlePath, bundleURL, "https://example.com/checksum.sha256"); err == nil {
			t.Fatalf("expected checksum download error")
		}
	})

	t.Run("hash file error", func(t *testing.T) {
		restore := saveHostAgentHooks()
		t.Cleanup(restore)

		bundlePath, bundleName := makeBundle(t, "bundle-content")
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte("bundle-content")))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(sum + "  " + bundleName + "\n"))
		}))
		t.Cleanup(server.Close)

		httpClient = server.Client()
		openFileFn = func(string) (*os.File, error) {
			return nil, errors.New("open fail")
		}

		bundleURL := "https://example.com/releases/" + bundleName
		if err := verifyHostAgentBundleChecksum(bundlePath, bundleURL, server.URL+"/checksum.sha256"); err == nil {
			t.Fatalf("expected hash file error")
		}
	})
}
