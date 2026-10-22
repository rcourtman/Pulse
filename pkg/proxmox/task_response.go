package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var errInvalidTaskResponse = errors.New("invalid Proxmox task response")

// Task lists replace retained observations; finished job logs can be cached
// indefinitely. Neither may publish a valid JSON prefix or a repaired,
// ambiguous envelope as a completed observation. Preserve additive metadata
// and the existing task/log field semantics, including running tasks without
// status/endtime and successful empty arrays.
func readTaskRows[T any](resp *http.Response, fields ...string) ([]T, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidTaskResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidTaskResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidTaskResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidTaskResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidTaskResponse
	}
	values := make([]T, 0, len(rows))
	for _, row := range rows {
		if _, valid := inventoryResponseObject(row, fields...); !valid {
			return nil, errInvalidTaskResponse
		}
		var value T
		if json.Unmarshal(row, &value) != nil {
			return nil, errInvalidTaskResponse
		}
		values = append(values, value)
	}
	return values, nil
}

func readTasks(resp *http.Response) ([]Task, error) {
	tasks, err := readTaskRows[Task](resp, "upid", "node", "pid", "pstart", "starttime", "type", "id", "user", "status", "endtime")
	if err != nil {
		return nil, err
	}
	// Live lists replace earlier observations and history can update a live
	// verdict. An unidentified row is not successful emptiness; repeated
	// executions cannot be resolved by whichever outcome a consumer keeps.
	// UPIDs stay opaque, and scheduled/running jobs need no guest ID/result.
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.UPID) == "" || strings.TrimSpace(task.Type) == "" {
			return nil, errInvalidTaskResponse
		}
		// A task's interval and verdict jointly establish whether it ended.
		// Missing start times or contradictory completion fields cannot rule
		// out an archive writer, replace a prior failure, or complete History.
		// Active tasks may omit status/endtime; terminal results (including
		// errors and warnings) require an end at or after their positive start.
		status := strings.TrimSpace(task.Status)
		active := status == "" || strings.EqualFold(status, "running")
		if task.StartTime <= 0 || task.EndTime < 0 ||
			!taskTimestampFitsJSON(task.StartTime) || !taskTimestampFitsJSON(task.EndTime) ||
			(active && task.EndTime != 0) || (!active && task.EndTime < task.StartTime) {
			return nil, errInvalidTaskResponse
		}
		if _, duplicate := seen[task.UPID]; duplicate {
			return nil, errInvalidTaskResponse
		}
		seen[task.UPID] = struct{}{}
	}
	return tasks, nil
}

// Polling converts task seconds to time.Time in served state. A positive int64
// (for example, milliseconds accidentally supplied as seconds) can exceed
// time.Time's JSON year range and prevent the entire response from encoding.
// Validate the same local-time conversion used by consumers; do not guess units,
// clamp the date or drop just the bad row from replacement inventory.
func taskTimestampFitsJSON(seconds int64) bool {
	year := time.Unix(seconds, 0).Year()
	return year >= 0 && year <= 9999
}

// This reader always requests start=0. Numbered, present text is necessary
// before a job log can supply per-guest verdicts or enter the immutable cache:
// a missing/null error line must not disappear into Go's zero-value string,
// and a gap or reordered line must not turn a partial log into success.
func readTaskLog(resp *http.Response) ([]TaskLogLine, error) {
	rows, err := readTaskRows[struct {
		Number *int64  `json:"n"`
		Text   *string `json:"t"`
	}](resp, "n", "t")
	if err != nil {
		return nil, err
	}
	lines := make([]TaskLogLine, 0, len(rows))
	for i, row := range rows {
		if row.Number == nil || *row.Number != int64(i)+1 || row.Text == nil {
			return nil, errInvalidTaskResponse
		}
		lines = append(lines, TaskLogLine{LineNumber: *row.Number, Text: *row.Text})
	}
	return lines, nil
}
