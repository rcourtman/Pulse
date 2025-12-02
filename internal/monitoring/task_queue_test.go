package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestTaskQueue_Snapshot(t *testing.T) {
	tests := []struct {
		name                 string
		tasks                []ScheduledTask
		wantDepth            int
		wantDueWithinSeconds int
		wantPerType          map[string]int
	}{
		{
			name:                 "empty queue",
			tasks:                []ScheduledTask{},
			wantDepth:            0,
			wantDueWithinSeconds: 0,
			wantPerType:          map[string]int{},
		},
		{
			name: "single task due soon (within 12 seconds)",
			tasks: []ScheduledTask{
				{
					InstanceName: "pve-1",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(5 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
			},
			wantDepth:            1,
			wantDueWithinSeconds: 1,
			wantPerType: map[string]int{
				"pve": 1,
			},
		},
		{
			name: "single task NOT due soon (>12 seconds away)",
			tasks: []ScheduledTask{
				{
					InstanceName: "pbs-1",
					InstanceType: InstanceTypePBS,
					NextRun:      time.Now().Add(20 * time.Second),
					Interval:     60 * time.Second,
					Priority:     1.0,
				},
			},
			wantDepth:            1,
			wantDueWithinSeconds: 0,
			wantPerType: map[string]int{
				"pbs": 1,
			},
		},
		{
			name: "multiple tasks of same type",
			tasks: []ScheduledTask{
				{
					InstanceName: "pve-1",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(5 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pve-2",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(15 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pve-3",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(10 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
			},
			wantDepth:            3,
			wantDueWithinSeconds: 2, // pve-1 and pve-3 are within 12 seconds
			wantPerType: map[string]int{
				"pve": 3,
			},
		},
		{
			name: "multiple tasks of different types",
			tasks: []ScheduledTask{
				{
					InstanceName: "pve-1",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(5 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pbs-1",
					InstanceType: InstanceTypePBS,
					NextRun:      time.Now().Add(8 * time.Second),
					Interval:     60 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pmg-1",
					InstanceType: InstanceTypePMG,
					NextRun:      time.Now().Add(25 * time.Second),
					Interval:     45 * time.Second,
					Priority:     1.0,
				},
			},
			wantDepth:            3,
			wantDueWithinSeconds: 2, // pve-1 and pbs-1 are within 12 seconds
			wantPerType: map[string]int{
				"pve": 1,
				"pbs": 1,
				"pmg": 1,
			},
		},
		{
			name: "boundary case: task exactly 12 seconds away",
			tasks: []ScheduledTask{
				{
					InstanceName: "pve-1",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(12 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
			},
			wantDepth:            1,
			wantDueWithinSeconds: 1, // <= 12 seconds should be included
			wantPerType: map[string]int{
				"pve": 1,
			},
		},
		{
			name: "mix of due-soon and not-due-soon tasks",
			tasks: []ScheduledTask{
				{
					InstanceName: "pve-1",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(1 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pve-2",
					InstanceType: InstanceTypePVE,
					NextRun:      time.Now().Add(50 * time.Second),
					Interval:     30 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pbs-1",
					InstanceType: InstanceTypePBS,
					NextRun:      time.Now().Add(11 * time.Second),
					Interval:     60 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pbs-2",
					InstanceType: InstanceTypePBS,
					NextRun:      time.Now().Add(100 * time.Second),
					Interval:     60 * time.Second,
					Priority:     1.0,
				},
				{
					InstanceName: "pmg-1",
					InstanceType: InstanceTypePMG,
					NextRun:      time.Now().Add(30 * time.Second),
					Interval:     45 * time.Second,
					Priority:     1.0,
				},
			},
			wantDepth:            5,
			wantDueWithinSeconds: 2, // pve-1 and pbs-1 are within 12 seconds
			wantPerType: map[string]int{
				"pve": 2,
				"pbs": 2,
				"pmg": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewTaskQueue()

			// Add all tasks to the queue
			for _, task := range tt.tasks {
				queue.Upsert(task)
			}

			// Get snapshot
			snapshot := queue.Snapshot()

			// Verify depth
			if snapshot.Depth != tt.wantDepth {
				t.Errorf("Depth = %d, want %d", snapshot.Depth, tt.wantDepth)
			}

			// Verify DueWithinSeconds
			if snapshot.DueWithinSeconds != tt.wantDueWithinSeconds {
				t.Errorf("DueWithinSeconds = %d, want %d", snapshot.DueWithinSeconds, tt.wantDueWithinSeconds)
			}

			// Verify PerType map
			if len(snapshot.PerType) != len(tt.wantPerType) {
				t.Errorf("PerType has %d entries, want %d", len(snapshot.PerType), len(tt.wantPerType))
			}

			for typeStr, wantCount := range tt.wantPerType {
				gotCount, ok := snapshot.PerType[typeStr]
				if !ok {
					t.Errorf("PerType missing entry for %s", typeStr)
					continue
				}
				if gotCount != wantCount {
					t.Errorf("PerType[%s] = %d, want %d", typeStr, gotCount, wantCount)
				}
			}

			// Verify no extra keys in PerType
			for typeStr := range snapshot.PerType {
				if _, ok := tt.wantPerType[typeStr]; !ok {
					t.Errorf("PerType has unexpected entry for %s", typeStr)
				}
			}
		})
	}
}

func TestTaskQueue_Upsert(t *testing.T) {
	t.Run("insert into empty queue", func(t *testing.T) {
		queue := NewTaskQueue()
		task := ScheduledTask{
			InstanceName: "pve-1",
			InstanceType: InstanceTypePVE,
			NextRun:      time.Now().Add(10 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}

		queue.Upsert(task)

		if queue.Size() != 1 {
			t.Errorf("Size() = %d, want 1", queue.Size())
		}
		verifyHeapInvariant(t, queue)
	})

	t.Run("upsert existing entry with different NextRun", func(t *testing.T) {
		queue := NewTaskQueue()
		task1 := ScheduledTask{
			InstanceName: "pve-1",
			InstanceType: InstanceTypePVE,
			NextRun:      time.Now().Add(10 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}
		queue.Upsert(task1)

		// Update same task with different NextRun
		task2 := ScheduledTask{
			InstanceName: "pve-1",
			InstanceType: InstanceTypePVE,
			NextRun:      time.Now().Add(20 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}
		queue.Upsert(task2)

		if queue.Size() != 1 {
			t.Errorf("Size() = %d, want 1 (not 2)", queue.Size())
		}

		// Verify the task was updated
		queue.mu.Lock()
		key := schedulerKey(InstanceTypePVE, "pve-1")
		entry := queue.entries[key]
		if !entry.task.NextRun.Equal(task2.NextRun) {
			t.Errorf("NextRun not updated: got %v, want %v", entry.task.NextRun, task2.NextRun)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("insert multiple entries - verify heap ordering", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(30 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(20 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		if queue.Size() != 3 {
			t.Errorf("Size() = %d, want 3", queue.Size())
		}

		// Verify heap root is earliest NextRun
		queue.mu.Lock()
		root := queue.heap[0]
		if root.task.InstanceName != "pve-1" {
			t.Errorf("heap root = %s, want pve-1 (earliest NextRun)", root.task.InstanceName)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("upsert changes heap position - earlier NextRun", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(20 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(30 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Update pve-3 to have earliest NextRun
		updatedTask := ScheduledTask{
			InstanceName: "pve-3",
			InstanceType: InstanceTypePVE,
			NextRun:      now.Add(5 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}
		queue.Upsert(updatedTask)

		if queue.Size() != 3 {
			t.Errorf("Size() = %d, want 3", queue.Size())
		}

		// Verify pve-3 is now at heap root
		queue.mu.Lock()
		root := queue.heap[0]
		if root.task.InstanceName != "pve-3" {
			t.Errorf("heap root = %s, want pve-3 (updated to earliest)", root.task.InstanceName)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("upsert changes heap position - later NextRun", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(20 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(30 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Update pve-1 to have latest NextRun
		updatedTask := ScheduledTask{
			InstanceName: "pve-1",
			InstanceType: InstanceTypePVE,
			NextRun:      now.Add(40 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}
		queue.Upsert(updatedTask)

		if queue.Size() != 3 {
			t.Errorf("Size() = %d, want 3", queue.Size())
		}

		// Verify pve-2 is now at heap root
		queue.mu.Lock()
		root := queue.heap[0]
		if root.task.InstanceName != "pve-2" {
			t.Errorf("heap root = %s, want pve-2 (pve-1 moved down)", root.task.InstanceName)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("upsert with priority ordering", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		// Same NextRun, different priorities
		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     0.5,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Higher priority should be at root
		queue.mu.Lock()
		root := queue.heap[0]
		if root.task.InstanceName != "pve-2" {
			t.Errorf("heap root = %s, want pve-2 (higher priority)", root.task.InstanceName)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("upsert with same time and priority - uses instance name for tiebreak", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()
		sameTime := now.Add(10 * time.Second)

		// Same NextRun, same priorities - should use InstanceName for tiebreak
		tasks := []ScheduledTask{
			{
				InstanceName: "pve-zebra",
				InstanceType: InstanceTypePVE,
				NextRun:      sameTime,
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-alpha",
				InstanceType: InstanceTypePVE,
				NextRun:      sameTime,
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-middle",
				InstanceType: InstanceTypePVE,
				NextRun:      sameTime,
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// When NextRun and Priority are equal, alphabetically earlier name should be first
		queue.mu.Lock()
		root := queue.heap[0]
		if root.task.InstanceName != "pve-alpha" {
			t.Errorf("heap root = %s, want pve-alpha (alphabetically first when time and priority equal)", root.task.InstanceName)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})
}

func TestTaskQueue_Remove(t *testing.T) {
	t.Run("remove from empty queue", func(t *testing.T) {
		queue := NewTaskQueue()

		// Should not panic
		queue.Remove(InstanceTypePVE, "pve-1")

		if queue.Size() != 0 {
			t.Errorf("Size() = %d, want 0", queue.Size())
		}
	})

	t.Run("remove non-existent key", func(t *testing.T) {
		queue := NewTaskQueue()
		task := ScheduledTask{
			InstanceName: "pve-1",
			InstanceType: InstanceTypePVE,
			NextRun:      time.Now().Add(10 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}
		queue.Upsert(task)

		// Remove different instance
		queue.Remove(InstanceTypePVE, "pve-2")

		if queue.Size() != 1 {
			t.Errorf("Size() = %d, want 1 (pve-1 should still exist)", queue.Size())
		}

		verifyHeapInvariant(t, queue)
	})

	t.Run("remove only entry", func(t *testing.T) {
		queue := NewTaskQueue()
		task := ScheduledTask{
			InstanceName: "pve-1",
			InstanceType: InstanceTypePVE,
			NextRun:      time.Now().Add(10 * time.Second),
			Interval:     30 * time.Second,
			Priority:     1.0,
		}
		queue.Upsert(task)

		queue.Remove(InstanceTypePVE, "pve-1")

		if queue.Size() != 0 {
			t.Errorf("Size() = %d, want 0", queue.Size())
		}

		queue.mu.Lock()
		if len(queue.entries) != 0 {
			t.Errorf("entries map has %d items, want 0", len(queue.entries))
		}
		if len(queue.heap) != 0 {
			t.Errorf("heap has %d items, want 0", len(queue.heap))
		}
		queue.mu.Unlock()
	})

	t.Run("remove entry from middle of queue", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(20 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(30 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Remove middle entry
		queue.Remove(InstanceTypePVE, "pve-2")

		if queue.Size() != 2 {
			t.Errorf("Size() = %d, want 2", queue.Size())
		}

		// Verify pve-2 is not in entries map
		queue.mu.Lock()
		key := schedulerKey(InstanceTypePVE, "pve-2")
		if _, exists := queue.entries[key]; exists {
			t.Errorf("pve-2 still exists in entries map")
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("remove heap root", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(20 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(30 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Remove root (pve-1 has earliest NextRun)
		queue.Remove(InstanceTypePVE, "pve-1")

		if queue.Size() != 2 {
			t.Errorf("Size() = %d, want 2", queue.Size())
		}

		// Verify pve-2 is now the root
		queue.mu.Lock()
		root := queue.heap[0]
		if root.task.InstanceName != "pve-2" {
			t.Errorf("heap root = %s, want pve-2 (next earliest)", root.task.InstanceName)
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})

	t.Run("remove all entries sequentially", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(20 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(30 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Remove each entry and verify invariant after each removal
		queue.Remove(InstanceTypePVE, "pve-2")
		if queue.Size() != 2 {
			t.Errorf("Size after removing pve-2 = %d, want 2", queue.Size())
		}
		verifyHeapInvariant(t, queue)

		queue.Remove(InstanceTypePVE, "pve-1")
		if queue.Size() != 1 {
			t.Errorf("Size after removing pve-1 = %d, want 1", queue.Size())
		}
		verifyHeapInvariant(t, queue)

		queue.Remove(InstanceTypePVE, "pve-3")
		if queue.Size() != 0 {
			t.Errorf("Size after removing pve-3 = %d, want 0", queue.Size())
		}
		verifyHeapInvariant(t, queue)
	})

	t.Run("remove different instance types", func(t *testing.T) {
		queue := NewTaskQueue()
		now := time.Now()

		tasks := []ScheduledTask{
			{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      now.Add(10 * time.Second),
				Interval:     30 * time.Second,
				Priority:     1.0,
			},
			{
				InstanceName: "pbs-1",
				InstanceType: InstanceTypePBS,
				NextRun:      now.Add(20 * time.Second),
				Interval:     60 * time.Second,
				Priority:     1.0,
			},
		}

		for _, task := range tasks {
			queue.Upsert(task)
		}

		// Remove PBS instance
		queue.Remove(InstanceTypePBS, "pbs-1")

		if queue.Size() != 1 {
			t.Errorf("Size() = %d, want 1", queue.Size())
		}

		// Verify PVE instance still exists
		queue.mu.Lock()
		key := schedulerKey(InstanceTypePVE, "pve-1")
		if _, exists := queue.entries[key]; !exists {
			t.Errorf("pve-1 should still exist in entries map")
		}
		queue.mu.Unlock()

		verifyHeapInvariant(t, queue)
	})
}

func TestTaskHeap_Pop(t *testing.T) {
	t.Run("pop from empty heap returns nil", func(t *testing.T) {
		h := &taskHeap{}

		result := h.Pop()

		if result != nil {
			t.Errorf("Pop() = %v, want nil", result)
		}
	})

	t.Run("pop from single-element heap returns that element and empties heap", func(t *testing.T) {
		entry := &scheduledTaskEntry{
			task: ScheduledTask{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      time.Now(),
				Priority:     1.0,
			},
			index: 0,
		}
		h := &taskHeap{entry}

		result := h.Pop()

		if result != entry {
			t.Errorf("Pop() returned wrong entry")
		}
		if entry.index != -1 {
			t.Errorf("entry.index = %d, want -1", entry.index)
		}
		if len(*h) != 0 {
			t.Errorf("heap length = %d, want 0", len(*h))
		}
	})

	t.Run("pop from multi-element heap returns last element with index set to -1", func(t *testing.T) {
		entry1 := &scheduledTaskEntry{
			task: ScheduledTask{
				InstanceName: "pve-1",
				InstanceType: InstanceTypePVE,
				NextRun:      time.Now(),
				Priority:     1.0,
			},
			index: 0,
		}
		entry2 := &scheduledTaskEntry{
			task: ScheduledTask{
				InstanceName: "pve-2",
				InstanceType: InstanceTypePVE,
				NextRun:      time.Now().Add(10 * time.Second),
				Priority:     1.0,
			},
			index: 1,
		}
		entry3 := &scheduledTaskEntry{
			task: ScheduledTask{
				InstanceName: "pve-3",
				InstanceType: InstanceTypePVE,
				NextRun:      time.Now().Add(20 * time.Second),
				Priority:     1.0,
			},
			index: 2,
		}
		h := &taskHeap{entry1, entry2, entry3}

		result := h.Pop()

		// Pop returns the last element in the slice (entry3)
		if result != entry3 {
			t.Errorf("Pop() returned wrong entry, got %v want entry3", result)
		}
		if entry3.index != -1 {
			t.Errorf("entry3.index = %d, want -1", entry3.index)
		}
		if len(*h) != 2 {
			t.Errorf("heap length = %d, want 2", len(*h))
		}
		// entry1 and entry2 should still be in the heap
		if (*h)[0] != entry1 || (*h)[1] != entry2 {
			t.Errorf("remaining heap entries are wrong")
		}
	})
}

// verifyHeapInvariant checks that the heap maintains its invariants:
// 1. len(entries) matches heap size
// 2. Each entry's index matches its actual position in heap
// 3. Heap property: parent is less than or equal to children
func verifyHeapInvariant(t *testing.T, queue *TaskQueue) {
	t.Helper()
	queue.mu.Lock()
	defer queue.mu.Unlock()

	// Check entries count matches heap size
	if len(queue.entries) != len(queue.heap) {
		t.Errorf("entries count %d != heap size %d", len(queue.entries), len(queue.heap))
	}

	// Check each entry's index is correct
	for _, entry := range queue.heap {
		if entry.index < 0 || entry.index >= len(queue.heap) {
			t.Errorf("entry %s has invalid index %d (heap size: %d)", entry.key(), entry.index, len(queue.heap))
			continue
		}
		if queue.heap[entry.index] != entry {
			t.Errorf("entry %s has index %d but is not at that position in heap", entry.key(), entry.index)
		}
	}

	// Check all entries in map are also in heap
	for key, entry := range queue.entries {
		found := false
		for _, heapEntry := range queue.heap {
			if heapEntry == entry {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("entry %s in map but not in heap", key)
		}
	}

	// Check heap property: parent <= children
	for i := 0; i < len(queue.heap); i++ {
		leftChild := 2*i + 1
		rightChild := 2*i + 2

		if leftChild < len(queue.heap) {
			if queue.heap.Less(leftChild, i) {
				t.Errorf("heap violation: child at %d is less than parent at %d", leftChild, i)
			}
		}

		if rightChild < len(queue.heap) {
			if queue.heap.Less(rightChild, i) {
				t.Errorf("heap violation: child at %d is less than parent at %d", rightChild, i)
			}
		}
	}
}

// TestScheduledTaskEntry_Key tests the key() method on scheduledTaskEntry
func TestScheduledTaskEntry_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		instType InstanceType
		instName string
		wantKey  string
	}{
		{
			name:     "PVE instance",
			instType: InstanceTypePVE,
			instName: "pve1",
			wantKey:  "pve::pve1",
		},
		{
			name:     "PBS instance",
			instType: InstanceTypePBS,
			instName: "pbs-cluster",
			wantKey:  "pbs::pbs-cluster",
		},
		{
			name:     "PMG instance",
			instType: InstanceTypePMG,
			instName: "pmg-01",
			wantKey:  "pmg::pmg-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &scheduledTaskEntry{
				task: ScheduledTask{
					InstanceType: tt.instType,
					InstanceName: tt.instName,
				},
			}

			got := entry.key()
			if got != tt.wantKey {
				t.Errorf("key() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

// TestWaitNext_ContextCancelled tests WaitNext returns when context is cancelled
func TestWaitNext_ContextCancelled(t *testing.T) {
	t.Parallel()

	queue := NewTaskQueue()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task, ok := queue.WaitNext(ctx)
	if ok {
		t.Error("WaitNext should return false when context is cancelled")
	}
	if task.InstanceName != "" {
		t.Error("WaitNext should return empty task when context is cancelled")
	}
}

// TestWaitNext_ImmediatelyDue tests WaitNext returns immediately for due tasks
func TestWaitNext_ImmediatelyDue(t *testing.T) {
	t.Parallel()

	queue := NewTaskQueue()

	// Add a task that's already due (NextRun in the past)
	queue.Upsert(ScheduledTask{
		InstanceName: "pve1",
		InstanceType: InstanceTypePVE,
		NextRun:      time.Now().Add(-1 * time.Second),
		Interval:     30 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	task, ok := queue.WaitNext(ctx)
	if !ok {
		t.Fatal("WaitNext should return true for immediately due task")
	}
	if task.InstanceName != "pve1" {
		t.Errorf("expected task pve1, got %s", task.InstanceName)
	}

	// Task should be removed from queue
	if queue.Size() != 0 {
		t.Errorf("queue size should be 0 after WaitNext, got %d", queue.Size())
	}
}

// TestWaitNext_WaitsForDue tests WaitNext waits until task is due
func TestWaitNext_WaitsForDue(t *testing.T) {
	t.Parallel()

	queue := NewTaskQueue()

	// Add a task due 200ms in the future
	dueTime := time.Now().Add(200 * time.Millisecond)
	queue.Upsert(ScheduledTask{
		InstanceName: "pve1",
		InstanceType: InstanceTypePVE,
		NextRun:      dueTime,
		Interval:     30 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	task, ok := queue.WaitNext(ctx)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("WaitNext should return true")
	}
	if task.InstanceName != "pve1" {
		t.Errorf("expected task pve1, got %s", task.InstanceName)
	}

	// Should have waited at least ~150ms (allowing some tolerance)
	if elapsed < 150*time.Millisecond {
		t.Errorf("WaitNext returned too early: elapsed %v, expected >= 150ms", elapsed)
	}
}

// TestWaitNext_EmptyQueueContextCancel tests WaitNext with empty queue and context cancel
func TestWaitNext_EmptyQueueContextCancel(t *testing.T) {
	t.Parallel()

	queue := NewTaskQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	task, ok := queue.WaitNext(ctx)
	elapsed := time.Since(start)

	if ok {
		t.Error("WaitNext should return false when context times out on empty queue")
	}
	if task.InstanceName != "" {
		t.Error("WaitNext should return empty task")
	}

	// Should have waited close to the timeout
	if elapsed < 150*time.Millisecond {
		t.Errorf("WaitNext returned too early: elapsed %v", elapsed)
	}
}

// TestWaitNext_MultipleTasks tests WaitNext returns earliest due task
func TestWaitNext_MultipleTasks(t *testing.T) {
	t.Parallel()

	queue := NewTaskQueue()
	now := time.Now()

	// Add tasks with different due times
	queue.Upsert(ScheduledTask{
		InstanceName: "pve3",
		InstanceType: InstanceTypePVE,
		NextRun:      now.Add(300 * time.Millisecond),
	})
	queue.Upsert(ScheduledTask{
		InstanceName: "pve1",
		InstanceType: InstanceTypePVE,
		NextRun:      now.Add(-10 * time.Millisecond), // Already due
	})
	queue.Upsert(ScheduledTask{
		InstanceName: "pve2",
		InstanceType: InstanceTypePVE,
		NextRun:      now.Add(200 * time.Millisecond),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should return pve1 first (earliest due)
	task, ok := queue.WaitNext(ctx)
	if !ok {
		t.Fatal("WaitNext should return true")
	}
	if task.InstanceName != "pve1" {
		t.Errorf("expected pve1 (earliest), got %s", task.InstanceName)
	}

	// Queue should still have 2 tasks
	if queue.Size() != 2 {
		t.Errorf("queue size should be 2, got %d", queue.Size())
	}
}
