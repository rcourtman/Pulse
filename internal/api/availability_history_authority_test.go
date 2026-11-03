package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

const availabilityAuthorityToken = "availability-reader-fixture.12345678"
const availabilityAuthorityControl = "availability-control-fixture.12345678"
const availabilityAuthorityTarget = "availability-history-private-fixture"
const availabilityAuthorityBody = `{"targetIds":["` + availabilityAuthorityTarget + `","missing"],"range":"24h"}`

// The request body is a natural wait after route authentication, before a
// history query. No auth stub, database lock or runtime test hook is needed.
type availabilityAuthorityBodyGate struct {
	io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	open    sync.Once
}

func (b *availabilityAuthorityBodyGate) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.Reader.Read(p)
}
func (b *availabilityAuthorityBodyGate) Close() error { return nil }
func (b *availabilityAuthorityBodyGate) unblock()     { b.open.Do(func() { close(b.release) }) }

type availabilityAuthorityFixture struct {
	router *Router
	server *httptest.Server
}

func newAvailabilityAuthorityFixture(t *testing.T, scopes []string) *availabilityAuthorityFixture {
	t.Helper()
	setMockModeForTest(t, false)
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, availabilityAuthorityToken, scopes, nil),
		newTokenRecord(t, availabilityAuthorityControl, []string{config.ScopeWildcard}, nil),
	)
	cfg.AuthUser, cfg.AuthPass = "operator", "fixture-nonempty-hash"
	InitSessionStore(cfg.DataPath)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownResourceStores)
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	setUnexportedField(t, monitor, "metricsStore", store)
	router.monitor = monitor
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID: availabilityAuthorityTarget, Name: "Synthetic gateway", Address: "gateway.invalid",
		TargetKind: config.AvailabilityTargetService, Protocol: config.AvailabilityProbeHTTPS, Enabled: true,
	})
	if err := router.persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	latency := int64(24)
	if err := store.WriteAvailabilityObservationSync(metrics.AvailabilityObservation{
		ObservationID: "availability-authority-fixture", TargetID: target.ID, ConfigRevision: target.ConfigRevision,
		Outcome: metrics.AvailabilityReachable, ObservedAt: at, TimelineAt: at, IngestedAt: at,
		ValidFor: 5 * time.Minute, ExecutionSource: metrics.AvailabilitySourceLocal, LatencyMillis: &latency,
	}); err != nil {
		t.Fatal(err)
	}
	f := &availabilityAuthorityFixture{router: router, server: httptest.NewServer(router.Handler())}
	t.Cleanup(f.server.Close)
	return f
}

func (f *availabilityAuthorityFixture) request(t *testing.T, raw, transport, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/availability-history", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	switch transport {
	case "header":
		req.Header.Set("X-API-Token", raw)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
		req.Header.Set("X-CSRF-Token", generateCSRFToken(raw))
	}
	return req
}

func (f *availabilityAuthorityFixture) begin(t *testing.T, raw, transport, body string) (<-chan *httptest.ResponseRecorder, func()) {
	t.Helper()
	req := f.request(t, raw, transport, body)
	gate := &availabilityAuthorityBodyGate{Reader: req.Body, entered: make(chan struct{}), release: make(chan struct{})}
	req.Body = gate
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.router.Handler().ServeHTTP(w, req)
		result <- w
	}()
	t.Cleanup(gate.unblock)
	select {
	case <-gate.entered:
	case w := <-result:
		t.Fatalf("history returned before body admission: %d %s", w.Code, w.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("history did not reach admitted body read")
	}
	return result, gate.unblock
}

func readAvailabilityAuthorityResult(t *testing.T, result <-chan *httptest.ResponseRecorder, status int) *httptest.ResponseRecorder {
	t.Helper()
	select {
	case w := <-result:
		if w.Code != status {
			t.Errorf("history status = %d, want %d: %s", w.Code, status, w.Body.String())
		}
		if status == http.StatusUnauthorized {
			if w.Body.String() != "Authentication required\n" || strings.Contains(w.Header().Get("Content-Type"), "json") || strings.Contains(w.Body.String(), availabilityAuthorityTarget) {
				t.Error("withdrawn history disclosed data or implied successful empty JSON")
			}
		}
		if status == http.StatusOK {
			var response availabilityHistoryResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Targets) != 2 {
				t.Fatalf("lost batch members: %s", w.Body.String())
			}
			history := response.Targets[0]
			if history.TargetID != availabilityAuthorityTarget || history.Summary == nil || history.Summary.ReachableSeconds != 300 || len(history.Buckets) == 0 || response.Targets[1].Error == nil || response.Targets[1].Error.Code != "not_found" {
				t.Errorf("history or per-target absence changed: %s", w.Body.String())
			}
			if w.Header().Get("Content-Type") != "application/json" {
				t.Error("history media changed")
			}
		}
		return w
	case <-time.After(5 * time.Second):
		t.Fatal("history did not finish after body release")
	}
	return nil
}

func (f *availabilityAuthorityFixture) revoke(t *testing.T) {
	t.Helper()
	id := f.router.config.APITokens[0].ID
	req, err := http.NewRequest(http.MethodDelete, f.server.URL+"/api/security/tokens/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Token", availabilityAuthorityControl)
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("token DELETE = %d", response.StatusCode)
	}
	tokens, err := f.router.persistence.LoadAPITokens()
	if err != nil || len(tokens) != 1 || tokens[0].ID == id {
		t.Fatalf("token withdrawal not persisted: %v", err)
	}
}

