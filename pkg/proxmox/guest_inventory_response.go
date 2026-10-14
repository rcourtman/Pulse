package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"id", "type", "node", "pool", "status", "name", "vmid", "cpu", "cpus", "maxcpu", "mem", "maxmem", "swap", "maxswap", "disk", "maxdisk", "netin", "netout", "diskread", "diskwrite", "uptime", "template", "tags", "lock", "agent", "hostname", "ip", "ip6", "ipv4", "ipv6", "network", "diskinfo", "rootfs",
}

func readGuestInventory[T any](resp *http.Response, identity func(T) (string, bool)) ([]T, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidGuestInventoryResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidGuestInventoryResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidGuestInventoryResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	// Preserve a scoped empty array, never a missing/null inventory. Read and
	// close the whole bounded HTTP body before publishing even a valid prefix.
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidGuestInventoryResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidGuestInventoryResponse
	}
	values := make([]T, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if _, valid := inventoryResponseObject(row, guestInventoryFields...); !valid {
			return nil, errInvalidGuestInventoryResponse
		}
		var value T
		if json.Unmarshal(row, &value) != nil {
			return nil, errInvalidGuestInventoryResponse
		}
		key, valid := identity(value)
		if !valid || seen[key] {
			return nil, errInvalidGuestInventoryResponse
		}
		seen[key] = true
		values = append(values, value)
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
