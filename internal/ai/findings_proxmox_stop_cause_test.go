package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// taskLog is a node's task log as the reader returns it, with what it was
// asked.
type taskLog struct {
	read  ProxmoxGuestTaskRead
	calls []taskLogCall
}

type taskLogCall struct {
	instance, node string
	vmid           int
	since, until   time.Time
}

func (l *taskLog) reader() ProxmoxGuestTaskReader {
	return func(_ context.Context, instance, node string, vmid int, since, until time.Time) ProxmoxGuestTaskRead {
		l.calls = append(l.calls, taskLogCall{instance, node, vmid, since, until})
		return l.read
	}
}

func wholeLog(tasks ...ProxmoxGuestTask) *taskLog {
	return &taskLog{read: ProxmoxGuestTaskRead{Tasks: tasks, Complete: true}}
}

func unreadLog(reason string, tasks ...ProxmoxGuestTask) *taskLog {
	return &taskLog{read: ProxmoxGuestTaskRead{Tasks: tasks, Reason: reason}}
}

func guestKinds() []struct {
	name         string
	id           string
	resourceType unifiedresources.ResourceType
	vmid         int
	stop         string
	shutdown     string
	reboot       string
} {
	return []struct {
		name         string
		id           string
		resourceType unifiedresources.ResourceType
		vmid         int
		stop         string
		shutdown     string
		reboot       string
	}{
		{"LXC", "system-container:101", unifiedresources.ResourceTypeSystemContainer, 101, "vzstop", "vzshutdown", "vzreboot"},
		{"VM", "vm:160", unifiedresources.ResourceTypeVM, 160, "qmstop", "qmshutdown", "qmreboot"},
	}
}

// stopGuest shows a watcher one guest running and then stopped ten seconds
// later, and returns what the second look raised.
func stopGuest(watcher *proxmoxGuestLifecycleWatcher, id string, resourceType unifiedresources.ResourceType, runningAt time.Time) []*Finding {
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource(id, resourceType, unifiedresources.StatusOnline, runningAt),
	)}, nil, runningAt)
	emit, _ := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource(id, resourceType, unifiedresources.StatusOffline, runningAt.Add(10*time.Second)),
	)}, nil, runningAt.Add(11*time.Second))
	return emit
}

// A guest a person or a job stopped through Proxmox gets no finding, so no
// investigation and no request to approve starting it again.
func TestGuestStoppedThroughProxmoxRaisesNothing(t *testing.T) {
	for _, guest := range guestKinds() {
		for _, task := range []string{guest.stop, guest.shutdown, "hastop"} {
			t.Run(guest.name+" "+task, func(t *testing.T) {
				now := time.Now().UTC().Truncate(time.Second)
				log := wholeLog(ProxmoxGuestTask{Type: task, User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(6 * time.Second)})
				watcher := newProxmoxGuestLifecycleWatcher()
				watcher.readTasks = log.reader()

				if emit := stopGuest(watcher, guest.id, guest.resourceType, now); len(emit) != 0 {
					t.Fatalf("a %s by root@pam raised %#v", task, emit)
				}
				if len(log.calls) != 1 {
					t.Fatalf("the task log was read %d times, want once", len(log.calls))
				}
				call := log.calls[0]
				if call.instance != "homelab" || call.node != "delly" || call.vmid != guest.vmid {
					t.Fatalf("asked about %s/%s/%d", call.instance, call.node, call.vmid)
				}
				// The window opens well before the last running look, because
				// Proxmox filters on when a task started and a shutdown can run
				// for minutes, and closes just after the stopped look.
				if !call.since.Equal(now.Add(-proxmoxGuestStopTaskLookback)) || !call.until.Equal(now.Add(10*time.Second).Add(proxmoxGuestStopClockSlack)) {
					t.Fatalf("window %s..%s", call.since, call.until)
				}
				record, ok := watcher.stopRecord(guest.id)
				if !ok || record.cause.kind != proxmoxGuestStopRequested || record.cause.user != "root@pam" || record.cause.task != task {
					t.Fatalf("stop record = %+v, %v", record, ok)
				}

				// It stays quiet while the guest stays stopped, and nothing
				// is asked twice.
				emit, _ := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
					proxmoxLifecycleTestResource(guest.id, guest.resourceType, unifiedresources.StatusOffline, now.Add(20*time.Second)),
				)}, nil, now.Add(21*time.Second))
				if len(emit) != 0 || len(log.calls) != 1 {
					t.Fatalf("a later look raised %#v after %d reads", emit, len(log.calls))
				}

				// Started again, the stop is forgotten.
				watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
					proxmoxLifecycleTestResource(guest.id, guest.resourceType, unifiedresources.StatusOnline, now.Add(30*time.Second)),
				)}, nil, now.Add(31*time.Second))
				if _, held := watcher.stopRecord(guest.id); held {
					t.Fatal("a running guest still has a stop on record")
				}
			})
		}
	}
}