func TestAvailabilityHistoryWithdrawnTokenCannotDisclose(t *testing.T) {
	for _, transport := range []string{"header", "bearer"} {
		for _, change := range []string{"revoked", "expired", "scope", "organization", "credential", "principal"} {
			t.Run(transport+"/"+change, func(t *testing.T) {
				f := newAvailabilityAuthorityFixture(t, []string{config.ScopeMonitoringRead})
				control, releaseControl := f.begin(t, availabilityAuthorityControl, "header", availabilityAuthorityBody)
				primary, release := f.begin(t, availabilityAuthorityToken, transport, availabilityAuthorityBody)
				if change == "revoked" {
					f.revoke(t)
				} else {
					config.Mu.Lock()
					record := &f.router.config.APITokens[0]
					switch change {
					case "expired":
						past := time.Now().Add(-time.Second)
						record.ExpiresAt = &past
					case "scope":
						record.Scopes = []string{config.ScopeSettingsRead}
					case "organization":
						record.OrgIDs = []string{"other-org"}
					case "credential":
						record.ID = "replacement-availability-credential"
					case "principal":
						record.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "replacement-availability-principal"}
					}
					config.Mu.Unlock()
				}
				release()
				releaseControl()
				withdrawn := readAvailabilityAuthorityResult(t, primary, http.StatusUnauthorized)
				allowed := readAvailabilityAuthorityResult(t, control, http.StatusOK)
				if transport == "header" && change == "revoked" {
					for name, response := range map[string]*httptest.ResponseRecorder{"withdrawn": withdrawn, "control": allowed} {
						wire, _ := json.Marshal(map[string]any{"status": response.Code, "header": response.Header(), "body": response.Body.String()})
						t.Logf("AVAILABILITY_WIRE_%s %s", name, wire)
					}
				}
				fresh := httptest.NewRecorder()
				f.router.Handler().ServeHTTP(fresh, f.request(t, availabilityAuthorityToken, transport, availabilityAuthorityBody))
				if change == "credential" || change == "principal" {
					// The raw key still has monitoring-read authority, now under
					// its replacement identity. Only the old admission is invalid;
					// a new request must use the current identity normally.
					result := make(chan *httptest.ResponseRecorder, 1)
					result <- fresh
					readAvailabilityAuthorityResult(t, result, http.StatusOK)
				} else if fresh.Code == http.StatusOK {
					t.Error("fresh read reused withdrawn authority")
				}
			})
		}
	}
}

func TestAvailabilityHistoryWithdrawnSessionCannotDisclose(t *testing.T) {
	for _, change := range []string{"logout", "expired", "replaced", "principal", "store"} {
		t.Run(change, func(t *testing.T) {
			f := newAvailabilityAuthorityFixture(t, []string{config.ScopeMonitoringRead})
			const raw = "synthetic-availability-session"
			GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
			control, releaseControl := f.begin(t, availabilityAuthorityControl, "header", availabilityAuthorityBody)
			primary, release := f.begin(t, raw, "session", availabilityAuthorityBody)
			if change == "logout" {
				logoutReadSession(t, f.server.URL, raw)
			} else if change == "store" {
				InitSessionStore(t.TempDir())
			} else {
				mutateReadSession(raw, change)
			}
			release()
			releaseControl()
			readAvailabilityAuthorityResult(t, primary, http.StatusUnauthorized)
			readAvailabilityAuthorityResult(t, control, http.StatusOK)
		})
	}
}

func TestAvailabilityHistoryWithdrawalWithholdsErrors(t *testing.T) {
	for _, body := range []string{`{`, `{"targetIds":["one"],"range":"14d"}`, availabilityAuthorityBody} {
		t.Run(body, func(t *testing.T) {
			f := newAvailabilityAuthorityFixture(t, []string{config.ScopeMonitoringRead})
			// The last case exercises an admitted provider-unavailable result.
			if body == availabilityAuthorityBody {
				f.router.monitor = nil
			}
			primary, release := f.begin(t, availabilityAuthorityToken, "header", body)
			f.revoke(t)
			release()
			readAvailabilityAuthorityResult(t, primary, http.StatusUnauthorized)
		})
	}
}

func TestAvailabilityHistoryKeepsAdmissionAndRecovery(t *testing.T) {
	f := newAvailabilityAuthorityFixture(t, []string{config.ScopeSettingsRead})
	for _, tc := range []struct {
		raw, transport string
		status         int
	}{
		{"", "header", http.StatusUnauthorized},
		{availabilityAuthorityToken, "header", http.StatusForbidden},
		{availabilityAuthorityControl, "header", http.StatusOK},
	} {
		w := httptest.NewRecorder()
		f.router.Handler().ServeHTTP(w, f.request(t, tc.raw, tc.transport, availabilityAuthorityBody))
		if w.Code != tc.status {
			t.Fatalf("admission = %d, want %d: %s", w.Code, tc.status, w.Body.String())
		}
	}
	// Restoring monitoring-read authorises a new request, not the old one.
	config.Mu.Lock()
	f.router.config.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
	config.Mu.Unlock()
	result, release := f.begin(t, availabilityAuthorityToken, "header", availabilityAuthorityBody)
	release()
	readAvailabilityAuthorityResult(t, result, http.StatusOK)
}
