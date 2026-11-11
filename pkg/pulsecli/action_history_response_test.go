package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type actionHistoryFixture struct {
	name, field, path, resourceID string
	args                          []string
}

func actionHistoryFixtures() []actionHistoryFixture {
	return []actionHistoryFixture{
		{"filtered-audit", "audits", "/api/audit/actions", "vm:42", []string{"actions", "audit", "--resource-id", "vm:42"}},
		{"all-audit", "audits", "/api/audit/actions", "", []string{"actions", "audit"}},
		{"events", "events", "/api/audit/actions/act_saved/events", "", []string{"actions", "events", "--action-id", "act_saved"}},
	}
}

func runActionHistoryFixture(t *testing.T, fixture actionHistoryFixture, body string, status int, chunked bool) (string, string, error) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != fixture.path || r.Header.Get("Authorization") != "Bearer synthetic-history-token" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected history request: method=%s path=%s bearer=%t accept=%s", r.Method, r.URL.Path, r.Header.Get("Authorization") == "Bearer synthetic-history-token", r.Header.Get("Accept"))
		}
		if r.URL.Query().Get("resourceId") != fixture.resourceID || r.URL.Query().Get("limit") != "5" || r.URL.Query().Get("since") != "2026-05-03T10:00:00Z" {
			t.Errorf("unexpected history query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if chunked {
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	cmd := newTestActionsRootCommand(map[string]string{"PULSE_API_TOKEN": "synthetic-history-token", "PULSE_API_URL": server.URL})
	args := append([]string(nil), fixture.args...)
	cmd.SetArgs(append(args, "--limit", "5", "--since", "2026-05-03T11:00:00+01:00"))
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := cmd.ExecuteContext(ctx)
	if got := requests.Load(); got != 1 {
		t.Fatalf("history read made %d requests, want one without retry", got)
	}
	return stdout.String(), stderr.String(), err
}

func historyEnvelope(fixture actionHistoryFixture, rows, count string) string {
	identity := ""
	if fixture.name == "events" {
		identity = `,"actionId":"act_saved"`
	} else if fixture.resourceID != "" {
		identity = `,"resourceId":"` + fixture.resourceID + `"`
	}
	return `{"` + fixture.field + `":` + rows + `,"count":` + count + identity + `}`
}

func TestActionsHistoryRejectsIncompleteResponses(t *testing.T) {
	for _, fixture := range actionHistoryFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			for _, framing := range []string{"fixed", "chunked"} {
				t.Run(framing, func(t *testing.T) {
					for _, tc := range []struct{ name, body string }{
						{"null-envelope", `null`},
						{"empty-object", `{}`},
						{"error-envelope", `{"error":"synthetic-private-history-detail"}`},
						{"missing-records", `{"count":0}`},
						{"missing-count", `{"` + fixture.field + `":[]}`},
						{"null-count", historyEnvelope(fixture, `[]`, `null`)},
						{"negative-count", historyEnvelope(fixture, `[]`, `-1`)},
						{"inconsistent-count", historyEnvelope(fixture, `[]`, `1`)},
						{"null-records-positive-count", historyEnvelope(fixture, `null`, `1`)},
						{"object-records", historyEnvelope(fixture, `{}`, `0`)},
						{"string-count", historyEnvelope(fixture, `[]`, `"0"`)},
						{"array-envelope", `[]`},
						{"empty-body", ``},
						{"truncated", `{"` + fixture.field + `":[`},
						{"trailing-json", historyEnvelope(fixture, `[]`, `0`) + ` null`},
					} {
						t.Run(tc.name, func(t *testing.T) {
							stdout, stderr, err := runActionHistoryFixture(t, fixture, tc.body, http.StatusOK, framing == "chunked")
							if err == nil || !strings.Contains(err.Error(), "response") {
								t.Errorf("incomplete history must return a response error, got %v", err)
							}
							if stdout != "" {
								t.Errorf("incomplete history emitted successful JSON: %s", stdout)
							}
							if strings.Contains(stderr, "synthetic-private-history-detail") || (err != nil && strings.Contains(err.Error(), "synthetic-private-history-detail")) {
								t.Error("invalid history diagnostic echoed unrelated response content")
							}
						})
					}
				})
			}
		})
	}
}

