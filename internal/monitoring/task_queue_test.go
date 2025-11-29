package monitoring

import (
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
