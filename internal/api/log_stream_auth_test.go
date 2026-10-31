package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const logStreamToken = "synthetic-log-stream-token.12345678"
const logStreamControl = "synthetic-log-control-token.12345678"

func newLogStreamRouter(t *testing.T) (*Router, *config.Config, *httptest.Server) {
	t.Helper()
	// System Logs needs settings:read, not monitoring:read or ai:execute.
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, logStreamToken, []string{config.ScopeSettingsRead}, nil),
		newTokenRecord(t, logStreamControl, []string{config.ScopeWildcard}, nil))
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return router, cfg, server
}

func openLogStream(t *testing.T, server *httptest.Server, raw, transport string) *bufio.Reader {
	t.Helper()
	marker := "admitted-" + t.Name() + "-" + transport
	logging.GetBroadcaster().Write([]byte(marker))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/logs/stream", nil)
	switch transport {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	default:
		req.Header.Set("X-API-Token", raw)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("log stream admission = %d: %s", resp.StatusCode, body)
	}
	reader := bufio.NewReader(resp.Body)
	readLogStreamMarker(t, reader, marker)
	return reader
}

func readLogStreamMarker(t *testing.T, reader *bufio.Reader, marker string) {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("log stream before marker %q: %v", marker, err)
		}
		if line == "data: "+marker+"\n" {
			return
		}
	}
}

func wantLogStreamClosed(t *testing.T, reader *bufio.Reader, forbidden string) {
	t.Helper()
	remaining, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("withdrawn log stream did not reach EOF: %v", err)
	}
	if strings.Contains(string(remaining), forbidden) {
		t.Fatalf("withdrawn log stream disclosed %q", forbidden)
	}
}

func TestLogStreamRejectsWithdrawnTokenAuthority(t *testing.T) {
	for _, transport := range []string{"header", "bearer"} {
		for _, change := range []string{"revoked", "expired", "settings_read_removed", "org_rebound", "multi_org_rebound", "identity_replaced", "principal_replaced", "invalid_role"} {
			t.Run(transport+"/"+change, func(t *testing.T) {
				_, cfg, server := newLogStreamRouter(t)
				control := openLogStream(t, server, logStreamControl, "header")
				primary := openLogStream(t, server, logStreamToken, transport)
				config.Mu.Lock()
				switch change {
				case "revoked":
					cfg.RemoveAPIToken(cfg.APITokens[0].ID)
				case "expired":
					past := time.Now().Add(-time.Second)
					cfg.APITokens[0].ExpiresAt = &past
				case "settings_read_removed":
					cfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
				case "org_rebound":
					cfg.APITokens[0].OrgID = "tenant-only"
				case "multi_org_rebound":
					cfg.APITokens[0].OrgIDs = []string{"default", "tenant-only"}
				case "identity_replaced":
					cfg.APITokens[0].ID = "other-credential"
				case "principal_replaced":
					cfg.APITokens[0].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "other-owner"}
				case "invalid_role":
					cfg.APITokens[0].Metadata = map[string]string{internalauth.RuntimeRoleMetadataKey: internalauth.RuntimeRoleActionRunner}
				}
				config.Mu.Unlock()
				marker := "not-for-withdrawn-reader-" + t.Name()
				logging.GetBroadcaster().Write([]byte(marker))
				wantLogStreamClosed(t, primary, marker)
				readLogStreamMarker(t, control, marker)
			})
		}
	}
}

func TestLogStreamDurableHTTPRevocation(t *testing.T) {
	router, cfg, server := newLogStreamRouter(t)
	primary := openLogStream(t, server, logStreamToken, "header")
	id := cfg.APITokens[0].ID
	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/security/tokens/"+id, nil)
	req.Header.Set("X-API-Token", logStreamControl)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("real token deletion = %d: %s", resp.StatusCode, body)
	}
	persisted, err := router.persistence.LoadAPITokens()
	if err != nil || len(persisted) != 1 || persisted[0].ID == id {
		t.Fatalf("deletion not persisted: %v, %d tokens", err, len(persisted))
	}
	const forbidden = "private-log-after-persisted-token-deletion"
	logging.GetBroadcaster().Write([]byte(forbidden))
	wantLogStreamClosed(t, primary, forbidden)
	req, _ = http.NewRequest(http.MethodGet, server.URL+"/api/logs/stream", nil)
	req.Header.Set("X-API-Token", logStreamToken)
	resp, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("withdrawn reconnect = %d, want 401", resp.StatusCode)
	}
}

