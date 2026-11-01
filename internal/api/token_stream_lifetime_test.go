package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const scopedReadToken = "synthetic-scoped-stream.12345678"
const scopedReadControl = "synthetic-scoped-control.12345678"
const scopedReadMonitor = "synthetic-monitor-reader.12345678"

var protectedReadKinds = []string{"updates", "patrol", "preflights", "jobs"}

type protectedReadFixture struct {
	router *Router
	cfg    *config.Config
	server *httptest.Server
	patrol *ai.PatrolService
}

// Full production router: real auth context, tenant authorization/guard,
// route admin/scope policy, token lifetime guard, handlers and broadcasters.
// Only fresh temporary stores and synthetic events; no updater, Patrol run,
// deployment, command, agent or provider is started.
func newProtectedReadFixture(t *testing.T, scopes []string) *protectedReadFixture {
	t.Helper()
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, scopedReadToken, scopes, nil),
		newTokenRecord(t, scopedReadControl, []string{config.ScopeWildcard}, nil),
		newTokenRecord(t, scopedReadMonitor, []string{config.ScopeMonitoringRead}, nil))
	cfg.AuthUser = "stream-admin"
	cfg.ProxyAuthSecret = "synthetic-stream-proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	monitor, state, _ := newTestMonitor(t)
	syncTestResourceStore(t, monitor, state)
	router := NewRouter(cfg, monitor, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	svc := router.aiSettingsHandler.GetAIService(context.Background())
	svc.SetStateProvider(&stubStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil || router.deployHandlers == nil {
		t.Fatal("missing fixture read producer")
	}
	patrol.DebugResetStreamForRun("synthetic-read-run")
	patrol.DebugSetStreamPhase("analyzing")
	patrol.DebugAppendStreamContent("admitted-stream")
	router.updateManager.GetSSEBroadcaster().Broadcast(updates.UpdateStatus{Status: "idle", Message: "admitted-stream"})
	for _, kind := range []string{"preflights", "jobs"} {
		job := &deploy.Job{ID: kind, OrgID: "default", Status: deploy.JobRunning, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := router.deployStore.CreateJob(context.Background(), job); err != nil {
			t.Fatal(err)
		}
		if err := router.deployStore.AppendEvent(context.Background(), &deploy.Event{ID: kind + "-initial", JobID: kind, Type: deploy.EventInstallOutput, Message: "admitted-stream", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return &protectedReadFixture{router: router, cfg: cfg, server: server, patrol: patrol}
}

func protectedReadPath(kind string) string {
	switch kind {
	case "updates":
		return "/api/updates/stream"
	case "patrol":
		return "/api/ai/patrol/stream"
	default:
		return "/api/agent-deploy/" + kind + "/" + kind + "/events"
	}
}

func (f *protectedReadFixture) request(t *testing.T, kind, raw, transport string) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 22*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, f.server.URL+protectedReadPath(kind), nil)
	// Keep the selected org explicit: a token rebound to another org must not
	// silently select that org instead of exercising the selected-org denial.
	req.Header.Set("X-Pulse-Org-ID", "default")
	switch transport {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	case "proxy":
		req.Header.Set("X-Proxy-Secret", f.cfg.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "stream-admin")
	default:
		if raw != "" {
			req.Header.Set("X-API-Token", raw)
		}
	}
	return req
}

func (f *protectedReadFixture) open(t *testing.T, kind, raw, transport string) *bufio.Reader {
	t.Helper()
	resp, err := f.server.Client().Do(f.request(t, kind, raw, transport))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s admission %d: %s", kind, resp.StatusCode, body)
	}
	reader := bufio.NewReader(resp.Body)
	readProtectedMarker(t, reader, "admitted-stream")
	return reader
}

func readProtectedMarker(t *testing.T, reader *bufio.Reader, marker string) {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("before %q: %v", marker, err)
		}
		if strings.HasPrefix(line, "data:") && strings.Contains(line, marker) {
			return
		}
	}
}

func (f *protectedReadFixture) emit(kind, marker string) {
	switch kind {
	case "updates":
		f.router.updateManager.GetSSEBroadcaster().Broadcast(updates.UpdateStatus{Status: "idle", Message: marker})
	case "patrol":
		f.patrol.DebugAppendStreamContent(marker)
	default:
		f.router.deployHandlers.broadcastSSE(kind, &deploy.Event{ID: "live", JobID: kind, Type: deploy.EventInstallOutput, Message: marker})
	}
}

