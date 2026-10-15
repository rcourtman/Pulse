package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Snapshot inventories drive removal in collected state, recovery points and
// alerts. A malformed success is not an empty inventory or an access refusal.
// Never include its potentially private contents in the diagnostic.
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
		var snapshot Snapshot
		if json.Unmarshal(row, &snapshot) != nil || strings.TrimSpace(snapshot.Name) == "" || seen[snapshot.Name] {
			return nil, errInvalidSnapshotInventoryResponse
		}
		seen[snapshot.Name] = true
		// PVE's current-position row is not a snapshot. It still participates
		// in validation; filtering cannot hide ambiguity in the inventory.
		if snapshot.Name != "current" {
			snapshot.VMID = vmid
			snapshots = append(snapshots, snapshot)
		}
	}
	return snapshots, nil
}
