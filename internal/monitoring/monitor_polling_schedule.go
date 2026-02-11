package monitoring

import (
	"sort"
	"time"
)

func (m *Monitor) describeInstancesForScheduler() []InstanceDescriptor {
	total := len(m.pveClients) + len(m.pbsClients) + len(m.pmgClients)
	if total == 0 {
		return nil
	}

	descriptors := make([]InstanceDescriptor, 0, total)

	if len(m.pveClients) > 0 {
		names := make([]string, 0, len(m.pveClients))
		for name := range m.pveClients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: InstanceTypePVE,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(InstanceTypePVE, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(InstanceTypePVE, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = TaskMetadata{ChangeHash: snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	if len(m.pbsClients) > 0 {
		names := make([]string, 0, len(m.pbsClients))
		for name := range m.pbsClients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: InstanceTypePBS,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(InstanceTypePBS, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(InstanceTypePBS, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = TaskMetadata{ChangeHash: snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	if len(m.pmgClients) > 0 {
		names := make([]string, 0, len(m.pmgClients))
		for name := range m.pmgClients {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: InstanceTypePMG,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(InstanceTypePMG, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(InstanceTypePMG, name); ok {
					desc.LastSuccess = snap.LastSuccess
					desc.LastFailure = snap.LastError
					desc.Metadata = TaskMetadata{ChangeHash: snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	return descriptors
}

func (m *Monitor) buildScheduledTasks(now time.Time) []ScheduledTask {
	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) == 0 {
		return nil
	}

	queueDepth := 0
	if m.taskQueue != nil {
		queueDepth = m.taskQueue.Size()
	}

	if m.scheduler == nil {
		tasks := make([]ScheduledTask, 0, len(descriptors))
		for _, desc := range descriptors {
			interval := m.baseIntervalForInstanceType(desc.Type)
			if interval <= 0 {
				interval = DefaultSchedulerConfig().BaseInterval
			}
			tasks = append(tasks, ScheduledTask{
				InstanceName: desc.Name,
				InstanceType: desc.Type,
				NextRun:      now,
				Interval:     interval,
			})
		}
		return tasks
	}

	return m.scheduler.BuildPlan(now, descriptors, queueDepth)
}
