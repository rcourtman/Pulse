package proxmox

import (
	"context"
	"fmt"
	"net/url"
)

// BackupTaskInventory is an unwindowed live read, not durable-history coverage.
// Every online node must return a complete page. Offline nodes are explicitly
// unread, so a scoped consumer can retain their previous observations verbatim.
// A node list with no visible nodes establishes no task replacement authority.
type BackupTaskInventory struct {
	Tasks        []Task
	OfflineNodes []string
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
		resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/tasks?typefilter=vzdump&source=all&limit=%d", url.PathEscape(node.Node), backupTaskListLimit))
		if err != nil {
			return BackupTaskInventory{}, fmt.Errorf("get backup tasks: %w", err)
		}
		tasks, err := readTasks(resp)
		if err != nil {
			return BackupTaskInventory{}, fmt.Errorf("read backup tasks: %w", err)
		}
		for _, task := range tasks {
			if task.Type != "vzdump" {
				continue
			}
			// Per-node replacement requires unambiguous node provenance. PVE
			// can omit this redundant field; it cannot supply a different node.
			if task.Node != "" && task.Node != node.Node {
				return BackupTaskInventory{}, fmt.Errorf("Proxmox backup task node does not match its collection scope")
			}
			task.Node = node.Node
			inventory.Tasks = append(inventory.Tasks, task)
		}
	}
	return inventory, nil
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
