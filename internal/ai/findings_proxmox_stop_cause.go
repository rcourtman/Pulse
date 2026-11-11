package ai

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rs/zerolog/log"
)

// Who stopped a Proxmox guest.
//
// A guest that goes from running to stopped either was asked to stop or was
// not. Proxmox knows which: every stop, shutdown, rollback or suspend that
// goes through it is a task in the node's log, with the user who asked. Pulse
// reads that log once, through its own Proxmox API connection, when it first
// sees the guest stopped.
//
// What Pulse does with the answer:
//
//   - A task asked for the stop: a person or a job did this on purpose. No
//     finding, no investigation and no start proposal.
//   - No such task, and the log was read in full: the guest was not stopped
//     through Proxmox. It stopped from inside or it failed. The finding says
//     so.
//   - The log could not be read in full: the finding stays, and it says Pulse
//     cannot tell whether a person stopped the guest. Unknown is never
//     treated as "nobody stopped it".

// ProxmoxGuestTask is one entry of a node's task log for a guest.
type ProxmoxGuestTask struct {
	Type      string
	User      string
	Status    string
	StartedAt time.Time
	// EndedAt is zero while the task is still running.
	EndedAt time.Time
}

// ProxmoxGuestTaskRead is a node's task log for one guest in a window of time.
// Complete is false when the read failed, was cut off, or Pulse's token is not
// confirmed to see other users' tasks. An empty read that is not complete says
// nothing about what happened.
type ProxmoxGuestTaskRead struct {
	Tasks    []ProxmoxGuestTask
	Complete bool
	Reason   string
}

// ProxmoxGuestTaskReader reads the tasks a node logged for one guest that
// started between since and until, through Pulse's own Proxmox API connection.
type ProxmoxGuestTaskReader func(ctx context.Context, instance, node string, vmid int, since, until time.Time) ProxmoxGuestTaskRead

const (
	// Proxmox filters a task window on when a task started. A shutdown can
	// run for a long time before the guest is down, so the window opens well
	// before the last time Pulse saw the guest running. A shutdown that began
	// earlier than this and was still running then is missed.
	proxmoxGuestStopTaskLookback = 2 * time.Hour
	// Pulse's clock, the node's clock and the age of a polled status differ.
	proxmoxGuestStopClockSlack = 2 * time.Minute
	// One look at the inventory may find many guests stopped at once. All of
	// their task log reads share this much time, so a Proxmox API that hangs
	// cannot hold a Patrol run. A read that gets no time is unknown.
	proxmoxGuestStopReadBudget = 30 * time.Second

	proxmoxGuestStopReasonNoReader = "no_connection"
)

type proxmoxGuestStopCauseKind string

const (
	// proxmoxGuestStopUnknown is the zero value on purpose: a cause nobody
	// worked out is unknown, never "nobody stopped it".
	proxmoxGuestStopUnknown   proxmoxGuestStopCauseKind = ""
	proxmoxGuestStopRequested proxmoxGuestStopCauseKind = "stop_task"
	proxmoxGuestStopNoTask    proxmoxGuestStopCauseKind = "no_stop_task"
	// proxmoxGuestStopRestart: a restart or a start was asked for and the
	// guest is down. Nobody asked for it to stay down.
	proxmoxGuestStopRestart proxmoxGuestStopCauseKind = "restart_task"
)

// proxmoxGuestStopCause is what the node's task log says about one stop.
type proxmoxGuestStopCause struct {
	kind proxmoxGuestStopCauseKind
	// task, user, taskAt and taskStatus describe the stop or restart task.
	// user is the Proxmox user who ran it. It goes to the debug log only. It
	// is never written into a finding, the scan's seed, a tool result or a
	// proposal reason, because all of those can reach a model provider.
	task       string
	user       string
	taskAt     time.Time
	taskStatus string
	// governed is set when the stop was an action Pulse itself ran after an
	// approval. Pulse knows that from its own action record, not the log.
	governed bool
	// unreadReason says why the log could not be read in full.
	unreadReason string
	// since and until bound the time the log was searched for a task that
	// explains the stop.
	since time.Time
	until time.Time
}

