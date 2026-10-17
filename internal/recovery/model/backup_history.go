package model

import "time"

// BackupHistoryCollection is a completed source read, including unsuccessful
// reads. Collection quality is distinct from the outcome of a backup job.
type BackupHistoryCollection struct {
	Provider   Provider
	Instance   string
	Node       string
	ObservedAt time.Time
	Complete   bool
	Reason     string
	MaxGap     time.Duration
}

type BackupHistorySource struct {
	Provider        Provider
	Instance        string
	Node            string
	FirstObservedAt time.Time
	LastObservedAt  time.Time
	Complete        bool
	Reason          string
}

type BackupHistoryInterval struct {
	Provider Provider
	Instance string
	Node     string
	Start    time.Time
	End      time.Time
}

// BackupHistoryCoverage describes Pulse's retained observation coverage. Source
// task enumeration never backdates this coverage: provider retention is external
// and a newly connected server cannot prove a previously unobserved period.
type BackupHistoryCoverage struct {
	Sources   []BackupHistorySource
	Intervals []BackupHistoryInterval
}

type BackupHealthWorkload struct {
	ID, Name, Instance, Node, Type string
	VMID                           int
}

type BackupOutcomeCounts struct {
	Success, Failed, Warning, Running, Unknown int
}

func (c *BackupOutcomeCounts) Add(outcome Outcome) {
	switch outcome {
	case OutcomeSuccess:
		c.Success++
	case OutcomeFailed:
		c.Failed++
	case OutcomeWarning:
		c.Warning++
	case OutcomeRunning:
		c.Running++
	default:
		c.Unknown++
	}
}

func (c BackupOutcomeCounts) Total() int {
	return c.Success + c.Failed + c.Warning + c.Running + c.Unknown
}

type BackupHealthWorkloadResult struct {
	BackupHealthWorkload
	Counts        BackupOutcomeCounts
	LastSuccessAt *time.Time
	HistoryState  string
	HistoryReason string
}

type BackupHealthDay struct {
	Date         string
	Counts       BackupOutcomeCounts
	HistoryState string
}

type BackupHealthFailure struct {
	WorkloadID, WorkloadName, Instance, TaskID, Detail string
	At                                                 time.Time
	Outcome                                            Outcome
}

type BackupHealthSummary struct {
	Start, End       time.Time
	Timezone         string
	Counts           BackupOutcomeCounts
	Workloads        []BackupHealthWorkloadResult
	Days             []BackupHealthDay
	Failures         []BackupHealthFailure
	HistoryState     string
	MissingWorkloads int
}
