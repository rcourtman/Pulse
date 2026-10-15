package pbs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	monitorerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These are pinned, guest-local HTTPS fixtures with synthetic credentials.
// No backup, guest-agent, production authentication or installed recovery runs.
type pbsSessionFixture struct {
	client                                                *Client
	ctx                                                   context.Context
	entered, finish, readsArrived                         chan struct{}
	finishOnce                                            sync.Once
	authCalls, jsonCalls, formCalls, apiCalls, mismatched atomic.Int32
	outcome                                               atomic.Value
	gateReads                                             atomic.Bool
	phase                                                 atomic.Int32
}

// Done observes arrival at a pending wait (or the old HTTP transport). It does
// not inspect production-only counters, sleep or choose a favourable repeat.
type pbsSessionJoinContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *pbsSessionJoinContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}
func pbsSessionAwait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("PBS session fixture barrier did not complete")
	}
}
func (f *pbsSessionFixture) release() { f.finishOnce.Do(func() { close(f.finish) }) }
func newPBSSessionFixture(t *testing.T, outcome string, gateReads bool) *pbsSessionFixture {
	t.Helper()
	f := &pbsSessionFixture{entered: make(chan struct{}), finish: make(chan struct{}), readsArrived: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	f.ctx = ctx
	f.outcome.Store(outcome)
	f.gateReads.Store(gateReads)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			n := f.authCalls.Add(1)
			form := r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
			if form {
				f.formCalls.Add(1)
			} else {
				f.jsonCalls.Add(1)
			}
			if n > 1 {
				if n == 2 {
					close(f.entered)
				}
				select {
				case <-f.finish:
				case <-ctx.Done():
					return
				}
				mode := f.outcome.Load().(string)
				if !form && (mode == "form-400" || mode == "form-415") {
					status := 400
					if mode == "form-415" {
						status = 415
					}
					w.WriteHeader(status)
					fmt.Fprint(w, "synthetic encoding refusal")
					return
				}
				switch mode {
				case "refused-401":
					w.WriteHeader(401)
					fmt.Fprint(w, "synthetic renewal refusal")
					return
				case "refused-403":
					w.WriteHeader(403)
					fmt.Fprint(w, "synthetic renewal refusal")
					return
				case "unavailable":
					w.WriteHeader(503)
					fmt.Fprint(w, "synthetic renewal unavailable")
					return
				case "malformed":
					fmt.Fprint(w, `{"data":`)
					return
				}
			}
			fmt.Fprintf(w, `{"data":{"ticket":"session-%d","CSRFPreventionToken":"csrf-%d"}}`, n, n)
			return
		}
		cookie, err := r.Cookie("PBSAuthCookie")
		if err != nil || !strings.HasPrefix(cookie.Value, "session-") {
			f.mismatched.Add(1)
			w.WriteHeader(401)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("CSRFPreventionToken") != "csrf-"+strings.TrimPrefix(cookie.Value, "session-") {
			f.mismatched.Add(1)
			w.WriteHeader(403)
			return
		}
		n := f.apiCalls.Add(1)
		if f.gateReads.Load() {
			if n == 8 {
				close(f.readsArrived)
			}
			select {
			case <-f.readsArrived:
			case <-ctx.Done():
				return
			}
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/version"):
			fmt.Fprint(w, `{"data":{"version":"3.4.2","release":"1"}}`)
		case strings.HasSuffix(r.URL.Path, "/admin/datastore"):
			fmt.Fprint(w, `{"data":[{"store":"main"},{"store":"archive"}]}`)
		case strings.HasSuffix(r.URL.Path, "/rrd"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/gc"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.Contains(r.URL.Path, "/admin/datastore/"):
			used := 250000000 + int64(f.phase.Load())*50000000
			if strings.Contains(r.URL.Path, "/archive/") {
				used = 800000000
			}
			fmt.Fprintf(w, `{"data":{"total":1000000000,"used":%d,"avail":%d}}`, used, 1000000000-used)
		default:
			used := int64(2147483648) + int64(f.phase.Load())*1073741824
			cpu := 0.25 + float64(f.phase.Load())*0.25
			fmt.Fprintf(w, `{"data":{"cpu":%f,"memory":{"total":8589934592,"used":%d,"free":%d},"uptime":%d}}`, cpu, used, 8589934592-used, 7200+f.phase.Load()*60)
		}
	}))
	pin := sha256.Sum256(server.Certificate().Raw)
	c, err := NewClient(ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 5 * time.Second})
	if err != nil {
		cancel()
		server.Close()
		t.Fatal(err)
	}
	f.client = c
	// No concurrent callers exist while the fixture expires the initial ticket.
	c.auth.expiresAt = time.Now().Add(-time.Hour)
	t.Cleanup(func() { cancel(); f.release(); c.httpClient.CloseIdleConnections(); server.Close() })
	return f
}
func (f *pbsSessionFixture) node(ctx context.Context) error {
	status, err := f.client.GetNodeStatus(ctx)
	if err == nil && (status == nil || status.CPU != 0.25 || status.Memory.Total != 8589934592 || status.Memory.Used != 2147483648 || status.Uptime != 7200) {
		return fmt.Errorf("ordinary renewed node read lost its CPU/memory/uptime tuple")
	}
	return err
}