// proxmoxGuestStopRecord is the cause of the stop of a guest that is still
// stopped. The watcher keeps it until the guest runs again or is gone.
type proxmoxGuestStopRecord struct {
	cause     proxmoxGuestStopCause
	name      string
	kind      string
	node      string
	vmid      int
	stoppedAt time.Time
}

// Task types that ask a guest to stop and stay stopped. hastop is what the
// API logs for both stop and shutdown of an HA-managed guest. A rollback
// stops the guest to put it back, and a suspend to disk leaves a VM stopped.
// Each is somebody's decision, whoever or whatever made it.
var proxmoxGuestStopTaskTypes = map[string]bool{
	"qmstop": true, "qmshutdown": true, "qmsuspend": true, "qmrollback": true,
	"vzstop": true, "vzshutdown": true, "vzsuspend": true, "vzrollback": true,
	"hastop": true,
}

// Task types that ask a guest to go down and come back.
var proxmoxGuestRestartTaskTypes = map[string]bool{"qmreboot": true, "vzreboot": true}

// Task types that ask a stopped guest to run.
var proxmoxGuestStartTaskTypes = map[string]bool{
	"qmstart": true, "vzstart": true, "hastart": true, "qmresume": true, "vzresume": true,
}

// proxmoxGuestTaskSucceeded reports whether a finished task ended well. A task
// still running has not failed.
func proxmoxGuestTaskSucceeded(task *ProxmoxGuestTask) bool {
	status := strings.ToUpper(strings.TrimSpace(task.Status))
	return task.EndedAt.IsZero() || status == "OK" || strings.HasPrefix(status, "WARNINGS")
}

// classifyProxmoxGuestStop decides what a task read says about a stop seen
// between the last running observation and the first stopped one.
//
// Only tasks that could bear on the stop are looked at: ones that started no
// later than the stopped observation and had not finished before the last
// running one. Of those, the last to start decides.
//
//   - A stop task last: somebody asked for the stop. Its exit status does not
//     matter, because it is the asking that makes the stop deliberate.
//   - A start that worked last: the guest was brought up after any stop, and
//     went down again with nothing asking. An earlier stop does not explain
//     that.
//   - A restart, or a start that failed, last: somebody wants the guest
//     running and it is not.
//
// A log Pulse could not read in full decides nothing, even when it shows a
// stop task: a later start may be among the tasks Pulse cannot see, and then
// the stop is not why the guest is down. The cause is unknown.
func classifyProxmoxGuestStop(read ProxmoxGuestTaskRead, runningAt, stoppedAt time.Time) proxmoxGuestStopCause {
	cause := proxmoxGuestStopCause{since: runningAt.UTC(), until: stoppedAt.UTC()}
	const (
		taskStops = iota + 1
		taskRestarts
		taskStarts
	)
	var last *ProxmoxGuestTask
	lastClass := 0
	for i := range read.Tasks {
		task := &read.Tasks[i]
		if task.StartedAt.IsZero() || task.StartedAt.After(stoppedAt.Add(proxmoxGuestStopClockSlack)) {
			continue
		}
		if !task.EndedAt.IsZero() && task.EndedAt.Before(runningAt.Add(-proxmoxGuestStopClockSlack)) {
			continue
		}
		class := 0
		switch kind := strings.ToLower(strings.TrimSpace(task.Type)); {
		case proxmoxGuestStopTaskTypes[kind]:
			class = taskStops
		case proxmoxGuestRestartTaskTypes[kind]:
			class = taskRestarts
		case proxmoxGuestStartTaskTypes[kind]:
			class = taskStarts
		default:
			continue
		}
		// Two tasks in the same second have no order Pulse can see. The one
		// that leaves a finding standing wins.
		if last == nil || task.StartedAt.After(last.StartedAt) || (task.StartedAt.Equal(last.StartedAt) && class > lastClass) {
			last, lastClass = task, class
		}
	}
	describe := func(kind proxmoxGuestStopCauseKind) proxmoxGuestStopCause {
		cause.kind = kind
		cause.task = strings.ToLower(strings.TrimSpace(last.Type))
		cause.user = strings.TrimSpace(last.User)
		cause.taskAt = last.StartedAt.UTC()
		cause.taskStatus = strings.TrimSpace(last.Status)
		return cause
	}
	switch {
	case !read.Complete:
		cause.kind = proxmoxGuestStopUnknown
		cause.unreadReason = strings.TrimSpace(read.Reason)
		if cause.unreadReason == "" {
			cause.unreadReason = "collection_failed"
		}
		return cause
	case lastClass == taskStops:
		return describe(proxmoxGuestStopRequested)
	case lastClass == taskRestarts, lastClass == taskStarts && !proxmoxGuestTaskSucceeded(last):
		return describe(proxmoxGuestStopRestart)
	case lastClass == taskStarts:
		// Started through Proxmox, and down again with nothing asking. The
		// start is named so the finding does not claim the log held no stop
		// at all: one may have come before this start.
		return describe(proxmoxGuestStopNoTask)
	default:
		cause.kind = proxmoxGuestStopNoTask
		return cause
	}
}

