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

// Status supports both monitoring samples and independent action observations.
// A decodable prefix is not a completed read; never include its body in errors.
var errInvalidGuestStatusResponse = errors.New("invalid Proxmox guest status response")

func readGuestStatus[T any](resp *http.Response, vmid int) (*T, error) {
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
	fields := append([]string{"balloon", "balloon_min", "ballooninfo", "freemem", "meminfo"}, guestInventoryFields...)
	status, valid := inventoryResponseObject(envelope["data"], fields...)
	if !valid {
		return nil, errInvalidGuestStatusResponse
	}
	var state string
	if json.Unmarshal(status["status"], &state) != nil || strings.TrimSpace(state) == "" {
		return nil, errInvalidGuestStatusResponse
	}
	// PVE may omit vmid in this endpoint. If present, it must identify the
	// requested guest exactly; retain LXC's integer-string compatibility, but
	// do not let FlexInt's general numeric truncation accept a different ID.
	if raw, exists := status["vmid"]; exists {
		var id int
		if json.Unmarshal(raw, &id) != nil {
			var text string
			if json.Unmarshal(raw, &text) != nil {
				return nil, errInvalidGuestStatusResponse
			}
			var err error
			id, err = strconv.Atoi(text)
			if err != nil {
				return nil, errInvalidGuestStatusResponse
			}
		}
		if id <= 0 || id != vmid {
			return nil, errInvalidGuestStatusResponse
		}
	}
	var value T
	if json.Unmarshal(envelope["data"], &value) != nil {
		return nil, errInvalidGuestStatusResponse
	}
	return &value, nil
}