// A guest that went down with no stop task in a log Pulse read in full gets a
// finding that says so, so the model and the reader see why a start is on the
// table.
func TestGuestThatStoppedByItselfSaysSoInItsFinding(t *testing.T) {
	for _, guest := range guestKinds() {
		t.Run(guest.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			// Tasks that do not ask a guest to stop are not a reason.
			log := wholeLog(
				ProxmoxGuestTask{Type: "vzdump", User: "root@pam", Status: "OK", StartedAt: now.Add(-time.Minute), EndedAt: now.Add(5 * time.Second)},
				ProxmoxGuestTask{Type: "vncproxy", User: "ops@pve", Status: "OK", StartedAt: now.Add(2 * time.Second), EndedAt: now.Add(3 * time.Second)},
			)
			watcher := newProxmoxGuestLifecycleWatcher()
			watcher.readTasks = log.reader()

			emit := stopGuest(watcher, guest.id, guest.resourceType, now)
			if len(emit) != 1 {
				t.Fatalf("emit = %#v, want the stopped finding", emit)
			}
			finding := emit[0]
			if proxmoxGuestStopCauseToken(finding) != "no_stop_task" {
				t.Fatalf("evidence = %q", finding.Evidence)
			}
			for _, want := range []string{"No stop or shutdown task for this guest in the node's task log", "it was not stopped through Proxmox"} {
				if !strings.Contains(finding.Evidence, want) {
					t.Errorf("evidence %q lacks %q", finding.Evidence, want)
				}
			}
			if !strings.Contains(finding.Description, "nothing in Proxmox asked it to") || !strings.Contains(finding.Description, "It stopped from inside the guest or it failed.") {
				t.Errorf("description = %q", finding.Description)
			}
			if strings.Contains(finding.Description+finding.Evidence+finding.Recommendation, "could not") {
				t.Errorf("a stop Pulse did work out reads as unknown: %q / %q", finding.Description, finding.Evidence)
			}

			// The open finding keeps its cause on every later look, and the
			// log is not read again.
			emit, _ = watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
				proxmoxLifecycleTestResource(guest.id, guest.resourceType, unifiedresources.StatusOffline, now.Add(20*time.Second)),
			)}, []*Finding{finding}, now.Add(21*time.Second))
			if len(emit) != 1 || proxmoxGuestStopCauseToken(emit[0]) != "no_stop_task" || len(log.calls) != 1 {
				t.Fatalf("later look: %#v after %d reads", emit, len(log.calls))
			}

			// After a restart Pulse no longer holds the cause. The finding's
			// own words stand: asking now would search the wrong time.
			restarted := newProxmoxGuestLifecycleWatcher()
			restarted.readTasks = log.reader()
			emit, _ = restarted.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
				proxmoxLifecycleTestResource(guest.id, guest.resourceType, unifiedresources.StatusOffline, now.Add(30*time.Second)),
			)}, []*Finding{finding}, now.Add(31*time.Second))
			if len(emit) != 1 || emit[0].Evidence != finding.Evidence || emit[0].Description != finding.Description || len(log.calls) != 1 {
				t.Fatalf("after a restart: %#v after %d reads", emit, len(log.calls))
			}
		})
	}
}

// When Pulse cannot read the task log in full, the finding stays and says
// Pulse cannot tell. An unread log is never taken to mean nobody stopped it.
func TestUnreadableTaskLogLeavesTheFindingAndSaysPulseCannotTell(t *testing.T) {
	for _, guest := range guestKinds() {
		for _, reason := range []string{"audit_permission_unconfirmed", "access_denied", "collection_failed", "history_truncated"} {
			t.Run(guest.name+" "+reason, func(t *testing.T) {
				now := time.Now().UTC().Truncate(time.Second)
				watcher := newProxmoxGuestLifecycleWatcher()
				watcher.readTasks = unreadLog(reason).reader()

				emit := stopGuest(watcher, guest.id, guest.resourceType, now)
				if len(emit) != 1 {
					t.Fatalf("emit = %#v, want the stopped finding kept", emit)
				}
				finding := emit[0]
				if proxmoxGuestStopCauseToken(finding) != "unknown" || !strings.Contains(finding.Evidence, "reason="+reason) {
					t.Fatalf("evidence = %q", finding.Evidence)
				}
				for field, text := range map[string]string{"description": finding.Description, "evidence": finding.Evidence} {
					if !strings.Contains(text, "cannot tell whether a person stopped this guest") {
						t.Errorf("%s does not say Pulse cannot tell: %q", field, text)
					}
				}
				if !strings.Contains(finding.Recommendation, "Check whether someone stopped this guest on purpose before you approve Start") {
					t.Errorf("recommendation = %q", finding.Recommendation)
				}
				for _, wrong := range []string{"nothing in Proxmox asked", "not stopped through Proxmox", "or it failed"} {
					if strings.Contains(finding.Description+" "+finding.Evidence, wrong) {
						t.Errorf("an unread log reads as a guest that went down by itself: %q", wrong)
					}
				}
				if len(finding.Evidence) > 384 {
					t.Errorf("evidence is %d characters", len(finding.Evidence))
				}
			})
		}
	}

	// No reader at all is unknown too, not an empty log.
	watcher := newProxmoxGuestLifecycleWatcher()
	emit := stopGuest(watcher, "vm:160", unifiedresources.ResourceTypeVM, time.Now().UTC())
	if len(emit) != 1 || proxmoxGuestStopCauseToken(emit[0]) != "unknown" {
		t.Fatalf("with no reader: %#v", emit)
	}
}

// A stop task found in a log Pulse could not read in full decides nothing. A
// start Pulse cannot see may have followed it, and then the stop is not why
// the guest is down. The finding stays and says Pulse cannot tell.
func TestStopTaskInAPartialReadDoesNotSilenceTheFinding(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	watcher := newProxmoxGuestLifecycleWatcher()
	watcher.readTasks = unreadLog("audit_permission_unconfirmed", ProxmoxGuestTask{Type: "vzstop", User: "pulse@pve", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}).reader()
	emit := stopGuest(watcher, "system-container:101", unifiedresources.ResourceTypeSystemContainer, now)
	if len(emit) != 1 || proxmoxGuestStopCauseToken(emit[0]) != "unknown" {
		t.Fatalf("a stop task in a partial read: %#v, want the finding kept as unknown", emit)
	}
}

