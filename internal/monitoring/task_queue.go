package monitoring

import (
	"container/heap"
	"context"
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

func (e *scheduledTaskEntry) key() string {
	return schedulerKey(e.task.InstanceType, e.task.InstanceName)
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
	mu          sync.Mutex
	entries     map[string]*scheduledTaskEntry
	heap        taskHeap
	earlierHead chan struct{} // lazily shared by workers waiting for new eligibility
}

// NewTaskQueue constructs an empty queue.
func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		entries: make(map[string]*scheduledTaskEntry),
		heap:    make(taskHeap, 0),
	}
	heap.Init(&tq.heap)
	return tq
}

// Upsert schedules an explicit request. If a worker has already claimed the
// target, retain the latest request for when that worker returns.
func (q *TaskQueue) Upsert(task ScheduledTask) {
	q.upsert(task, false)
}

// upsertPlan adds or refreshes periodic work without replacing an explicit
// request, a retry, or the work the queue has already dispatched.
func (q *TaskQueue) upsertPlan(task ScheduledTask) {
	q.upsert(task, true)
}

func (q *TaskQueue) upsert(task ScheduledTask, fromPlanner bool) {
	key := schedulerKey(task.InstanceType, task.InstanceName)
	q.mu.Lock()
	defer q.mu.Unlock()

	previous, hadHead := q.headTimeLocked()
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
		entry.task = task
		entry.keepSchedule = !fromPlanner
		heap.Fix(&q.heap, entry.index)
		q.wakeEarlierHeadLocked(previous, hadHead)
		return
	}

	entry := &scheduledTaskEntry{task: task, keepSchedule: !fromPlanner}
	heap.Push(&q.heap, entry)
	q.entries[key] = entry
	q.wakeEarlierHeadLocked(previous, hadHead)
}

// completeClaim returns ownership and optionally queues its successor in the
// same critical section. An explicit edit or removal made during the claim
// takes precedence over the returning worker's normal schedule.
func (q *TaskQueue) completeClaim(task, next ScheduledTask, reschedule pollReschedule) {
	key := schedulerKey(task.InstanceType, task.InstanceName)
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
	heap.Push(&q.heap, entry)
	q.wakeEarlierHeadLocked(previous, hadHead)
}

// Get returns the queued task for an instance, if present.
func (q *TaskQueue) Get(instanceType InstanceType, instance string) (ScheduledTask, bool) {
	key := schedulerKey(instanceType, instance)
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
	key := schedulerKey(instanceType, instance)
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, ok := q.entries[key]
	if !ok {
		return
	}
	if entry.index < 0 {
		entry.followup = dropWorkerSchedule
		return
	}
	heap.Remove(&q.heap, entry.index)
	delete(q.entries, key)
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
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	done := ctx.Done()
	for ctx.Err() == nil {
		q.mu.Lock()
		next, hasHead := q.headTimeLocked()
		if hasHead && time.Until(next) <= 0 {
			task := heap.Pop(&q.heap).(*scheduledTaskEntry).task
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
			if timer == nil {
				timer = time.NewTimer(delay)
			} else {
				timer.Reset(delay)
			}
			timerCh = timer.C
		} else if timer != nil {
			timer.Stop()
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

	snapshot := emptyQueueSnapshot()
	snapshot.Depth = len(q.heap)

	now := time.Now()
	var oldestOverdue time.Duration
	for _, entry := range q.heap {
		typeStr := string(entry.task.InstanceType)
		snapshot.PerType[typeStr]++

		lateness := now.Sub(entry.task.NextRun)
		if lateness >= -12*time.Second {
			snapshot.DueWithinSeconds++
		}
		// A zero deadline means immediately eligible, not a measured wait
		// since year one. Claimed work is not in the heap at all.
		if !entry.task.NextRun.IsZero() {
			oldestOverdue = max(oldestOverdue, lateness)
		}
	}
	snapshot.OldestOverdueSeconds = oldestOverdue.Seconds()

	return snapshot
}

// DeadLetterTask represents a task in the dead-letter queue.
type DeadLetterTask struct {
	Instance  string    `json:"instance"`
	Type      string    `json:"type"`
	NextRun   time.Time `json:"nextRun"`
	LastError string    `json:"lastError,omitempty"`
	Failures  int       `json:"failures"`
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
