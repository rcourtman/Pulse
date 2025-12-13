package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleDownloadUnifiedAgentSetsChecksumAndInvalidatesOnChange(t *testing.T) {
	binDir := setupTempPulseBin(t)
	filePath := filepath.Join(binDir, "pulse-agent-linux-amd64")

	payload1 := []byte("agent-binary-v1")
	if err := os.WriteFile(filePath, payload1, 0o755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/download/pulse-agent?arch=linux-amd64", nil)
	rr1 := httptest.NewRecorder()

	router := &Router{checksumCache: make(map[string]checksumCacheEntry)}
	router.handleDownloadUnifiedAgent(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr1.Code)
	}

	expected1 := fmt.Sprintf("%x", sha256.Sum256(payload1))
	if got := rr1.Header().Get("X-Checksum-Sha256"); got != expected1 {
		t.Fatalf("unexpected checksum header: got %q want %q", got, expected1)
	}

	// Ensure modtime changes for invalidation.
	time.Sleep(10 * time.Millisecond)
	payload2 := []byte("agent-binary-v2")
	if err := os.WriteFile(filePath, payload2, 0o755); err != nil {
		t.Fatalf("failed to rewrite test binary: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/download/pulse-agent?arch=linux-amd64", nil)
	rr2 := httptest.NewRecorder()
	router.handleDownloadUnifiedAgent(rr2, req2)

	expected2 := fmt.Sprintf("%x", sha256.Sum256(payload2))
	if got := rr2.Header().Get("X-Checksum-Sha256"); got != expected2 {
		t.Fatalf("checksum did not update after file change: got %q want %q", got, expected2)
	}

	if strings.TrimSpace(rr2.Body.String()) != string(payload2) {
		t.Fatalf("unexpected response body after update")
	}
}