// A stop Pulse itself ran after an approval stays suppressed, from Pulse's
// own action record and without reading the task log.
func TestStopPulseRanIsStillSuppressedWithoutReadingTheLog(t *testing.T) {
	now := time.Now().UTC()
	log := wholeLog()
	audit := proxmoxLifecycleGovernedStopAudit("vm:160", "shutdown", now.Add(3*time.Second), now.Add(8*time.Second))
	watcher := newProxmoxGuestLifecycleWatcher(func(string, time.Time, int) ([]unifiedresources.ActionAuditRecord, error) {
		return []unifiedresources.ActionAuditRecord{audit}, nil
	})
	watcher.readTasks = log.reader()
	if emit := stopGuest(watcher, "vm:160", unifiedresources.ResourceTypeVM, now); len(emit) != 0 {
		t.Fatalf("Pulse's own stop raised %#v", emit)
	}
	if len(log.calls) != 0 {
		t.Fatalf("the task log was read %d times for a stop Pulse ran", len(log.calls))
	}
	record, ok := watcher.stopRecord("vm:160")
	if !ok || record.cause.kind != proxmoxGuestStopRequested || !record.cause.governed {
		t.Fatalf("stop record = %+v, %v", record, ok)
	}
	if got := record.requestedSentence(); !strings.Contains(got, "stopped by Pulse") || !strings.Contains(got, "after a person approved it") {
		t.Fatalf("sentence = %q", got)
	}

	// With the action record unreadable, the task log still answers.
	failing := newProxmoxGuestLifecycleWatcher(func(string, time.Time, int) ([]unifiedresources.ActionAuditRecord, error) {
		return nil, errors.New("audit store unavailable")
	})
	failing.readTasks = wholeLog(ProxmoxGuestTask{Type: "qmshutdown", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(7 * time.Second)}).reader()
	if emit := stopGuest(failing, "vm:160", unifiedresources.ResourceTypeVM, now); len(emit) != 0 {
		t.Fatalf("a stop the task log explains raised %#v", emit)
	}
}

