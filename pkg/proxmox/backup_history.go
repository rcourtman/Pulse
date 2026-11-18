package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	backupHistoryPageSize  = 200
	backupHistoryPageLimit = 30
)

// BackupTaskHistory preserves collection quality alongside execution evidence.
// A successful HTTP response can still contain only the caller's own tasks.
type BackupTaskHistory struct {
	Tasks []Task
	Nodes []BackupTaskHistoryNode
}

type BackupTaskHistoryNode struct {
	Node     string
	Complete bool
	Reason   string
}

// GetBackupTaskHistory lists retained vzdump tasks in a bounded, paginated
// window. Bounds, offline nodes and missing audit authority are explicit.
func (c *Client) GetBackupTaskHistory(ctx context.Context, since time.Time) (BackupTaskHistory, error) {
	var history BackupTaskHistory
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return history, err
	}
	until := time.Now().UTC().Unix()
	for _, node := range nodes {
		observation := BackupTaskHistoryNode{Node: node.Node}
		if node.Status != "online" {
			observation.Reason = "node_offline"
			history.Nodes = append(history.Nodes, observation)
			continue
		}
		seen := make(map[string]struct{})
		var nodeTasks []Task
		for page := 0; page < backupHistoryPageLimit; page++ {
			params := url.Values{"typefilter": {"vzdump"}, "source": {"all"}, "limit": {fmt.Sprint(backupHistoryPageSize)}, "start": {fmt.Sprint(page * backupHistoryPageSize)}, "since": {fmt.Sprint(since.Unix())}, "until": {fmt.Sprint(until)}}
			resp, readErr := c.get(ctx, "/nodes/"+url.PathEscape(node.Node)+"/tasks?"+params.Encode())
			if readErr != nil {
				observation.Reason = "collection_failed"
				if status, known := APIErrorStatus(readErr); known && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
					observation.Reason = "access_denied"
				}
				break
			}
			tasks, readErr := readBackupTaskInventory(resp, node.Node)
			if readErr != nil {
				observation.Reason = "collection_failed"
				break
			}
			overlap := false
			before := len(nodeTasks)
			for _, task := range tasks {
				if _, alreadySeen := seen[task.UPID]; alreadySeen {
					overlap = true
					continue
				}
				seen[task.UPID] = struct{}{}
				if task.Type == "vzdump" {
					nodeTasks = append(nodeTasks, task)
				}
			}
			// Overlap can leave a gap in the window even when later pages
			// supply new jobs. Keep those jobs without qualifying coverage
			// or advancing the checkpoint, including when we reach the cap.
			if overlap {
				observation.Reason = "collection_failed"
			}
			if len(tasks) < backupHistoryPageSize {
				observation.Complete = observation.Reason == ""
				break
			}
			if len(nodeTasks) == before {
				observation.Reason = "collection_failed"
				break
			}
			if page == backupHistoryPageLimit-1 && observation.Reason == "" {
				observation.Reason = "history_truncated"
			}
		}
		if observation.Complete && !c.backupHistoryAuditPermission(ctx, node.Node) {
			observation.Complete = false
			observation.Reason = "audit_permission_unconfirmed"
		}
		if observation.Reason != "access_denied" {
			history.Tasks = append(history.Tasks, nodeTasks...)
		}
		history.Nodes = append(history.Nodes, observation)
	}
	return history, nil
}

func (c *Client) backupHistoryAuditPermission(ctx context.Context, node string) bool {
	path := "/nodes/" + node
	resp, err := c.get(ctx, "/access/permissions?"+url.Values{"path": {path}}.Encode())
	if err != nil {
		return false
	}
	return readBackupHistoryAuditPermission(resp, path)
}

func (cc *ClusterClient) GetBackupTaskHistory(ctx context.Context, since time.Time) (BackupTaskHistory, error) {
	var result BackupTaskHistory
	err := cc.executeWithFailover(ctx, func(client *Client) error {
		var err error
		result, err = client.GetBackupTaskHistory(ctx, since)
		return err
	})
	return result, err
}
