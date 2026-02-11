package monitoring

import (
	"sort"
	"time"
)

func (m *Monitor) describeInstancesForScheduler() []InstanceDescriptor {
	m.mu.RLock()
	pveNames := make([]string, 0, len(m.pveClients))
	for name := range m.pveClients {
		pveNames = append(pveNames, name)
	}

	pbsNames := make([]string, 0, len(m.pbsClients))
	for name := range m.pbsClients {
		pbsNames = append(pbsNames, name)
	}

	pmgNames := make([]string, 0, len(m.pmgClients))
	for name := range m.pmgClients {
		pmgNames = append(pmgNames, name)
	}
	m.mu.RUnlock()

	total := len(pveNames) + len(pbsNames) + len(pmgNames)
	if total == 0 {
		return nil
	}

	descriptors := make([]InstanceDescriptor, 0, total)

	if len(pveNames) > 0 {
		sort.Strings(pveNames)
		for _, name := range pveNames {
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
					desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	if len(pbsNames) > 0 {
		sort.Strings(pbsNames)
		for _, name := range pbsNames {
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
					desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
				}
			}
			descriptors = append(descriptors, desc)
		}
	}

	if len(pmgNames) > 0 {
		sort.Strings(pmgNames)
		for _, name := range pmgNames {
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
					desc.Metadata = map[string]any{"changeHash": snap.ChangeHash}
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
