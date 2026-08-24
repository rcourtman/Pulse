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
		fixedProvider, _ := provider.(FixedIntervalPollProvider)
		for _, name := range names {
			desc := InstanceDescriptor{
				Name: name,
				Type: providerType,
			}
			if fixedProvider != nil {
				desc.FixedInterval = fixedProvider.FixedInstanceInterval(m, name)
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
			interval := desc.FixedInterval
			if interval <= 0 {
				interval = m.baseIntervalForInstanceType(desc.Type)
			}
			if interval <= 0 {
				interval = DefaultSchedulerConfig().BaseInterval
			}
			// Planning passes run every poll tick, and Upsert overwrites the
			// queued slot. Stamping NextRun=now here re-arms every instance as
			// due immediately each tick, so a fixed-interval instance polls at
			// the tick cadence instead of its configured interval (#1745).
			// Keep a pending future slot, tightening only when the freshly
			// computed interval justifies an earlier run, mirroring the
			// adaptive path's #1437 handling.
			nextRun := now
			if m.taskQueue != nil {
				if pending, ok := m.taskQueue.Get(desc.Type, desc.Name); ok && pending.NextRun.After(now) {
					nextRun = pending.NextRun
					if candidate := now.Add(interval); candidate.Before(nextRun) {
						nextRun = candidate
					}
				}
			}
			tasks = append(tasks, ScheduledTask{
				InstanceName: desc.Name,
				InstanceType: desc.Type,
				NextRun:      nextRun,
				Interval:     interval,
			})
		}
		return tasks
	}

	return m.scheduler.BuildPlan(now, descriptors, queueDepth)
}
