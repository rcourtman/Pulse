package monitoring

import (
	"container/heap"
	"context"
	"maps"
	"sync"
	"time"
)

type claimFollowup uint8

const (
	followWorkerSchedule claimFollowup = iota
	followExplicitSchedule
	dropWorkerSchedule
)

type pollReschedule uint8

const (
	noPollReschedule pollReschedule = iota
	pollOnCadence
	pollAfterBackoff
)

type scheduledTaskEntry struct {
	task         ScheduledTask // queued task, or the explicit successor of a claim
	index        int           // -1 while a worker owns the entry
	followup     claimFollowup
	keepSchedule bool // queued intent set by an explicit edit or a worker's retry
}

// Keep the existing type and name strings as separate fields. Routine plan
// refreshes and worker completions do not need to allocate a composite string
// while holding the queue lock.
type taskQueueKey struct {
	kind InstanceType
	name string
}

type taskHeap []*scheduledTaskEntry

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	if h[i].task.NextRun.Equal(h[j].task.NextRun) {
		if h[i].task.Priority == h[j].task.Priority {
			return h[i].task.InstanceName < h[j].task.InstanceName
		}
		return h[i].task.Priority > h[j].task.Priority
	}
	return h[i].task.NextRun.Before(h[j].task.NextRun)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x interface{}) {
	entry := x.(*scheduledTaskEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	if n == 0 {
		return nil
	}
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*h = old[:n-1]
	return entry
}

// TaskQueue is a thread-safe min-heap over scheduled tasks. The index also
// holds claimed tasks, which are not part of the heap or queue snapshots.
type TaskQueue struct {
	mu           sync.Mutex
	entries      map[taskQueueKey]*scheduledTaskEntry
	heap         taskHeap
	queuedByType map[string]int // heap membership only; claimed entries do not count
	earlierHead  chan struct{}  // lazily shared by workers waiting for new eligibility

	// Serialize availability settings refreshes from catalog read through
	// reconciliation. Planners and workers never need the refresh mutex.
	availabilityRefreshMu        sync.Mutex
	availabilityScheduleRevision uint64
}

// NewTaskQueue constructs an empty queue.
func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		entries:      make(map[taskQueueKey]*scheduledTaskEntry),
		heap:         make(taskHeap, 0),
		queuedByType: make(map[string]int),
	}
	heap.Init(&tq.heap)
	return tq
}

// Upsert schedules an explicit request. If a worker has already claimed the
// target, retain the latest request for when that worker returns.
func (q *TaskQueue) Upsert(task ScheduledTask) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.upsertLocked(task, false)
}

// upsertPlan adds or refreshes periodic work without replacing an explicit
// request, a retry, or the work the queue has already dispatched.
func (q *TaskQueue) upsertPlan(task ScheduledTask, availabilityRevision uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if task.InstanceType == InstanceTypeAvailability && availabilityRevision != q.availabilityScheduleRevision {
		return // A settings refresh superseded the planner's availability roster.
	}
	q.upsertLocked(task, true)
}

func (q *TaskQueue) upsertLocked(task ScheduledTask, fromPlanner bool) {
	key := taskQueueKey{task.InstanceType, task.InstanceName}
	if entry, ok := q.entries[key]; ok {
		if entry.index < 0 {
			if !fromPlanner {
				entry.task = task
				entry.followup = followExplicitSchedule
			}
			return
		}
		if fromPlanner {
			if entry.keepSchedule {
				return
			}
			// The live queue owns eligibility. In particular, passing the due
			// time does not mean a worker has run the poll. Only a shorter
			// cadence can tighten a pending slot; it can never postpone it.
			task.NextRun = entry.task.NextRun
			if task.Interval > 0 {
				if sooner := time.Now().Add(task.Interval); sooner.Before(task.NextRun) {
					task.NextRun = sooner
				}
			}
		}
		// Skip heap work only when all ordering fields are unchanged.
		// Compare the exact time value to preserve its monotonic clock
		// reading, not just equality of the represented instant.
		if task.NextRun == entry.task.NextRun && task.Priority == entry.task.Priority &&
			task.InstanceName == entry.task.InstanceName {
			entry.task = task
			entry.keepSchedule = !fromPlanner
			return
		}
		previous, hadHead := q.headTimeLocked()
		entry.task = task
		entry.keepSchedule = !fromPlanner
		heap.Fix(&q.heap, entry.index)
		q.wakeEarlierHeadLocked(previous, hadHead)
		return
	}

	previous, hadHead := q.headTimeLocked()
	entry := &scheduledTaskEntry{task: task, keepSchedule: !fromPlanner}
	q.pushLocked(entry)
	q.entries[key] = entry
	q.wakeEarlierHeadLocked(previous, hadHead)
}

func (q *TaskQueue) availabilityPlanRevision() uint64 {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.availabilityScheduleRevision
}

