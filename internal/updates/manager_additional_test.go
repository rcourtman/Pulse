package updates

import (
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGetCachedUpdateInfo_WithChannel(t *testing.T) {
	info := &UpdateInfo{LatestVersion: "9.9.9"}
	manager := &Manager{
		config:     &config.Config{UpdateChannel: "stable"},
		checkCache: map[string]*UpdateInfo{"stable": info},
	}

	got := manager.GetCachedUpdateInfo()
	if got != info {
		t.Fatalf("expected cached info, got %+v", got)
	}
}

func TestSanitizeErrorTruncation(t *testing.T) {
	if got := sanitizeError(nil); got != "" {
		t.Fatalf("expected empty string for nil error, got %q", got)
	}

	if got := sanitizeError(errors.New("short")); got != "short" {
		t.Fatalf("expected short error, got %q", got)
	}

	long := strings.Repeat("x", 600)
	got := sanitizeError(errors.New(long))
	if len(got) <= 500 {
		t.Fatalf("expected truncated error > 500 chars, got len=%d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated error suffix ..., got %q", got[len(got)-3:])
	}
}