type logStreamMutationWriter struct {
	*httptest.ResponseRecorder
	afterWrite func(string)
	afterFlush func()
}

func (w *logStreamMutationWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseRecorder.Write(p)
	if w.afterWrite != nil {
		w.afterWrite(string(p))
	}
	return n, err
}

func (w *logStreamMutationWriter) Flush() {
	w.ResponseRecorder.Flush()
	if w.afterFlush != nil {
		f := w.afterFlush
		w.afterFlush = nil
		f()
	}
}

func TestLogStreamChecksHistoryAndQueuedLines(t *testing.T) {
	for _, stage := range []string{"before_history", "between_history_lines", "queued_live_line"} {
		t.Run(stage, func(t *testing.T) {
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, logStreamToken, []string{config.ScopeSettingsRead}, nil))
			revoke := func() {
				config.Mu.Lock()
				cfg.RemoveAPIToken(cfg.APITokens[0].ID)
				config.Mu.Unlock()
			}
			first, forbidden := "first-history-"+t.Name(), "private-later-line-"+t.Name()
			logging.GetBroadcaster().Write([]byte(first))
			logging.GetBroadcaster().Write([]byte(forbidden))
			writer := &logStreamMutationWriter{ResponseRecorder: httptest.NewRecorder()}
			if stage == "between_history_lines" {
				writer.afterWrite = func(line string) {
					if strings.Contains(line, first) {
						revoke()
					}
				}
			}
			if stage == "queued_live_line" {
				// Different sentinel: this one was never in the replay snapshot.
				forbidden += "-queued"
				writer.afterFlush = func() {
					revoke()
					logging.GetBroadcaster().Write([]byte(forbidden))
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			req := httptest.NewRequest(http.MethodGet, "/api/logs/stream", nil).WithContext(ctx)
			req.Header.Set("X-API-Token", logStreamToken)
			handler := NewLogHandlers(cfg, nil)
			AuthContextMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stage == "before_history" {
					revoke()
				}
				handler.HandleStreamLogs(w, r)
			})).ServeHTTP(writer, req)
			if strings.Contains(writer.Body.String(), forbidden) {
				t.Fatalf("withdrawn reader received %q at %s", forbidden, stage)
			}
			if stage == "before_history" && writer.Code != http.StatusUnauthorized {
				t.Fatalf("withdrawn initial admission = %d, want 401", writer.Code)
			}
		})
	}
}

func TestLogStreamIdleTokenWithdrawal(t *testing.T) {
	_, cfg, server := newLogStreamRouter(t)
	primary := openLogStream(t, server, logStreamToken, "header")
	config.Mu.Lock()
	cfg.RemoveAPIToken(cfg.APITokens[0].ID)
	config.Mu.Unlock()
	// No log write wakes the handler: use the production 15-second recheck.
	wantLogStreamClosed(t, primary, "data:")
}

func TestLogStreamPreservesAdminSession(t *testing.T) {
	_, cfg, server := newLogStreamRouter(t)
	config.Mu.Lock()
	cfg.AuthUser = "log-admin"
	config.Mu.Unlock()
	session := fmt.Sprintf("synthetic-log-session-%d", time.Now().UnixNano())
	GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", "log-admin")
	t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
	reader := openLogStream(t, server, session, "session")
	config.Mu.Lock()
	cfg.RemoveAPIToken(cfg.APITokens[0].ID)
	config.Mu.Unlock()
	const marker = "permitted-admin-session-after-unrelated-token-removal"
	logging.GetBroadcaster().Write([]byte(marker))
	readLogStreamMarker(t, reader, marker)
}