// readStopCause asks the node's task log what stopped a guest. It reads both
// nodes when the guest was seen on a different one before it stopped. ctx
// carries the time all of one look's reads share.
func (w *proxmoxGuestLifecycleWatcher) readStopCause(ctx context.Context, before, after proxmoxGuestLifecycleSnapshot) proxmoxGuestStopCause {
	var reader ProxmoxGuestTaskReader
	if w != nil {
		w.mu.Lock()
		reader = w.readTasks
		w.mu.Unlock()
	}
	if reader == nil {
		return proxmoxGuestStopCause{unreadReason: proxmoxGuestStopReasonNoReader, since: before.observedAt, until: after.observedAt}
	}
	if ctx.Err() != nil {
		return proxmoxGuestStopCause{unreadReason: "collection_failed", since: before.observedAt, until: after.observedAt}
	}
	since := before.observedAt.Add(-proxmoxGuestStopTaskLookback)
	until := after.observedAt.Add(proxmoxGuestStopClockSlack)
	merged := ProxmoxGuestTaskRead{Complete: true}
	nodes := []string{strings.TrimSpace(after.node)}
	if previous := strings.TrimSpace(before.node); previous != "" && !strings.EqualFold(previous, nodes[0]) {
		nodes = append(nodes, previous)
	}
	for _, node := range nodes {
		read := reader(ctx, after.instance, node, after.vmid, since, until)
		merged.Tasks = append(merged.Tasks, read.Tasks...)
		if !read.Complete {
			merged.Complete = false
			if merged.Reason == "" {
				merged.Reason = read.Reason
			}
		}
	}
	return classifyProxmoxGuestStop(merged, before.observedAt, after.observedAt)
}

