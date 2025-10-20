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
	for {
		select {
		case <-ctx.Done():
			return ScheduledTask{}, false
		default:
		}

		q.mu.Lock()
		if len(q.heap) == 0 {
			q.mu.Unlock()
			select {
			case <-ctx.Done():
				return ScheduledTask{}, false
			case <-time.After(100 * time.Millisecond):
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
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ScheduledTask{}, false
		case <-timer.C:
		}
	}
}

// Size returns the number of tasks currently queued.
func (q *TaskQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.heap)
}
