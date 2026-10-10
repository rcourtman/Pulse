package proxmox

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// guestAgentConfigLock accepts only one complete, unambiguous config envelope.
// In particular, encoding/json's last-key-wins and case-insensitive struct
// matching must not erase a backup lock. Ordinary config decoding is unchanged.
func guestAgentConfigLock(body []byte) (string, bool) {
	// encoding/json replaces malformed UTF-8 within strings instead of rejecting
	// it. A repaired config is not authoritative evidence of an absent lock.
	if !utf8.Valid(body) {
		return "", false
	}
	envelope, ok := guestAgentResponseFields(body, "data")
	if !ok {
		return "", false
	}
	config, ok := guestAgentResponseFields(envelope["data"])
	if !ok {
		return "", false
	}
	for name := range config {
		if name != "lock" && strings.EqualFold(name, "lock") {
			return "", false
		}
	}
	value, exists := config["lock"]
	if !exists {
		return "", true // PVE omits this field when no operation holds the lock.
	}
	var lock *string
	if json.Unmarshal(value, &lock) != nil || lock == nil {
		return "", false
	}
	return *lock, true
}
