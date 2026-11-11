package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// With no monitor to read through, Patrol is told the task log is unread. An
// empty answer marked complete would read as "nobody stopped this guest".
func TestProxmoxGuestTaskReaderWithoutAMonitorIsUnread(t *testing.T) {
	since, until := time.Unix(1700000000, 0), time.Unix(1700000600, 0)
	for name, resolve := range map[string]func() *monitoring.Monitor{
		"no resolver": nil,
		"no monitor":  func() *monitoring.Monitor { return nil },
	} {
		read := newProxmoxGuestTaskReader(resolve)(context.Background(), "pmxlab", "pmxlab", 112, since, until)
		if read.Complete || read.Reason != monitoring.ProxmoxGuestTasksNoConnection || len(read.Tasks) != 0 {
			t.Fatalf("%s: read = %+v, want an unread log", name, read)
		}
	}
}

// The reason an admin reads before approving a start carries Pulse's own
// sentence when Pulse could not tell whether a person stopped the guest,
// whatever the model wrote. It is added once, however often the same proposal
// is planned or submitted.
func TestStartProposalReasonCarriesWhatPulseCouldNotTell(t *testing.T) {
	const note = "Pulse could not read the node's task log, so it cannot tell whether a person stopped this guest. Check that before you approve a start."
	agents := newFakeProxmoxStartSessionCommander(guestStartSession("agent-pmxlab", "pmxlab"))
	lab := newGuestStartLab(t, true, agents)
	var asked []aicontracts.ActionProposal
	broker := NewPatrolActionBrokerWithProposalNote("default", lab.handlers, func(_ context.Context, orgID string, proposal aicontracts.ActionProposal) string {
		asked = append(asked, proposal)
		if orgID != "default" || proposal.FindingID != "proxmox:guest:stopped:"+lab.stopped {
			return ""
		}
		return note
	}).(*patrolActionBroker)

	reasonOf := func(actionID string) string {
		t.Helper()
		record, found, err := lab.handlers.ActionLifecycle().Get("default", actionID)
		if err != nil || !found {
			t.Fatalf("action %q: found %v, err %v", actionID, found, err)
		}
		return record.Request.Reason
	}
	proposal := aicontracts.ActionProposal{
		ProposalID: "proposal-start", FindingID: "proxmox:guest:stopped:" + lab.stopped, InvestigationID: "investigation-1",
		ResourceID: lab.stopped, CapabilityName: "start", Reason: "gitea is stopped and start on boot is on",
	}
	planned, err := broker.Plan(context.Background(), proposal)
	if err != nil {
		t.Fatalf("Plan = %v", err)
	}
	want := "gitea is stopped and start on boot is on. " + note
	if got := reasonOf(planned.ActionID); got != want {
		t.Fatalf("planned reason = %q, want %q", got, want)
	}

	// The same proposal submitted afterwards is the same action with the same
	// reason, and a reason that already carries the sentence is left alone.
	for _, reason := range []string{proposal.Reason, want} {
		proposal.Reason = reason
		submitted, err := broker.Submit(context.Background(), proposal)
		if err != nil {
			t.Fatalf("Submit(%q) = %v", reason, err)
		}
		if submitted.ActionID != planned.ActionID {
			t.Fatalf("Submit made action %q, want the planned %q", submitted.ActionID, planned.ActionID)
		}
		if got := reasonOf(submitted.ActionID); got != want || strings.Count(got, note) != 1 {
			t.Fatalf("reason after Submit(%q) = %q", reason, got)
		}
	}
	if len(asked) != 3 {
		t.Fatalf("the note provider was asked %d times, want once per plan or submit", len(asked))
	}

	// A proposal that answers another finding is planned as its author wrote it.
	other, err := broker.Plan(context.Background(), aicontracts.ActionProposal{
		ProposalID: "proposal-other", FindingID: "finding-2", InvestigationID: "investigation-2",
		ResourceID: lab.stopped, CapabilityName: "start", Reason: "gitea died",
	})
	if err != nil {
		t.Fatalf("Plan(other) = %v", err)
	}
	if got := reasonOf(other.ActionID); got != "gitea died" {
		t.Fatalf("reason = %q, want it untouched", got)
	}
}
