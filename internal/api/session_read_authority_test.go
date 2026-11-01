package api

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
)

func mutateReadSession(raw, change string) {
	store := GetSessionStore()
	if change == "invalidated" {
		store.InvalidateSession(raw)
		return
	}
	if change == "replaced" {
		prior := store.GetSession(raw)
		store.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", prior.Username)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session := store.sessions[sessionHash(raw)]
	switch change {
	case "expired":
		session.ExpiresAt = time.Now().Add(-time.Second)
	case "principal":
		session.Username = "other-operator"
	case "recovery":
		session.RecoveryBypass = !session.RecoveryBypass
	case "recovery_binding":
		session.IP = "192.0.2.1"
	}
}

func TestSessionReadSnapshot(t *testing.T) {
	for _, cookieName := range []string{cookieNameSession, cookieNameSessionSecure} {
		for _, change := range []string{"invalidated", "expired", "principal", "replaced", "recovery"} {
			t.Run(cookieName+"/"+change, func(t *testing.T) {
				InitSessionStore(t.TempDir())
				const raw = "synthetic-read-snapshot"
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
				req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/state", nil)
				if cookieName == cookieNameSessionSecure {
					req.TLS = &tls.ConnectionState{}
				}
				req.AddCookie(&http.Cookie{Name: cookieName, Value: raw})
				if !CheckAuth(&config.Config{}, httptest.NewRecorder(), req) {
					t.Fatal("session admission failed")
				}
				allowed := sessionReadRevalidator(req)
				before := GetSessionStore().GetSession(raw).ExpiresAt
				if allowed == nil || !allowed() {
					t.Fatal("missing admitted session")
				}
				if !GetSessionStore().GetSession(raw).ExpiresAt.Equal(before) {
					t.Fatal("read extended session expiry")
				}
				// Returning middleware/header changes cannot retarget the reader.
				req.Header.Set("Cookie", cookieName+"=different")
				mutateReadSession(raw, change)
				if allowed() {
					t.Fatal("old read retained changed session authority")
				}
			})
		}
	}
}

func TestSessionReadAdmissionPrecedence(t *testing.T) {
	InitSessionStore(t.TempDir())
	const raw = "synthetic-read-precedence"
	GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/state", nil)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	// An unselected cookie and a mutable response header are not authority.
	if sessionReadRevalidator(req) != nil {
		t.Fatal("ambient cookie became admission")
	}
	writer := httptest.NewRecorder()
	writer.Header().Set("X-Auth-Method", "session")
	if sessionReadRevalidator(req) != nil {
		t.Fatal("response header became admission")
	}
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, logStreamControl, []string{config.ScopeWildcard}, nil))
	if !CheckAuth(cfg, writer, req) || sessionReadRevalidator(req) == nil {
		t.Fatal("actual session missing")
	}
	req.Header.Set("X-API-Token", logStreamControl)
	if !CheckAuth(cfg, writer, req) || sessionReadRevalidator(req) != nil {
		t.Fatal("explicit token inherited session marker")
	}
	req.Header.Del("X-API-Token")
	cfg.ProxyAuthSecret = "synthetic-read-proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "proxy-user")
	if !CheckAuth(cfg, writer, req) || sessionReadRevalidator(req) != nil {
		t.Fatal("proxy admission inherited session marker")
	}
	// Failed auth also clears a previous marker; no fallback is created.
	req.Header.Del("X-Proxy-Secret")
	req.Header.Set("X-API-Token", "invalid")
	if CheckAuth(cfg, writer, req) || sessionReadRevalidator(req) != nil {
		t.Fatal("failed auth retained session marker")
	}
}

func TestSessionReadStoreReplacementAndRecoveryBinding(t *testing.T) {
	for _, change := range []string{"store", "recovery_binding"} {
		t.Run(change, func(t *testing.T) {
			InitSessionStore(t.TempDir())
			const raw = "synthetic-recovery-read"
			GetSessionStore().CreateRecoverySession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/state", nil)
			req.RemoteAddr = "127.0.0.1:1234"
			req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
			if !CheckAuth(&config.Config{}, httptest.NewRecorder(), req) {
				t.Fatal("recovery admission failed")
			}
			allowed := sessionReadRevalidator(req)
			if allowed == nil || !allowed() {
				t.Fatal("missing recovery admission")
			}
			if change == "store" {
				InitSessionStore(t.TempDir())
			} else {
				mutateReadSession(raw, change)
			}
			if allowed() {
				t.Fatal("recovery retained replaced store/binding")
			}
		})
	}
}

