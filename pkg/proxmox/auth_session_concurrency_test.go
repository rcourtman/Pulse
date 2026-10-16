package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// All reads have already left with the same rejected session before renewal
// can start. Late 401s must reuse the new session, not invalidate it again.
// The fixture issues synthetic tickets only and never queues guest work.
func TestClientSessionConcurrent401(t *testing.T) {
	const readers = 16
	var authentications, oldRequests, newRequests, mismatchedPairs atomic.Int32
	oldArrived := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			n := authentications.Add(1)
			fmt.Fprintf(w, `{"data":{"ticket":"session-%d","CSRFPreventionToken":"csrf-%d"}}`, n, n)
			return
		}
		cookie, err := r.Cookie("PVEAuthCookie")
		if err != nil {
			t.Error("request missing synthetic session")
			w.WriteHeader(401)
			return
		}
		if cookie.Value == "session-1" {
			if oldRequests.Add(1) == readers {
				close(oldArrived)
			}
			select {
			case <-oldArrived:
			case <-ctx.Done():
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"synthetic session expired"}`)
			return
		}
		if r.Method != http.MethodGet {
			n := strings.TrimPrefix(cookie.Value, "session-")
			if r.Header.Get("CSRFPreventionToken") != "csrf-"+n {
				mismatchedPairs.Add(1)
				w.WriteHeader(403)
				return
			}
		}
		newRequests.Add(1)
		fmt.Fprint(w, `{"data":[{"node":"session-safe-node","status":"online","cpu":0.05,"maxcpu":4,"mem":268435456,"maxmem":2147483648}]}`)
	}))
	defer server.Close()
	c, err := NewClient(ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.httpClient.CloseIdleConnections()
	errors := make(chan error, readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes, err := c.GetNodes(ctx)
			if err == nil && (len(nodes) != 1 || nodes[0].Node != "session-safe-node" || nodes[0].Status != "online") {
				err = fmt.Errorf("renewed session lost ordinary node result")
			}
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
	// Check ticket/CSRF pairing through the same production request path.
	for i := 0; i < readers; i++ {
		resp, err := c.request(ctx, http.MethodPost, "/synthetic-read", nil)
		if err != nil {
			t.Error(err)
			continue
		}
		_, err = readResponseBodyLimited(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}
	if got := authentications.Load(); got != 2 {
		t.Errorf("session-renewal stampede: authentications=%d, want initial plus one renewal", got)
	}
	if oldRequests.Load() != readers || newRequests.Load() != 2*readers || mismatchedPairs.Load() != 0 {
		t.Errorf("old/new/pair-mismatch=%d/%d/%d, want %d/%d/0", oldRequests.Load(), newRequests.Load(), mismatchedPairs.Load(), readers, 2*readers)
	}
	t.Log("ordinary reads=" + strconv.Itoa(readers) + "; one shared renewal required; coherent ticket/CSRF pairs checked")
}

// Done signals the exact point at which a caller joins a pending renewal.
// There is no sleep or production-only instrumentation in these controls.
type authJoinContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *authJoinContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}

func awaitSessionSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("session fixture barrier did not complete")
	}
}

func expireSessionFixture(c *Client) {
	c.authMu.Lock()
	c.auth.expiresAt = time.Now().Add(-time.Hour)
	c.authMu.Unlock()
}

func TestClientSessionConcurrentExpiryAndFailureRecovery(t *testing.T) {
	for _, failRenewal := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%t", failRenewal), func(t *testing.T) {
			const readers = 8
			entered, finish := make(chan struct{}), make(chan struct{})
			var authCalls, apiCalls atomic.Int32
			var fail atomic.Bool
			fail.Store(failRenewal)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/access/ticket") {
					n := authCalls.Add(1)
					if n == 2 {
						close(entered)
						select {
						case <-finish:
						case <-ctx.Done():
							return
						}
						if fail.Load() {
							w.WriteHeader(http.StatusServiceUnavailable)
							fmt.Fprint(w, "synthetic renewal unavailable")
							return
						}
					}
					fmt.Fprintf(w, `{"data":{"ticket":"session-%d","CSRFPreventionToken":"csrf-%d"}}`, n, n)
					return
				}
				apiCalls.Add(1)
				fmt.Fprint(w, `{"data":[{"node":"session-safe-node","status":"online"}]}`)
			}))
			defer server.Close()
			c, err := NewClient(ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer c.httpClient.CloseIdleConnections()
			expireSessionFixture(c)
			done := make(chan error, readers)
			go func() { _, err := c.GetNodes(ctx); done <- err }()
			awaitSessionSignal(t, entered)
			for i := 1; i < readers; i++ {
				joined := &authJoinContext{Context: ctx, joined: make(chan struct{})}
				go func() { _, err := c.GetNodes(joined); done <- err }()
				awaitSessionSignal(t, joined.joined)
			}
			if authCalls.Load() != 2 || apiCalls.Load() != 0 {
				t.Errorf("expired readers bypassed shared renewal: auth/api=%d/%d", authCalls.Load(), apiCalls.Load())
			}
			close(finish)
			for i := 0; i < readers; i++ {
				err := <-done
				if failRenewal {
					if err == nil || !strings.Contains(err.Error(), "synthetic renewal unavailable") {
						t.Errorf("renewal failure not shared: %v", err)
					}
				} else if err != nil {
					t.Error(err)
				}
			}
			if authCalls.Load() != 2 {
				t.Error("joined callers retried completed renewal")
			}
			if failRenewal {
				snapshot, _ := c.authSnapshot()
				if snapshot.ticket != "session-1" || snapshot.csrfToken != "csrf-1" || !snapshot.expiresAt.Before(time.Now()) {
					t.Error("failed renewal published or refreshed the old session")
				}
				if apiCalls.Load() != 0 {
					t.Error("failed authentication admitted API reads")
				}
				// A new ordinary call may recover; the failed attempt is not
				// automatically replayed by any of its joined callers.
				fail.Store(false)
				if nodes, err := c.GetNodes(ctx); err != nil || len(nodes) != 1 {
					t.Errorf("ordinary later recovery failed: %v", err)
				}
				if authCalls.Load() != 3 || apiCalls.Load() != 1 {
					t.Errorf("later recovery counts auth/api=%d/%d", authCalls.Load(), apiCalls.Load())
				}
			} else if apiCalls.Load() != readers {
				t.Errorf("renewed session lost %d reads", readers-apiCalls.Load())
			}
		})
	}
}

func TestClientSessionRenewalWaiterCancellation(t *testing.T) {
	entered, finish := make(chan struct{}), make(chan struct{})
	var authCalls, apiCalls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			n := authCalls.Add(1)
			if n == 2 {
				close(entered)
				select {
				case <-finish:
				case <-ctx.Done():
					return
				}
			}
			fmt.Fprintf(w, `{"data":{"ticket":"session-%d","CSRFPreventionToken":"csrf-%d"}}`, n, n)
			return
		}
		apiCalls.Add(1)
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	c, err := NewClient(ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.httpClient.CloseIdleConnections()
	expireSessionFixture(c)
	owner := make(chan error, 1)
	go func() { _, err := c.GetNodes(ctx); owner <- err }()
	awaitSessionSignal(t, entered)
	waiterCtx, cancelWaiter := context.WithCancel(ctx)
	defer cancelWaiter()
	joined := &authJoinContext{Context: waiterCtx, joined: make(chan struct{})}
	waiter := make(chan error, 1)
	go func() { _, err := c.GetNodes(joined); waiter <- err }()
	awaitSessionSignal(t, joined.joined)
	cancelWaiter()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled waiter result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter remained blocked behind another caller's renewal")
	}
	if authCalls.Load() != 2 || apiCalls.Load() != 0 {
		t.Error("cancelled waiter replayed renewal or admitted a read")
	}
	select {
	case err := <-owner:
		t.Fatalf("waiter cancellation ended the owner's renewal: %v", err)
	default:
	}
	close(finish)
	if err := <-owner; err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetNodes(ctx); err != nil {
		t.Fatal(err)
	}
	if authCalls.Load() != 2 || apiCalls.Load() != 2 {
		t.Errorf("owner/later recovery counts auth/api=%d/%d", authCalls.Load(), apiCalls.Load())
	}
}

func TestClientSessionTokenAuthDoesNotRenew(t *testing.T) {
	var authCalls, apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			authCalls.Add(1)
			w.WriteHeader(500)
			return
		}
		apiCalls.Add(1)
		if r.Header.Get("Authorization") != "PVEAPIToken=synthetic@pve!pulse=synthetic-token" || r.Header.Get("Cookie") != "" || r.Header.Get("CSRFPreventionToken") != "" {
			t.Error("token read acquired session headers")
		}
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	c, err := NewClient(ClientConfig{Host: server.URL, TokenName: "synthetic@pve!pulse", TokenValue: "synthetic-token", Password: "unused-synthetic-password", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.httpClient.CloseIdleConnections()
	done := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() { _, err := c.GetNodes(context.Background()); done <- err }()
	}
	for i := 0; i < 16; i++ {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	if authCalls.Load() != 0 || apiCalls.Load() != 16 {
		t.Errorf("token auth/API calls=%d/%d", authCalls.Load(), apiCalls.Load())
	}
}

// The caller that happens to start a shared renewal may be a short-lived read
// (a guest-agent call, a cancelled metadata fetch, a disconnected API client).
// Its cancellation ends only its own wait: live waiters keep the login, a
// genuine login failure still reaches them, and a login whose starter has left
// still completes and is reused instead of being repeated.
func TestClientSessionRenewalStarterCancellation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		waiter  bool
		failure bool
	}{
		{name: "live waiter keeps login", waiter: true},
		{name: "login failure reaches live waiter", waiter: true, failure: true},
		{name: "abandoned login is reused", waiter: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered, finish := make(chan struct{}), make(chan struct{})
			var authCalls, apiCalls atomic.Int32
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/access/ticket") {
					n := authCalls.Add(1)
					if n == 2 {
						close(entered)
						select {
						case <-finish:
						case <-ctx.Done():
							return
						}
						if tc.failure {
							w.WriteHeader(http.StatusUnauthorized)
							fmt.Fprint(w, "synthetic credentials rejected")
							return
						}
					}
					fmt.Fprintf(w, `{"data":{"ticket":"session-%d","CSRFPreventionToken":"csrf-%d"}}`, n, n)
					return
				}
				apiCalls.Add(1)
				fmt.Fprint(w, `{"data":[{"node":"session-safe-node","status":"online"}]}`)
			}))
			defer server.Close()
			c, err := NewClient(ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer c.httpClient.CloseIdleConnections()
			expireSessionFixture(c)

			starterCtx, cancelStarter := context.WithCancel(ctx)
			defer cancelStarter()
			starter := make(chan error, 1)
			go func() { _, err := c.GetNodes(starterCtx); starter <- err }()
			awaitSessionSignal(t, entered)

			waiter := make(chan error, 1)
			if tc.waiter {
				joined := &authJoinContext{Context: ctx, joined: make(chan struct{})}
				go func() { _, err := c.GetNodes(joined); waiter <- err }()
				awaitSessionSignal(t, joined.joined)
			}

			cancelStarter()
			select {
			case err := <-starter:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("cancelled starter result: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled starter remained blocked behind its shared login")
			}
			if tc.waiter {
				select {
				case err := <-waiter:
					t.Fatalf("starter cancellation ended a live waiter: %v", err)
				default:
				}
			}

			close(finish)
			if tc.waiter {
				err := <-waiter
				switch {
				case tc.failure && (err == nil || !strings.Contains(err.Error(), "synthetic credentials rejected")):
					t.Errorf("login failure not shared with live waiter: %v", err)
				case tc.failure && errors.Is(err, context.Canceled):
					t.Errorf("waiter received the starter's cancellation: %v", err)
				case !tc.failure && err != nil:
					t.Errorf("live waiter failed: %v", err)
				}
			}
			if tc.failure {
				if apiCalls.Load() != 0 {
					t.Error("failed login admitted API reads")
				}
				return
			}
			// A later read joins or reuses the detached login; it never logs in again.
			if _, err := c.GetNodes(ctx); err != nil {
				t.Fatal(err)
			}
			snapshot, _ := c.authSnapshot()
			if snapshot.ticket != "session-2" || !snapshot.expiresAt.After(time.Now()) {
				t.Errorf("detached login not published: ticket=%q", snapshot.ticket)
			}
			wantAPI := int32(1)
			if tc.waiter {
				wantAPI = 2
			}
			if authCalls.Load() != 2 || apiCalls.Load() != wantAPI {
				t.Errorf("auth/api=%d/%d, want 2/%d", authCalls.Load(), apiCalls.Load(), wantAPI)
			}
		})
	}
}

// A detached login cannot outlive the client's request timeout: a stalled
// Proxmox login ends, releases the in-flight slot, and a later read can renew.
func TestClientSessionRenewalStalledLoginIsBounded(t *testing.T) {
	var authCalls atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			if authCalls.Add(1) == 2 {
				select {
				case <-release:
				case <-r.Context().Done():
				}
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"session","CSRFPreventionToken":"csrf"}}`)
			return
		}
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	// The stalled handler never reads its body, so the server cannot see the
	// client hang up; release it before server.Close waits for it.
	defer close(release)
	c, err := NewClient(ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer c.httpClient.CloseIdleConnections()
	expireSessionFixture(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := c.GetNodes(ctx); err == nil {
		t.Fatal("stalled login reported success")
	}
	c.authMu.Lock()
	inFlight := c.authInFlight
	c.authMu.Unlock()
	if inFlight != nil {
		t.Fatal("stalled login kept the shared renewal slot")
	}
	if _, err := c.GetNodes(ctx); err != nil {
		t.Fatalf("later renewal after stalled login: %v", err)
	}
	if authCalls.Load() != 3 {
		t.Errorf("auth calls=%d, want initial, stalled and recovery", authCalls.Load())
	}
}
