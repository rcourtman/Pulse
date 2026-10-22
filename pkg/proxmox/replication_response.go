package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

var errInvalidReplicationResponse = errors.New("invalid Proxmox replication response")

// A complete JSON prefix is not a complete observation. Inventory replaces
// retained jobs; status can supply their last successful sync. Require EOF,
// successful close and one unambiguous envelope before either can publish.
func readReplicationData(resp *http.Response) (json.RawMessage, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidReplicationResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidReplicationResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidReplicationResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || (data[0] != '[' && data[0] != '{') {
		return nil, errInvalidReplicationResponse
	}
	return data, nil
}

func decodeReplicationRows(data json.RawMessage) ([]map[string]json.RawMessage, error) {
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidReplicationResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidReplicationResponse
	}
	entries := make([]map[string]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		entry, valid := decodeReplicationRow(row)
		if !valid {
			return nil, errInvalidReplicationResponse
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func decodeReplicationRow(row json.RawMessage) (map[string]json.RawMessage, bool) {
	entry, valid := inventoryResponseObject(row,
		"id", "jobid", "guest", "jobnum", "source", "target", "source-storage", "source_storage",
		"target-storage", "target_storage", "schedule", "type", "comment", "remove_job", "remove-job",
		"enabled", "disable", "active", "state", "status", "error", "last_sync_error", "fail_count", "fail-count",
		"last_sync_status", "last-sync-status", "last_sync_state", "last-sync-state",
		"last_sync", "last-sync", "last_sync_time", "next_sync", "next-sync", "next_sync_time",
		"duration", "last_sync_duration", "last-sync-duration", "last_sync_duration_sec", "rate")
	if !valid {
		return nil, false
	}
	// Do not stringify an object, null or number into a request coordinate.
	// Missing/empty source retains the existing config-only unknown result.
	for _, field := range []string{"id", "jobid", "source"} {
		if raw, exists := entry[field]; exists {
			var value string
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &value) != nil {
				return nil, false
			}
		}
	}
	// Supported legacy aliases must not conceal contradictory identity or
	// verdict evidence behind whichever spelling the parser prefers. Numeric
	// and string zero remain compatible; unique additive metadata is allowed.
	for _, aliases := range [][]string{
		{"id", "jobid"},
		{"fail_count", "fail-count"},
		{"last_sync_status", "last-sync-status", "last_sync_state", "last-sync-state"},
		{"state", "status"},
	} {
		value, present := "", false
		for _, field := range aliases {
			if raw, exists := entry[field]; exists {
				next := stringFromAny(decodeRaw(raw))
				if aliases[0] == "last_sync_status" || aliases[0] == "state" {
					next = strings.ToLower(next)
					switch next {
					case "ok", "success", "completed":
						next = "ok"
					case "idle":
						if aliases[0] == "state" {
							next = "ok"
						}
					case "error", "failed":
						next = "error"
					}
				}
				if present && next != value {
					return nil, false
				}
				value, present = next, true
			}
		}
	}
	return entry, true
}
