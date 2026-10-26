package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// These inventories govern storage and backup removal. A malformed success is
// not an empty enumeration or a permission refusal; never echo its body.
var errInvalidStorageInventoryResponse = errors.New("invalid Proxmox storage inventory response")

func readStorageCatalog(resp *http.Response, nodeScoped bool) ([]Storage, error) {
	return readStorageInventory(resp, []string{
		"storage", "type", "content", "active", "enabled", "shared", "nodes", "pool", "path", "total", "used", "avail",
	}, func(storage Storage) (string, bool) {
		// /storage is configuration: optional defaults need not be expanded.
		// /nodes/{node}/storage declares type and supported content; without
		// those discriminators a backup target could silently be skipped.
		valid := strings.TrimSpace(storage.Storage) != ""
		if nodeScoped {
			valid = valid && strings.TrimSpace(storage.Type) != "" && strings.TrimSpace(storage.Content) != ""
		}
		return storage.Storage, valid
	})
}

func readStorageContentInventory(resp *http.Response) ([]StorageContent, error) {
	return readStorageInventory(resp, []string{
		"volid", "content", "ctime", "format", "size", "used", "vmid", "notes", "protected", "encryption", "verification", "verified",
	}, func(content StorageContent) (string, bool) {
		// Validate every row before the caller filters content kinds. A missing
		// discriminator is unread history, not an uninteresting file type.
		return content.Volid, strings.TrimSpace(content.Volid) != "" && strings.TrimSpace(content.Content) != ""
	})
}

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
		if _, valid := inventoryResponseObject(row, fields...); !valid {
			return nil, errInvalidStorageInventoryResponse
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
