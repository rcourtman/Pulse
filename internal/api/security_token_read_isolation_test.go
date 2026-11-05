package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Probe before the response reaches a potentially slow network writer. Both
// successful JSON and http.Error's WriteHeader must run without config.Mu.
type tokenReadProbeWriter struct {
	*httptest.ResponseRecorder
	probe  func()
	probed bool
}

func (w *tokenReadProbeWriter) beforeWrite() {
	if !w.probed {
		w.probed = true
		w.probe()
	}
}

func (w *tokenReadProbeWriter) WriteHeader(status int) {
	w.beforeWrite()
	w.ResponseRecorder.WriteHeader(status)
}

func (w *tokenReadProbeWriter) Write(body []byte) (int, error) {
	w.beforeWrite()
	return w.ResponseRecorder.Write(body)
}

func TestSecurityTokenReadResponsesDoNotHoldConfigLock(t *testing.T) {
	for _, name := range []string{"list", "org list", "empty list", "detail", "missing detail", "other org detail"} {
		t.Run(name, func(t *testing.T) {
			f := newOrgTokenFixture(t)
			path := "/api/security/tokens"
			handler := f.router.handleListAPITokens
			status := http.StatusOK
			var caller *config.APITokenRecord
			var want any
			switch name {
			case "list":
				tokens := make([]apiTokenDTO, 0, len(f.cfg.APITokens))
				for _, record := range f.cfg.APITokens {
					tokens = append(tokens, toAPITokenDTO(record))
				}
				want = map[string]any{"tokens": tokens}
			case "org list":
				caller = &f.orgA
				want = map[string]any{"tokens": []apiTokenDTO{toAPITokenDTO(f.orgA)}}
			case "empty list":
				f.cfg.APITokens = nil
				want = map[string]any{"tokens": []apiTokenDTO{}}
			case "detail":
				caller = &f.orgA
				path += "/" + f.orgA.ID
				handler = f.router.handleGetAPIToken
				want = map[string]any{"record": toAPITokenDTO(f.orgA)}
			case "missing detail":
				path += "/missing"
				handler = f.router.handleGetAPIToken
				status = http.StatusNotFound
			case "other org detail":
				caller = &f.orgA
				path += "/" + f.orgB.ID
				handler = f.router.handleGetAPIToken
				status = http.StatusNotFound
			}
			var expected []byte
			if want != nil {
				var err error
				expected, err = json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			attachAPITokenRecord(req, caller)
			writer := &tokenReadProbeWriter{ResponseRecorder: httptest.NewRecorder()}
			writer.probe = func() {
				// No background router runs in this control, so failure to acquire
				// proves the handler itself still owns the global lock.
				if !config.Mu.TryLock() {
					t.Error("token read holds the global config lock during response output")
					return
				}
				defer config.Mu.Unlock()
				// A writer can now replace the live inventory without changing
				// the already-selected, redacted response snapshot.
				f.cfg.APITokens = nil
			}
			handler(writer, req)
			if !writer.probed {
				t.Fatal("response did not exercise the output probe")
			}
			if writer.Code != status {
				t.Fatalf("status = %d, want %d", writer.Code, status)
			}
			if status == http.StatusOK {
				if !bytes.Equal(bytes.TrimSpace(writer.Body.Bytes()), expected) {
					t.Fatal("response snapshot, inventory ordering, redaction or organization filtering changed")
				}
			} else if writer.Body.String() != "Token not found\n" {
				t.Fatal("missing and hidden credentials no longer have the same response")
			}
		})
	}
}

func TestSecurityTokenReadRoutesAllowAuthenticationDuringOutput(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	for _, kind := range []string{"list", "detail", "missing"} {
		t.Run(kind, func(t *testing.T) {
			const raw = "synthetic-token-read-isolation.12345678"
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, raw, []string{config.ScopeWildcard}, nil))
			router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
			path := "/api/security/tokens"
			if kind == "detail" {
				path += "/" + cfg.APITokens[0].ID
			} else if kind == "missing" {
				path += "/missing"
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-API-Token", raw)
			writer := &tokenReadProbeWriter{ResponseRecorder: httptest.NewRecorder()}
			var completed chan bool
			writer.probe = func() {
				completed = make(chan bool, 1)
				go func() {
					authReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
					authReq.Header.Set("X-API-Token", raw)
					completed <- CheckAuth(cfg, httptest.NewRecorder(), authReq)
				}()
				select {
				case authenticated := <-completed:
					if !authenticated {
						t.Error("independent token authentication was denied")
					}
					completed = nil
				case <-time.After(2 * time.Second):
					t.Error("independent token authentication stalled behind response output")
				}
			}
			router.Handler().ServeHTTP(writer, req)
			// Even the failing baseline releases its stalled authentication when
			// the response finishes. Do not leak the diagnostic goroutine.
			if completed != nil {
				select {
				case <-completed:
				case <-time.After(2 * time.Second):
					t.Fatal("authentication did not finish after response completion")
				}
			}
			if !writer.probed {
				t.Fatal("route did not reach the response writer")
			}
			want := http.StatusOK
			if kind == "missing" {
				want = http.StatusNotFound
			}
			if writer.Code != want {
				t.Fatalf("status = %d, want %d", writer.Code, want)
			}
			if strings.Contains(writer.Body.String(), raw) || strings.Contains(writer.Body.String(), cfg.APITokens[0].Hash) {
				t.Error("token read disclosed a secret or its hash")
			}
		})
	}
}

