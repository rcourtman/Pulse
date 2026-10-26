package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

func TestHandleAgentVersionMatchesReleaseExecutableIdentity(t *testing.T) {
	original := updates.BuildVersion
	t.Cleanup(func() { updates.BuildVersion = original })
	for _, tc := range []struct {
		name, build, target, wire string
		sourceMarker              bool
	}{
		{name: "stable", build: "6.6.0", target: "6.6.0", wire: "v6.6.0"},
		{name: "prefixed stable", build: "v6.6.0", target: "6.6.0", wire: "v6.6.0"},
		{name: "beta", build: "6.7.0-beta.1", target: "6.7.0-beta.1", wire: "v6.7.0-beta.1"},
		{name: "candidate", build: "v6.6.0-rc.6", target: "6.6.0-rc.6", wire: "v6.6.0-rc.6"},
		{name: "diagnostic", build: "6.6.0+git.1", wire: "dev"},
		{name: "source build", build: "6.6.0", wire: "dev", sourceMarker: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			updates.BuildVersion = tc.build
			if tc.sourceMarker {
				if err := os.WriteFile("BUILD_FROM_SOURCE", []byte("1"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// Fleet/display callers keep their existing canonical representation.
			if got := currentAgentTargetVersion(); got != tc.target {
				t.Fatalf("display target = %q, want %q", got, tc.target)
			}
			rr := httptest.NewRecorder()
			(&Router{}).handleAgentVersion(rr, httptest.NewRequest(http.MethodGet, "/api/agent/version", nil))
			var payload AgentVersionResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if rr.Code != http.StatusOK || payload.Version != tc.wire {
				t.Fatalf("machine version = %q (HTTP %d), want %q", payload.Version, rr.Code, tc.wire)
			}
			if tc.target != "" && payload.Version != expectedAgentBinaryVersion(tc.target) {
				t.Fatal("helper request and download validator disagree on executable identity")
			}
			if got := rr.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
				t.Fatalf("cache protection = %q", got)
			}
		})
	}
}
