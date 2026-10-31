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

var errInvalidBackupInventoryResponse = errors.New("invalid PBS backup inventory response")

// A list can reconcile absence or renew backup protection. Accept only one
// unambiguous envelope from a fully read, bounded response, never a JSON prefix.
// Keep provider payloads out of format diagnostics (including reported errors).
func readBackupInventory[T any](resp *http.Response) ([]T, error) {
	body, err := readBackupInventoryBody(resp)
	if err != nil {
		return nil, err
	}
	return decodeBackupInventory[T](body)
}

// Datastore discovery needs the same transport admission, but retains its
// existing HTML/HTTPS guidance and payload-free byte-count diagnostics.
func readBackupInventoryBody(resp *http.Response) ([]byte, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidBackupInventoryResponse
	}
	return body, nil
}

func decodeBackupInventory[T any](body []byte) ([]T, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errInvalidBackupInventoryResponse
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errInvalidBackupInventoryResponse
		}
		name, ok := token.(string)
		if !ok {
			return nil, errInvalidBackupInventoryResponse
		}
		if _, duplicate := fields[name]; duplicate ||
			(name != "data" && strings.EqualFold(name, "data")) ||
			(name != "errors" && strings.EqualFold(name, "errors")) {
			return nil, errInvalidBackupInventoryResponse
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, errInvalidBackupInventoryResponse
		}
		fields[name] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, errInvalidBackupInventoryResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidBackupInventoryResponse
	}
	if failure, present := fields["errors"]; present && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidBackupInventoryResponse
	}
	data := bytes.TrimSpace(fields["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidBackupInventoryResponse
	}
	var rows []T
	if json.Unmarshal(data, &rows) != nil {
		return nil, errInvalidBackupInventoryResponse
	}
	return rows, nil
}
