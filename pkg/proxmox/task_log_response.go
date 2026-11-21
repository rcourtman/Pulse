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

var errInvalidTaskLogResponse = errors.New("invalid Proxmox task log response")

// Guest results depend on log order. Accept only a complete bounded response
// containing the requested prefix, not a decodable success before a read error
// or reordered/ambiguous lines. Never copy log text into format diagnostics.
func readTaskLogResponse(resp *http.Response) ([]TaskLogLine, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidTaskLogResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidTaskLogResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidTaskLogResponse
	}
	decoder := json.NewDecoder(bytes.NewReader(envelope["data"]))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return nil, errInvalidTaskLogResponse
	}
	lines := make([]TaskLogLine, 0)
	for decoder.More() {
		if len(lines) == taskLogLineLimit {
			return nil, errInvalidTaskLogResponse
		}
		line, valid := readTaskLogLine(decoder)
		if !valid || line.LineNumber != int64(len(lines)+1) {
			return nil, errInvalidTaskLogResponse
		}
		lines = append(lines, line)
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim(']') {
		return nil, errInvalidTaskLogResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidTaskLogResponse
	}
	// An explicit empty log and a full capped prefix remain usable reads, but
	// neither proves that all guests/results have been observed. The collector
	// keeps its existing terminal-marker and attribution checks for finality.
	return lines, nil
}

// Read the wire fields directly into the validated line. The JSON decoder
// owns syntax and escaping; in particular keys are compared after unescaping.
// Check all row keys for duplicates, even for ignored provider extensions.
func readTaskLogLine(decoder *json.Decoder) (TaskLogLine, bool) {
	var line TaskLogLine
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return line, false
	}
	// Keep the set row-local: clearing a reused map would retain the
	// allocation of an earlier row with more provider fields.
	keys := make(map[string]struct{})
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok {
			return line, false
		}
		if _, duplicate := keys[name]; duplicate {
			return line, false
		}
		keys[name] = struct{}{}
		switch name {
		case "n":
			// Decode pointers so explicit null is distinct from zero/empty.
			var number *int64
			if decoder.Decode(&number) != nil || number == nil {
				return line, false
			}
			line.LineNumber = *number
		case "t":
			var text *string
			if decoder.Decode(&text) != nil || text == nil {
				return line, false
			}
			line.Text = *text
		default:
			if strings.EqualFold(name, "n") || strings.EqualFold(name, "t") {
				return line, false
			}
			var ignored json.RawMessage
			if decoder.Decode(&ignored) != nil {
				return line, false
			}
		}
	}
	end, err := decoder.Token()
	_, hasText := keys["t"]
	// Missing n leaves zero and fails the caller's one-based order check.
	return line, err == nil && end == json.Delim('}') && hasText
}
