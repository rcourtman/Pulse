package monitoring

import (
	"context"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// ProxmoxGuestTask is one entry of a node's task log for a guest.
type ProxmoxGuestTask struct {
	Type      string
	User      string
	Status    string
	Node      string
	StartedAt time.Time
	// EndedAt is zero while the task is still running.
	EndedAt time.Time
}

// ProxmoxGuestTasks is what a node's task log holds for one guest in a window
// of time, read through Pulse's own Proxmox API connection.
type ProxmoxGuestTasks struct {
	Tasks []ProxmoxGuestTask
	// Complete is false when the read failed, was cut off, or Pulse's token is
	// not confirmed to see other users' tasks on the node. An empty Tasks
	// then says nothing about what happened to the guest.
	Complete bool
	// Reason says why Complete is false: one of the proxmox.GuestTasks*
	// reasons, or ProxmoxGuestTasksNoConnection.
	Reason string
}

// ProxmoxGuestTasksNoConnection is the reason when Pulse has no Proxmox API
// connection that can read a node's task log for the guest.
const ProxmoxGuestTasksNoConnection = "no_connection"

type proxmoxGuestTaskClient interface {
	GetGuestTasks(ctx context.Context, node string, vmid int, since, until time.Time) (proxmox.GuestTaskWindow, error)
}

// ReadProxmoxGuestTasks reads the tasks a node logged for one guest that
// started between since and until. It reads live through the configured
// Proxmox API connection and never from cached state or an agent, so the
// answer is Proxmox's own account of who asked the guest to do what.
func (m *Monitor) ReadProxmoxGuestTasks(ctx context.Context, instance, node string, vmid int, since, until time.Time) ProxmoxGuestTasks {
	instance = strings.TrimSpace(instance)
	node = strings.TrimSpace(node)
	if m == nil || instance == "" || node == "" || vmid <= 0 {
		return ProxmoxGuestTasks{Reason: ProxmoxGuestTasksNoConnection}
	}
	client, ok := m.getPVEClient(instance)
	if !ok || client == nil {
		return ProxmoxGuestTasks{Reason: ProxmoxGuestTasksNoConnection}
	}
	reader, ok := client.(proxmoxGuestTaskClient)
	if !ok {
		return ProxmoxGuestTasks{Reason: ProxmoxGuestTasksNoConnection}
	}
	window, err := reader.GetGuestTasks(ctx, node, vmid, since, until)
	read := ProxmoxGuestTasks{Complete: err == nil && window.Complete, Reason: window.Reason}
	if err != nil && read.Reason == "" {
		read.Reason = proxmox.GuestTasksCollectionFailed
	}
	for _, task := range window.Tasks {
		entry := ProxmoxGuestTask{
			Type:      strings.TrimSpace(task.Type),
			User:      strings.TrimSpace(task.User),
			Status:    strings.TrimSpace(task.Status),
			Node:      strings.TrimSpace(task.Node),
			StartedAt: time.Unix(task.StartTime, 0).UTC(),
		}
		if task.EndTime > 0 {
			entry.EndedAt = time.Unix(task.EndTime, 0).UTC()
		}
		read.Tasks = append(read.Tasks, entry)
	}
	return read
}