func proxmoxGuestStopTime(t time.Time) string {
	if t.IsZero() {
		return "an unknown time"
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// askedFor names what a restart or start task asked for.
func (c proxmoxGuestStopCause) askedFor() string {
	if proxmoxGuestStartTaskTypes[c.task] {
		return "a start"
	}
	return "a restart"
}

// unreadReasonText says in plain words why the task log could not be read.
func (c proxmoxGuestStopCause) unreadReasonText(node string) string {
	switch c.unreadReason {
	case proxmoxGuestStopReasonNoReader:
		return "it has no Proxmox API connection for this guest"
	case "access_denied":
		return "Proxmox refused the request"
	case "audit_permission_unconfirmed":
		return fmt.Sprintf("its Proxmox API access is not confirmed to see other users' tasks on this node, which takes Sys.Audit on /nodes/%s", node)
	case "history_truncated":
		return "the log held too many entries for this guest to read at once"
	default:
		return "the request failed"
	}
}

// proxmoxGuestStopUnknownNote is the sentence that follows an unreadable task
// log wherever a start is proposed or approved.
const proxmoxGuestStopUnknownNote = "Pulse could not read the node's task log, so it cannot tell whether a person stopped this guest. Check that before you approve a start."

// applyTo writes the cause into a stopped-guest finding: what happened, what
// to do about it, and a stop_cause token in the evidence that survives a
// restart and that the start proposal reads.
func (c proxmoxGuestStopCause) applyTo(finding *Finding, snapshot proxmoxGuestLifecycleSnapshot) {
	name := finding.ResourceName
	token := string(c.kind)
	switch c.kind {
	case proxmoxGuestStopNoTask:
		finding.Recommendation = "Review the guest and approve the governed Start action if it should be running."
		if c.task != "" {
			finding.Description = fmt.Sprintf("%s went from running to stopped and nothing in Proxmox asked it to. It was last started through Proxmox (%s at %s) and the node's task log has no stop or shutdown task after that. It stopped from inside the guest or it failed.",
				name, c.task, proxmoxGuestStopTime(c.taskAt))
			finding.Evidence += " stop_cause=" + token + ". No stop or shutdown task for this guest after its last start in the node's task log: it was not stopped through Proxmox."
			break
		}
		finding.Description = fmt.Sprintf("%s went from running to stopped and nothing in Proxmox asked it to. The node's task log has no stop or shutdown task for it between %s and %s. It stopped from inside the guest or it failed.",
			name, proxmoxGuestStopTime(c.since), proxmoxGuestStopTime(c.until))
		finding.Evidence += " stop_cause=" + token + ". No stop or shutdown task for this guest in the node's task log: it was not stopped through Proxmox."
	case proxmoxGuestStopRestart:
		finding.Description = fmt.Sprintf("%s went from running to stopped after %s was asked for through Proxmox (%s at %s). It did not come back.",
			name, c.askedFor(), c.task, proxmoxGuestStopTime(c.taskAt))
		finding.Recommendation = "Review why it did not come back and approve the governed Start action if the guest should be running."
		finding.Evidence += fmt.Sprintf(" stop_cause=%s task=%s at=%s. Somebody asked for %s and the guest did not come back.",
			token, c.task, c.taskAt.UTC().Format(time.RFC3339), c.askedFor())
	default:
		why := c.unreadReasonText(snapshot.node)
		token = "unknown"
		finding.Description = fmt.Sprintf("%s went from running to stopped. Pulse could not read the node's task log: %s. So it cannot tell whether a person stopped this guest.", name, why)
		finding.Recommendation = "Check whether someone stopped this guest on purpose before you approve Start. Pulse could not tell."
		reason := c.unreadReason
		if reason == "" {
			reason = "collection_failed"
		}
		finding.Evidence += fmt.Sprintf(" stop_cause=%s reason=%s. Pulse could not read the node's task log, so it cannot tell whether a person stopped this guest.", token, reason)
	}
}

// proxmoxGuestStopCauseToken reads the stop_cause token back from a stopped
// guest finding's evidence.
func proxmoxGuestStopCauseToken(finding *Finding) string {
	if finding == nil || finding.Source != proxmoxGuestLifecycleFindingSource {
		return ""
	}
	_, rest, found := strings.Cut(finding.Evidence, " stop_cause=")
	if !found {
		return ""
	}
	end := strings.IndexAny(rest, ". ")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}

// rememberStop keeps the cause of a stop while the guest stays stopped. The
// task log is read outside the lock, so by now another look may have seen the
// guest running again, gone, or stopped a second time. episode is the number
// Observe gave this stop when it first saw it. The cause is kept only while
// that is still the guest's current stop, and the answer says whether it was.
func (w *proxmoxGuestLifecycleWatcher) rememberStop(snapshot proxmoxGuestLifecycleSnapshot, episode uint64, cause proxmoxGuestStopCause) bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if episode == 0 || w.stopEpisodes[snapshot.id].seq != episode {
		return false
	}
	if w.stops == nil {
		w.stops = make(map[string]proxmoxGuestStopRecord)
	}
	name := snapshot.name
	if name == "" {
		name = snapshot.id
	}
	w.stops[snapshot.id] = proxmoxGuestStopRecord{cause: cause, name: name, kind: snapshot.kind, node: snapshot.node, vmid: snapshot.vmid, stoppedAt: snapshot.observedAt}
	return true
}

// forgetStopLocked drops what Pulse held about a guest's stop, and with it any
// read still in flight for that stop. The caller holds w.mu.
func (w *proxmoxGuestLifecycleWatcher) forgetStopLocked(id string) {
	delete(w.stops, id)
	delete(w.stopEpisodes, id)
}

// stopRecord returns what Pulse worked out about a guest that is still
// stopped.
func (w *proxmoxGuestLifecycleWatcher) stopRecord(resourceID string) (proxmoxGuestStopRecord, bool) {
	if w == nil {
		return proxmoxGuestStopRecord{}, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	record, ok := w.stops[strings.TrimSpace(resourceID)]
	return record, ok
}

// stopRecordForReport finds the stop a model's report is about. ids are the
// ID the report gave and the inventory ID it resolves to. A report may also
// give the guest's name where its ID belongs, which the report boundary
// accepts, so a submitted ID that is exactly one stopped guest's name matches
// that guest. The report's separate name field is not used: another resource
// with its own ID can share a stopped guest's name.
func (w *proxmoxGuestLifecycleWatcher) stopRecordForReport(submittedID string, ids ...string) (string, proxmoxGuestStopRecord, bool) {
	if w == nil {
		return "", proxmoxGuestStopRecord{}, false
	}
	submittedID = strings.TrimSpace(submittedID)
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range append([]string{submittedID}, ids...) {
		if record, ok := w.stops[strings.TrimSpace(id)]; ok {
			return strings.TrimSpace(id), record, true
		}
	}
	// An ID that resolved to an inventory resource names that resource, and
	// it has no stop on record.
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			return "", proxmoxGuestStopRecord{}, false
		}
	}
	var matchID string
	var match proxmoxGuestStopRecord
	matches := 0
	for id, record := range w.stops {
		if submittedID != "" && strings.EqualFold(record.name, submittedID) {
			matchID, match = id, record
			matches++
		}
	}
	if matches != 1 {
		return "", proxmoxGuestStopRecord{}, false
	}
	return matchID, match, true
}