func TestPBSClientSessionConcurrentExpiry(t *testing.T) {
	for _, mode := range []string{"json", "form-400", "form-415", "refused-401", "refused-403", "unavailable", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			successful := mode == "json" || strings.HasPrefix(mode, "form-")
			f := newPBSSessionFixture(t, mode, successful)
			original := f.client.auth
			done := make(chan error, 8)
			go func() { done <- f.node(f.ctx) }()
			pbsSessionAwait(t, f.entered)
			for i := 1; i < 8; i++ {
				joined := &pbsSessionJoinContext{Context: f.ctx, joined: make(chan struct{})}
				go func() { done <- f.node(joined) }()
				pbsSessionAwait(t, joined.joined)
			}
			// A pending renewal cannot admit reads or another password request.
			if f.apiCalls.Load() != 0 || f.authCalls.Load() != 2 {
				t.Errorf("pending auth/API=%d/%d, want 2/0", f.authCalls.Load(), f.apiCalls.Load())
			}
			f.release()
			for i := 0; i < 8; i++ {
				select {
				case err := <-done:
					if successful && err != nil {
						t.Error(err)
					}
					if !successful && err == nil {
						t.Error("failed renewal admitted an ordinary read")
					}
				case <-f.ctx.Done():
					t.Fatal("concurrent node reads did not complete independently")
				}
			}
			want := int32(2)
			if strings.HasPrefix(mode, "form-") {
				want = 3
			}
			if f.authCalls.Load() != want || f.mismatched.Load() != 0 {
				t.Errorf("renewal/pair mismatch=%d/%d, want %d/0", f.authCalls.Load(), f.mismatched.Load(), want)
			}
			if f.jsonCalls.Load() != 2 || f.formCalls.Load() != want-2 {
				t.Errorf("JSON/form=%d/%d, want 2/%d", f.jsonCalls.Load(), f.formCalls.Load(), want-2)
			}
			f.gateReads.Store(false)
			if successful {
				if f.apiCalls.Load() != 8 {
					t.Errorf("ordinary reads=%d, want 8", f.apiCalls.Load())
				}
				// The same request producer pairs cookie and CSRF on the existing write
				// path; this targets only a synthetic endpoint, not any PBS operation.
				resp, err := f.client.post(f.ctx, "/synthetic-read", url.Values{"fixture": {"preserved"}})
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
			} else {
				if f.client.auth != original || f.apiCalls.Load() != 0 {
					t.Error("failed renewal changed the prior tuple or admitted a read")
				}
				f.outcome.Store("json")
				if err := f.node(f.ctx); err != nil {
					t.Fatal(err)
				}
				if f.authCalls.Load() != 3 || f.apiCalls.Load() != 1 {
					t.Errorf("later ordinary recovery auth/API=%d/%d, want 3/1", f.authCalls.Load(), f.apiCalls.Load())
				}
			}
			t.Logf("pinned HTTPS: renewal=%s; eight callers, one shared attempt; existing encoding/failure semantics retained", mode)
		})
	}
}