func TestActionsHistoryRejectsMismatchedResponseIdentity(t *testing.T) {
	for _, fixture := range actionHistoryFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			field := "resourceId"
			if fixture.name == "events" {
				field = "actionId"
			}
			for _, id := range []string{"", "synthetic-unrelated-history", "   "} {
				// A global audit response has no resource binding; an omitted/blank
				// resourceId there is not a missing identity.
				if fixture.name == "all-audit" && strings.TrimSpace(id) == "" {
					continue
				}
				t.Run("identity-"+id, func(t *testing.T) {
					body := `{"` + fixture.field + `":[],"count":0,"` + field + `":"` + id + `"}`
					stdout, stderr, err := runActionHistoryFixture(t, fixture, body, http.StatusOK, false)
					if err == nil || !strings.Contains(err.Error(), "response") {
						t.Errorf("unbound history must return a response error, got %v", err)
					}
					if stdout != "" {
						t.Errorf("unbound history emitted successful JSON: %s", stdout)
					}
					if strings.Contains(stderr, "synthetic-unrelated-history") || (err != nil && strings.Contains(err.Error(), "synthetic-unrelated-history")) {
						t.Error("mismatch diagnostic echoed unrelated response identity")
					}
				})
			}
		})
	}
}

func TestActionsHistoryPreservesCanonicalResponses(t *testing.T) {
	for _, fixture := range actionHistoryFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			rows := `[{"id":"act_saved","state":"planned","request":{"resourceId":"vm:42"},"futureField":true}]`
			if fixture.name == "events" {
				rows = `[{"actionId":"act_saved","state":"planned","message":"Saved plan","futureField":true}]`
			}
			for _, framing := range []string{"fixed", "chunked"} {
				t.Run(framing, func(t *testing.T) {
					for _, tc := range []struct{ name, rows, count string }{{"empty", `[]`, `0`}, {"legacy-empty", `null`, `0`}, {"populated", rows, `1`}} {
						t.Run(tc.name, func(t *testing.T) {
							body := historyEnvelope(fixture, tc.rows, tc.count)
							body = strings.TrimSuffix(body, "}") + `,"futureEnvelopeField":true}`
							stdout, stderr, err := runActionHistoryFixture(t, fixture, body, http.StatusOK, framing == "chunked")
							if err != nil || stderr != "" {
								t.Fatalf("canonical history rejected: %v, stderr=%q", err, stderr)
							}
							var want any
							if fixture.name == "events" {
								var events actionLifecycleEventsResponse
								if err := json.Unmarshal([]byte(body), &events); err != nil {
									t.Fatal(err)
								}
								if events.Events == nil {
									events.Events = []unified.ActionLifecycleEvent{}
								}
								want = events
							} else {
								var audits actionAuditListResponse
								if err := json.Unmarshal([]byte(body), &audits); err != nil {
									t.Fatal(err)
								}
								if audits.Audits == nil {
									audits.Audits = []unified.ActionAuditRecord{}
								}
								want = audits
							}
							encoded, err := json.MarshalIndent(want, "", "  ")
							if err != nil {
								t.Fatal(err)
							}
							if stdout != string(encoded)+"\n" {
								t.Fatalf("canonical history output changed:\n%s", stdout)
							}
						})
					}
				})
			}
		})
	}
}

func TestActionsHistoryPreservesHTTPFailures(t *testing.T) {
	for _, fixture := range actionHistoryFixtures() {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable} {
			t.Run(fixture.name+"-"+http.StatusText(status), func(t *testing.T) {
				stdout, _, err := runActionHistoryFixture(t, fixture, `{"error":"synthetic refusal"}`, status, false)
				if err == nil || !strings.Contains(err.Error(), http.StatusText(status)) {
					t.Fatalf("HTTP failure lost: %v", err)
				}
				if stdout != "" {
					t.Fatalf("HTTP failure wrote history JSON: %s", stdout)
				}
			})
		}
	}
}