// SetProxmoxGuestTaskReader gives Patrol the read of a node's task log that
// tells a deliberate stop from a guest that went down by itself.
func (p *PatrolService) SetProxmoxGuestTaskReader(reader ProxmoxGuestTaskReader) {
	if p == nil || p.proxmoxGuestLifecycleWatcher == nil {
		return
	}
	w := p.proxmoxGuestLifecycleWatcher
	w.mu.Lock()
	w.readTasks = reader
	w.mu.Unlock()
}

// ProxmoxGuestStartProposalNote is the sentence a start proposal carries when
// it answers a stopped-guest finding whose cause Pulse could not work out. The
// person approving must not read "start it" as "Pulse knows it should run".
func (p *PatrolService) ProxmoxGuestStartProposalNote(findingID string) string {
	if p == nil || p.findings == nil {
		return ""
	}
	finding := p.findings.Get(strings.TrimSpace(findingID))
	if finding == nil || proxmoxGuestStopCauseToken(finding) != "unknown" {
		return ""
	}
	return proxmoxGuestStopUnknownNote
}

// proxmoxGuestStoppedOnPurpose reports whether Pulse saw this guest stop and
// the node's task log shows somebody asked for it. It holds while the guest
// stays stopped.
func (p *PatrolService) proxmoxGuestStoppedOnPurpose(resourceID string) (proxmoxGuestStopRecord, bool) {
	if p == nil {
		return proxmoxGuestStopRecord{}, false
	}
	record, ok := p.proxmoxGuestLifecycleWatcher.stopRecord(resourceID)
	if !ok || record.cause.kind != proxmoxGuestStopRequested {
		return proxmoxGuestStopRecord{}, false
	}
	return record, true
}

