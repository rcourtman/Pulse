package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

// Do not copy a malformed successful response into health diagnostics. It may
// contain private proxy details; it is not an authentication refusal either.
var errInvalidNodeInventoryResponse = errors.New("invalid Proxmox node inventory response")

func decodeNodeInventory(body []byte) ([]Node, error) {
	if !utf8.Valid(body) {
		return nil, errInvalidNodeInventoryResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidNodeInventoryResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidNodeInventoryResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	// An explicit empty array remains valid for scoped tokens. Missing/null
	// data is not an empty authoritative inventory and must not erase nodes.
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidNodeInventoryResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidNodeInventoryResponse
	}
	nodes := make([]Node, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		fields, valid := inventoryResponseObject(row, "node", "status", "cpu", "maxcpu", "mem", "maxmem", "disk", "maxdisk", "uptime", "level")
		if !valid {
			return nil, errInvalidNodeInventoryResponse
		}
		var name string
		if json.Unmarshal(fields["node"], &name) != nil || strings.TrimSpace(name) == "" || seen[name] {
			return nil, errInvalidNodeInventoryResponse
		}
		var node Node
		if json.Unmarshal(row, &node) != nil {
			return nil, errInvalidNodeInventoryResponse
		}
		seen[name] = true
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// The existing object reader rejects duplicates and extra JSON values. Keep
// unique provider metadata, but never let encoding/json's case-insensitive
// struct matching replace canonical inventory fields with another spelling.
func inventoryResponseObject(body []byte, canonical ...string) (map[string]json.RawMessage, bool) {
	fields, valid := guestAgentResponseFields(body)
	if !valid {
		return nil, false
	}
	for name := range fields {
		for _, expected := range canonical {
			if name != expected && strings.EqualFold(name, expected) {
				return nil, false
			}
		}
	}
	return fields, true
}
