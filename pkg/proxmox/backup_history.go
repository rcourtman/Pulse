package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"
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
			// A decoded JSON prefix is not a completed page. Use the same
			// bounded, unambiguous full-response admission as live task reads.
			pageTasks, readErr := readTasks(resp)
			if readErr != nil {
				observation.Reason = "collection_failed"
				break
			}
			// History may update a matching live task's outcome. Admit the
			// whole page's node scope before appending any row or skipping
			// already-seen UPIDs: a foreign task is not evidence from this
			// node, even when it duplicates an earlier observed execution.
			for _, task := range pageTasks {
				if task.Type == "vzdump" && task.Node != "" && task.Node != node.Node {
					observation.Reason = "collection_failed"
					break
				}
			}
			if observation.Reason != "" {
				break
			}
			added := 0
			for _, task := range pageTasks {
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
			if len(pageTasks) < 200 {
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
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil || !utf8.Valid(body) {
		return false
	}
	envelope, ok := inventoryResponseObject(body, "data", "errors")
	if !ok {
		return false
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return false
	}
	paths, ok := inventoryResponseObject(envelope["data"], path)
	if !ok {
		return false
	}
	privileges, ok := inventoryResponseObject(paths[path], "Sys.Audit")
	if !ok {
		return false
	}
	var audit int
	return json.Unmarshal(privileges["Sys.Audit"], &audit) == nil && audit == 1
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
