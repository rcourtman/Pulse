package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Storage and content enumerations authorise removal of known archive copies.
// Neither a valid JSON prefix nor a missing/null list establishes emptiness.
// Keep malformed provider/proxy payloads out of diagnostics.
var errInvalidStorageInventoryResponse = errors.New("invalid Proxmox storage inventory response")

func readStorageInventory[T any](resp *http.Response, fields []string, identity func(T) (string, bool)) ([]T, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidStorageInventoryResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidStorageInventoryResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidStorageInventoryResponse
	}
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidStorageInventoryResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidStorageInventoryResponse
	}
	values := make([]T, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		properties, valid := inventoryResponseObject(row, fields...)
		if !valid {
			return nil, errInvalidStorageInventoryResponse
		}
		// Present scalar fields must not silently decode null as a zero/empty
		// routing flag, identity or measurement. Optional descriptive metadata
		// and verification retain their existing nullable representation.
		for _, field := range fields {
			switch field {
			case "nodes", "pool", "path", "notes", "encryption", "verification":
				continue
			}
			if bytes.Equal(bytes.TrimSpace(properties[field]), []byte("null")) {
				return nil, errInvalidStorageInventoryResponse
			}
		}
		var value T
		if json.Unmarshal(row, &value) != nil {
			return nil, errInvalidStorageInventoryResponse
		}
		key, valid := identity(value)
		if !valid || seen[key] {
			return nil, errInvalidStorageInventoryResponse
		}
		seen[key] = true
		values = append(values, value)
	}
	return values, nil
}

func readStorageList(resp *http.Response) ([]Storage, error) {
	return readStorageInventory(resp, []string{"storage", "type", "content", "active", "enabled", "shared", "nodes", "pool", "path", "total", "used", "avail"}, func(storage Storage) (string, bool) {
		return storage.Storage, strings.TrimSpace(storage.Storage) != ""
	})
}

func readStorageContentInventory(resp *http.Response) ([]StorageContent, error) {
	// Snapshot sizing and container root usage need disk volumes too. Keep
	// this boundary lossless; each consumer selects its own content kinds only
	// after the whole response has passed the same integrity checks.
	return readStorageInventory(resp, []string{"volid", "content", "ctime", "format", "size", "used", "vmid", "notes", "protected", "encryption", "verification", "verified"}, func(content StorageContent) (string, bool) {
		return content.Volid, strings.TrimSpace(content.Volid) != "" && strings.TrimSpace(content.Content) != ""
	})
}
