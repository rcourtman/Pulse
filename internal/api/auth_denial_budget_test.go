package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Deferred housekeeping must not defer a caller's own window expiry, even
// when unrelated callers arrived after its old warning.
func TestAuthDenialBudgetWindowBoundary(t *testing.T) {
	for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		t.Run(offset.String(), func(t *testing.T) {
			captureAuthDenialLogs(t)
			now := time.Unix(1_700_000_000, 0)
			authDenialNow = func() time.Time { return now }
			for i := 1; i <= authDenialWarnThreshold; i++ {
				warn, count := recordAuthDenial("user:viewer")
				if warn != (i == authDenialWarnThreshold) || count != i {
					t.Fatalf("initial denial %d: warn=%t count=%d", i, warn, count)
				}
			}

			now = now.Add(authDenialWindow + offset)
			if warn, count := recordAuthDenial("ip:192.0.2.1"); warn || count != 1 {
				t.Fatalf("unrelated caller: warn=%t count=%d", warn, count)
			}
			warn, count := recordAuthDenial("user:viewer")
			if warn {
				t.Fatal("old warning repeated, or fresh window escalated early")
			}
			if offset < 0 {
				if count != authDenialWarnThreshold+1 {
					t.Fatalf("live window count=%d", count)
				}
				return
			}
			if count != 1 {
				t.Fatalf("expired window count=%d, want 1", count)
			}
			for i := 2; i <= authDenialWarnThreshold; i++ {
				warn, count = recordAuthDenial("user:viewer")
				if warn != (i == authDenialWarnThreshold) || count != i {
					t.Fatalf("rearmed denial %d: warn=%t count=%d", i, warn, count)
				}
			}
		})
	}
}

func TestAuthDenialBudgetCapacity(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("expired=%t", expired), func(t *testing.T) {
			captureAuthDenialLogs(t)
			now := time.Unix(1_700_000_000, 0)
			authDenialNow = func() time.Time { return now }
			// Unique times make oldest-live eviction deterministic. The
			// newer half is one denial short of its first warning.
			authDenialMu.Lock()
			for i := 0; i < authDenialMaxTracked; i++ {
				start := now.Add(-time.Second + time.Duration(i))
				if expired && i < authDenialMaxTracked/2 {
					start = now.Add(-authDenialWindow)
				}
				authDenials[fmt.Sprint(i)] = &authDenialCounter{windowStart: start, count: authDenialWarnThreshold - 1}
			}
			authDenialMu.Unlock()

			if warn, count := recordAuthDenial("new"); warn || count != 1 {
				t.Fatalf("new caller: warn=%t count=%d", warn, count)
			}
			authDenialMu.Lock()
			if got := len(authDenials); got != authDenialMaxTracked/2+1 {
				t.Errorf("tracked=%d, want expired/oldest half removed and new caller added", got)
			}
			for i := 0; i < authDenialMaxTracked; i++ {
				_, present := authDenials[fmt.Sprint(i)]
				if present != (i >= authDenialMaxTracked/2) {
					t.Errorf("caller %d present=%t", i, present)
				}
			}
			authDenialMu.Unlock()
			for i := authDenialMaxTracked / 2; i < authDenialMaxTracked; i++ {
				key := fmt.Sprint(i)
				if warn, count := recordAuthDenial(key); !warn || count != authDenialWarnThreshold {
					t.Fatalf("retained caller %s lost its warning: warn=%t count=%d", key, warn, count)
				}
				if warn, _ := recordAuthDenial(key); warn {
					t.Fatalf("retained caller %s warned twice", key)
				}
			}
		})
	}
}

func TestAuthDenialBudgetConcurrentCapacity(t *testing.T) {
	captureAuthDenialLogs(t)
	now := time.Unix(1_700_000_000, 0)
	authDenialNow = func() time.Time { return now }
	authDenialMu.Lock()
	for i := 0; i < authDenialMaxTracked; i++ {
		authDenials[fmt.Sprint(i)] = &authDenialCounter{windowStart: now.Add(-time.Second + time.Duration(i))}
	}
	authDenialMu.Unlock()

	const callers = 128
	var warned atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < authDenialWarnThreshold*2; j++ {
				if warn, _ := recordAuthDenial(fmt.Sprintf("new-%d", i)); warn {
					warned.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()
	if got := warned.Load(); got != callers {
		t.Fatalf("warnings=%d, want one for each of %d concurrent callers", got, callers)
	}
	authDenialMu.Lock()
	defer authDenialMu.Unlock()
	if got := len(authDenials); got != authDenialMaxTracked/2+callers {
		t.Fatalf("tracked=%d, want %d", got, authDenialMaxTracked/2+callers)
	}
	for i := 0; i < callers; i++ {
		entry := authDenials[fmt.Sprintf("new-%d", i)]
		if entry == nil || !entry.warned || entry.count != authDenialWarnThreshold*2 {
			t.Fatalf("concurrent caller %d lost its count or warning: %+v", i, entry)
		}
	}
}

// These are the actual consumers of the tracker, not just calls to its helper.
// Housekeeping cannot admit protected work, alter an error or leak a cookie.
func TestAuthDenialBudgetHTTPConsumers(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprintf("admin=%t", admin), func(t *testing.T) {
			buf := captureAuthDenialLogs(t)
			now := time.Unix(1_700_000_000, 0)
			authDenialNow = func() time.Time { return now }
			cfg := &config.Config{AuthUser: "admin", AuthPass: "configured", DataPath: t.TempDir()}
			called := false
			next := func(http.ResponseWriter, *http.Request) { called = true }
			handler := RequireAuth(cfg, next)
			wantStatus := http.StatusUnauthorized
			if admin {
				cfg.ProxyAuthSecret = "synthetic-proxy-secret"
				cfg.ProxyAuthUserHeader = "X-Proxy-User"
				cfg.ProxyAuthRoleHeader = "X-Proxy-Roles"
				cfg.ProxyAuthAdminRole = "admin"
				handler = RequireAdmin(cfg, next)
				wantStatus = http.StatusForbidden
			}
			var firstBody string
			for window := 0; window < 2; window++ {
				for i := 0; i < authDenialWarnThreshold*2; i++ {
					req := httptest.NewRequest(http.MethodGet, "/api/private", nil)
					req.RemoteAddr = "192.0.2.1:4000"
					if admin {
						req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
						req.Header.Set("X-Proxy-User", "viewer")
						req.Header.Set("X-Proxy-Roles", "viewer")
						// User attribution survives a different address per request.
						req.RemoteAddr = fmt.Sprintf("192.0.2.%d:4000", i+1)
					}
					rec := httptest.NewRecorder()
					handler(rec, req)
					if called || rec.Code != wantStatus || len(rec.Result().Cookies()) != 0 {
						t.Fatalf("denial changed: reached=%t status=%d cookies=%v", called, rec.Code, rec.Result().Cookies())
					}
					if firstBody == "" {
						firstBody = rec.Body.String()
					}
					if rec.Body.String() != firstBody {
						t.Fatalf("denial body changed: %q, want %q", rec.Body.String(), firstBody)
					}
				}
				if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != window+1 {
					t.Fatalf("window %d warnings=%d, want %d", window, got, window+1)
				}
				now = now.Add(authDenialWindow)
			}
		})
	}
}
