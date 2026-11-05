package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity/identitytest"
)

func TestUnifiedAgentValidatedDescriptorAndCache(t *testing.T) {
	source := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", Target: "linux-amd64", ReportPath: canonicalUnifiedAgentReportPath})
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	router := &Router{}
	file, info, digest, err := router.openValidatedUnifiedAgent(path, "v6.2.1", "linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.Sum256(data)
	if digest != hex.EncodeToString(hash[:]) {
		t.Fatal("digest does not match admitted bytes")
	}
	// Atomic publication may change the pathname while a download is open. The
	// response must continue reading exactly the descriptor that was admitted.
	replacement := path + ".new"
	invalid := make([]byte, len(data))
	if err := os.WriteFile(replacement, invalid, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	served, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != string(data) {
		t.Fatal("descriptor served replacement pathname bytes")
	}
	// Same path, size and timestamp cannot reuse facts belonging to another inode.
	if candidate, _, _, err := router.openValidatedUnifiedAgent(path, "v6.2.1", "linux-amd64"); err == nil {
		_ = candidate.Close()
		t.Fatal("cache admitted replacement inode")
	}
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	cached, _, _, err := router.openValidatedUnifiedAgent(path, "v6.2.1", "linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	_ = cached.Close()
	for _, expect := range [][2]string{{"v6.2.0", "linux-amd64"}, {"v6.2.1", "linux-armv6"}} {
		if candidate, _, _, err := router.openValidatedUnifiedAgent(path, expect[0], expect[1]); err == nil {
			_ = candidate.Close()
			t.Fatalf("cache ignored expected identity %v", expect)
		}
	}
}

func TestUnifiedAgentCacheRejectsInPlacePublicationWithPreservedMtime(t *testing.T) {
	source := identitytest.Build(t, identitytest.Options{Version: "v6.2.1", Target: "linux-amd64", ReportPath: canonicalUnifiedAgentReportPath})
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	router := &Router{}
	file, info, _, err := router.openValidatedUnifiedAgent(path, "v6.2.1", "linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	// Model a later publication, allowing coarse Unix change clocks to advance.
	time.Sleep(25 * time.Millisecond)
	if err := os.WriteFile(path, make([]byte, len(data)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if file, _, _, err := router.openValidatedUnifiedAgent(path, "v6.2.1", "linux-amd64"); err == nil {
		_ = file.Close()
		t.Fatal("cache admitted in-place replacement with unchanged size/mtime")
	}
}

func TestProxyRejectsInvalidReleaseVersion(t *testing.T) {
	router := &Router{serverVersion: "6.2.1.0"}
	response := httptest.NewRecorder()
	router.proxyAgentBinaryFromGitHub(response, httptest.NewRequest(http.MethodGet, "/download/pulse-agent", nil), "linux-amd64")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "invalid server release version") {
		t.Fatalf("invalid-version proxy: %d %s", response.Code, response.Body.String())
	}
}