func (r proxmoxGuestStopRecord) label() string {
	return fmt.Sprintf("%s (%s %s on %s)", r.name, strings.ToLower(r.kind), strconv.Itoa(r.vmid), r.node)
}

func (r proxmoxGuestStopRecord) requestedSentence() string {
	if r.cause.governed {
		return fmt.Sprintf("stopped by Pulse at %s, after a person approved it", proxmoxGuestStopTime(r.cause.taskAt))
	}
	return fmt.Sprintf("stopped through Proxmox at %s (%s)", proxmoxGuestStopTime(r.cause.taskAt), r.cause.task)
}

// seedProxmoxGuestStops tells the scan what Pulse already knows about guests
// it saw stop, so the model neither reports a deliberate stop nor files a
// second finding beside the one Pulse filed.
func (p *PatrolService) seedProxmoxGuestStops(inScope func(resourceID, resourceName string) bool) string {
	if p == nil || p.proxmoxGuestLifecycleWatcher == nil {
		return ""
	}
	w := p.proxmoxGuestLifecycleWatcher
	w.mu.Lock()
	ids := make([]string, 0, len(w.stops))
	records := make(map[string]proxmoxGuestStopRecord, len(w.stops))
	for id, record := range w.stops {
		ids = append(ids, id)
		records[id] = record
	}
	w.mu.Unlock()
	sort.Strings(ids)

	var lines []string
	for _, id := range ids {
		record := records[id]
		if inScope != nil && !inScope(id, record.name) {
			continue
		}
		if record.cause.kind == proxmoxGuestStopRequested {
			lines = append(lines, fmt.Sprintf("- %s: %s. This was deliberate. Do not report it as a problem and do not propose starting it.", record.label(), record.requestedSentence()))
			continue
		}
		filed := p.activeProxmoxGuestStoppedFinding(id)
		if filed == nil {
			continue
		}
		known := "Proxmox's task log shows nothing asked it to stop, so it was not stopped through Proxmox."
		switch record.cause.kind {
		case proxmoxGuestStopRestart:
			known = fmt.Sprintf("Somebody asked for %s through Proxmox (%s) and it did not come back.", record.cause.askedFor(), record.cause.task)
		case proxmoxGuestStopUnknown:
			known = "Pulse could not read the node's task log, so it cannot tell whether a person stopped it."
		}
		lines = append(lines, fmt.Sprintf("- %s: Pulse already filed finding %s (%q) for this stop. %s Do not file another finding about this guest being stopped or not running. Give your verdict on that finding.",
			record.label(), filed.ID, filed.Title, known))
	}
	if len(lines) == 0 {
		return ""
	}
	return "# Proxmox Guests Pulse Saw Stop\nWhat Proxmox's own task log says about each one.\n\n" + strings.Join(lines, "\n") + "\n\n"
}

func (p *PatrolService) activeProxmoxGuestStoppedFinding(resourceID string) *Finding {
	if p == nil || p.findings == nil {
		return nil
	}
	finding := p.findings.Get(proxmoxGuestStoppedFindingIDPrefix + strings.TrimSpace(resourceID))
	if finding == nil || !finding.IsActive() {
		return nil
	}
	return finding
}

