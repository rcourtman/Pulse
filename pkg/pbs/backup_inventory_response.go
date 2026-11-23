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
	var rows []T
	err := readBackupInventoryFields(body, func(decoder *json.Decoder, name string) error {
		switch name {
		case "data":
			var err error
			rows, err = decodeBackupInventoryRows[T](decoder, body)
			return err
		case "errors":
			status, err := decoder.Token()
			if err != nil || status != nil {
				return errInvalidBackupInventoryResponse
			}
			return nil
		default:
			return discardBackupInventoryField(decoder, name)
		}
	}, "data", "errors")
	// A decoded array is provisional until the complete envelope is accepted.
	if err != nil || rows == nil {
		return nil, errInvalidBackupInventoryResponse
	}
	return rows, nil
}

// Decode one row at a time rather than buffering a second copy of the
// complete array in the decoder and a third in a json.RawMessage.
func decodeBackupInventoryRows[T any](decoder *json.Decoder, body []byte) ([]T, error) {
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return nil, errInvalidBackupInventoryResponse
	}
	arrayStart := decoder.InputOffset() - 1
	rows := []T{}
	var row, zero T
	for decoder.More() {
		if err := decoder.Decode(&row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
		// Do not carry optional values forward or reuse nested storage from
		// a row that the result already owns.
		row = zero
	}
	end, err := decoder.Token()
	// Preserve the JSON decoder's nesting limit on the array as a whole,
	// not just on each decoded row. Check the original bytes without a copy.
	if err != nil || end != json.Delim(']') || !json.Valid(body[arrayStart:decoder.InputOffset()]) {
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
		// Missing and null counts both leave the sentinel untouched; an
		// explicit zero is the valid evidence of an empty group.
		group := BackupGroup{BackupCount: -1}
		if readBackupInventoryFields(row, discardBackupInventoryField, "backup-type", "backup-id", "backup-count", "last-backup", "files", "owner") != nil ||
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

// Allow unknown provider metadata but reject duplicate keys and alternate
// spellings of canonical fields. The visitor consumes exactly one value
// per call. Format errors never expose the provider's contents.
func readBackupInventoryFields(body []byte, readField func(*json.Decoder, string) error, canonical ...string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return errInvalidBackupInventoryResponse
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return errInvalidBackupInventoryResponse
		}
		name, ok := token.(string)
		if !ok {
			return errInvalidBackupInventoryResponse
		}
		if _, duplicate := seen[name]; duplicate {
			return errInvalidBackupInventoryResponse
		}
		for _, key := range canonical {
			if name != key && strings.EqualFold(name, key) {
				return errInvalidBackupInventoryResponse
			}
		}
		seen[name] = struct{}{}
		if readField(decoder, name) != nil {
			return errInvalidBackupInventoryResponse
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errInvalidBackupInventoryResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errInvalidBackupInventoryResponse
	}
	return nil
}

func discardBackupInventoryField(decoder *json.Decoder, _ string) error {
	// Decode still validates JSON syntax without building or copying a value.
	return decoder.Decode(&discardedBackupInventoryValue{})
}

type discardedBackupInventoryValue struct{}

func (*discardedBackupInventoryValue) UnmarshalJSON([]byte) error { return nil }
