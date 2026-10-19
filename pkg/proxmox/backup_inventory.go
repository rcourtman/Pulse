package proxmox

import (
	"context"
	"fmt"
	"net/url"
)

// BackupTaskInventory is a bounded live view, not durable-history coverage.
// Successful reads without confirmed Sys.Audit may include only the caller's
// own tasks. A full recent page also cannot rule out an older active writer.
// UnconfirmedNodes carry useful positive rows for the existing bounded live
// view, but cannot qualify new archive output as completed. Offline nodes are
// explicitly unread, preserving their previous live observations verbatim.
// A node list with no visible nodes establishes no task replacement authority.
type BackupTaskInventory struct {
	Tasks            []Task
	OfflineNodes     []string
	UnconfirmedNodes []string
}

func (c *Client) GetBackupTaskInventory(ctx context.Context) (BackupTaskInventory, error) {
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return BackupTaskInventory{}, err
	}
	if len(nodes) == 0 {
		return BackupTaskInventory{}, fmt.Errorf("Proxmox backup task inventory has no observed nodes")
	}
	var inventory BackupTaskInventory
	for _, node := range nodes {
		if node.Status != "online" {
			inventory.OfflineNodes = append(inventory.OfflineNodes, node.Node)
			continue
		}
		tasks, complete, err := c.backupTasksForNode(ctx, node.Node, "all")
		if err != nil {
			return BackupTaskInventory{}, err
		}
		if !complete {
			// Keep the ordinary bounded recent view, but also read active
			// tasks when it fills. Do not scan arbitrarily far into history
			// or permanently stall busy nodes that always have 200 tasks.
			active, activeComplete, activeErr := c.backupTasksForNode(ctx, node.Node, "active")
			if activeErr == nil {
				seen := make(map[string]bool, len(active))
				for _, task := range active {
					seen[task.UPID] = true
				}
				for _, task := range tasks {
					if !seen[task.UPID] {
						active = append(active, task)
					}
				}
				tasks = active // Prioritise current writers in the live UI cap.
			}
			complete = activeErr == nil && activeComplete
		}
		if !complete || !c.backupHistoryAuditPermission(ctx, node.Node) {
			inventory.UnconfirmedNodes = append(inventory.UnconfirmedNodes, node.Node)
		}
		inventory.Tasks = append(inventory.Tasks, tasks...)
	}
	return inventory, nil
}

func (c *Client) backupTasksForNode(ctx context.Context, node, source string) ([]Task, bool, error) {
	params := url.Values{"typefilter": {"vzdump"}, "source": {source}, "limit": {fmt.Sprint(backupTaskListLimit)}}
	resp, err := c.get(ctx, "/nodes/"+url.PathEscape(node)+"/tasks?"+params.Encode())
	if err != nil {
		return nil, false, fmt.Errorf("get backup tasks: %w", err)
	}
	tasks, err := readTasks(resp)
	if err != nil {
		return nil, false, fmt.Errorf("read backup tasks: %w", err)
	}
	complete := len(tasks) < backupTaskListLimit
	var backups []Task
	for _, task := range tasks {
		if task.Type != "vzdump" {
			continue
		}
		// PVE may omit the redundant node field, not supply a different node.
		if task.Node != "" && task.Node != node {
			return nil, false, fmt.Errorf("Proxmox backup task node does not match its collection scope")
		}
		task.Node = node
		backups = append(backups, task)
	}
	return backups, complete, nil
}

func (cc *ClusterClient) GetBackupTaskInventory(ctx context.Context) (BackupTaskInventory, error) {
	var result BackupTaskInventory
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		inventory, err := client.GetBackupTaskInventory(ctx)
		if err != nil {
			return err
		}
		result = inventory
		return nil
	})
	return result, err
}
