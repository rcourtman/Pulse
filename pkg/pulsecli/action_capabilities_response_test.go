package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func runActionCapabilitiesFixture(t *testing.T, body string, status int, chunked bool) (string, string, error) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/resources/vm:42/facets" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer synthetic-capability-token" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected capabilities request: method=%s path=%s query=%s bearer=%t accept=%s", r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization") == "Bearer synthetic-capability-token", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if chunked {
			w.(http.Flusher).Flush()
		}
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	cmd := newTestActionsRootCommand(map[string]string{"PULSE_API_TOKEN": "synthetic-capability-token", "PULSE_API_URL": server.URL + "/api"})
	cmd.SetArgs([]string{"actions", "capabilities", "--resource-id", " vm:42 "})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := cmd.ExecuteContext(ctx)
	if got := requests.Load(); got != 1 {
		t.Fatalf("capabilities read made %d requests, want one without retry", got)
	}
	return stdout.String(), stderr.String(), err
}

func TestActionsCapabilitiesRejectsUnboundResponses(t *testing.T) {
	for _, framing := range []string{"fixed", "chunked"} {
		t.Run(framing, func(t *testing.T) {
			for _, tc := range []struct{ name, body string }{
				{"null-envelope", `null`},
				{"empty-object", `{}`},
				{"error-envelope", `{"error":"synthetic-private-capability-detail"}`},
				{"missing-id-empty", `{"capabilities":[]}`},
				{"missing-id-populated", `{"capabilities":[{"name":"restart"}]}`},
				{"null-id", `{"resourceId":null,"capabilities":[]}`},
				{"blank-id", `{"resourceId":"   ","capabilities":[]}`},
				{"wrong-id-empty", `{"resourceId":"synthetic-unrelated-resource","capabilities":[]}`},
				{"wrong-id-populated", `{"resourceId":"synthetic-unrelated-resource","capabilities":[{"name":"restart"}]}`},
				{"numeric-id", `{"resourceId":42,"capabilities":[]}`},
				{"array-envelope", `[]`},
				{"empty-body", ``},
				{"truncated", `{"resourceId":"vm:42","capabilities":[`},
				{"trailing-json", `{"resourceId":"vm:42","capabilities":[]} null`},
				{"object-capabilities", `{"resourceId":"vm:42","capabilities":{}}`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					stdout, stderr, err := runActionCapabilitiesFixture(t, tc.body, http.StatusOK, framing == "chunked")
					if err == nil || !strings.Contains(err.Error(), "response") {
						t.Errorf("unbound or malformed capabilities must return a response error, got %v", err)
					}
					if stdout != "" {
						t.Errorf("unbound or malformed capabilities emitted successful JSON: %s", stdout)
					}
					combined := stderr + fmt.Sprint(err)
					for _, private := range []string{"synthetic-private-capability-detail", "synthetic-unrelated-resource", "synthetic-capability-token"} {
						if strings.Contains(combined, private) {
							t.Error("invalid capability diagnostic echoed unrelated response content or the bearer")
						}
					}
				})
			}
		})
	}
}

func TestActionsCapabilitiesPreservesBoundResponses(t *testing.T) {
	for _, framing := range []string{"fixed", "chunked"} {
		t.Run(framing, func(t *testing.T) {
			for _, tc := range []struct {
				name, body string
				count      int
			}{
				// The actual facets handler uses capabilities,omitempty. A bound
				// resource without that optional field is an explicit empty result.
				{"omitted-capabilities", `{"resourceId":"vm:42","recentChanges":[],"counts":{"recentChanges":0}}`, 0},
				{"null-capabilities", `{"resourceId":"vm:42","capabilities":null}`, 0},
				{"empty-capabilities", `{"resourceId":"vm:42","capabilities":[]}`, 0},
				{"trimmed-id", `{"resourceId":" vm:42 ","capabilities":[]}`, 0},
				{"populated-capabilities", `{"resourceId":"vm:42","capabilities":[{"name":"restart","description":"Restart VM","internalHandler":"synthetic-private-handler","futureField":true,"params":[{"name":"mode","type":"string","enum":["graceful","force"],"defaultValue":"graceful"}]}],"futureField":true}`, 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					stdout, _, err := runActionCapabilitiesFixture(t, tc.body, http.StatusOK, framing == "chunked")
					if err != nil {
						t.Fatalf("bound capabilities failed: %v", err)
					}
					var output actionCapabilitiesResponse
					if err := json.Unmarshal([]byte(stdout), &output); err != nil {
						t.Fatalf("invalid JSON output: %v", err)
					}
					if output.ResourceID != "vm:42" || output.Count != tc.count || len(output.Capabilities) != tc.count || output.Capabilities == nil {
						t.Fatalf("changed bound capabilities output: %+v", output)
					}
					if strings.Contains(stdout, "synthetic-private-handler") || strings.Contains(stdout, "futureField") {
						t.Fatal("capabilities output no longer retains its existing typed projection")
					}
					if tc.count == 1 && (output.Capabilities[0].Name != "restart" || output.Capabilities[0].Params[0].Name != "mode") {
						t.Fatal("lost capability name or parameter details")
					}
				})
			}
		})
	}
}

func TestActionsCapabilitiesPreservesHTTPRefusals(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			stdout, _, err := runActionCapabilitiesFixture(t, `{"resourceId":"vm:42","capabilities":[]}`, status, false)
			if err == nil || !strings.Contains(err.Error(), http.StatusText(status)) {
				t.Errorf("HTTP refusal must remain an error, got %v", err)
			}
			if stdout != "" {
				t.Error("HTTP refusal emitted a successful capabilities result")
			}
		})
	}
}