func TestClassifyProxmoxGuestStop(t *testing.T) {
	running := time.Date(2026, 11, 11, 10, 20, 0, 0, time.UTC)
	stopped := running.Add(time.Minute)
	task := func(kind string, start, end time.Duration, status string) ProxmoxGuestTask {
		entry := ProxmoxGuestTask{Type: kind, User: "ops@pve", Status: status, StartedAt: running.Add(start)}
		if end != 0 {
			entry.EndedAt = running.Add(end)
		}
		return entry
	}
	for _, tc := range []struct {
		name string
		read ProxmoxGuestTaskRead
		want proxmoxGuestStopCauseKind
		task string
	}{
		{name: "stop between the two looks", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstop", 20*time.Second, 22*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "vzstop"},
		{name: "long shutdown that began before the last running look", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("qmshutdown", -10*time.Minute, 30*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "qmshutdown"},
		{name: "shutdown still running", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzshutdown", 10*time.Second, 0, "")}}, want: proxmoxGuestStopRequested, task: "vzshutdown"},
		{name: "a stop that failed was still asked for", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("qmshutdown", 5*time.Second, 50*time.Second, "VM quit/powerdown failed - got timeout")}}, want: proxmoxGuestStopRequested, task: "qmshutdown"},
		{name: "HA stop", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("hastop", 5*time.Second, 6*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "hastop"},
		{name: "suspend to disk", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("qmsuspend", 5*time.Second, 40*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "qmsuspend"},
		{name: "rollback", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzrollback", 5*time.Second, 40*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "vzrollback"},
		{name: "the later of two stops is named", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzshutdown", 5*time.Second, 30*time.Second, "timeout"), task("vzstop", 35*time.Second, 36*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "vzstop"},
		// The guest was seen running after this stop finished, so it was
		// started again and something else took it down.
		{name: "stop that finished long before the last running look", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstop", -20*time.Minute, -19*time.Minute, "OK")}}, want: proxmoxGuestStopNoTask},
		{name: "stop that began after the stopped look", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstop", 10*time.Minute, 11*time.Minute, "OK")}}, want: proxmoxGuestStopNoTask},
		{name: "start, backup and console are not stops", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstart", 5*time.Second, 6*time.Second, "OK"), task("vzdump", -time.Minute, 30*time.Second, "OK"), task("vncproxy", 5*time.Second, 6*time.Second, "OK")}}, want: proxmoxGuestStopNoTask, task: "vzstart"},
		{name: "backup and console alone", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzdump", -time.Minute, 30*time.Second, "OK"), task("vncproxy", 5*time.Second, 6*time.Second, "OK")}}, want: proxmoxGuestStopNoTask},
		{name: "nothing in a whole read", read: ProxmoxGuestTaskRead{Complete: true}, want: proxmoxGuestStopNoTask},
		{name: "nothing in a partial read", read: ProxmoxGuestTaskRead{Reason: "audit_permission_unconfirmed"}, want: proxmoxGuestStopUnknown},
		{name: "a stop in a partial read", read: ProxmoxGuestTaskRead{Reason: "audit_permission_unconfirmed", Tasks: []ProxmoxGuestTask{task("vzstop", 20*time.Second, 22*time.Second, "OK")}}, want: proxmoxGuestStopUnknown},
		{name: "restart that did not come back", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzreboot", 5*time.Second, 40*time.Second, "OK")}}, want: proxmoxGuestStopRestart, task: "vzreboot"},
		{name: "restart in a partial read is not enough to rule out a stop", read: ProxmoxGuestTaskRead{Reason: "history_truncated", Tasks: []ProxmoxGuestTask{task("qmreboot", 5*time.Second, 40*time.Second, "OK")}}, want: proxmoxGuestStopUnknown},
		{name: "a stop after a restart is the last word", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("qmreboot", 5*time.Second, 20*time.Second, "OK"), task("qmstop", 25*time.Second, 26*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "qmstop"},
		// Stopped on purpose, started again, then down with nothing asking.
		// The earlier stop does not explain the guest being down now.
		{name: "a start that worked after the stop", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstop", 10*time.Second, 12*time.Second, "OK"), task("vzstart", 20*time.Second, 22*time.Second, "OK")}}, want: proxmoxGuestStopNoTask, task: "vzstart"},
		{name: "an HA start after the stop", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("hastop", 10*time.Second, 12*time.Second, "OK"), task("hastart", 20*time.Second, 22*time.Second, "OK")}}, want: proxmoxGuestStopNoTask, task: "hastart"},
		{name: "stopped again after being started", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstop", 10*time.Second, 12*time.Second, "OK"), task("vzstart", 20*time.Second, 22*time.Second, "OK"), task("vzshutdown", 30*time.Second, 40*time.Second, "OK")}}, want: proxmoxGuestStopRequested, task: "vzshutdown"},
		// Somebody stopped it and then wanted it back, and the start failed.
		{name: "a start that failed after the stop", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstop", 10*time.Second, 12*time.Second, "OK"), task("vzstart", 20*time.Second, 22*time.Second, "startup for container '101' failed")}}, want: proxmoxGuestStopRestart, task: "vzstart"},
		{name: "a stop and a start in the same second leave the finding", read: ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{task("vzstart", 10*time.Second, 12*time.Second, "OK"), task("vzstop", 10*time.Second, 11*time.Second, "OK")}}, want: proxmoxGuestStopNoTask, task: "vzstart"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := classifyProxmoxGuestStop(tc.read, running, stopped)
			if cause.kind != tc.want || cause.task != tc.task {
				t.Fatalf("cause = %+v, want kind %q task %q", cause, tc.want, tc.task)
			}
			if tc.want == proxmoxGuestStopUnknown && cause.unreadReason == "" {
				t.Fatal("an unknown cause has no reason")
			}
		})
	}
}

// Many guests found stopped in one look share one allowance of time for their
// task log reads, and a read that gets none is unknown.
func TestTaskLogReadsShareOneBudgetAndRunOutAsUnknown(t *testing.T) {
	watcher := newProxmoxGuestLifecycleWatcher()
	reads := 0
	watcher.readTasks = func(context.Context, string, string, int, time.Time, time.Time) ProxmoxGuestTaskRead {
		reads++
		return ProxmoxGuestTaskRead{Complete: true}
	}
	spent, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Now().UTC()
	before := proxmoxGuestLifecycleSnapshot{id: "vm:160", node: "delly", instance: "homelab", vmid: 160, observedAt: now}
	after := proxmoxGuestLifecycleSnapshot{id: "vm:160", node: "delly", instance: "homelab", vmid: 160, observedAt: now.Add(10 * time.Second)}
	cause := watcher.readStopCause(spent, before, after)
	if cause.kind != proxmoxGuestStopUnknown || cause.unreadReason != "collection_failed" || reads != 0 {
		t.Fatalf("with no time left: %+v after %d reads", cause, reads)
	}
	// A guest that moved nodes is asked about on both.
	var nodes []string
	watcher.readTasks = func(_ context.Context, _ string, node string, _ int, _, _ time.Time) ProxmoxGuestTaskRead {
		nodes = append(nodes, node)
		return ProxmoxGuestTaskRead{Complete: node == "delly", Reason: "collection_failed"}
	}
	before.node = "minipc"
	moved := watcher.readStopCause(context.Background(), before, after)
	if strings.Join(nodes, ",") != "delly,minipc" || moved.kind != proxmoxGuestStopUnknown {
		t.Fatalf("moved guest: asked %v, cause %+v, want both nodes and unknown when one is unread", nodes, moved)
	}
}

// A stop on record does not outlive the stop. A guest that is running again
// loses it even when the check does not otherwise act on the guest, and a
// read that finishes after the guest was seen running again records nothing.
func TestStopOnRecordDoesNotOutliveTheStop(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	watcher := newProxmoxGuestLifecycleWatcher()
	watcher.readTasks = wholeLog(ProxmoxGuestTask{Type: "qmstop", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}).reader()
	stopGuest(watcher, "vm:160", unifiedresources.ResourceTypeVM, now)
	if _, held := watcher.stopRecord("vm:160"); !held {
		t.Fatal("the stop was not recorded")
	}

	// Running again, with a backup lock on it.
	locked := proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now.Add(30*time.Second))
	locked.Proxmox.Lock = "backup"
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(locked)}, nil, now.Add(31*time.Second))
	if _, held := watcher.stopRecord("vm:160"); held {
		t.Fatal("a running guest with a lock kept its stop on record")
	}

	// A read that comes back after a newer look saw the guest running.
	stopped := proxmoxGuestLifecycleSnapshot{id: "vm:160", status: unifiedresources.StatusOffline, observedAt: now.Add(10 * time.Second)}
	watcher.mu.Lock()
	watcher.prior["vm:160"] = proxmoxGuestLifecycleSnapshot{id: "vm:160", status: unifiedresources.StatusOnline, observedAt: now.Add(40 * time.Second)}
	watcher.mu.Unlock()
	if watcher.rememberStop(stopped, 1, proxmoxGuestStopCause{kind: proxmoxGuestStopRequested}) {
		t.Fatal("a late read was taken for a guest that is running")
	}
	if _, held := watcher.stopRecord("vm:160"); held {
		t.Fatal("a late read recorded a stop for a guest that is running")
	}

	// Stopped, started and stopped again while the first stop's read was out.
	// The late answer belongs to a stop that is over.
	again := newProxmoxGuestLifecycleWatcher()
	again.readTasks = wholeLog().reader()
	stopGuest(again, "vm:160", unifiedresources.ResourceTypeVM, now)
	again.mu.Lock()
	first := again.stopEpisodes["vm:160"].seq
	again.mu.Unlock()
	stopGuest(again, "vm:160", unifiedresources.ResourceTypeVM, now.Add(time.Minute))
	if again.rememberStop(stopped, first, proxmoxGuestStopCause{kind: proxmoxGuestStopRequested, task: "qmstop"}) {
		t.Fatal("the first stop's late read was taken for the second stop")
	}
	if record, _ := again.stopRecord("vm:160"); record.cause.kind != proxmoxGuestStopNoTask {
		t.Fatalf("the first stop's late read replaced the second stop's cause: %+v", record.cause)
	}

	// A running observation from before the stop, held by a look that
	// overlapped a newer one, does not undo the stop.
	older := newProxmoxGuestLifecycleWatcher()
	older.readTasks = wholeLog(ProxmoxGuestTask{Type: "qmstop", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}).reader()
	stopGuest(older, "vm:160", unifiedresources.ResourceTypeVM, now)
	older.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now.Add(5*time.Second)),
	)}, nil, now.Add(12*time.Second))
	if _, ok := older.stopRecord("vm:160"); !ok {
		t.Fatal("a running observation older than the stop cleared it")
	}

	// A Proxmox source that has gone stale proves nothing, whatever another
	// source says about the guest answering.
	held := newProxmoxGuestLifecycleWatcher()
	held.readTasks = wholeLog(ProxmoxGuestTask{Type: "qmstop", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}).reader()
	stopGuest(held, "vm:160", unifiedresources.ResourceTypeVM, now)
	stale := proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now.Add(30*time.Second))
	stale.SourceStatus[unifiedresources.SourceProxmox] = unifiedresources.SourceStatus{Status: "stale", LastSeen: now.Add(10 * time.Second)}
	held.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(stale)}, nil, now.Add(31*time.Second))
	if _, ok := held.stopRecord("vm:160"); !ok {
		t.Fatal("a stale Proxmox source cleared a deliberate stop")
	}
}

