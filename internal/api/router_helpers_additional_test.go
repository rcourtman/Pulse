package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentbinaries"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "empty",
			header: "",
			want:   "",
		},
		{
			name:   "basic auth",
			header: "Basic abc123",
			want:   "",
		},
		{
			name:   "bearer lower",
			header: "bearer token-123",
			want:   "token-123",
		},
		{
			name:   "bearer upper",
			header: "Bearer token-456",
			want:   "token-456",
		},
		{
			name:   "bearer extra whitespace",
			header: "  Bearer   token-789  ",
			want:   "token-789",
		},
		{
			name:   "short header",
			header: "bearer",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractBearerToken(tt.header); got != tt.want {
				t.Fatalf("extractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestExtractSetupToken(t *testing.T) {
	t.Run("prefers header token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/setup?auth_token=query-token", nil)
		req.Header.Set("X-Setup-Token", " header-token ")
		req.Header.Set("Authorization", "Bearer auth-token")

		if got := extractSetupToken(req); got != "header-token" {
			t.Fatalf("expected header token, got %q", got)
		}
	})

	t.Run("falls back to bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/setup?auth_token=query-token", nil)
		req.Header.Set("X-Setup-Token", " ")
		req.Header.Set("Authorization", "Bearer auth-token")

		if got := extractSetupToken(req); got != "auth-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
	})

	t.Run("falls back to query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/setup?auth_token=query-token", nil)

		if got := extractSetupToken(req); got != "query-token" {
			t.Fatalf("expected query token, got %q", got)
		}
	})

	t.Run("returns empty when missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/setup", nil)
		if got := extractSetupToken(req); got != "" {
			t.Fatalf("expected empty token, got %q", got)
		}
	})
}

func TestDedupeStrings(t *testing.T) {
	input := []string{"alpha", "beta", "alpha", "", "gamma", "beta", " "}
	want := []string{"alpha", "beta", "gamma", " "}

	if got := dedupeStrings(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeStrings() = %v, want %v", got, want)
	}
}

func TestSortedHostAgentKeys(t *testing.T) {
	if got := sortedHostAgentKeys(nil); got != nil {
		t.Fatalf("expected nil result for nil map, got %v", got)
	}

	missing := map[string]agentbinaries.HostAgentBinary{
		"zeta":  {},
		"alpha": {},
		"beta":  {},
	}

	want := []string{"alpha", "beta", "zeta"}
	if got := sortedHostAgentKeys(missing); !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedHostAgentKeys() = %v, want %v", got, want)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if !fileExists(filePath) {
		t.Fatalf("expected fileExists true for %s", filePath)
	}
	if fileExists(filepath.Join(dir, "missing.txt")) {
		t.Fatalf("expected fileExists false for missing file")
	}
}

func TestCachedSHA256(t *testing.T) {
	router := &Router{}
	if _, err := router.cachedSHA256("", nil); err == nil {
		t.Fatalf("expected error for empty file path")
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "checksum.txt")
	payload := []byte("checksum-data")
	if err := os.WriteFile(filePath, payload, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	wantHash := sha256.Sum256(payload)
	want := hex.EncodeToString(wantHash[:])

	sum, err := router.cachedSHA256(filePath, nil)
	if err != nil {
		t.Fatalf("cachedSHA256 error: %v", err)
	}
	if sum != want {
		t.Fatalf("cachedSHA256 = %q, want %q", sum, want)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	sum, err = router.cachedSHA256(filePath, info)
	if err != nil {
		t.Fatalf("cachedSHA256 second call: %v", err)
	}
	if sum != want {
		t.Fatalf("cachedSHA256 cached = %q, want %q", sum, want)
	}
}

func TestServeChecksum(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "checksum.txt")
	payload := []byte("checksum-body")
	if err := os.WriteFile(filePath, payload, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	wantHash := sha256.Sum256(payload)
	want := hex.EncodeToString(wantHash[:])

	router := &Router{}
	rec := httptest.NewRecorder()
	router.serveChecksum(rec, filePath)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("expected text/plain, got %q", ct)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("checksum body %q, want %q", got, want)
	}
}

func TestDeriveResourceTypeFromAlert(t *testing.T) {
	tests := []struct {
		name  string
		alert *alerts.Alert
		want  string
	}{
		{
			name:  "nil alert",
			alert: nil,
			want:  "",
		},
		{
			name:  "node alert",
			alert: &alerts.Alert{Type: "NodeDown", ResourceID: "node-1"},
			want:  "node",
		},
		{
			name:  "qemu alert",
			alert: &alerts.Alert{Type: "qemu_cpu", ResourceID: "cluster/qemu/100"},
			want:  "vm",
		},
		{
			name:  "lxc alert",
			alert: &alerts.Alert{Type: "lxc_disk", ResourceID: "cluster/lxc/200"},
			want:  "container",
		},
		{
			name:  "docker alert",
			alert: &alerts.Alert{Type: "docker_health", ResourceID: "docker-1"},
			want:  "docker",
		},
		{
			name:  "storage alert",
			alert: &alerts.Alert{Type: "storage_high", ResourceID: "storage-1"},
			want:  "storage",
		},
		{
			name:  "pbs alert",
			alert: &alerts.Alert{Type: "pbs_backup", ResourceID: "pbs-1"},
			want:  "pbs",
		},
		{
			name:  "kubernetes alert",
			alert: &alerts.Alert{Type: "kubernetes", ResourceID: "k8s-1"},
			want:  "kubernetes",
		},
		{
			name:  "fallback by resource id",
			alert: &alerts.Alert{Type: "custom", ResourceID: "cluster/qemu/101"},
			want:  "vm",
		},
		{
			name:  "fallback default",
			alert: &alerts.Alert{Type: "custom", ResourceID: "guest-1"},
			want:  "guest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveResourceTypeFromAlert(tt.alert); got != tt.want {
				t.Fatalf("deriveResourceTypeFromAlert() = %q, want %q", got, tt.want)
			}
		})
	}
}
