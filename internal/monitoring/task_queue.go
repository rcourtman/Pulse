package monitoring

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

type scheduledTaskEntry struct {
	task  ScheduledTask
	index int
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
	entry.index = -1
	*h = old[:n-1]
	return entry
}

// TaskQueue is a thread-safe min-heap over scheduled tasks.
type TaskQueue struct {
	mu      sync.Mutex
	entries map[string]*scheduledTaskEntry
	heap    taskHeap
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

// Upsert inserts or updates a scheduled task in the queue.
func (q *TaskQueue) Upsert(task ScheduledTask) {
	key := schedulerKey(task.InstanceType, task.InstanceName)
	q.mu.Lock()
	defer q.mu.Unlock()

	if entry, ok := q.entries[key]; ok {
		entry.task = task
		heap.Fix(&q.heap, entry.index)
		return
	}

	entry := &scheduledTaskEntry{task: task}
	heap.Push(&q.heap, entry)
	q.entries[key] = entry
}

// Remove deletes a task by key if present.
func (q *TaskQueue) Remove(instanceType InstanceType, instance string) {
	key := schedulerKey(instanceType, instance)
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, ok := q.entries[key]
	if !ok {
		return
	}
	heap.Remove(&q.heap, entry.index)
	delete(q.entries, key)
}

// WaitNext blocks until a task is due or context is cancelled.
func (q *TaskQueue) WaitNext(ctx context.Context) (ScheduledTask, bool) {
	var (
		timer      *time.Timer
		resetTimer = func(d time.Duration) <-chan time.Time {
			if d <= 0 {
				d = time.Millisecond
			}
			if timer == nil {
				timer = time.NewTimer(d)
				return timer.C
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(d)
			return timer.C
		}
		stopTimer = func() {
			if timer == nil {
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
	)
	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			return ScheduledTask{}, false
		default:
		}

		q.mu.Lock()
		if len(q.heap) == 0 {
			q.mu.Unlock()
			timerCh := resetTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				return ScheduledTask{}, false
			case <-timerCh:
				continue
			}
		}

		entry := q.heap[0]
		delay := time.Until(entry.task.NextRun)
		if delay <= 0 {
			heap.Pop(&q.heap)
			delete(q.entries, entry.key())
			task := entry.task
			q.mu.Unlock()
			return task, true
		}

		q.mu.Unlock()
		if delay > 250*time.Millisecond {
			delay = 250 * time.Millisecond
		}
		timerCh := resetTimer(delay)
		select {
		case <-ctx.Done():
			return ScheduledTask{}, false
		case <-timerCh:
		}
	}
}

// Size returns the number of tasks currently queued.
func (q *TaskQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.heap)
}

// QueueSnapshot represents the current state of the task queue.
type QueueSnapshot struct {
	Depth            int            `json:"depth"`
	DueWithinSeconds int            `json:"dueWithinSeconds"`
	PerType          map[string]int `json:"perType"`
}

// Snapshot returns a snapshot of the queue state for API exposure.
func (q *TaskQueue) Snapshot() QueueSnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()

	snapshot := QueueSnapshot{
		Depth:   len(q.heap),
		PerType: make(map[string]int),
	}

	now := time.Now()
	for _, entry := range q.heap {
		typeStr := string(entry.task.InstanceType)
		snapshot.PerType[typeStr]++

		if entry.task.NextRun.Sub(now) <= 12*time.Second {
			snapshot.DueWithinSeconds++
		}
	}

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
