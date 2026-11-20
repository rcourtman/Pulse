package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber() // Preserve integer fields exactly, including values above 2^53.
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return nil, errInvalidBackupHistoryResponse
	}
	tasks := make([]Task, 0)
	seen := make(map[string]bool)
	fields := make(map[string]struct{})
	for decoder.More() {
		task, valid := readBackupTaskRow(decoder, fields)
		if !valid || strings.TrimSpace(task.UPID) == "" || seen[task.UPID] ||
			strings.TrimSpace(task.Type) == "" || task.Type != strings.TrimSpace(task.Type) || task.StartTime <= 0 ||
			task.EndTime < 0 || (task.EndTime > 0 && task.EndTime < task.StartTime) || (task.Node != "" && task.Node != node) {
			return nil, errInvalidBackupHistoryResponse
		}
		// PVE may omit the status of a running task, but an exit result is
		// not completion evidence without an end clock. Equally, a live
		// status cannot contradict an observed end. Treat the page as unread
		// rather than replacing accepted running intent and thereby allowing
		// its still-growing archive to appear complete.
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if (task.EndTime == 0 && status != "" && status != "running") ||
			(task.EndTime > 0 && (status == "running" || status == "active")) {
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
	end, err := decoder.Token()
	if err != nil || end != json.Delim(']') {
		return nil, errInvalidBackupHistoryResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidBackupHistoryResponse
	}
	return tasks, nil
}

// Validate exact field names and decode their values in one pass. The standard
// decoder still owns JSON syntax and escaping; every row key, including unknown
// provider metadata, must be unique. A present scalar must have its wire type,
// never null. Keep these wire names and types in sync with Task.
func readBackupTaskRow(decoder *json.Decoder, fields map[string]struct{}) (Task, bool) {
	var task Task
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return task, false
	}
	clear(fields)
	for decoder.More() {
		name, valid := backupTaskScalar[string](decoder)
		if !valid {
			return task, false
		}
		if _, duplicate := fields[name]; duplicate {
			return task, false
		}
		fields[name] = struct{}{}
		switch name {
		case "upid":
			task.UPID, valid = backupTaskScalar[string](decoder)
		case "node":
			task.Node, valid = backupTaskScalar[string](decoder)
		case "pid":
			var pid int64
			pid, valid = backupTaskInteger(decoder, strconv.IntSize)
			task.PID = int(pid)
		case "pstart":
			task.PStart, valid = backupTaskInteger(decoder, 64)
		case "starttime":
			task.StartTime, valid = backupTaskInteger(decoder, 64)
		case "type":
			task.Type, valid = backupTaskScalar[string](decoder)
		case "id":
			task.ID, valid = backupTaskScalar[string](decoder)
		case "user":
			task.User, valid = backupTaskScalar[string](decoder)
		case "status":
			task.Status, valid = backupTaskScalar[string](decoder)
		case "endtime":
			task.EndTime, valid = backupTaskInteger(decoder, 64)
		default:
			for _, canonical := range [...]string{"upid", "node", "pid", "pstart", "starttime", "type", "id", "user", "status", "endtime"} {
				if strings.EqualFold(name, canonical) {
					return task, false
				}
			}
			var ignored json.RawMessage
			valid = decoder.Decode(&ignored) == nil
		}
		if !valid {
			return task, false
		}
	}
	end, err := decoder.Token()
	return task, err == nil && end == json.Delim('}')
}

func backupTaskScalar[T string | json.Number](decoder *json.Decoder) (T, bool) {
	token, err := decoder.Token()
	value, ok := token.(T)
	return value, err == nil && ok
}

func backupTaskInteger(decoder *json.Decoder, bits int) (int64, bool) {
	number, valid := backupTaskScalar[json.Number](decoder)
	if !valid {
		return 0, false
	}
	value, err := strconv.ParseInt(string(number), 10, bits)
	return value, err == nil
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
