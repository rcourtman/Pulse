package pbs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

var errInvalidBackupInventoryResponse = errors.New("invalid PBS backup inventory response")

// A list can reconcile absence or renew backup protection. Accept only one
// unambiguous envelope from a fully read, bounded response, never a JSON prefix.
// Keep provider payloads out of format diagnostics (including reported errors).
func readBackupInventory[T any](resp *http.Response) ([]T, error) {
	body, err := readBackupInventoryBody(resp)
	if err != nil {
		return nil, err
	}
	return decodeBackupInventory[T](body)
}

// Datastore discovery needs the same transport admission, but retains its
// existing HTML/HTTPS guidance and payload-free byte-count diagnostics.
func readBackupInventoryBody(resp *http.Response) ([]byte, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidBackupInventoryResponse
	}
	return body, nil
}

func decodeBackupInventory[T any](body []byte) ([]T, error) {
	fields, err := decodeBackupInventoryObject(body, "data", "errors")
	if err != nil {
		return nil, err
	}
	if failure, present := fields["errors"]; present && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidBackupInventoryResponse
	}
	data := bytes.TrimSpace(fields["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidBackupInventoryResponse
	}
	var rows []T
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidBackupInventoryResponse
	}
	return rows, nil
}

// Neither an absent count nor an ambiguous row proves that a group is empty
// or removed. Validate the complete namespace listing before the poller
// reuses snapshots, issues new requests, or reconciles history.
func readBackupGroupInventory(resp *http.Response) ([]BackupGroup, error) {
	rows, err := readBackupInventory[json.RawMessage](resp)
	if err != nil {
		return nil, err
	}
	groups := []BackupGroup{}
	type groupID struct{ kind, id string }
	seen := make(map[groupID]struct{})
	for i, row := range rows {
		fields, err := decodeBackupInventoryObject(row, "backup-type", "backup-id", "backup-count", "last-backup", "files", "owner")
		if err != nil {
			return nil, fmt.Errorf("invalid PBS backup group inventory row %d", i)
		}
		count := bytes.TrimSpace(fields["backup-count"])
		var group BackupGroup
		if len(count) == 0 || bytes.Equal(count, []byte("null")) ||
			json.Unmarshal(row, &group) != nil || group.BackupCount < 0 ||
			group.BackupType == "" || strings.TrimSpace(group.BackupType) != group.BackupType ||
			group.BackupID == "" || strings.TrimSpace(group.BackupID) != group.BackupID {
			return nil, fmt.Errorf("invalid PBS backup group inventory row %d", i)
		}
		key := groupID{group.BackupType, group.BackupID}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("PBS backup group inventory row %d repeats a group", i)
		}
		seen[key] = struct{}{}
		groups = append(groups, group)
	}
	return groups, nil
}

// Preserve unknown provider metadata while rejecting duplicate fields and
// alternate spellings that encoding/json would map onto typed fields.
func decodeBackupInventoryObject(body []byte, canonical ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errInvalidBackupInventoryResponse
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errInvalidBackupInventoryResponse
		}
		name, ok := token.(string)
		if !ok {
			return nil, errInvalidBackupInventoryResponse
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, errInvalidBackupInventoryResponse
		}
		for _, key := range canonical {
			if name != key && strings.EqualFold(name, key) {
				return nil, errInvalidBackupInventoryResponse
			}
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, errInvalidBackupInventoryResponse
		}
		fields[name] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, errInvalidBackupInventoryResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidBackupInventoryResponse
	}
	return fields, nil
}