// refreshAvailabilitySchedule applies one successfully read local roster. An
// unavailable roster must not be sent here as an empty set. The caller
// serializes settings refreshes without holding q.mu during catalog reads.
func (q *TaskQueue) refreshAvailabilitySchedule(localTasks map[string]ScheduledTask) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.availabilityScheduleRevision++
	for key, entry := range q.entries {
		if entry.task.InstanceType == InstanceTypeAvailability {
			if _, keep := localTasks[entry.task.InstanceName]; !keep {
				q.removeLocked(key, entry)
			}
		}
	}
	for _, task := range localTasks {
		q.upsertLocked(task, false)
	}
}

// completeClaim returns ownership and optionally queues its successor in the
// same critical section. An explicit edit or removal made during the claim
// takes precedence over the returning worker's normal schedule.
func (q *TaskQueue) completeClaim(task, next ScheduledTask, reschedule pollReschedule) {
	key := taskQueueKey{task.InstanceType, task.InstanceName}
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, ok := q.entries[key]
	if !ok || entry.index >= 0 {
		return
	}
	switch entry.followup {
	case followExplicitSchedule:
		// Upsert has already stored the replacement in the claimed entry.
		entry.keepSchedule = true
	case followWorkerSchedule:
		if reschedule == noPollReschedule {
			delete(q.entries, key)
			return
		}
		entry.task = next
		entry.keepSchedule = reschedule == pollAfterBackoff
	case dropWorkerSchedule:
		delete(q.entries, key)
		return
	}
	entry.followup = followWorkerSchedule
	previous, hadHead := q.headTimeLocked()
	q.pushLocked(entry)
	q.wakeEarlierHeadLocked(previous, hadHead)
}

// Get returns the queued task for an instance, if present.
func (q *TaskQueue) Get(instanceType InstanceType, instance string) (ScheduledTask, bool) {
	key := taskQueueKey{instanceType, instance}
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, ok := q.entries[key]
	if !ok || entry.index < 0 {
		return ScheduledTask{}, false
	}
	return entry.task, true
}

// Remove deletes a queued task or cancels the successor of a claimed task.
func (q *TaskQueue) Remove(instanceType InstanceType, instance string) {
	key := taskQueueKey{instanceType, instance}
	q.mu.Lock()
	defer q.mu.Unlock()

	if entry, ok := q.entries[key]; ok {
		q.removeLocked(key, entry)
	}
}

func (q *TaskQueue) removeLocked(key taskQueueKey, entry *scheduledTaskEntry) {
	if entry.index < 0 {
		entry.followup = dropWorkerSchedule
		return
	}
	heap.Remove(&q.heap, entry.index)
	q.uncountLocked(entry.task.InstanceType)
	delete(q.entries, key)
}

func (q *TaskQueue) pushLocked(entry *scheduledTaskEntry) {
	heap.Push(&q.heap, entry)
	q.queuedByType[string(entry.task.InstanceType)]++
}

func (q *TaskQueue) uncountLocked(kind InstanceType) {
	key := string(kind)
	if remaining := q.queuedByType[key] - 1; remaining > 0 {
		q.queuedByType[key] = remaining
	} else {
		delete(q.queuedByType, key)
	}
}

func (q *TaskQueue) headTimeLocked() (time.Time, bool) {
	if len(q.heap) == 0 {
		return time.Time{}, false
	}
	return q.heap[0].task.NextRun, true
}

// wakeEarlierHeadLocked is called after a queued insertion or replacement.
// Changes that only delay work need no broadcast: waiters recheck the head
// before claiming anything, even if a previously armed timer wakes them.
func (q *TaskQueue) wakeEarlierHeadLocked(previous time.Time, hadHead bool) {
	if q.earlierHead != nil && (!hadHead || q.heap[0].task.NextRun.Before(previous)) {
		close(q.earlierHead)
		q.earlierHead = nil
	}
}

// WaitNext blocks until a task is due or context is cancelled. A successful
// caller owns the target until it calls completeClaim, even after a deadline.
func (q *TaskQueue) WaitNext(ctx context.Context) (ScheduledTask, bool) {
	return q.waitNext(ctx, &taskQueueWaiter{})
}

// A worker owns its waiter for the lifetime of its loop. The timer is
// allocated only when needed and stopped before returning to the worker.
type taskQueueWaiter struct {
	timer *time.Timer
}

func (w *taskQueueWaiter) stop() {
	if w.timer != nil {
		w.timer.Stop()
	}
}

func (q *TaskQueue) waitNext(ctx context.Context, waiter *taskQueueWaiter) (ScheduledTask, bool) {
	defer waiter.stop()
	done := ctx.Done()
	for ctx.Err() == nil {
		q.mu.Lock()
		next, hasHead := q.headTimeLocked()
		if hasHead && time.Until(next) <= 0 {
			task := heap.Pop(&q.heap).(*scheduledTaskEntry).task
			q.uncountLocked(task.InstanceType)
			q.mu.Unlock()
			return task, true
		}
		// Observe the deadline and its change signal under the same lock so
		// an insertion before this worker parks cannot lose its wakeup.
		if q.earlierHead == nil {
			q.earlierHead = make(chan struct{})
		}
		earlier := q.earlierHead
		q.mu.Unlock()

		var timerCh <-chan time.Time
		if hasHead {
			delay := time.Until(next)
			if waiter.timer == nil {
				waiter.timer = time.NewTimer(delay)
			} else {
				waiter.timer.Reset(delay)
			}
			timerCh = waiter.timer.C
		} else {
			waiter.stop()
		}
		select {
		case <-done:
			return ScheduledTask{}, false
		case <-earlier:
		case <-timerCh:
		}
	}
	return ScheduledTask{}, false
}

