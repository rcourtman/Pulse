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

// stubReleaseSignatureVerification replaces the package-level signature
// verifier and fetch-and-verify helper with no-op stubs so tests that
// exercise update paths don't need ssh-keygen or a real .sshsig endpoint.
// The previous values are restored via t.Cleanup.
func stubReleaseSignatureVerification(t *testing.T) {
	t.Helper()
	prevFetch := fetchAndVerifyReleaseSignature
	prevVerify := verifyReleaseSignatureFunc
	fetchAndVerifyReleaseSignature = func(ctx context.Context, assetURL, assetPath string) error { return nil }
	verifyReleaseSignatureFunc = func(ctx context.Context, targetPath, signaturePath string) error { return nil }
	t.Cleanup(func() {
		fetchAndVerifyReleaseSignature = prevFetch
		verifyReleaseSignatureFunc = prevVerify
	})
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

func TestFetchAndVerifyReleaseSignatureSuccess(t *testing.T) {
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

	if err := fetchAndVerifyReleaseSignatureReal(context.Background(), server.URL+"/asset.bin", assetPath); err != nil {
		t.Fatalf("fetchAndVerifyReleaseSignatureReal error: %v", err)
	}
}

func TestFetchAndVerifyReleaseSignatureFailsOnMissingSidecar(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := fetchAndVerifyReleaseSignatureReal(context.Background(), server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got: %v", err)
	}
}

func TestFetchAndVerifyReleaseSignatureFailsOnEmptySignature(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := fetchAndVerifyReleaseSignatureReal(context.Background(), server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty-signature error, got: %v", err)
	}
}

func TestFetchAndVerifyReleaseSignatureFailsWhenVerifierRejects(t *testing.T) {
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

	err := fetchAndVerifyReleaseSignatureReal(context.Background(), server.URL+"/asset.bin", assetPath)
	if err == nil || !strings.Contains(err.Error(), "signature does not match") {
		t.Fatalf("expected verifier rejection, got: %v", err)
	}
}

func TestFetchAndVerifyReleaseSignatureFailsOnOversizedSignature(t *testing.T) {
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

	err := fetchAndVerifyReleaseSignatureReal(context.Background(), server.URL+"/asset.bin", assetPath)
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
