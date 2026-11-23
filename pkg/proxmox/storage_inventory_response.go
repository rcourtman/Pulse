package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// These inventories govern storage and backup removal. A malformed success is
// not an empty enumeration or a permission refusal; never echo its body.
var errInvalidStorageInventoryResponse = errors.New("invalid Proxmox storage inventory response")

func readStorageCatalog(resp *http.Response, nodeScoped bool) ([]Storage, error) {
	body, err := readStorageInventoryBody(resp)
	if err != nil {
		return nil, err
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
	fields := []string{"storage", "type", "content", "active", "enabled", "shared", "nodes", "pool", "path", "total", "used", "avail"}
	values := make([]Storage, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if _, valid := inventoryResponseObject(row, fields...); !valid {
			return nil, errInvalidStorageInventoryResponse
		}
		var storage Storage
		if json.Unmarshal(row, &storage) != nil {
			return nil, errInvalidStorageInventoryResponse
		}
		// /storage is configuration: optional defaults need not be expanded.
		// /nodes/{node}/storage declares type and supported content; without
		// those discriminators a backup target could silently be skipped.
		if strings.TrimSpace(storage.Storage) == "" || seen[storage.Storage] ||
			(nodeScoped && (strings.TrimSpace(storage.Type) == "" || strings.TrimSpace(storage.Content) == "")) {
			return nil, errInvalidStorageInventoryResponse
		}
		seen[storage.Storage] = true
		values = append(values, storage)
	}
	return values, nil
}

// Content lists can contain many archives. Decode the rows without copying
// the entire "data" field into an intermediate RawMessage. Use the same
// complete-response boundary as catalogs and return no prefix on error.
func readStorageContentInventory(resp *http.Response) ([]StorageContent, error) {
	body, err := readStorageInventoryBody(resp)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return nil, errInvalidStorageInventoryResponse
	}
	seen := make(map[string]struct{})
	var contents []StorageContent
	var metadata json.RawMessage
	for decoder.More() {
		token, err := decoder.Token()
		name, named := token.(string)
		if _, duplicate := seen[name]; err != nil || !named || duplicate {
			return nil, errInvalidStorageInventoryResponse
		}
		if (name != "data" && strings.EqualFold(name, "data")) ||
			(name != "errors" && strings.EqualFold(name, "errors")) {
			return nil, errInvalidStorageInventoryResponse
		}
		seen[name] = struct{}{}
		if name == "data" {
			var valid bool
			contents, valid = decodeStorageContentRows(decoder)
			if !valid {
				return nil, errInvalidStorageInventoryResponse
			}
		} else if decoder.Decode(&metadata) != nil ||
			(name == "errors" && !bytes.Equal(bytes.TrimSpace(metadata), []byte("null"))) {
			return nil, errInvalidStorageInventoryResponse
		}
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, errInvalidStorageInventoryResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidStorageInventoryResponse
	}
	if _, found := seen["data"]; !found {
		return nil, errInvalidStorageInventoryResponse
	}
	return contents, nil
}

func decodeStorageContentRows(decoder *json.Decoder) ([]StorageContent, bool) {
	if start, err := decoder.Token(); err != nil || start != json.Delim('[') {
		return nil, false
	}
	fields := []string{"volid", "content", "ctime", "format", "size", "used", "vmid", "notes", "protected", "encryption", "verification", "verified"}
	seen := make(map[string]struct{})
	var row json.RawMessage // reused across the array
	var contents []StorageContent
	for decoder.More() {
		if decoder.Decode(&row) != nil {
			return nil, false
		}
		if _, valid := inventoryResponseObject(row, fields...); !valid {
			return nil, false
		}
		var content StorageContent
		if json.Unmarshal(row, &content) != nil || strings.TrimSpace(content.Volid) == "" || strings.TrimSpace(content.Content) == "" {
			return nil, false
		}
		if _, duplicate := seen[content.Volid]; duplicate {
			return nil, false
		}
		seen[content.Volid] = struct{}{}
		// Uninteresting rows still have to pass admission. Otherwise an
		// unread listing could incorrectly remove existing backup evidence.
		if content.Content == "backup" || content.Content == "vztmpl" {
			contents = append(contents, content)
		}
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
		return nil, false
	}
	return contents, true
}

func readStorageInventoryBody(resp *http.Response) ([]byte, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidStorageInventoryResponse
	}
	return body, nil
}