// proxmoxGuestStopOwnsFinding refuses a finding the model files about a guest
// being down when Pulse has already accounted for that stop.
//
// A deliberate stop is not a problem, so the report is dropped. A stop Pulse
// filed its own finding for already has one, and a second would ask the
// operator to approve the same start twice. What the model learned belongs
// in its verdict on that finding.
//
// The report must be a reliability finding whose key or title is about the
// guest being down (proxmoxGuestReportIsAboutBeingDown). A missing backup, a
// failing replication or a full disk on the same guest stays reportable. Its
// description and evidence are not read: a separate problem often mentions
// that the guest is stopped.
func (p *PatrolService) proxmoxGuestStopOwnsFinding(finding *Finding) error {
	if p == nil || finding == nil || finding.Category != FindingCategoryReliability || finding.Source == proxmoxGuestLifecycleFindingSource {
		return nil
	}
	if !proxmoxGuestReportIsAboutBeingDown(finding) {
		return nil
	}
	// The report may carry a source ID. The inventory says which resource
	// that is.
	resolved := ""
	if p.hasPatrolRuntimeInputs() {
		if binding, ok := patrolAlertReferenceResolver(p.currentPatrolRuntimeState()).ResolveAlertResourceReference(finding.ResourceID); ok && !binding.Component {
			resolved = strings.TrimSpace(binding.ResourceID)
		}
	}
	resourceID, record, ok := p.proxmoxGuestLifecycleWatcher.stopRecordForReport(finding.ResourceID, resolved)
	if !ok {
		return nil
	}
	if record.cause.kind == proxmoxGuestStopRequested {
		log.Debug().Str("resource_id", resourceID).Str("finding_key", finding.Key).Str("task", record.cause.task).Str("user", record.cause.user).
			Msg("AI Patrol: dropped a model finding on a guest that was stopped on purpose")
		return &tools.PatrolFindingOwnedByGuestStopError{
			FindingID: finding.ID,
			Note:      fmt.Sprintf("%s was %s. A deliberate stop is not a finding. Nothing was filed.", record.name, record.requestedSentence()),
		}
	}
	filed := p.activeProxmoxGuestStoppedFinding(resourceID)
	if filed == nil {
		return nil
	}
	return &tools.PatrolFindingOwnedByGuestStopError{
		FindingID:      finding.ID,
		OwnerFindingID: filed.ID,
		Note:           fmt.Sprintf("Pulse already has finding %s for %s being stopped. A second finding was not filed. Put what you learned in your verdict on that finding.", filed.ID, record.name),
	}
}

// Words in a finding's key that say the guest is down or will not stay up.
var proxmoxGuestDownKeyParts = map[string]bool{
	"stopped": true, "stop": true, "down": true, "offline": true, "exited": true, "dead": true,
	"crash": true, "crashed": true, "restart": true, "restarting": true, "boot": true,
	"onboot": true, "poweredoff": true, "halted": true,
}

// Phrases in a finding's title that say the same.
var proxmoxGuestDownTitlePhrases = []string{
	"stopped", "not running", "is down", "went down", "offline", "powered off", "powered-off",
	"crash", "did not start", "failed to start", "not started", "start-on-boot", "start on boot",
	"restart", "did not recover",
}

// proxmoxGuestReportIsAboutBeingDown reads a report's key and title, the two
// fields that name its subject.
func proxmoxGuestReportIsAboutBeingDown(finding *Finding) bool {
	parts := strings.FieldsFunc(normalizeFindingKey(finding.Key), func(r rune) bool { return r == '-' })
	for i, part := range parts {
		if proxmoxGuestDownKeyParts[part] {
			return true
		}
		if part == "not" && i+1 < len(parts) && (parts[i+1] == "running" || parts[i+1] == "started") {
			return true
		}
	}
	title := strings.ToLower(finding.Title)
	for _, phrase := range proxmoxGuestDownTitlePhrases {
		if strings.Contains(title, phrase) {
			return true
		}
	}
	return false
}
