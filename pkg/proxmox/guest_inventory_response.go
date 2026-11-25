package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Inventories own removal. A malformed success is neither a complete empty
// enumeration nor an authentication refusal; keep its body out of diagnostics.
var errInvalidGuestInventoryResponse = errors.New("invalid Proxmox guest inventory response")

// The three enumerations share identity, telemetry and metadata fields. Reject
// case-insensitive struct overwrites without excluding unique provider metadata
// or changing the existing endpoint-specific decoders (notably LXC FlexInt).
var guestInventoryFields = []string{
	"id", "type", "node", "pool", "status", "name", "vmid", "uid", "rtime", "cpu", "cpus", "maxcpu", "mem", "maxmem", "swap", "maxswap", "disk", "maxdisk", "netin", "netout", "diskread", "diskwrite", "uptime", "template", "tags", "lock", "agent", "hostname", "ip", "ip6", "ipv4", "ipv6", "network", "diskinfo", "rootfs",
}

func readGuestInventory[T any](resp *http.Response, decode func(*T, []byte, IOCounterPresence) error, identity func(T) (string, bool)) ([]T, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidGuestInventoryResponse
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return nil, errInvalidGuestInventoryResponse
	}
	seen := make(map[string]struct{})
	var values []T
	for decoder.More() {
		token, err := decoder.Token()
		name, named := token.(string)
		if _, duplicate := seen[name]; err != nil || !named || duplicate {
			return nil, errInvalidGuestInventoryResponse
		}
		seen[name] = struct{}{}
		switch name {
		case "data":
			values, err = readGuestInventoryRows(decoder, body, decode, identity)
			if err != nil {
				return nil, err
			}
		case "errors":
			failure, err := decoder.Token()
			if err != nil || failure != nil {
				return nil, errInvalidGuestInventoryResponse
			}
		default:
			if strings.EqualFold(name, "data") || strings.EqualFold(name, "errors") {
				return nil, errInvalidGuestInventoryResponse
			}
			if decoder.Decode(&discardedGuestInventoryMetadata{}) != nil {
				return nil, errInvalidGuestInventoryResponse
			}
		}
	}
	// Rows are provisional until the entire envelope is accepted. An explicit
	// scoped empty array returns a non-nil slice; missing data never does.
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') || values == nil {
		return nil, errInvalidGuestInventoryResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidGuestInventoryResponse
	}
	return values, nil
}

// The decoder checks syntax; provider metadata outside data has no consumer.
type discardedGuestInventoryMetadata struct{}

func (*discardedGuestInventoryMetadata) UnmarshalJSON([]byte) error { return nil }

func readGuestInventoryRows[T any](decoder *json.Decoder, body []byte, decode func(*T, []byte, IOCounterPresence) error, identity func(T) (string, bool)) ([]T, error) {
	if start, err := decoder.Token(); err != nil || start != json.Delim('[') {
		return nil, errInvalidGuestInventoryResponse
	}
	start := decoder.InputOffset() - 1
	// Keep only one raw row beside the decoded inventory. The type decoder
	// owns any nested bytes it retains, so the scratch row can be reused.
	var row json.RawMessage
	values := make([]T, 0)
	seen := make(map[string]bool)
	for decoder.More() {
		if err := decoder.Decode(&row); err != nil {
			return nil, errInvalidGuestInventoryResponse
		}
		fields, valid := inventoryResponseObject(row, guestInventoryFields...)
		if !valid {
			return nil, errInvalidGuestInventoryResponse
		}
		// Validation has already captured the original counter fields. Reuse
		// their presence instead of rebuilding a raw-field map in UnmarshalJSON.
		var value T
		if decode(&value, row, counterPresence(fields)) != nil {
			return nil, errInvalidGuestInventoryResponse
		}
		key, valid := identity(value)
		if !valid || seen[key] {
			return nil, errInvalidGuestInventoryResponse
		}
		seen[key] = true
		values = append(values, value)
	}
	// Decoding a row separately does not count the array in the JSON nesting
	// limit. Validate the array in place, as the bulk decoder did.
	if end, err := decoder.Token(); err != nil || end != json.Delim(']') || !json.Valid(body[start:decoder.InputOffset()]) {
		return nil, errInvalidGuestInventoryResponse
	}
	return values, nil
}

func guestInventoryID(vmid int) (string, bool) {
	return strconv.Itoa(vmid), vmid > 0
}

func clusterInventoryID(resource ClusterResource) (string, bool) {
	switch resource.Type {
	case "qemu", "lxc":
		key, valid := guestInventoryID(resource.VMID)
		// VMIDs are cluster-wide, including across a migration or guest kind.
		return "guest/" + key, valid && strings.TrimSpace(resource.Node) != ""
	default:
		// Unfiltered calls may include nodes, storage, pools and future provider
		// resource kinds. They retain their provider identity and optional fields.
		return resource.ID, strings.TrimSpace(resource.Type) != "" && strings.TrimSpace(resource.ID) != ""
	}
}
