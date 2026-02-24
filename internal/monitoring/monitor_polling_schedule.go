package monitoring

import (
	"sort"
	"time"
)

func (m *Monitor) describeInstancesForScheduler() []InstanceDescriptor {
	providers := m.pollProviderSnapshotWithBuiltins()
	if len(providers) == 0 {
		return nil
	}

	total := 0
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		total += len(provider.ListInstances(m))
	}
	if total == 0 {
		return nil
	}

	descriptors := make([]InstanceDescriptor, 0, total)
	for _, provider := range providers {
		if provider == nil {
			continue
		}

		names := append([]string(nil), provider.ListInstances(m)...)
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)

		providerType := provider.Type()
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: providerType,
			}
			if m.scheduler != nil {
				if last, ok := m.scheduler.LastScheduled(providerType, name); ok {
					desc.LastScheduled = last.NextRun
					desc.LastInterval = last.Interval
				}
			}
			if m.stalenessTracker != nil {
				if snap, ok := m.stalenessTracker.snapshot(providerType, name); ok {
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
