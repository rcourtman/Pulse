package updates

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// downloadAndVerifyReleaseSignatureForTest resolves rawURL and runs the
// manager's fetch-and-verify path against it.
func downloadAndVerifyReleaseSignatureForTest(t *testing.T, rawURL, assetPath string) error {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	return NewManager(nil).downloadAndVerifyReleaseSignature(context.Background(), parsed, assetPath)
}

func TestSignatureURLForAppendsSshsig(t *testing.T) {
	parsed, err := url.Parse("https://github.com/rcourtman/Pulse/releases/download/v6.0.0/pulse-v6.0.0-linux-amd64.tar.gz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := signatureURLFor(parsed).String()
	want := "https://github.com/rcourtman/Pulse/releases/download/v6.0.0/pulse-v6.0.0-linux-amd64.tar.gz.sshsig"
	if got != want {
		t.Fatalf("signatureURLFor() = %q, want %q", got, want)
	}
}

func TestManagerDownloadAndVerifyReleaseSignatureSuccess(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".sshsig") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("-----BEGIN SSH SIGNATURE-----\nfake\n-----END SSH SIGNATURE-----\n"))
	}))
	defer server.Close()

	prev := verifyReleaseSignatureFunc
	verifyReleaseSignatureFunc = func(ctx context.Context, targetPath, signaturePath string) error {
		if targetPath != assetPath {
			t.Errorf("target path = %q, want %q", targetPath, assetPath)
		}
		if _, err := os.Stat(signaturePath); err != nil {
			t.Errorf("signature file missing: %v", err)
		}
		return nil
	}
	t.Cleanup(func() { verifyReleaseSignatureFunc = prev })

	if err := downloadAndVerifyReleaseSignatureForTest(t, server.URL+"/asset.bin", assetPath); err != nil {
		t.Fatalf("downloadAndVerifyReleaseSignature error: %v", err)
	}
}

func TestManagerDownloadAndVerifyReleaseSignatureFailsOnMissingSidecar(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := downloadAndVerifyReleaseSignatureForTest(t, server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got: %v", err)
	}
}

func TestManagerDownloadAndVerifyReleaseSignatureFailsOnEmptySignature(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := downloadAndVerifyReleaseSignatureForTest(t, server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty-signature error, got: %v", err)
	}
}

func TestManagerDownloadAndVerifyReleaseSignatureFailsWhenVerifierRejects(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not a real signature"))
	}))
	defer server.Close()

	prev := verifyReleaseSignatureFunc
	verifyReleaseSignatureFunc = func(ctx context.Context, targetPath, signaturePath string) error {
		return errors.New("signature does not match")
	}
	t.Cleanup(func() { verifyReleaseSignatureFunc = prev })

	err := downloadAndVerifyReleaseSignatureForTest(t, server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "signature does not match") {
		t.Fatalf("expected verifier rejection, got: %v", err)
	}
}

func TestManagerDownloadAndVerifyReleaseSignatureFailsOnOversizedSignature(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		oversized := make([]byte, maxReleaseSignatureBytes+128)
		for i := range oversized {
			oversized[i] = 'x'
		}
		w.Write(oversized)
	}))
	defer server.Close()

	err := downloadAndVerifyReleaseSignatureForTest(t, server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversize error, got: %v", err)
	}
}

func TestManagerDownloadAndVerifyReleaseSignatureAbortsOnFailure(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-signature-content"))
	}))
	defer server.Close()

	prev := verifyReleaseSignatureFunc
	verifyReleaseSignatureFunc = func(ctx context.Context, targetPath, signaturePath string) error {
		return errors.New("rejected")
	}
	t.Cleanup(func() { verifyReleaseSignatureFunc = prev })

	manager := NewManager(nil)
	parsed, _ := url.Parse(server.URL + "/asset.bin")
	err := manager.downloadAndVerifyReleaseSignature(context.Background(), parsed, assetPath)
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("expected verifier rejection, got: %v", err)
	}
}
