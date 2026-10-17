package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// Diagnostics is readable with settings:read and lists API credentials: IDs,
// names, timestamps, key hints and which agents use each token. The token
// inventory is global, so a token caller must see only the tokens it may
// manage, as on the token endpoints themselves.
type diagnosticsTokenFixture struct {
	cfg             *config.Config
	orgA, def, orgB config.APITokenRecord
}

func newDiagnosticsTokenFixture(t *testing.T) *diagnosticsTokenFixture {
	t.Helper()
	mk := func(raw, name, org string, scopes []string) config.APITokenRecord {
		record, err := config.NewAPITokenRecord(raw, name, scopes)
		if err != nil {
			t.Fatalf("new token record %s: %v", name, err)
		}
		record.OrgID = org
		return *record
	}
	f := &diagnosticsTokenFixture{
		orgA: mk("diag-org-a-wildcard.12345678", "org-a admin", "org-a", []string{config.ScopeWildcard}),
		def:  mk("diag-default-wildcard.12345678", "default admin", "default", []string{config.ScopeWildcard}),
		orgB: mk("diag-org-b-scoped.12345678", "org-b reader", "org-b", []string{config.ScopeMonitoringRead}),
	}
	f.cfg = &config.Config{DataPath: t.TempDir(), APITokens: []config.APITokenRecord{f.orgA, f.def, f.orgB}}
	f.cfg.SortAPITokens()
	return f
}

func tokenIDs(diag *APITokenDiagnostic) map[string]bool {
	ids := map[string]bool{}
	for _, token := range diag.Tokens {
		ids[token.ID] = true
	}
	return ids
}

func TestAPITokenDiagnosticShowsOnlyTokensTheCallerMayManage(t *testing.T) {
	f := newDiagnosticsTokenFixture(t)
	monitor := newMonitorForDiagnostics(t, f.cfg)
	now := time.Now().UTC()
	for i, token := range []config.APITokenRecord{f.orgA, f.def, f.orgB} {
		report := agentsdocker.Report{
			Agent:     agentsdocker.AgentInfo{ID: "agent-" + token.Name, Version: "1.0.0"},
			Host:      agentsdocker.HostInfo{Hostname: "docker-" + token.Name, MachineID: "machine-" + token.Name},
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if _, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: token.ID}); err != nil {
			t.Fatalf("ApplyDockerReport %s: %v", token.Name, err)
		}
	}

	all := buildAPITokenDiagnostic(f.cfg, monitor, nil)
	if all.TokenCount != 3 || len(all.Usage) != 3 {
		t.Fatalf("an unfiltered view should list all three tokens and usages, got %d tokens, %d usages", all.TokenCount, len(all.Usage))
	}

	view := buildAPITokenDiagnostic(f.cfg, monitor, &f.orgA)
	ids := tokenIDs(view)
	if len(ids) != 1 || !ids[f.orgA.ID] || view.TokenCount != 1 {
		t.Fatalf("org-a caller should see only its own token, got ids %v count %d", ids, view.TokenCount)
	}
	if len(view.Usage) != 1 || view.Usage[0].TokenID != f.orgA.ID {
		t.Fatalf("usage should be limited to the caller's token, got %+v", view.Usage)
	}
	if view.UnusedTokenCount != 1 {
		t.Fatalf("unused count should be computed over the visible tokens, got %d", view.UnusedTokenCount)
	}
	// The existence of other tokens still shows in the global flag only.
	if !view.Enabled || view.RecommendTokenSetup {
		t.Fatalf("enabled=%v recommend=%v, want enabled and no setup recommendation", view.Enabled, view.RecommendTokenSetup)
	}

	instanceWide := buildAPITokenDiagnostic(f.cfg, monitor, &f.def)
	if instanceWide.TokenCount != 3 {
		t.Fatalf("an instance-wide caller passed to the builder sees %d tokens, want 3 (callers filter nothing for it)", instanceWide.TokenCount)
	}
}