// A guest stopped on purpose, started again and then down with nothing asking
// gets a finding that does not claim the log held no stop at all.
func TestGuestDownAfterALaterStartNamesTheStart(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	watcher := newProxmoxGuestLifecycleWatcher()
	watcher.readTasks = wholeLog(
		ProxmoxGuestTask{Type: "vzstop", User: "root@pam", Status: "OK", StartedAt: now.Add(2 * time.Second), EndedAt: now.Add(3 * time.Second)},
		ProxmoxGuestTask{Type: "vzstart", User: "root@pam", Status: "OK", StartedAt: now.Add(5 * time.Second), EndedAt: now.Add(6 * time.Second)},
	).reader()
	emit := stopGuest(watcher, "system-container:101", unifiedresources.ResourceTypeSystemContainer, now)
	if len(emit) != 1 || proxmoxGuestStopCauseToken(emit[0]) != "no_stop_task" {
		t.Fatalf("emit = %#v", emit)
	}
	if !strings.Contains(emit[0].Description, "It was last started through Proxmox (vzstart at ") || !strings.Contains(emit[0].Evidence, "No stop or shutdown task for this guest after its last start") {
		t.Fatalf("description %q, evidence %q", emit[0].Description, emit[0].Evidence)
	}
	if strings.Contains(emit[0].Description+emit[0].Evidence, "no stop or shutdown task for it between") {
		t.Fatalf("the finding says the log held no stop: %q", emit[0].Description)
	}
}

// A restart that left the guest down is not a deliberate stop. The finding
// stays and names the restart.
func TestRestartThatDidNotComeBackKeepsTheFinding(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	watcher := newProxmoxGuestLifecycleWatcher()
	watcher.readTasks = wholeLog(ProxmoxGuestTask{Type: "vzreboot", User: "ops@pve", Status: "OK", StartedAt: now.Add(3 * time.Second), EndedAt: now.Add(8 * time.Second)}).reader()
	emit := stopGuest(watcher, "system-container:101", unifiedresources.ResourceTypeSystemContainer, now)
	if len(emit) != 1 || proxmoxGuestStopCauseToken(emit[0]) != "restart_task" {
		t.Fatalf("emit = %#v", emit)
	}
	if !strings.Contains(emit[0].Description, "a restart was asked for through Proxmox (vzreboot at ") || !strings.Contains(emit[0].Description, "It did not come back.") {
		t.Fatalf("description = %q", emit[0].Description)
	}
}

// stoppedGuestPatrol runs Patrol's own stopped-guest check over one guest
// that goes from running to stopped, with the given task log.
func stoppedGuestPatrol(t *testing.T, id string, resourceType unifiedresources.ResourceType, log *taskLog) *PatrolService {
	t.Helper()
	now := time.Now().UTC()
	patrol := NewPatrolService(nil, nil)
	patrol.SetProxmoxGuestTaskReader(log.reader())
	patrol.SetReadState(proxmoxLifecycleReadState(proxmoxLifecycleTestResource(id, resourceType, unifiedresources.StatusOnline, now)))
	patrol.runPatrol(context.Background())
	patrol.SetReadState(proxmoxLifecycleReadState(proxmoxLifecycleTestResource(id, resourceType, unifiedresources.StatusOffline, now.Add(10*time.Second))))
	patrol.runPatrol(context.Background())
	return patrol
}

