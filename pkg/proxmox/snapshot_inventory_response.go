package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// An accepted snapshot inventory owns removal and recovery-point evidence.
// Never turn a malformed success into absence, or copy its body into diagnostics.
var errInvalidSnapshotInventoryResponse = errors.New("invalid Proxmox snapshot inventory response")

func readSnapshotInventory(resp *http.Response, vmid int) ([]Snapshot, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidSnapshotInventoryResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidSnapshotInventoryResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidSnapshotInventoryResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	// Only an explicit array is an enumeration. Read and close the whole
	// bounded response before admitting even an otherwise valid JSON prefix.
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidSnapshotInventoryResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidSnapshotInventoryResponse
	}
	snapshots := make([]Snapshot, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if _, valid := inventoryResponseObject(row, "name", "description", "snaptime", "parent", "vmid"); !valid {
			return nil, errInvalidSnapshotInventoryResponse
		}
		var snap Snapshot
		if json.Unmarshal(row, &snap) != nil || strings.TrimSpace(snap.Name) == "" || seen[snap.Name] {
			return nil, errInvalidSnapshotInventoryResponse
		}
		seen[snap.Name] = true
		// Proxmox's current-position marker is not a recovery artifact and
		// legitimately has no creation time. Named snapshots must have one.
		if snap.Name == "current" {
			continue
		}
		if snap.SnapTime <= 0 {
			return nil, errInvalidSnapshotInventoryResponse
		}
		snap.VMID = vmid
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}
