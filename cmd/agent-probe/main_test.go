package main

import (
	"testing"
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