// The sentence a start proposal carries comes from the finding it answers,
// and only when Pulse could not work out who stopped the guest.
func TestStartProposalNoteFollowsTheFindingsCause(t *testing.T) {
	now := time.Now().UTC()
	unknown := stoppedGuestPatrol(t, "vm:160", unifiedresources.ResourceTypeVM, unreadLog("audit_permission_unconfirmed"))
	finding := unknown.GetFindings().Get("proxmox:guest:stopped:vm:160")
	if finding == nil || proxmoxGuestStopCauseToken(finding) != "unknown" {
		t.Fatalf("finding = %#v", finding)
	}
	note := unknown.ProxmoxGuestStartProposalNote(" proxmox:guest:stopped:vm:160 ")
	if note != proxmoxGuestStopUnknownNote || !strings.Contains(note, "cannot tell whether a person stopped this guest") {
		t.Fatalf("note = %q", note)
	}

	died := stoppedGuestPatrol(t, "vm:160", unifiedresources.ResourceTypeVM, wholeLog())
	if note := died.ProxmoxGuestStartProposalNote("proxmox:guest:stopped:vm:160"); note != "" {
		t.Fatalf("a guest that went down by itself carries %q", note)
	}
	if note := died.ProxmoxGuestStartProposalNote("no-such-finding"); note != "" {
		t.Fatalf("a missing finding carries %q", note)
	}

	// A person's stop files nothing, so there is nothing to carry a note.
	quiet := stoppedGuestPatrol(t, "vm:160", unifiedresources.ResourceTypeVM, wholeLog(ProxmoxGuestTask{Type: "qmstop", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}))
	if finding := quiet.GetFindings().Get("proxmox:guest:stopped:vm:160"); finding != nil {
		t.Fatalf("a person's stop filed %#v", finding)
	}
}

// The model's own path. While a guest Pulse saw stop is still stopped, a
// reliability finding on it is about that stop. A deliberate stop gets none,
// and a stop Pulse already filed does not get a second one, which would ask
// the operator to approve the same start twice.
func TestModelCannotFileASecondFindingForAStopPulseAccountedFor(t *testing.T) {
	now := time.Now().UTC()
	modelFinding := func(category FindingCategory) *Finding {
		return &Finding{ID: "864e0900543944cd", Key: "restart-loop", Category: category, Severity: FindingSeverityWarning,
			ResourceID: "system-container:101", ResourceName: "gitea", ResourceType: "system-container",
			Title: "gitea LXC has start-on-boot enabled but stopped and did not recover on its own", Source: "ai-analysis"}
	}

	t.Run("deliberate stop", func(t *testing.T) {
		patrol := stoppedGuestPatrol(t, "system-container:101", unifiedresources.ResourceTypeSystemContainer,
			wholeLog(ProxmoxGuestTask{Type: "vzstop", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}))
		var owned *tools.PatrolFindingOwnedByGuestStopError
		if err := patrol.proxmoxGuestStopOwnsFinding(modelFinding(FindingCategoryReliability)); !errors.As(err, &owned) {
			t.Fatalf("err = %v, want the report refused", err)
		}
		if owned.OwnerFindingID != "" || !strings.Contains(owned.Note, "stopped through Proxmox at ") || !strings.Contains(owned.Note, "A deliberate stop is not a finding") {
			t.Fatalf("refusal = %+v", owned)
		}
		// Something else about the guest is still reportable.
		if err := patrol.proxmoxGuestStopOwnsFinding(modelFinding(FindingCategoryBackup)); err != nil {
			t.Fatalf("a backup finding was refused: %v", err)
		}
		// The report boundary accepts the guest's name in place of its ID.
		byName := modelFinding(FindingCategoryReliability)
		byName.ResourceID, byName.ResourceName = "guest-system-container:101", "guest-system-container:101"
		if err := patrol.proxmoxGuestStopOwnsFinding(byName); !errors.As(err, &owned) {
			t.Fatalf("a report that names the guest by name was not refused: %v", err)
		}
		// Another resource with its own ID can share the stopped guest's
		// name. A report on it is not about this stop.
		namesake := modelFinding(FindingCategoryReliability)
		namesake.ResourceID, namesake.ResourceName = "homelab:other:202", "guest-system-container:101"
		if err := patrol.proxmoxGuestStopOwnsFinding(namesake); err != nil {
			t.Fatalf("a report on another resource of the same name was refused: %v", err)
		}
	})

	t.Run("stop Pulse filed", func(t *testing.T) {
		patrol := stoppedGuestPatrol(t, "system-container:101", unifiedresources.ResourceTypeSystemContainer, wholeLog())
		var owned *tools.PatrolFindingOwnedByGuestStopError
		if err := patrol.proxmoxGuestStopOwnsFinding(modelFinding(FindingCategoryReliability)); !errors.As(err, &owned) {
			t.Fatalf("err = %v, want the second finding refused", err)
		}
		if owned.OwnerFindingID != "proxmox:guest:stopped:system-container:101" || !strings.Contains(owned.Note, "A second finding was not filed") {
			t.Fatalf("refusal = %+v", owned)
		}
		if err := patrol.proxmoxGuestStopOwnsFinding(modelFinding(FindingCategoryBackup)); err != nil {
			t.Fatalf("a backup finding was refused: %v", err)
		}
		// A separate reliability problem on the same guest is not about it
		// being down, even when its text mentions that it is.
		replication := modelFinding(FindingCategoryReliability)
		replication.Key, replication.Title = "replication-target-unavailable", "Replication destination unavailable for three days"
		replication.Description = "The guest is stopped. Its replication job reports an error."
		if err := patrol.proxmoxGuestStopOwnsFinding(replication); err != nil {
			t.Fatalf("a replication finding was refused: %v", err)
		}
		// Pulse's own finding is not refused by its own rule.
		own := patrol.GetFindings().Get("proxmox:guest:stopped:system-container:101")
		if err := patrol.proxmoxGuestStopOwnsFinding(own); err != nil {
			t.Fatalf("Pulse's own finding was refused: %v", err)
		}

		// Running again, the guest is an ordinary resource.
		patrol.SetReadState(proxmoxLifecycleReadState(proxmoxLifecycleTestResource("system-container:101", unifiedresources.ResourceTypeSystemContainer, unifiedresources.StatusOnline, time.Now().UTC().Add(20*time.Second))))
		patrol.runPatrol(context.Background())
		if err := patrol.proxmoxGuestStopOwnsFinding(modelFinding(FindingCategoryReliability)); err != nil {
			t.Fatalf("a finding on a running guest was refused: %v", err)
		}
	})

	t.Run("a guest Pulse never saw stop", func(t *testing.T) {
		patrol := NewPatrolService(nil, nil)
		if err := patrol.proxmoxGuestStopOwnsFinding(modelFinding(FindingCategoryReliability)); err != nil {
			t.Fatalf("refused: %v", err)
		}
	})
}

