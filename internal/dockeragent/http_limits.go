package dockeragent

import (
	"fmt"
	"io"
)

const (
	maxPulseResponseBodyBytes    = 1 * 1024 * 1024
	maxVersionResponseBodyBytes  = 64 * 1024
	maxRegistryManifestBodyBytes = 4 * 1024 * 1024
	maxRegistryTokenBodyBytes    = 1 * 1024 * 1024
)

func readBodyWithLimit(r io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBytes)
	}
	return body, nil
}
