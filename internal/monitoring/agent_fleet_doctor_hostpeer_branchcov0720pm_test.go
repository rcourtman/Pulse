package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// This file adds branch-coverage tests for findLikelyHostPeer in
// agent_fleet_doctor.go. The function iterates the supplied hosts, SKIPS any
// host whose ID == subject.id (skip-self takes precedence over any identity
// match), and returns the first host for which sameAgentIdentity is true;
// otherwise it returns the zero models.Host and false.
//
// Because findLikelyHostPeer forwards host.ID as both the `id` and `agentID`
// arguments to sameAgentIdentity, an "agent id" match happens when
// subject.agentID == host.ID. Each match case below isolates exactly one
// identity signal so the branch of sameAgentIdentity being exercised is
// unambiguous. Tests are selected by `-run 'HostPeer|FindLikelyHostPeer|Branchcov0720pm'`.

func TestFindLikelyHostPeerBranchcov0720pm(t *testing.T) {
	t.Parallel()

	// A host that is unrelated to the subject on every identity axis; useful as
	// a non-matching filler when building multi-host slices.
	unrelated := func(id string) models.Host {
		return models.Host{ID: id, Hostname: "totally-unrelated", TokenID: "tok-unrelated"}
	}

	cases := []struct {
		name             string
		subject          agentFleetSubject
		hosts            []models.Host
		wantOK           bool
		wantHostID       string
		wantHostHostname string
	}{
		// Branch: empty/nil hosts slice -> loop body never runs -> (zero, false).
		{
			name:    "empty hosts slice returns zero and false",
			subject: agentFleetSubject{id: "self", agentID: "a1", tokenID: "t1", hostname: "h"},
			hosts:   nil,
			wantOK:  false,
		},

		// Branch: skip-self (host.ID == subject.id) taken, then loop ends ->
		// (zero, false). The subject carries identity signals but the only host
		// is self, so it is skipped before sameAgentIdentity is ever consulted.
		{
			name:    "only host is self id skipped returns zero and false",
			subject: agentFleetSubject{id: "self", agentID: "self", tokenID: "t1", hostname: "h"},
			hosts:   []models.Host{{ID: "self", Hostname: "h", TokenID: "t1"}},
			wantOK:  false,
		},

		// Branch: match via the agentID signal. subject.agentID == host.ID
		// (both id and agentID args are host.ID). subject.tokenID/hostname are
		// empty so the match is unambiguously the agentID branch.
		{
			name:             "match by agent id subject agentID equals host ID",
			subject:          agentFleetSubject{id: "self", agentID: "agent-x"},
			hosts:            []models.Host{{ID: "agent-x", Hostname: "unrelated", TokenID: ""}},
			wantOK:           true,
			wantHostID:       "agent-x",
			wantHostHostname: "unrelated",
		},

		// Branch: match via the tokenID signal. subject.agentID is empty (so the
		// agentID branch is skipped) and subject.hostname is empty (so the
		// hostname branch is skipped); only the token branch can fire.
		{
			name:             "match by token id",
			subject:          agentFleetSubject{id: "self", tokenID: "tok-1"},
			hosts:            []models.Host{{ID: "host-a", Hostname: "unrelated", TokenID: "tok-1"}},
			wantOK:           true,
			wantHostID:       "host-a",
			wantHostHostname: "unrelated",
		},

		// Branch: match via the hostname signal, case-insensitively
		// (subject.hostname "Node1" vs host.Hostname "node1"). subject.agentID
		// and subject.tokenID are empty and host.TokenID is empty, so only the
		// hostname branch of sameAgentIdentity can match.
		{
			name:             "match by hostname case insensitive",
			subject:          agentFleetSubject{id: "self", hostname: "Node1"},
			hosts:            []models.Host{{ID: "host-b", Hostname: "node1", TokenID: ""}},
			wantOK:           true,
			wantHostID:       "host-b",
			wantHostHostname: "node1",
		},

		// Branch: skip-self takes precedence over match. The single host has
		// ID == subject.id AND would match on ALL THREE identity signals
		// (agentID, tokenID, hostname) — yet it is still skipped, yielding
		// (zero, false). This pins the ordering: the self check runs before
		// sameAgentIdentity is called.
		{
			name:    "self host that would match is still skipped",
			subject: agentFleetSubject{id: "self-id", agentID: "self-id", tokenID: "tok-1", hostname: "node1"},
			hosts:   []models.Host{{ID: "self-id", Hostname: "node1", TokenID: "tok-1"}},
			wantOK:  false,
		},

		// Branch: first non-self match wins. host[0] neither matches nor is
		// self (exercises a sameAgentIdentity==false iteration); host[1] is the
		// first match and must be returned; host[2] would also match but must
		// never be reached.
		{
			name:    "first non-self match wins among multiple hosts",
			subject: agentFleetSubject{id: "self", hostname: "match-host"},
			hosts: []models.Host{
				unrelated("nope-1"),
				{ID: "winner", Hostname: "match-host", TokenID: ""},
				{ID: "also-match", Hostname: "match-host", TokenID: ""},
			},
			wantOK:           true,
			wantHostID:       "winner",
			wantHostHostname: "match-host",
		},

		// Branch: no host matches. The single candidate is not self but
		// sameAgentIdentity returns false on every signal -> loop completes ->
		// (zero, false).
		{
			name:    "no host matches returns zero and false",
			subject: agentFleetSubject{id: "self", agentID: "a1", tokenID: "t1", hostname: "real"},
			hosts:   []models.Host{{ID: "other", Hostname: "different", TokenID: "t2"}},
			wantOK:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := findLikelyHostPeer(tc.subject, tc.hosts)

			if ok != tc.wantOK {
				t.Fatalf("findLikelyHostPeer ok = %v, want %v (subject=%+v, hosts=%#v)",
					ok, tc.wantOK, tc.subject, tc.hosts)
			}

			if tc.wantOK {
				// Assert the EXACT host was returned so ordering/identity is
				// verified, not just "some host".
				if got.ID != tc.wantHostID {
					t.Fatalf("returned host ID = %q, want %q", got.ID, tc.wantHostID)
				}
				if got.Hostname != tc.wantHostHostname {
					t.Fatalf("returned host Hostname = %q, want %q", got.Hostname, tc.wantHostHostname)
				}
				return
			}

			// Non-match path must return the zero-value models.Host exactly.
			if !reflect.DeepEqual(got, models.Host{}) {
				t.Fatalf("findLikelyHostPeer returned non-zero host on no-match: %#v", got)
			}
		})
	}
}
