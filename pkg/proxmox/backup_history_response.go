package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

var errInvalidBackupHistoryResponse = errors.New("invalid Proxmox backup history response")

// History completeness and audit authority require the whole bounded response,
// not a decodable prefix. Never include provider data in a format diagnostic.
func readBackupHistoryData(resp *http.Response) (json.RawMessage, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidBackupHistoryResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidBackupHistoryResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidBackupHistoryResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, errInvalidBackupHistoryResponse
	}
	return data, nil
}

func readBackupTaskInventory(resp *http.Response, node string) ([]Task, error) {
	data, err := readBackupHistoryData(resp)
	if err != nil {
		return nil, err
	}
	if data[0] != '[' {
		return nil, errInvalidBackupHistoryResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidBackupHistoryResponse
	}
	tasks := make([]Task, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		fields, valid := inventoryResponseObject(row, "upid", "node", "pid", "pstart", "starttime", "type", "id", "user", "status", "endtime")
		if !valid {
			return nil, errInvalidBackupHistoryResponse
		}
		// Null scalar fields cannot silently become zero/empty observations.
		for _, name := range []string{"upid", "node", "pid", "pstart", "starttime", "type", "id", "user", "status", "endtime"} {
			if raw, present := fields[name]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				return nil, errInvalidBackupHistoryResponse
			}
		}
		var task Task
		if json.Unmarshal(row, &task) != nil || strings.TrimSpace(task.UPID) == "" || seen[task.UPID] ||
			strings.TrimSpace(task.Type) == "" || task.Type != strings.TrimSpace(task.Type) || task.StartTime <= 0 ||
			task.EndTime < 0 || (task.EndTime > 0 && task.EndTime < task.StartTime) || (task.Node != "" && task.Node != node) {
			return nil, errInvalidBackupHistoryResponse
		}
		if task.Type == "vzdump" && task.ID != "" {
			vmid, err := strconv.ParseUint(task.ID, 10, 32)
			if err != nil || vmid == 0 {
				return nil, errInvalidBackupHistoryResponse
			}
		}
		seen[task.UPID] = true
		// The requested node supplies an absent legacy field, never overrides a
		// conflicting origin. An empty ID still denotes a multi-guest job.
		task.Node = node
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func readBackupHistoryAuditPermission(resp *http.Response, path string) bool {
	data, err := readBackupHistoryData(resp)
	if err != nil {
		return false
	}
	paths, valid := inventoryResponseObject(data)
	if !valid {
		return false
	}
	permissions, valid := inventoryResponseObject(paths[path], "Sys.Audit")
	if !valid {
		return false
	}
	var granted int
	return json.Unmarshal(permissions["Sys.Audit"], &granted) == nil && granted == 1
}
