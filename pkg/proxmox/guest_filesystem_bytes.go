package proxmox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Filesystem usage is an observation, not a defaultable configuration value.
// Preserve numeric/string compatibility without rounding, truncating negative
// or fractional readings, or converting absent usage into a healthy zero.
// The signed limit matches models.Disk, where these counters are published.
func parseVMFilesystemBytes(raw json.RawMessage, required bool) (uint64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		if !required {
			return 0, nil
		}
		return 0, fmt.Errorf("guest filesystem byte count is missing")
	}
	s := string(raw)
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, err
		}
		s = strings.TrimSpace(s)
	}
	invalid := fmt.Errorf("guest filesystem byte count is not a non-negative signed-range integer")
	if s == "" || strings.HasPrefix(s, "-") {
		return 0, invalid
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 63)
	}
	s = strings.TrimPrefix(s, "+")
	var exponent int64
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		var err error
		exponent, err = strconv.ParseInt(s[i+1:], 10, 32)
		if err != nil {
			return 0, invalid
		}
		s = s[:i]
	}
	var fractionalDigits int64
	if i := strings.IndexByte(s, '.'); i >= 0 {
		fractionalDigits = int64(len(s) - i - 1)
		s = s[:i] + s[i+1:]
	}
	if s == "" {
		return 0, invalid
	}
	for _, digit := range s {
		if digit < '0' || digit > '9' {
			return 0, invalid
		}
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return 0, nil // An explicitly reported zero, including 0.0 or 0e3.
	}
	shift := exponent - fractionalDigits
	if shift < 0 {
		// Only an integral decimal is a byte count; do not truncate fractions.
		if -shift >= int64(len(s)) {
			return 0, invalid
		}
		end := int64(len(s)) + shift
		if strings.Trim(s[end:], "0") != "" {
			return 0, invalid
		}
		s = s[:end]
	} else {
		// Bound expansion before allocating, even for an enormous exponent.
		if int64(len(s))+shift > 19 {
			return 0, invalid
		}
		s += strings.Repeat("0", int(shift))
	}
	return strconv.ParseUint(s, 10, 63)
}
