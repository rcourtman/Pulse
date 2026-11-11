package ai

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
