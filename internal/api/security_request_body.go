package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Security requests carry only credentials and small control fields. Keeping a
// shared bound prevents public authentication and recovery routes from
// allocating attacker-controlled JSON strings without limit.
const maxSecurityRequestBodyBytes int64 = 16 * 1024

func decodeSecurityRequestBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return io.EOF
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSecurityRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return err
	}

	// A second decode both rejects concatenated JSON documents and forces the
	// bounded reader to consume trailing whitespace. Without it, a valid first
	// object could hide an arbitrarily large unread request tail.
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}

func securityRequestErrorStatus(err error) int {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
