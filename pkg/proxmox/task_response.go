package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
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
	return readTaskRows[Task](resp, "upid", "node", "pid", "pstart", "starttime", "type", "id", "user", "status", "endtime")
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
