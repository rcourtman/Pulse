package models

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBackupTaskReplacementRetainsUnreadNodeReceipts(t *testing.T) {
	s := NewState()
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	a := BackupTask{ID: "a-old", Instance: "site", Node: "a", VMID: 100, Status: "error", ObservedAt: now, StartTime: now}
	b := BackupTask{ID: "b", Instance: "site", Node: "b", VMID: 200, Status: "warning", ObservedAt: now, StartTime: now}
	unknown := BackupTask{ID: "unscoped", Instance: "site", VMID: 300, Status: "running", ObservedAt: now, StartTime: now}
	other := BackupTask{ID: "other", Instance: "other", Node: "b", VMID: 200, Status: "error", ObservedAt: now, StartTime: now}
	s.UpdateBackupTasksForInstance("site", []BackupTask{a, b, unknown})
	s.UpdateBackupTasksForInstance("other", []BackupTask{other})
	next := a
	next.ID = "a-next"
	next.Status = "OK"
	next.ObservedAt = now.Add(time.Minute)
	unread := map[string]bool{"b": true, "": true}
	s.UpdateBackupTasksForInstanceWithUnreadNodes("site", []BackupTask{next}, unread)
	read := func() map[string]BackupTask {
		tasks := map[string]BackupTask{}
		for _, task := range s.GetBackupTasks() {
			tasks[task.ID] = task
		}
		return tasks
	}
	require.Equal(t, map[string]BackupTask{next.ID: next, b.ID: b, unknown.ID: unknown, other.ID: other}, read())
	s.UpdateBackupTasksForInstanceWithUnreadNodes("site", nil, unread)
	require.Equal(t, map[string]BackupTask{b.ID: b, unknown.ID: unknown, other.ID: other}, read(), "complete empty A cannot withdraw B or refresh its receipt")
	s.UpdateBackupTasksForInstance("site", nil)
	require.Equal(t, map[string]BackupTask{other.ID: other}, read(), "a complete instance replacement can withdraw all old node scopes")
}