func TestDiagnosticsFiltersTokenSectionAfterTheSharedCache(t *testing.T) {
	f := newDiagnosticsTokenFixture(t)
	router := &Router{config: f.cfg}

	// The cache holds the instance-wide result and is shared across callers.
	diagnosticsCacheMu.Lock()
	prevCache := diagnosticsCache
	diagnosticsCache = map[string]cachedDiagnosticsEntry{
		diagnosticsScopeKey(context.Background()): {
			diag:     DiagnosticsInfo{Version: "cached", APITokens: buildAPITokenDiagnostic(f.cfg, nil, nil)},
			cachedAt: time.Now(),
		},
	}
	diagnosticsCacheMu.Unlock()
	t.Cleanup(func() {
		diagnosticsCacheMu.Lock()
		diagnosticsCache = prevCache
		diagnosticsCacheMu.Unlock()
	})

	get := func(caller *config.APITokenRecord) (*httptest.ResponseRecorder, DiagnosticsInfo) {
		req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
		if caller != nil {
			attachAPITokenRecord(req, caller)
		}
		rec := httptest.NewRecorder()
		router.handleDiagnostics(rec, req)
		var payload DiagnosticsInfo
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode diagnostics: %v", err)
		}
		return rec, payload
	}

	rec, scoped := get(&f.orgA)
	if rec.Code != http.StatusOK || scoped.Version != "cached" {
		t.Fatalf("status %d version %q, want a 200 served from the cache", rec.Code, scoped.Version)
	}
	if ids := tokenIDs(scoped.APITokens); len(ids) != 1 || !ids[f.orgA.ID] {
		t.Fatalf("org-a caller saw tokens %v, want only its own", ids)
	}
	for _, other := range []config.APITokenRecord{f.def, f.orgB} {
		if strings.Contains(rec.Body.String(), other.ID) || strings.Contains(rec.Body.String(), other.Name) {
			t.Fatalf("the response leaked token %q: %s", other.Name, rec.Body.String())
		}
	}

	// A caller served right after still gets its own view, not the previous one.
	for name, caller := range map[string]*config.APITokenRecord{"instance-wide token": &f.def, "browser session": nil} {
		_, full := get(caller)
		if got := len(tokenIDs(full.APITokens)); got != 3 {
			t.Fatalf("%s saw %d tokens, want all 3 after a scoped caller was served from the same cache", name, got)
		}
	}
}

// Token writers change config.APITokens in place under config.Mu: scope edits,
// delete-then-append and rotation, and every token request records LastUsedAt.
// Diagnostics reads the inventory on every request from a scoped token, and
// when it rebuilds the cached instance view, so both paths must read it under
// the lock. Run with -race: an unlocked read is reported as a data race.
func TestDiagnosticsTokenSectionReadsTheInventoryUnderTheConfigLock(t *testing.T) {
	f := newDiagnosticsTokenFixture(t)
	router := &Router{config: f.cfg}

	diagnosticsCacheMu.Lock()
	prevCache := diagnosticsCache
	diagnosticsCache = map[string]cachedDiagnosticsEntry{
		diagnosticsScopeKey(context.Background()): {
			diag:     DiagnosticsInfo{Version: "cached", APITokens: &APITokenDiagnostic{}},
			cachedAt: time.Now(),
		},
	}
	diagnosticsCacheMu.Unlock()
	t.Cleanup(func() {
		diagnosticsCacheMu.Lock()
		diagnosticsCache = prevCache
		diagnosticsCacheMu.Unlock()
	})

	stop := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		scopeSets := [][]string{{config.ScopeWildcard}, {config.ScopeMonitoringRead, config.ScopeSettingsRead}}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			config.Mu.Lock()
			idx := i % len(f.cfg.APITokens)
			edited := f.cfg.APITokens[idx]
			edited.Scopes = append([]string(nil), scopeSets[i%len(scopeSets)]...)
			f.cfg.APITokens = append(f.cfg.APITokens[:idx], f.cfg.APITokens[idx+1:]...)
			f.cfg.APITokens = append(f.cfg.APITokens, edited)
			used := time.Now().UTC()
			f.cfg.APITokens[0].LastUsedAt = &used
			config.Mu.Unlock()
		}
	}()

	for i := 0; i < 200; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
		attachAPITokenRecord(req, &f.orgA)
		rec := httptest.NewRecorder()
		router.handleDiagnostics(rec, req)
		var payload DiagnosticsInfo
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode diagnostics: %v", err)
		}
		if ids := tokenIDs(payload.APITokens); len(ids) != 1 || !ids[f.orgA.ID] {
			t.Fatalf("org-a caller saw tokens %v during concurrent token edits, want only its own", ids)
		}

		if all := buildAPITokenDiagnostic(f.cfg, nil, nil); all.TokenCount != 3 {
			t.Fatalf("instance view counted %d tokens during concurrent token edits, want 3", all.TokenCount)
		}
	}

	close(stop)
	<-writerDone
}
