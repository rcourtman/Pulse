package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
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
	data := bytes.TrimSpace(envelope["data"])
	if len(data) == 0 || data[0] != '[' {
		return nil, errInvalidTaskLogResponse
	}
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) != nil || len(rows) > taskLogLineLimit {
		return nil, errInvalidTaskLogResponse
	}
	lines := make([]TaskLogLine, 0, len(rows))
	for i, row := range rows {
		fields, valid := inventoryResponseObject(row, "n", "t")
		if !valid {
			return nil, errInvalidTaskLogResponse
		}
		for _, name := range []string{"n", "t"} {
			value := bytes.TrimSpace(fields[name])
			if len(value) == 0 || bytes.Equal(value, []byte("null")) {
				return nil, errInvalidTaskLogResponse
			}
		}
		var line TaskLogLine
		if json.Unmarshal(row, &line) != nil || line.LineNumber != int64(i+1) {
			return nil, errInvalidTaskLogResponse
		}
		lines = append(lines, line)
	}
	// An explicit empty log and a full capped prefix remain usable reads, but
	// neither proves that all guests/results have been observed. The collector
	// keeps its existing terminal-marker and attribution checks for finality.
	return lines, nil
}
