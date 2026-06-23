package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// TestPickFocus_PrefersCriticalThenWarningThenInfo pins the focus
// rule the probe uses for "where do I look first?". The unit test
// is here rather than against the substrate because the rule is a
// probe-side triage convention, not a contract Pulse owns; agents
// building on the substrate can implement their own ordering.
func TestPickFocus_PrefersCriticalThenWarningThenInfo(t *testing.T) {
	mk := func(id string, c, w, i, p int) fleetResource {
		r := fleetResource{CanonicalID: id, PendingApprovalCount: p}
		r.Findings.Critical = c
		r.Findings.Warning = w
		r.Findings.Info = i
		r.Findings.Total = c + w + i
		return r
	}

	cases := []struct {
		name      string
		resources []fleetResource
		want      string
	}{
		{
			name: "single critical beats many warnings",
			resources: []fleetResource{
				mk("vm:noisy", 0, 50, 0, 0),
				mk("vm:critical", 1, 0, 0, 0),
				mk("vm:quiet", 0, 0, 0, 0),
			},
			want: "vm:critical",
		},
		{
			name: "tie on severity broken by count",
			resources: []fleetResource{
				mk("vm:warm", 0, 1, 0, 0),
				mk("vm:warmer", 0, 3, 0, 0),
			},
			want: "vm:warmer",
		},
		{
			name: "no findings — pending approvals as tiebreaker",
			resources: []fleetResource{
				mk("vm:idle", 0, 0, 0, 0),
				mk("vm:waiting", 0, 0, 0, 2),
			},
			want: "vm:waiting",
		},
		{
			name: "no findings or approvals — first wins so depth step still runs",
			resources: []fleetResource{
				mk("vm:first", 0, 0, 0, 0),
				mk("vm:second", 0, 0, 0, 0),
			},
			want: "vm:first",
		},
		{
			name:      "empty fleet returns nil",
			resources: nil,
			want:      "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickFocus(tc.resources)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("expected nil for empty fleet; got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q; got nil", tc.want)
			}
			if got.CanonicalID != tc.want {
				t.Errorf("focus = %q; want %q", got.CanonicalID, tc.want)
			}
		})
	}
}

func TestFetchFleetUsesSharedCapabilityBodyExecutor(t *testing.T) {
	var got struct {
		Method string
		Path   string
		Token  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Token = r.Header.Get(agentcapabilities.AgentAPITokenHeader)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"generatedAt": "2026-06-18T07:00:00Z",
			"resources": [
				{
					"canonicalId": "vm:101",
					"resourceType": "vm",
					"resourceName": "web",
					"findings": {"total": 1, "critical": 1, "warning": 0, "info": 0},
					"pendingApprovalCount": 2
				}
			]
		}`))
	}))
	defer server.Close()

	fleet, err := fetchFleet(context.Background(), server.Client(), server.URL, []agentcapabilities.Capability{{
		Name:   agentcapabilities.FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   "/api/agent/fleet-context",
	}}, "probe-token")
	if err != nil {
		t.Fatalf("fetchFleet: %v", err)
	}
	if len(fleet.Resources) != 1 || fleet.Resources[0].CanonicalID != "vm:101" || fleet.Resources[0].Findings.Critical != 1 {
		t.Fatalf("fleet = %+v", fleet)
	}
	if got.Method != http.MethodGet || got.Path != "/api/agent/fleet-context" {
		t.Fatalf("request method/path = %s %s", got.Method, got.Path)
	}
	if got.Token != "probe-token" {
		t.Fatalf("token = %q", got.Token)
	}
}

func TestReadOneSSEEventUsesSharedActionableFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != agentcapabilities.AgentSSEAccept {
			t.Errorf("Accept header = %q, want %q", r.Header.Get("Accept"), agentcapabilities.AgentSSEAccept)
		}
		if r.Header.Get(agentcapabilities.AgentAPITokenHeader) != "probe-token" {
			t.Errorf("%s header = %q", agentcapabilities.AgentAPITokenHeader, r.Header.Get(agentcapabilities.AgentAPITokenHeader))
		}
		w.Header().Set("Content-Type", agentcapabilities.AgentSSEAccept)
		_, _ = w.Write([]byte(strings.Join([]string{
			"event: " + string(agentcapabilities.EventKindStreamConnected),
			"data: {}",
			"",
			"event: " + string(agentcapabilities.EventKindHeartbeat),
			"",
			"event: " + string(agentcapabilities.EventKindFindingCreated),
			"data: {\"findingId\":\"f1\"}",
			"",
		}, "\n")))
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	err = readOneSSEEvent(context.Background(), server.URL, agentcapabilities.AgentEventsPath, "probe-token")
	_ = w.Close()
	os.Stdout = oldStdout
	output, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if err != nil {
		t.Fatalf("readOneSSEEvent: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "event: "+string(agentcapabilities.EventKindFindingCreated)) {
		t.Fatalf("stdout missing finding event: %s", text)
	}
	if strings.Contains(text, string(agentcapabilities.EventKindStreamConnected)) || strings.Contains(text, string(agentcapabilities.EventKindHeartbeat)) {
		t.Fatalf("stdout included transport event: %s", text)
	}
}
