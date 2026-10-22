package pbs

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// These reads decide whether a job/writer exists. Missing data is not a
// successful empty listing, and a valid JSON prefix is not a complete reply.
// Keep malformed provider/proxy bodies out of error, state and UI diagnostics.
var errInvalidJobResponse = errors.New("invalid PBS job response")

var pbsTaskFields = []string{
	"upid", "worker-type", "worker_type", "type", "worker-id", "worker_id", "id",
	"status", "starttime", "start-time", "start_time", "endtime", "end-time", "end_time", "store", "datastore",
}
var pbsJobConfigFields = []string{
	"id", "job-id", "job_id", "name", "store", "datastore", "remote-store", "remote_store",
	"remote", "ns", "namespace", "schedule", "comment", "last-run-state", "last_run_state",
	"last-run-upid", "last_run_upid", "last-run-endtime", "last_run_endtime", "next-run", "next_run", "disable", "disabled",
}

func readPBSJobData(resp *http.Response) (json.RawMessage, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		// Do not attach arbitrary Reader/Closer error text to published evidence.
		return nil, errInvalidJobResponse
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidJobResponse
	}
	envelope, ok := backupInventoryObject(body, "data", "errors")
	if !ok {
		return nil, errInvalidJobResponse
	}
	if failure, present := envelope["errors"]; present && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidJobResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, errInvalidJobResponse
	}
	return data, nil
}

func decodePBSJobObject(raw json.RawMessage, fields []string) (map[string]interface{}, error) {
	if _, ok := backupInventoryObject(raw, fields...); !ok {
		return nil, errInvalidJobResponse
	}
	var row map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(&row) != nil {
		return nil, errInvalidJobResponse
	}
	return row, nil
}

func readPBSJobList(resp *http.Response, fields []string, valid func(map[string]interface{}) bool) ([]map[string]interface{}, error) {
	data, err := readPBSJobData(resp)
	if err != nil {
		return nil, err
	}
	if data[0] != '[' {
		return nil, errInvalidJobResponse
	}
	var rawRows []json.RawMessage
	if json.Unmarshal(data, &rawRows) != nil {
		return nil, errInvalidJobResponse
	}
	rows := make([]map[string]interface{}, 0, len(rawRows))
	for _, raw := range rawRows {
		row, err := decodePBSJobObject(raw, fields)
		if err != nil || !valid(row) {
			return nil, errInvalidJobResponse
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// Aliases are compatibility spellings, not a priority order for conflicting
// writer identities. Choosing one can hide an active writer and authoritatively
// mark its incomplete snapshot idle. A missing worker ID remains conservative
// instance-wide activity; a present but invalid/conflicting ID is unreadable.
func validPBSRunningTaskIdentity(row map[string]interface{}) bool {
	return pbsRunningStringAliasesAgree(row, true, true, "worker-type", "worker_type", "type") &&
		pbsRunningStringAliasesAgree(row, false, false, "worker-id", "worker_id", "id")
}

func pbsRunningStringAliasesAgree(row map[string]interface{}, required, foldCase bool, keys ...string) bool {
	var identity string
	seen := false
	for _, key := range keys {
		raw, exists := row[key]
		if !exists {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return false
		}
		value = strings.TrimSpace(value)
		if foldCase {
			value = strings.ToLower(value)
		}
		if seen && value != identity {
			return false
		}
		identity, seen = value, true
	}
	return !required || (seen && identity != "")
}
