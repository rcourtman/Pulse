package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
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
		seen := map[string]bool{}
		for page := 0; page < 30; page++ {
			params := url.Values{"typefilter": {"vzdump"}, "source": {"all"}, "limit": {"200"}, "start": {fmt.Sprint(page * 200)}, "since": {fmt.Sprint(since.Unix())}, "until": {fmt.Sprint(until)}}
			resp, readErr := c.get(ctx, "/nodes/"+url.PathEscape(node.Node)+"/tasks?"+params.Encode())
			if readErr != nil {
				observation.Reason = "collection_failed"
				break
			}
			var result struct {
				Data []Task `json:"data"`
			}
			readErr = json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			if readErr != nil {
				observation.Reason = "collection_failed"
				break
			}
			added := 0
			for _, task := range result.Data {
				if task.Type != "vzdump" || task.UPID == "" || seen[task.UPID] {
					continue
				}
				seen[task.UPID] = true
				if task.Node == "" {
					task.Node = node.Node
				}
				history.Tasks = append(history.Tasks, task)
				added++
			}
			if len(result.Data) < 200 {
				observation.Complete = true
				break
			}
			if added == 0 || page == 29 {
				observation.Reason = "history_truncated"
				break
			}
		}
		if observation.Complete && !c.backupHistoryAuditPermission(ctx, node.Node) {
			observation.Complete = false
			observation.Reason = "audit_permission_unconfirmed"
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
	defer resp.Body.Close()
	var result struct {
		Data map[string]map[string]int `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return false
	}
	return result.Data[path]["Sys.Audit"] == 1
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
