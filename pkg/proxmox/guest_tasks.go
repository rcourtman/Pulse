package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// guestTaskPageLimit is the most tasks read for one guest in one window. A
// window is minutes wide and one guest has a handful of tasks in it, so a
// full page means the window is not what the caller thinks it is.
const guestTaskPageLimit = 200

// Reasons a guest's task window could not be read in full.
const (
	GuestTasksCollectionFailed = "collection_failed"
	GuestTasksAccessDenied     = "access_denied"
	GuestTasksAuditUnconfirmed = "audit_permission_unconfirmed"
	GuestTasksTruncated        = "history_truncated"
)

// GuestTaskWindow is what a node's task log holds for one guest between two
// times, with whether that is the whole of it.
//
// A node lists every task only to a caller with Sys.Audit on /nodes/<node>.
// Any other caller gets its own tasks and a successful, shorter answer. So an
// empty window means "nothing happened" only when Complete is true.
type GuestTaskWindow struct {
	Tasks []Task
	// Complete is true when the request succeeded, was not cut off, and the
	// caller is confirmed to see other users' tasks on the node.
	Complete bool
	// Reason says why Complete is false.
	Reason string
}

// GetGuestTasks lists the tasks a node logged for one guest whose start time
// lies between since and until, running or finished. Proxmox filters the
// window on a task's start time, so a caller that cares when a task ended
// must start the window early enough.
func (c *Client) GetGuestTasks(ctx context.Context, node string, vmid int, since, until time.Time) (GuestTaskWindow, error) {
	node = strings.TrimSpace(node)
	if node == "" || vmid <= 0 {
		return GuestTaskWindow{Reason: GuestTasksCollectionFailed}, fmt.Errorf("guest tasks need a node and a vmid")
	}
	params := url.Values{
		"vmid":   {strconv.Itoa(vmid)},
		"source": {"all"},
		"limit":  {strconv.Itoa(guestTaskPageLimit)},
		"since":  {strconv.FormatInt(since.Unix(), 10)},
		"until":  {strconv.FormatInt(until.Unix(), 10)},
	}
	resp, err := c.get(ctx, "/nodes/"+url.PathEscape(node)+"/tasks?"+params.Encode())
	if err != nil {
		window := GuestTaskWindow{Reason: GuestTasksCollectionFailed}
		if status, known := APIErrorStatus(err); known && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
			window.Reason = GuestTasksAccessDenied
		}
		return window, err
	}
	tasks, err := readBackupTaskInventory(resp, node)
	if err != nil {
		return GuestTaskWindow{Reason: GuestTasksCollectionFailed}, err
	}
	window := GuestTaskWindow{Tasks: make([]Task, 0, len(tasks))}
	id := strconv.Itoa(vmid)
	for _, task := range tasks {
		// The node filters on the guest already. A row for another guest
		// would mean the filter was ignored, and is never this guest's.
		if strings.TrimSpace(task.ID) != id {
			continue
		}
		if task.Node == "" {
			task.Node = node
		}
		window.Tasks = append(window.Tasks, task)
	}
	switch {
	case len(tasks) >= guestTaskPageLimit:
		window.Reason = GuestTasksTruncated
	case !c.backupHistoryAuditPermission(ctx, node):
		window.Reason = GuestTasksAuditUnconfirmed
	default:
		window.Complete = true
	}
	return window, nil
}

// GetGuestTasks reads one guest's task window through any reachable cluster
// member. Each member proxies the request to the node that ran the tasks.
func (cc *ClusterClient) GetGuestTasks(ctx context.Context, node string, vmid int, since, until time.Time) (GuestTaskWindow, error) {
	var result GuestTaskWindow
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		var err error
		result, err = client.GetGuestTasks(ctx, node, vmid, since, until)
		return err
	})
	return result, err
}
