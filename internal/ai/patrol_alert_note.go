package ai

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// patrolActiveAlertFindingRule says when a run that looks at an alert the
// operator can already see should report a finding.
//
// The rule used to end "report nothing when the condition is expected or only
// a person can change it". On a host whose agent has commands off, or a
// Proxmox node with no action runner, only a person can change anything, so
// that sentence silenced every fault there. A sustained run on a worker that
// had crashed with a segfault wrote that it "will stay down until the operator
// intervenes" and filed nothing, and the same run on a host with commands on
// filed and the worker was fixed. A workload that failed and will not come
// back is a fault whoever has to fix it: the finding starts the investigation
// that tells the person what to do. What stays quiet is the expected
// condition, named by what it is and not by who could change it.
const patrolActiveAlertFindingRule = "Report a finding when one of these is true: you found the cause; " +
	"the workload has failed and will not come back by itself (it crashed or exited with an error and nothing is restarting it, it keeps restarting and failing, or it is unhealthy and staying that way); " +
	"Pulse can run an action for this resource that would fix it (the ones it can run are listed below); " +
	"or reading logs or disk usage on the machine would likely establish the cause. " +
	"A failed workload gets a finding even when Pulse cannot act or read logs there and only a person can fix it: say what failed and what the person should do. " +
	"Do not report a finding that only repeats the alert's reading, such as a disk percentage with nothing known about what fills it. " +
	"Report nothing when the condition is expected: a finished one-shot job that exited cleanly, a workload someone stopped on purpose, a deliberate setting, or hardware wear nobody can act on."

// patrolAskedResourceAlert returns the most severe alert active on the
// resources a person's targeted check was asked about. It returns nil for
// every other run: a full run, a run an alert started (it has its own note),
// and the checks Pulse starts by itself for a verification, an objective or
// an anomaly, which carry their own question.
func (p *PatrolService) patrolAskedResourceAlert(scope *PatrolScope) *models.Alert {
	if p == nil || scope == nil || scope.AlertContext != nil || scope.Reason != TriggerReasonManual {
		return nil
	}
	active := p.patrolScopeActiveAlerts(*scope)
	if len(active) == 0 {
		return nil
	}
	return &active[0]
}

// patrolAskedResourceAlertNote is the note for a person's check on a resource
// with an alert active. The alert is what to look at, under the same
// rule as a run the alert started after staying active. Another fact on the
// resource's row is not an answer to it.
func patrolAskedResourceAlertNote(alert *models.Alert) string {
	name := strings.TrimSpace(alert.ResourceName)
	if name == "" {
		name = strings.TrimSpace(alert.ResourceID)
	}
	level := strings.ToLower(strings.TrimSpace(alert.Level))
	if level == "" {
		level = "an"
	} else {
		level = "a " + level
	}
	return fmt.Sprintf("Note: Pulse has %s alert active on %q (%s: %q). "+
		"The operator can already see this alert, and this check was asked for on that resource, so look at the alert first. Look for what it does not say: the cause, what changed, what is using the space, or why the workload stopped. "+
		patrolActiveAlertFindingRule+" "+
		"Another fact about the resource, such as its backup age, is not an answer to this alert. Do not report it in place of looking at the alert, and do not join it to the alert's condition in one finding.\n\n",
		level, name, alert.Type, strings.TrimSpace(alert.Message))
}
