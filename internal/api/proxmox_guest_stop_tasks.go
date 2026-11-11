package api

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// newProxmoxGuestTaskReader gives Patrol the node task log for one guest, read
// live through the organization's own Proxmox API connection. Patrol uses it
// to tell a guest somebody stopped through Proxmox from one that went down by
// itself. With no monitor the log is unread, which is not the same as empty.
func newProxmoxGuestTaskReader(resolveMonitor func() *monitoring.Monitor) ai.ProxmoxGuestTaskReader {
	return func(ctx context.Context, instance, node string, vmid int, since, until time.Time) ai.ProxmoxGuestTaskRead {
		var monitor *monitoring.Monitor
		if resolveMonitor != nil {
			monitor = resolveMonitor()
		}
		read := monitor.ReadProxmoxGuestTasks(ctx, instance, node, vmid, since, until)
		result := ai.ProxmoxGuestTaskRead{Complete: read.Complete, Reason: read.Reason}
		for _, task := range read.Tasks {
			result.Tasks = append(result.Tasks, ai.ProxmoxGuestTask{
				Type: task.Type, User: task.User, Status: task.Status, StartedAt: task.StartedAt, EndedAt: task.EndedAt,
			})
		}
		return result
	}
}
