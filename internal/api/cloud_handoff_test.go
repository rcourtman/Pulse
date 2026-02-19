package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
)

func TestHandleCloudHandoffRejectsReplay(t *testing.T) {
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	token, err := cloudauth.Sign(key, "alice@example.com", "tenant-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}

	handler := HandleCloudHandoff(dataPath)

	firstReq := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	firstRec := httptest.NewRecorder()
	handler(firstRec, firstReq)

	if firstRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("first use status = %d, want %d", firstRec.Code, http.StatusTemporaryRedirect)
	}
	if got := firstRec.Header().Get("Location"); got != "/" {
		t.Fatalf("first use redirect = %q, want %q", got, "/")
	}
	if cookieHeaders := firstRec.Header().Values("Set-Cookie"); len(cookieHeaders) == 0 || !strings.Contains(strings.Join(cookieHeaders, ";"), "pulse_session=") {
		t.Fatalf("first use should set pulse_session cookie, got headers: %v", cookieHeaders)
	}

	replayReq := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	replayRec := httptest.NewRecorder()
	handler(replayRec, replayReq)

	if replayRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("replay status = %d, want %d", replayRec.Code, http.StatusTemporaryRedirect)
	}
	if got := replayRec.Header().Get("Location"); got != "/login?error=handoff_replayed" {
		t.Fatalf("replay redirect = %q, want %q", got, "/login?error=handoff_replayed")
	}
}