func protectedReaderEOF(t *testing.T, reader *bufio.Reader, forbidden string) {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if strings.Contains(line, forbidden) {
			t.Fatalf("withdrawn reader disclosed %q", forbidden)
		}
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("withdrawn stream did not close: %v", err)
		}
	}
}

func (f *protectedReadFixture) assertJobAndSubscription(t *testing.T, kind string, wantReaders int) {
	t.Helper()
	if kind == "patrol" {
		return
	} // The HTTP handler owns Patrol unsubscription.
	deadline := time.Now().Add(time.Second)
	for {
		count := 0
		if kind == "updates" {
			count = f.router.updateManager.GetSSEBroadcaster().GetClientCount()
		} else {
			h := f.router.deployHandlers
			h.sseMu.Lock()
			sub := h.sseSubs[kind]
			h.sseMu.Unlock()
			if sub != nil {
				sub.mu.Lock()
				count = len(sub.clients)
				sub.mu.Unlock()
			}
		}
		if count == wantReaders {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s has %d subscribers, want %d", kind, count, wantReaders)
		}
		time.Sleep(time.Millisecond)
	}
	if kind == "jobs" || kind == "preflights" {
		job, err := f.router.deployStore.GetJob(context.Background(), kind)
		if err != nil || job == nil || job.Status != deploy.JobRunning {
			t.Fatalf("read withdrawal changed job: %+v, %v", job, err)
		}
		// A later independent state transition still works; no job cancel ran.
		if err := f.router.deployStore.UpdateJobStatus(context.Background(), kind, deploy.JobSucceeded); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProtectedReadStreamsWithdrawAuthority(t *testing.T) {
	for _, kind := range protectedReadKinds {
		for _, transport := range []string{"header", "bearer"} {
			for _, change := range []string{"revoked", "expired", "scope_removed", "org_access_removed", "credential_replaced", "principal_replaced", "invalid_role"} {
				t.Run(kind+"/"+transport+"/"+change, func(t *testing.T) {
					scope := config.ScopeSettingsRead
					if kind == "patrol" {
						scope = config.ScopeAIExecute
					}
					f := newProtectedReadFixture(t, []string{scope})
					control := f.open(t, kind, scopedReadControl, "header")
					primary := f.open(t, kind, scopedReadToken, transport)
					config.Mu.Lock()
					switch change {
					case "revoked":
						f.cfg.RemoveAPIToken(f.cfg.APITokens[0].ID)
					case "expired":
						past := time.Now().Add(-time.Second)
						f.cfg.APITokens[0].ExpiresAt = &past
					case "scope_removed":
						f.cfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
					case "org_access_removed":
						f.cfg.APITokens[0].OrgIDs = []string{"different-org"}
					case "credential_replaced":
						f.cfg.APITokens[0].ID = "different-credential"
					case "principal_replaced":
						f.cfg.APITokens[0].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "different-principal"}
					case "invalid_role":
						f.cfg.APITokens[0].Metadata = map[string]string{internalauth.RuntimeRoleMetadataKey: internalauth.RuntimeRoleActionRunner}
					}
					config.Mu.Unlock()
					forbidden := "private-after-withdrawal"
					f.emit(kind, forbidden)
					protectedReaderEOF(t, primary, forbidden)
					readProtectedMarker(t, control, forbidden)
					f.assertJobAndSubscription(t, kind, 1)
					if change == "revoked" || change == "expired" || change == "scope_removed" || change == "org_access_removed" || change == "invalid_role" {
						resp, err := f.server.Client().Do(f.request(t, kind, scopedReadToken, transport))
						if err != nil {
							t.Fatal(err)
						}
						resp.Body.Close()
						want := http.StatusForbidden
						if change == "revoked" || change == "expired" || change == "invalid_role" {
							want = http.StatusUnauthorized
						}
						if resp.StatusCode != want {
							t.Fatalf("fresh %s = %d want %d", change, resp.StatusCode, want)
						}
					}
				})
			}
		}
	}
}

func TestProtectedReadStreamsDurableDeletion(t *testing.T) {
	for _, kind := range protectedReadKinds {
		t.Run(kind, func(t *testing.T) {
			f := newProtectedReadFixture(t, []string{config.ScopeSettingsRead, config.ScopeAIExecute})
			primary := f.open(t, kind, scopedReadToken, "header")
			control := f.open(t, kind, scopedReadControl, "header")
			id := f.cfg.APITokens[0].ID
			req, _ := http.NewRequest(http.MethodDelete, f.server.URL+"/api/security/tokens/"+id, nil)
			req.Header.Set("X-API-Token", scopedReadControl)
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("token DELETE = %d: %s", resp.StatusCode, body)
			}
			persisted, err := f.router.persistence.LoadAPITokens()
			if err != nil || len(persisted) != 2 {
				t.Fatalf("durable tokens = %d: %v", len(persisted), err)
			}
			for _, record := range persisted {
				if record.ID == id {
					t.Fatal("deleted credential remains on disk")
				}
			}
			f.emit(kind, "after-durable-delete")
			protectedReaderEOF(t, primary, "after-durable-delete")
			readProtectedMarker(t, control, "after-durable-delete")
			f.assertJobAndSubscription(t, kind, 1)
		})
	}
}