func TestSecurityTokenReadDTOOwnsMutableFields(t *testing.T) {
	used, expires := time.Now().UTC(), time.Now().UTC().Add(time.Hour)
	record := config.APITokenRecord{
		ID: "stable-id", Name: "original", CreatedAt: used,
		LastUsedAt: &used, ExpiresAt: &expires,
		Scopes:   []string{config.ScopeSettingsRead},
		Metadata: map[string]string{apiTokenMetadataOwnerUserID: "original-owner"},
	}
	want := toAPITokenDTO(record.Clone())
	snapshot := toAPITokenDTO(record)
	record.Name = "replacement"
	record.Scopes[0] = config.ScopeWildcard
	record.Metadata[apiTokenMetadataOwnerUserID] = "replacement-owner"
	*record.LastUsedAt = used.Add(time.Minute)
	*record.ExpiresAt = expires.Add(time.Hour)
	if !reflect.DeepEqual(snapshot, want) {
		t.Error("live metadata edits changed the detached response DTO")
	}
	if dto := toAPITokenDTO(config.APITokenRecord{}); dto.LastUsedAt != nil || dto.ExpiresAt != nil || dto.Scopes == nil {
		t.Error("unset timestamps and empty scope collection changed their JSON shape")
	}
}

func TestSecurityTokenReadDTOJSONPreservesWireShape(t *testing.T) {
	used := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	expires := used.Add(time.Hour)
	record := config.APITokenRecord{
		ID: "snapshot-token", Name: "Read-only inventory", Hash: "never-return-this-hash",
		Prefix: "pulse_", Suffix: "1234", CreatedAt: used.Add(-time.Hour),
		LastUsedAt: &used, ExpiresAt: &expires,
		Scopes: []string{config.ScopeSettingsRead, config.ScopeMonitoringRead},
		OrgID:  "private-org", OrgIDs: []string{"private-org", "another-org"},
		Metadata: map[string]string{apiTokenMetadataOwnerUserID: "inventory-owner", "internal": "never-return-this-value"},
	}
	got, err := json.Marshal(toAPITokenDTO(record))
	if err != nil {
		t.Fatal(err)
	}
	// Independent literal: do not derive the wire-compatibility oracle from
	// the same conversion that produces the actual response.
	const want = `{"id":"snapshot-token","name":"Read-only inventory","prefix":"pulse_","suffix":"1234","createdAt":"2026-11-05T09:00:00Z","lastUsedAt":"2026-11-05T10:00:00Z","expiresAt":"2026-11-05T11:00:00Z","scopes":["settings:read","monitoring:read"],"ownerUserId":"inventory-owner"}`
	if string(got) != want {
		t.Fatal("token read changed its field values, scope ordering or metadata redaction")
	}
}
