package pbs

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Malformed success bodies may contain private proxy/provider details. They
// are neither backup observations nor authentication/permission diagnoses.
var errInvalidBackupInventory = errors.New("invalid PBS backup inventory response")

// Backup lists replace retained history and can renew the snapshot cache.
// Read through EOF and close before admitting any rows, not just a JSON prefix.
func readBackupInventory[T any](resp *http.Response, fields []string, validate func(map[string]json.RawMessage, T) bool) ([]T, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errInvalidBackupInventory
	}
	return decodeBackupInventory(body, fields, validate)
}

func decodeBackupInventory[T any](body []byte, fields []string, validate func(map[string]json.RawMessage, T) bool) ([]T, error) {
	if !utf8.Valid(body) {
		return nil, errInvalidBackupInventory
	}
	envelope, ok := backupInventoryObject(body, "data", "errors")
	if !ok {
		return nil, errInvalidBackupInventory
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidBackupInventory
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidBackupInventory
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidBackupInventory
	}
	values := make([]T, 0, len(rows))
	for _, row := range rows {
		object, ok := backupInventoryObject(row, fields...)
		var value T
		if !ok || json.Unmarshal(row, &value) != nil || !validate(object, value) {
			return nil, errInvalidBackupInventory
		}
		values = append(values, value)
	}
	return values, nil
}

type datastoreListEntry struct {
	Store string `json:"store"`
}

func decodeBackupDatastores(body []byte) ([]datastoreListEntry, error) {
	seen := make(map[string]bool)
	return decodeBackupInventory(body, []string{"store"}, func(_ map[string]json.RawMessage, datastore datastoreListEntry) bool {
		if datastore.Store == "" || strings.TrimSpace(datastore.Store) != datastore.Store || seen[datastore.Store] {
			return false
		}
		seen[datastore.Store] = true
		return true
	})
}

// Keep additive provider fields, but do not let duplicate keys or json's
// case-insensitive struct matching select a different identity or outcome.
func backupInventoryObject(body []byte, canonical ...string) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, false
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok {
			return nil, false
		}
		if _, exists := fields[name]; exists {
			return nil, false
		}
		for _, expected := range canonical {
			if name != expected && strings.EqualFold(name, expected) {
				return nil, false
			}
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, false
		}
		fields[name] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	return fields, true
}

func backupInventoryInteger(fields map[string]json.RawMessage, key string) bool {
	value := bytes.TrimSpace(fields[key])
	return len(value) > 0 && !bytes.Equal(value, []byte("null"))
}

func readBackupGroups(resp *http.Response) ([]BackupGroup, error) {
	seen := make(map[[2]string]bool)
	return readBackupInventory(resp, []string{"backup-type", "backup-id", "last-backup", "backup-count", "files", "owner"}, func(fields map[string]json.RawMessage, group BackupGroup) bool {
		key := [2]string{group.BackupType, group.BackupID}
		if strings.TrimSpace(group.BackupType) == "" || strings.TrimSpace(group.BackupID) == "" || seen[key] ||
			!backupInventoryInteger(fields, "backup-count") || group.BackupCount < 0 ||
			(group.BackupCount > 0 && (!backupInventoryInteger(fields, "last-backup") || group.LastBackup <= 0)) {
			return false
		}
		seen[key] = true
		return true
	})
}

func readBackupSnapshots(resp *http.Response, backupType, backupID string) ([]BackupSnapshot, error) {
	seen := make(map[int64]bool)
	return readBackupInventory(resp, []string{"backup-type", "backup-id", "backup-time", "files", "size", "protected", "comment", "owner", "verification"}, func(fields map[string]json.RawMessage, snapshot BackupSnapshot) bool {
		if snapshot.BackupType != backupType || snapshot.BackupID != backupID ||
			strings.TrimSpace(snapshot.BackupType) == "" || strings.TrimSpace(snapshot.BackupID) == "" ||
			snapshot.BackupTime <= 0 || snapshot.Size < 0 || seen[snapshot.BackupTime] {
			return false
		}
		// Verification is version-dependent (string, object or absent). Keep
		// those forms, including unknown states, without accepting conflicting
		// nested state values as a newly verified recovery point.
		if raw := bytes.TrimSpace(fields["verification"]); len(raw) > 0 && raw[0] == '{' {
			if _, ok := backupInventoryObject(raw, "state"); !ok {
				return false
			}
		}
		seen[snapshot.BackupTime] = true
		return true
	})
}

// Namespace discovery determines which backup scopes may be declared absent.
// Use the same complete-body admission as the resulting group/snapshot lists.
func readBackupNamespaces(resp *http.Response) ([]Namespace, error) {
	seen := make(map[string]bool)
	namespaces, err := readBackupInventory(resp, []string{"ns", "path", "name", "parent"}, func(fields map[string]json.RawMessage, _ Namespace) bool {
		// These are compatibility spellings for the collection path, not
		// independent scopes. An explicit root is an identity too: do not
		// silently fall through from ns:"" to a different path/name. Doing so
		// can certify a complete scan that never visited a retained namespace.
		var path string
		identified := false
		for _, key := range []string{"ns", "path", "name"} {
			if raw, ok := fields[key]; ok {
				var text string
				if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &text) != nil {
					return false
				}
				if text == "/" {
					text = ""
				}
				if identified && text != path {
					return false
				}
				path = text
				identified = true
			}
		}
		if !identified || seen[path] {
			return false
		}
		seen[path] = true
		return true
	})
	if err != nil {
		// Reader/closer errors can contain private provider/proxy details too.
		return nil, errors.New("invalid PBS namespace inventory response")
	}
	return namespaces, nil
}