func TestProtectedReadStreamsIdleWithdrawal(t *testing.T) {
	f := newProtectedReadFixture(t, []string{config.ScopeSettingsRead, config.ScopeAIExecute})
	readers := make(map[string]*bufio.Reader)
	for _, kind := range protectedReadKinds {
		readers[kind] = f.open(t, kind, scopedReadToken, "header")
	}
	config.Mu.Lock()
	f.cfg.RemoveAPIToken(f.cfg.APITokens[0].ID)
	config.Mu.Unlock()
	start := time.Now()
	// No synthetic event wakes these handlers. The production 15-second
	// recheck must end even the silent update/deployment connections.
	for _, kind := range protectedReadKinds {
		protectedReaderEOF(t, readers[kind], "private-idle-withdrawn")
		f.assertJobAndSubscription(t, kind, 0)
	}
	if time.Since(start) > 20*time.Second {
		t.Fatal("idle withdrawal exceeded bounded recheck")
	}
}

func TestProtectedReadStreamsPreserveSessionAndProxy(t *testing.T) {
	for _, kind := range protectedReadKinds {
		for _, transport := range []string{"session", "proxy"} {
			t.Run(kind+"/"+transport, func(t *testing.T) {
				f := newProtectedReadFixture(t, []string{config.ScopeSettingsRead, config.ScopeAIExecute})
				session := fmt.Sprintf("synthetic-read-session-%d", time.Now().UnixNano())
				GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", "stream-admin")
				t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
				reader := f.open(t, kind, session, transport)
				config.Mu.Lock()
				f.cfg.RemoveAPIToken(f.cfg.APITokens[0].ID)
				config.Mu.Unlock()
				f.emit(kind, "wanted-after-unrelated-deletion")
				readProtectedMarker(t, reader, "wanted-after-unrelated-deletion")
			})
		}
	}
}

func TestProtectedReadStreamsRetainAdmissionPolicy(t *testing.T) {
	f := newProtectedReadFixture(t, []string{config.ScopeSettingsRead, config.ScopeAIExecute})
	for _, kind := range protectedReadKinds {
		for _, raw := range []string{"", scopedReadMonitor} {
			t.Run(kind+"/"+raw, func(t *testing.T) {
				resp, err := f.server.Client().Do(f.request(t, kind, raw, "header"))
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				want := http.StatusForbidden
				if raw == "" {
					want = http.StatusUnauthorized
				}
				if resp.StatusCode != want {
					t.Fatalf("admission = %d want %d", resp.StatusCode, want)
				}
			})
		}
	}
	// A job belonging to another org stays hidden from an admitted default reader.
	if err := f.router.deployStore.CreateJob(context.Background(), &deploy.Job{ID: "foreign", OrgID: "other-org", Status: deploy.JobRunning}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"jobs", "preflights"} {
		req := f.request(t, kind, scopedReadControl, "header")
		req.URL.Path = "/api/agent-deploy/" + kind + "/foreign/events"
		resp, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("foreign %s = %d", kind, resp.StatusCode)
		}
	}
}

func TestProtectedReadStreamsKeepOrgAndInstanceSplit(t *testing.T) {
	for _, kind := range protectedReadKinds {
		t.Run(kind, func(t *testing.T) {
			f := newProtectedReadFixture(t, []string{config.ScopeSettingsRead, config.ScopeAIExecute})
			reader := f.open(t, kind, scopedReadToken, "header")
			config.Mu.Lock()
			f.cfg.APITokens[0].OrgIDs = []string{"default", "another-org"}
			config.Mu.Unlock()
			f.emit(kind, "still-in-selected-org")
			if kind == "updates" {
				protectedReaderEOF(t, reader, "still-in-selected-org")
			} else {
				readProtectedMarker(t, reader, "still-in-selected-org")
			}
		})
	}
}