func TestPBSClientSessionWaiterCancellation(t *testing.T) {
	f := newPBSSessionFixture(t, "json", false)
	owner := make(chan error, 1)
	go func() { owner <- f.node(f.ctx) }()
	pbsSessionAwait(t, f.entered)
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	joined := &pbsSessionJoinContext{Context: ctx, joined: make(chan struct{})}
	waiter := make(chan error, 1)
	go func() { waiter <- f.node(joined) }()
	pbsSessionAwait(t, joined.joined)
	cancel()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.Canceled) || monitorerrors.IsAuthError(err) {
			t.Errorf("waiter result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter remained blocked")
	}
	if f.authCalls.Load() != 2 || f.apiCalls.Load() != 0 {
		t.Error("waiter replayed authentication or admitted a read")
	}
	select {
	case err := <-owner:
		t.Fatalf("waiter cancelled the owner: %v", err)
	default:
	}
	f.release()
	if err := <-owner; err != nil {
		t.Fatal(err)
	}
	if err := f.node(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.authCalls.Load() != 2 || f.apiCalls.Load() != 2 {
		t.Errorf("owner/later auth/API=%d/%d, want 2/2", f.authCalls.Load(), f.apiCalls.Load())
	}
}

func TestPBSClientSessionOwnerCancellation(t *testing.T) {
	f := newPBSSessionFixture(t, "json", false)
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	done := make(chan error, 2)
	go func() { done <- f.node(ctx) }()
	pbsSessionAwait(t, f.entered)
	joined := &pbsSessionJoinContext{Context: f.ctx, joined: make(chan struct{})}
	go func() { done <- f.node(joined) }()
	pbsSessionAwait(t, joined.joined)
	cancel()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) || monitorerrors.IsAuthError(err) {
				t.Errorf("shared owner cancellation: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("joined caller lost cancelled renewal result")
		}
	}
	if f.authCalls.Load() != 2 || f.apiCalls.Load() != 0 {
		t.Error("cancelled renewal admitted work or another attempt")
	}
	f.release()
	if err := f.node(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.authCalls.Load() != 3 || f.apiCalls.Load() != 1 {
		t.Errorf("later recovery auth/API=%d/%d, want 3/1", f.authCalls.Load(), f.apiCalls.Load())
	}
}

func TestPBSClientSessionTuplePublication(t *testing.T) {
	f := newPBSSessionFixture(t, "json", false)
	f.release()
	if err := f.node(f.ctx); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() {
		for i := 0; i < 32; i++ {
			if err := f.client.authenticate(f.ctx); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	go func() {
		for i := 0; i < 64; i++ {
			resp, err := f.client.post(f.ctx, "/synthetic-read", url.Values{"fixture": {"preserved"}})
			if err != nil {
				done <- err
				return
			}
			resp.Body.Close()
		}
		done <- nil
	}()
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	if f.mismatched.Load() != 0 || f.apiCalls.Load() != 65 {
		t.Errorf("pair mismatches/API=%d/%d, want 0/65", f.mismatched.Load(), f.apiCalls.Load())
	}
}

func TestPBSClientSessionTokenAndAPIRefusals(t *testing.T) {
	for _, mode := range []string{"token", "session"} {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			t.Run(fmt.Sprintf("%s/%d", mode, status), func(t *testing.T) {
				var authCalls, apiCalls atomic.Int32
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/access/ticket") {
						authCalls.Add(1)
						fmt.Fprint(w, `{"data":{"ticket":"initial","CSRFPreventionToken":"initial-csrf"}}`)
						return
					}
					apiCalls.Add(1)
					if mode == "token" && (r.Header.Get("Authorization") != "PBSAPIToken=synthetic@pbs!pulse:synthetic-token" || r.Header.Get("Cookie") != "" || r.Header.Get("CSRFPreventionToken") != "") {
						t.Error("token acquired session authority")
					}
					w.WriteHeader(status)
					fmt.Fprint(w, "synthetic API refusal")
				}))
				defer server.Close()
				pin := sha256.Sum256(server.Certificate().Raw)
				cfg := ClientConfig{Host: server.URL, User: "synthetic@pam", Password: "synthetic-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: time.Second}
				if mode == "token" {
					cfg.TokenName = "synthetic@pbs!pulse"
					cfg.TokenValue = "synthetic-token"
				}
				c, err := NewClient(cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer c.httpClient.CloseIdleConnections()
				_, err = c.GetVersion(context.Background())
				if got, known := HTTPStatus(err); !known || got != status {
					t.Errorf("typed refusal lost: %v", err)
				}
				want := int32(0)
				if mode == "session" {
					want = 1
				}
				if authCalls.Load() != want || apiCalls.Load() != 1 {
					t.Errorf("API refusal replayed: auth/API=%d/%d, want %d/1", authCalls.Load(), apiCalls.Load(), want)
				}
			})
		}
	}
}

