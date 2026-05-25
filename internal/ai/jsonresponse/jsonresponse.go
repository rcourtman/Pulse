// Package jsonresponse extracts JSON payloads from model responses.
package jsonresponse

import (
	"encoding/json"
	"strings"
)

// ExtractObject returns the first valid JSON object embedded in a model
// response. Models sometimes wrap JSON in markdown, inline code spans, or a
// leading language tag, so callers should not rely on the response being a
// clean JSON document.
func ExtractObject(response string) (string, bool) {
	response = strings.TrimSpace(response)
	if response == "" {
		return "", false
	}

	for i, r := range response {
		if r != '{' {
			continue
		}

		decoder := json.NewDecoder(strings.NewReader(response[i:]))
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			continue
		}
		if len(raw) > 0 && raw[0] == '{' {
			return string(raw), true
		}
	}

	return "", false
}