func TestTokenStreamWriterChecksReplayQueueAndFinalClosure(t *testing.T) {
	for _, stage := range []string{"before_replay", "between_replay", "queued_write", "flush", "after_handler"} {
		t.Run(stage, func(t *testing.T) {
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, scopedReadToken, []string{config.ScopeSettingsRead}, nil))
			revoke := func() { config.Mu.Lock(); cfg.RemoveAPIToken(cfg.APITokens[0].ID); config.Mu.Unlock() }
			var retained http.ResponseWriter
			handler := retainTokenStreamAuthority(cfg, config.ScopeSettingsRead, true, func(w http.ResponseWriter, r *http.Request) {
				retained = w
				if stage == "before_replay" {
					revoke()
				}
				fmt.Fprint(w, "data: admitted\n\n")
				if stage != "before_replay" && stage != "after_handler" {
					revoke()
				}
				if stage == "flush" {
					w.(http.Flusher).Flush()
					if r.Context().Err() == nil {
						t.Error("withdrawn flush did not end read")
					}
					return
				}
				if stage == "after_handler" {
					return
				}
				if stage == "queued_write" {
					var wg sync.WaitGroup
					wg.Add(1)
					go func() { defer wg.Done(); fmt.Fprint(w, "data: forbidden\n\n") }()
					wg.Wait()
				} else {
					fmt.Fprint(w, "data: forbidden\n\n")
				}
				if r.Context().Err() == nil {
					t.Error("withdrawn write did not end read")
				}
			})
			req := httptest.NewRequest(http.MethodGet, "/api/updates/stream", nil)
			req.Header.Set("X-API-Token", scopedReadToken)
			rec := httptest.NewRecorder()
			AuthContextMiddleware(cfg, nil, handler).ServeHTTP(rec, req)
			if stage == "after_handler" {
				if _, err := retained.Write([]byte("data: forbidden\n\n")); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("late write = %v", err)
				}
			}
			if strings.Contains(rec.Body.String(), "forbidden") {
				t.Fatal("guard disclosed withdrawn/late frame")
			}
			if stage == "before_replay" && rec.Body.Len() != 0 {
				t.Fatal("guard disclosed initial replay")
			}
		})
	}
}

// The update broadcaster can still hold a writer after its HTTP handler has
// observed cancellation. Closure must join the active write and reject later
// queued writes before the ResponseWriter returns to the HTTP server.
type blockingTokenStreamWriter struct {
	*httptest.ResponseRecorder
	entered chan struct{}
	release chan struct{}
}

func (w *blockingTokenStreamWriter) Write(p []byte) (int, error) {
	close(w.entered)
	<-w.release
	return w.ResponseRecorder.Write(p)
}
func TestTokenStreamWriterCloseJoinsActiveWrite(t *testing.T) {
	base := &blockingTokenStreamWriter{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &tokenStreamWriter{writer: base, flusher: base, ctx: ctx, cancel: cancel, allowed: func() bool { return true }}
	wrote, closed := make(chan struct{}), make(chan struct{})
	go func() { defer close(wrote); _, _ = writer.Write([]byte("admitted")) }()
	<-base.entered
	go func() { writer.close(); close(closed) }()
	select {
	case <-closed:
		t.Error("closure returned during an active write")
	case <-time.After(20 * time.Millisecond):
	}
	close(base.release)
	<-wrote
	<-closed
	if _, err := writer.Write([]byte("forbidden")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("late write = %v", err)
	}
	if base.Body.String() != "admitted" {
		t.Fatalf("late bytes = %q", base.Body.String())
	}
}

func TestTokenStreamWriterNonTokenReadClosesBeforeReturn(t *testing.T) {
	var retained http.ResponseWriter
	handler := retainTokenStreamAuthority(&config.Config{}, config.ScopeSettingsRead, true, func(w http.ResponseWriter, r *http.Request) {
		retained = w
		fmt.Fprint(w, "data: admitted-session\n\n")
		w.(http.Flusher).Flush()
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/updates/stream", nil))
	if _, err := retained.Write([]byte("late-broadcast")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("non-token writer outlived HTTP handler: %v", err)
	}
	if strings.Contains(rec.Body.String(), "late-broadcast") {
		t.Fatal("non-token read accepted a write after handler return")
	}
}
