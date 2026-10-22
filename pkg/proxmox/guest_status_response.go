package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// A partial successful status is not a new observation. Keep provider/proxy
// contents out of this error and do not classify it as an access refusal.
var errInvalidGuestStatusResponse = errors.New("invalid Proxmox guest status response")

// Both status/current decoders consume these fields. Preserve unique additive
// metadata, but never let case-insensitive struct matching overwrite a lock,
// identity or reading. Nested payloads retain their existing type semantics.
var guestStatusFields = append([]string{
	"balloon", "balloon_min", "ballooninfo", "freemem", "meminfo",
}, guestInventoryFields...)

func readGuestStatus[T any](resp *http.Response) (*T, error) {
	body, readErr := readResponseBodyLimited(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if resp.StatusCode != http.StatusOK || !utf8.Valid(body) {
		return nil, errInvalidGuestStatusResponse
	}
	envelope, valid := inventoryResponseObject(body, "data", "errors")
	if !valid {
		return nil, errInvalidGuestStatusResponse
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return nil, errInvalidGuestStatusResponse
	}
	data := envelope["data"]
	fields, valid := inventoryResponseObject(data, guestStatusFields...)
	if !valid {
		return nil, errInvalidGuestStatusResponse
	}
	// Running and stopped replies both carry status. Missing/null/empty data
	// must not become a freshly timestamped zero-value guest. Do not invent an
	// allowlist of provider states or require optional metrics to be present.
	var status string
	if json.Unmarshal(fields["status"], &status) != nil || strings.TrimSpace(status) == "" {
		return nil, errInvalidGuestStatusResponse
	}
	var value T
	if json.Unmarshal(data, &value) != nil {
		return nil, errInvalidGuestStatusResponse
	}
	return &value, nil
}