// The actual client/decoder feeds the existing canonical PBS projection. The
// node/datastore-to-model assignment is the unchanged monitor projection; this
// is not an installed poll, stored History or whole-application acceptance.
func TestPBSClientSessionCanonicalProducer(t *testing.T) {
	f := newPBSSessionFixture(t, "json", false)
	f.release()
	project := func() unifiedresources.Resource {
		status, err := f.client.GetNodeStatus(f.ctx)
		if err != nil || status == nil {
			t.Fatalf("node observation: %v", err)
		}
		version, err := f.client.GetVersion(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		stores, err := f.client.GetDatastores(f.ctx)
		if err != nil || len(stores) != 2 {
			t.Fatalf("datastore observations: %d, %v", len(stores), err)
		}
		instance := models.PBSInstance{ID: "pbs-session", Name: "pbs-session", Host: "https://pbs-session.example.invalid:8007", Status: "online", ConnectionHealth: "healthy", Version: version.Version, CPU: status.CPU, Memory: float64(status.Memory.Used) / float64(status.Memory.Total) * 100, MemoryUsed: status.Memory.Used, MemoryTotal: status.Memory.Total, Uptime: status.Uptime, LastSeen: time.Now()}
		for _, ds := range stores {
			instance.Datastores = append(instance.Datastores, models.PBSDatastore{Name: ds.Store, Total: ds.Total, Used: ds.Used, Free: ds.Avail, Usage: float64(ds.Used) / float64(ds.Total) * 100, Status: ds.Status})
		}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(models.StateSnapshot{PBSInstances: []models.PBSInstance{instance}})
		resources := registry.ListByType(unifiedresources.ResourceTypePBS)
		if len(resources) != 1 {
			t.Fatalf("canonical PBS count=%d", len(resources))
		}
		return resources[0]
	}
	before := project()
	f.phase.Store(1)
	f.client.auth.expiresAt = time.Now().Add(-time.Hour) // No concurrent reader.
	after := project()
	if f.authCalls.Load() != 3 || f.mismatched.Load() != 0 {
		t.Fatalf("ordinary producer renewal/pair mismatch = %d/%d", f.authCalls.Load(), f.mismatched.Load())
	}
	encoded, err := json.Marshal(map[string]unifiedresources.Resource{"before": before, "after": after})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("PULSE_PBS_SESSION_PRODUCER=" + string(encoded))
}

// A caller's exhausted budget is not a credential refusal. In particular,
// token reads must not acquire a misleading authentication-failed wrapper.
func TestPBSClientSessionCancelledReadKeepsErrorClass(t *testing.T) {
	for _, mode := range []string{"token", "fresh-session", "expired-session"} {
		t.Run(mode, func(t *testing.T) {
			f := newPBSSessionFixture(t, "json", false)
			c := f.client
			if mode == "token" {
				cfg := c.config
				cfg.TokenName = "synthetic@pbs!pulse"
				cfg.TokenValue = "synthetic-token"
				var err error
				c, err = NewClient(cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer c.httpClient.CloseIdleConnections()
			} else if mode == "fresh-session" {
				c.auth.expiresAt = time.Now().Add(time.Hour)
			}
			ctx, cancel := context.WithCancel(f.ctx)
			cancel()
			_, err := c.GetVersion(ctx)
			if !errors.Is(err, context.Canceled) || monitorerrors.IsAuthError(err) {
				t.Errorf("unstarted/cancelled read acquired authentication failure: %v", err)
			}
			if f.authCalls.Load() != 1 || f.apiCalls.Load() != 0 {
				t.Errorf("cancelled read admitted auth/API=%d/%d, want initial only/0", f.authCalls.Load(), f.apiCalls.Load())
			}
		})
	}
}