// The same rule at the boundary the model's report crosses: nothing is stored
// and the refusal names the finding Pulse already filed.
func TestModelReportOnAGuestPulseSawStopIsNotStored(t *testing.T) {
	patrol := stoppedGuestPatrol(t, "system-container:101", unifiedresources.ResourceTypeSystemContainer, wholeLog())
	adapter := newPatrolFindingCreatorAdapterState(patrol, patrol.currentPatrolRuntimeState())
	adapter.setFindingScope([]string{"system-container:101"})

	report := tools.PatrolFindingInput{
		ResourceID: "system-container:101", ResourceName: "guest-system-container:101", ResourceType: "system-container",
		Key: "restart-loop", Severity: "warning", Category: "reliability",
		Title:       "LXC has start-on-boot enabled but stopped and did not recover on its own",
		Description: "The container is stopped.", Recommendation: "Start it.", Evidence: "status=stopped onboot=1",
	}
	findingID, isNew, err := adapter.CreateFinding(report)
	var owned *tools.PatrolFindingOwnedByGuestStopError
	if !errors.As(err, &owned) || isNew || owned.OwnerFindingID != "proxmox:guest:stopped:system-container:101" {
		t.Fatalf("report = (%q, %t, %v), want it refused for the finding Pulse filed", findingID, isNew, err)
	}
	if stored := patrol.GetFindings().Get(findingID); stored != nil || len(adapter.getReportedFindingIDs()) != 0 {
		t.Fatalf("the refused report was stored: %+v, reported %v", stored, adapter.getReportedFindingIDs())
	}
	onGuest := 0
	for _, finding := range patrol.GetFindings().GetActive(FindingSeverityInfo) {
		if finding.ResourceID == "system-container:101" {
			onGuest++
		}
	}
	if onGuest != 1 {
		t.Fatalf("the guest has %d open findings, want only the one Pulse filed", onGuest)
	}

	// A different matter on the same guest is stored as before.
	backup := report
	backup.Key, backup.Category, backup.Title = "never-backed-up", "backup", "Guest has never been backed up"
	if _, created, err := adapter.CreateFinding(backup); err != nil || !created {
		t.Fatalf("a backup finding on the stopped guest = (%t, %v), want it stored", created, err)
	}
}

// The Proxmox user who ran a task is for the debug log. Findings, the scan's
// seed and tool results can all reach a model provider, so none carries it.
func TestProxmoxUserNeverReachesModelFacingText(t *testing.T) {
	now := time.Now().UTC()
	const user = "alice.example@corp-ldap"
	all := func(string, string) bool { return true }
	texts := map[string]string{}

	stopped := stoppedGuestPatrol(t, "system-container:101", unifiedresources.ResourceTypeSystemContainer,
		wholeLog(ProxmoxGuestTask{Type: "vzstop", User: user, Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}))
	if record, ok := stopped.proxmoxGuestStoppedOnPurpose("system-container:101"); !ok || record.cause.user != user {
		t.Fatalf("the user is not on the stop record: %+v", record)
	} else {
		texts["requested sentence"] = record.requestedSentence()
	}
	texts["seed for a deliberate stop"] = stopped.seedProxmoxGuestStops(all)
	if err := stopped.proxmoxGuestStopOwnsFinding(&Finding{ID: "x", Key: "guest-stopped", Category: FindingCategoryReliability, ResourceID: "system-container:101", Title: "Guest stopped"}); err != nil {
		texts["tool refusal"] = err.Error()
	}

	for name, tasks := range map[string][]ProxmoxGuestTask{
		"restart": {{Type: "vzreboot", User: user, Status: "OK", StartedAt: now.Add(3 * time.Second), EndedAt: now.Add(8 * time.Second)}},
		"start":   {{Type: "vzstart", User: user, Status: "OK", StartedAt: now.Add(3 * time.Second), EndedAt: now.Add(4 * time.Second)}},
	} {
		patrol := stoppedGuestPatrol(t, "system-container:101", unifiedresources.ResourceTypeSystemContainer, wholeLog(tasks...))
		finding := patrol.GetFindings().Get("proxmox:guest:stopped:system-container:101")
		if finding == nil {
			t.Fatalf("%s: no finding", name)
		}
		texts[name+" finding"] = finding.Title + " " + finding.Description + " " + finding.Recommendation + " " + finding.Evidence + " " + finding.Impact
		texts[name+" seed"] = patrol.seedProxmoxGuestStops(all)
	}
	for name, text := range texts {
		if text == "" {
			t.Errorf("%s is empty", name)
		}
		if strings.Contains(text, user) || strings.Contains(text, "alice") {
			t.Errorf("%s carries the Proxmox user: %q", name, text)
		}
	}
}

