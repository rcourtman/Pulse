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
	// Only an explicit array is an enumeration. Read and close the whole
	// bounded response before admitting even an otherwise valid JSON prefix.
	data := bytes.TrimSpace(envelope["data"])
	if len(data) < 2 || data[0] != '[' {
		return nil, errInvalidSnapshotInventoryResponse
	}
	// The envelope reader has already checked the JSON value, including its
	// closing bracket. Keep a successful empty read non-nil without a decoder.
	if len(bytes.TrimSpace(data[1:len(data)-1])) == 0 {
		return []Snapshot{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if start, err := decoder.Token(); err != nil || start != json.Delim('[') {
		return nil, errInvalidSnapshotInventoryResponse
	}
	snapshots := make([]Snapshot, 0)
	seen := make(map[string]bool)
	keys := make(map[string]struct{})
	for decoder.More() {
		snap, valid := readSnapshotInventoryRow(decoder, keys)
		if !valid || strings.TrimSpace(snap.Name) == "" || seen[snap.Name] {
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
	if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
		return nil, errInvalidSnapshotInventoryResponse
	}
	return snapshots, nil
}

// Validate field names while decoding their typed values. Avoid copying each
// row and all of its fields into RawMessages before decoding it again. The
// containing envelope has already validated the entire data value.
func readSnapshotInventoryRow(decoder *json.Decoder, keys map[string]struct{}) (Snapshot, bool) {
	var snap Snapshot
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return snap, false
	}
	clear(keys)
	var ignored ignoredSnapshotInventoryValue
	for decoder.More() {
		token, err := decoder.Token()
		name, named := token.(string)
		if _, duplicate := keys[name]; err != nil || !named || duplicate {
			return snap, false
		}
		keys[name] = struct{}{}

		var target any
		switch name {
		case "name":
			target = &snap.Name
		case "description":
			target = &snap.Description
		case "snaptime":
			target = &snap.SnapTime
		case "parent":
			target = &snap.Parent
		case "vmid":
			target = &snap.VMID // Validate the supplied field before replacing it with the requested VMID.
		case "vmstate":
			// A missing flag means no memory claim. An explicit null leaves
			// scalar fields unchanged in encoding/json, so start outside 0/1.
			snap.VMState = -1
			target = &snap.VMState
		default:
			for _, expected := range [...]string{"name", "description", "snaptime", "parent", "vmid", "vmstate"} {
				if strings.EqualFold(name, expected) {
					return snap, false
				}
			}
			target = &ignored
		}
		if decoder.Decode(target) != nil || (name == "vmstate" && snap.VMState != 0 && snap.VMState != 1) {
			return snap, false
		}
	}
	end, err := decoder.Token()
	return snap, err == nil && end == json.Delim('}')
}

// encoding/json validates skipped values without retaining their bytes or
// interpreting provider metadata as part of the snapshot.
type ignoredSnapshotInventoryValue struct{}

func (*ignoredSnapshotInventoryValue) UnmarshalJSON([]byte) error { return nil }