func TestSessionReadLogoutLogsAndEvents(t *testing.T) {
	t.Run("logs", func(t *testing.T) {
		_, cfg, server := newLogStreamRouter(t)
		cfg.AuthUser = "operator"
		const primaryRaw, controlRaw = "synthetic-logout-logs", "synthetic-logout-logs-control"
		for _, raw := range []string{primaryRaw, controlRaw} {
			GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
			t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
		}
		primary := openLogStream(t, server, primaryRaw, "session")
		control := openLogStream(t, server, controlRaw, "session")
		logoutReadSession(t, server.URL, primaryRaw)
		const marker = "private-log-after-session-logout"
		logging.GetBroadcaster().Write([]byte(marker))
		readLogStreamMarker(t, control, marker)
		wantLogStreamClosed(t, primary, marker)
	})
	t.Run("events", func(t *testing.T) {
		router, _, server := newAgentLifecycleRouter(t, time.Hour)
		const primaryRaw, controlRaw = "synthetic-logout-events", "synthetic-logout-events-control"
		for _, raw := range []string{primaryRaw, controlRaw} {
			GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
			t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
		}
		primary := openAgentLifecycleStream(t, server, primaryRaw, "session")
		control := openAgentLifecycleStream(t, server, controlRaw, "session")
		logoutReadSession(t, server.URL, primaryRaw)
		router.agentEventBroadcaster.PublishActionCompletedRecord(agentDisclosureRecord(t, false))
		_ = control.next(t)
		primary.wantClosed(t)
	})
}

func TestSessionReadLogoutAssistantStreams(t *testing.T) {
	for _, kind := range []string{"execute", "investigate-alert", "chat"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantStreamFixture(t, kind, []string{assistantScope(kind)})
			const primaryRaw, controlRaw = "synthetic-logout-assistant", "synthetic-logout-assistant-control"
			for _, raw := range []string{primaryRaw, controlRaw} {
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
			}
			primary := f.open(t, "primary", primaryRaw, "session")
			control := f.open(t, "control", controlRaw, "session")
			logoutReadSession(t, f.server.URL, primaryRaw)
			f.primary.unblock()
			f.control.unblock()
			readAssistantResult(t, primary, false)
			readAssistantResult(t, control, true)
			if kind != "chat" && f.primary.cancelled.Load() {
				t.Fatal("logout cancelled independent legacy work")
			}
		})
	}
}

func TestSessionReadIdleExpiry(t *testing.T) {
	InitSessionStore(t.TempDir())
	const raw = "synthetic-idle-session"
	GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/progress", nil)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	if !CheckAuth(&config.Config{}, httptest.NewRecorder(), req) {
		t.Fatal("session admission failed")
	}
	entered, finished := make(chan struct{}), make(chan struct{})
	handler := retainTokenStreamAuthority(&config.Config{}, config.ScopeMonitoringRead, false, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(entered)
		<-req.Context().Done()
		_, err := io.WriteString(w, "private-idle-session")
		if err == nil {
			t.Error("withdrawn idle writer accepted output")
		}
	})
	go func() { defer close(finished); handler(httptest.NewRecorder(), req) }()
	<-entered
	mutateReadSession(raw, "expired")
	select {
	case <-finished:
	case <-time.After(18 * time.Second):
		t.Fatal("idle session read exceeded bounded recheck")
	}
}

func TestSessionReadJSONBodylessExpiry(t *testing.T) {
	InitSessionStore(t.TempDir())
	const raw = "synthetic-bodyless-session"
	GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/result", nil)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	if !CheckAuth(&config.Config{}, httptest.NewRecorder(), req) {
		t.Fatal("session admission failed")
	}
	recorder := httptest.NewRecorder()
	retainTokenResponseAuthority(&config.Config{}, config.ScopeAIExecute, true, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Private", "private-header")
		mutateReadSession(raw, "expired")
	})(recorder, req)
	if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("X-Private") != "" || strings.Contains(recorder.Body.String(), "private") {
		t.Fatalf("bodyless withdrawn result: %d %s", recorder.Code, recorder.Body)
	}
}