// The scan is told what Pulse knows about each guest it saw stop.
func TestScanSeedSaysWhoStoppedEachGuest(t *testing.T) {
	now := time.Now().UTC()
	all := func(string, string) bool { return true }

	deliberate := stoppedGuestPatrol(t, "system-container:101", unifiedresources.ResourceTypeSystemContainer,
		wholeLog(ProxmoxGuestTask{Type: "vzstop", User: "root@pam", Status: "OK", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(5 * time.Second)}))
	seed := deliberate.seedProxmoxGuestStops(all)
	for _, want := range []string{"# Proxmox Guests Pulse Saw Stop", "(lxc 101 on delly)", "stopped through Proxmox at ", "(vzstop)", "This was deliberate. Do not report it as a problem and do not propose starting it."} {
		if !strings.Contains(seed, want) {
			t.Errorf("seed %q lacks %q", seed, want)
		}
	}
	if got := deliberate.seedProxmoxGuestStops(func(string, string) bool { return false }); got != "" {
		t.Errorf("a guest outside the run's scope is in its seed: %q", got)
	}
	// The seed is part of what every scan reads.
	context, _ := deliberate.seedFindingsAndContextState(nil, deliberate.currentPatrolRuntimeState())
	if !strings.Contains(context, "This was deliberate. Do not report it as a problem") {
		t.Errorf("the scan's context lacks the stop: %q", context)
	}

	died := stoppedGuestPatrol(t, "vm:160", unifiedresources.ResourceTypeVM, wholeLog())
	seed = died.seedProxmoxGuestStops(all)
	for _, want := range []string{"Pulse already filed finding proxmox:guest:stopped:vm:160", "it was not stopped through Proxmox", "Do not file another finding about this guest being stopped or not running."} {
		if !strings.Contains(seed, want) {
			t.Errorf("seed %q lacks %q", seed, want)
		}
	}

	unknown := stoppedGuestPatrol(t, "vm:160", unifiedresources.ResourceTypeVM, unreadLog("collection_failed"))
	if seed := unknown.seedProxmoxGuestStops(all); !strings.Contains(seed, "it cannot tell whether a person stopped it") {
		t.Errorf("seed = %q", seed)
	}

	if seed := NewPatrolService(nil, nil).seedProxmoxGuestStops(all); seed != "" {
		t.Errorf("a Patrol that saw nothing stop seeds %q", seed)
	}
}

// End to end through an alert run. A container a person stopped gets a
// powered-off alert. The alert wakes Patrol. Nothing is filed, nothing is
// investigated, and the model is told the stop was deliberate and is not told
// that Pulse could start the guest.
func TestAlertRunOnAGuestAPersonStoppedFilesNothing(t *testing.T) {
	for _, sustained := range []bool{true, false} {
		name := "alert that fired"
		if sustained {
			name = "alert that stayed active"
		}
		t.Run(name, func(t *testing.T) {
			rig := newDeadGuestRig(t, config.PatrolAutonomyApproval)
			log := wholeLog()
			rig.ps.SetProxmoxGuestTaskReader(func(ctx context.Context, instance, node string, vmid int, since, until time.Time) ProxmoxGuestTaskRead {
				// The person stopped the container. The VM is left running.
				if vmid == 101 {
					return ProxmoxGuestTaskRead{Complete: true, Tasks: []ProxmoxGuestTask{{Type: "vzstop", User: "root@pam", Status: "OK", StartedAt: since.Add(proxmoxGuestStopTaskLookback).Add(time.Millisecond)}}}
				}
				return log.reader()(ctx, instance, node, vmid, since, until)
			})

			rig.collect("running", "running")
			rig.ps.runPatrol(context.Background())

			rig.ct.Status = "stopped"
			alert := poweredOffAlertFromManager(t, rig.ct, "homelab")
			rig.collect("stopped", "running", alert)
			rig.prompts = nil
			scope := AlertTriggeredPatrolScope(alert.ID, alert.ResourceID, "system-container", alert.Type)
			if sustained {
				scope = SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "system-container", alert.Type)
			}
			scope.NoStream = true
			rig.ps.runScopedPatrol(context.Background(), scope)

			if finding := rig.stoppedFinding(rig.ctID); finding != nil {
				t.Fatalf("a container a person stopped has a stopped finding: %#v", finding)
			}
			for _, finding := range rig.ps.GetFindings().GetActive(FindingSeverityInfo) {
				if finding.ResourceID == rig.ctID {
					t.Fatalf("a finding was opened on a container a person stopped: %#v", finding)
				}
			}
			if id := rig.investigated(); id != "" {
				t.Fatalf("an investigation started for %s", id)
			}
			if len(rig.prompts) != 1 {
				t.Fatalf("the model was asked %d times, want once", len(rig.prompts))
			}
			for _, want := range []string{
				"This guest was stopped through Proxmox at ",
				"That was deliberate. Report nothing about it being stopped and do not propose starting it.",
			} {
				if !strings.Contains(rig.prompts[0], want) {
					t.Errorf("the alert run's prompt lacks %q", want)
				}
			}
			for _, wrong := range []string{"so it is meant to be running", "Pulse can run these actions", "Pulse cannot run an action for"} {
				if strings.Contains(rig.prompts[0], wrong) {
					t.Errorf("the prompt of a run on a guest a person stopped says %q", wrong)
				}
			}
		})
	}
}