// Size returns the number of tasks currently queued.
func (q *TaskQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.heap)
}

// QueueSnapshot represents the current state of the task queue.
type QueueSnapshot struct {
	Depth                int            `json:"depth"`
	DueWithinSeconds     int            `json:"dueWithinSeconds"`
	PerType              map[string]int `json:"perType"`
	OldestOverdueSeconds float64        `json:"oldestOverdueSeconds"`
}

func emptyQueueSnapshot() QueueSnapshot {
	return QueueSnapshot{
		PerType: map[string]int{},
	}
}

// Snapshot returns a snapshot of the queue state for API exposure.
func (q *TaskQueue) Snapshot() QueueSnapshot {
	if q == nil {
		return emptyQueueSnapshot()
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	snapshot := q.snapshotTimingLocked()
	snapshot.PerType = maps.Clone(q.queuedByType)
	return snapshot
}

type queuedTypeCount struct {
	kind  string
	count int
}

// snapshotWithTypeCounts captures timing and counts under one lock, like
// Snapshot, but copies counts into caller-owned scratch space rather than
// cloning the per-type map. The returned QueueSnapshot has no PerType map.
func (q *TaskQueue) snapshotWithTypeCounts(dst []queuedTypeCount) (QueueSnapshot, []queuedTypeCount) {
	q.mu.Lock()
	defer q.mu.Unlock()

	snapshot := q.snapshotTimingLocked()
	dst = dst[:0]
	for kind, count := range q.queuedByType {
		dst = append(dst, queuedTypeCount{kind: kind, count: count})
	}
	return snapshot, dst
}

func (q *TaskQueue) snapshotTimingLocked() QueueSnapshot {
	now := time.Now()
	snapshot := QueueSnapshot{
		Depth:            len(q.heap),
		DueWithinSeconds: q.heap.countDue(0, now.Add(12*time.Second)),
	}
	if oldest := q.heap.oldestTimestamped(0); !oldest.IsZero() {
		snapshot.OldestOverdueSeconds = max(0, now.Sub(oldest)).Seconds()
	}

	return snapshot
}

// A subtree rooted after the cutoff contains no due tasks. Follow only the
// eligible part of the time-ordered heap while holding the worker lock.
func (h taskHeap) countDue(i int, cutoff time.Time) int {
	if i >= len(h) || h[i].task.NextRun.After(cutoff) {
		return 0
	}
	return 1 + h.countDue(2*i+1, cutoff) + h.countDue(2*i+2, cutoff)
}

func (h taskHeap) oldestTimestamped(i int) time.Time {
	if i >= len(h) {
		return time.Time{}
	}
	if next := h[i].task.NextRun; !next.IsZero() {
		return next // The root is the earliest task in this subtree.
	}
	// A zero deadline means immediately eligible, not a wait since year one.
	left := h.oldestTimestamped(2*i + 1)
	right := h.oldestTimestamped(2*i + 2)
	if left.IsZero() || (!right.IsZero() && right.Before(left)) {
		return right
	}
	return left
}

// DeadLetterTask represents a task in the dead-letter queue.
type DeadLetterTask struct {
	Instance  string    `json:"instance"`
	Type      string    `json:"type"`
	NextRun   time.Time `json:"nextRun"`
	LastError string    `json:"lastError,omitempty"`
	Failures  int       `json:"failures"`
}

type queuedTaskLabel struct {
	kind InstanceType
	name string
}

// snapshotLabels copies only heap membership for collectors that do not need
// full task records. The caller can reuse its scratch space across queues.
func (q *TaskQueue) snapshotLabels(dst []queuedTaskLabel) []queuedTaskLabel {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := len(q.heap)
	if cap(dst) < n {
		dst = make([]queuedTaskLabel, n)
	} else {
		dst = dst[:n]
	}
	for i, entry := range q.heap {
		dst[i] = queuedTaskLabel{kind: entry.task.InstanceType, name: entry.task.InstanceName}
	}
	return dst
}

// PeekAll returns up to 'limit' dead-letter tasks for inspection.
func (q *TaskQueue) PeekAll(limit int) []DeadLetterTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	if limit <= 0 || limit > len(q.heap) {
		limit = len(q.heap)
	}

	result := make([]DeadLetterTask, 0, limit)
	for i := 0; i < limit && i < len(q.heap); i++ {
		entry := q.heap[i]
		result = append(result, DeadLetterTask{
			Instance: entry.task.InstanceName,
			Type:     string(entry.task.InstanceType),
			NextRun:  entry.task.NextRun,
			Failures: 0, // will be populated by Monitor
		})
	}

	return result
}
