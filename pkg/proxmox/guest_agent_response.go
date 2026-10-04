package proxmox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Keep the wire status separate from the existing diagnostic text. In
// particular, a proxy's error body can quote a different HTTP status.
type apiResponseError struct {
	statusCode           int
	guestCommandRejected bool
	cause                error
}

func (e *apiResponseError) Error() string { return e.cause.Error() }
func (e *apiResponseError) Unwrap() error { return e.cause }

// Only specific, complete terminal failures from the command endpoint allow
// another command after an HTTP 500. An unexplained 5xx, even with a complete
// body, does not establish that a serial QGA command has finished.
func guestAgentTerminalFailure(path string, body []byte) bool {
	_, vmid, ok := guestAgentPath(path)
	if !ok {
		return false
	}
	message, ok := guestAgentErrorMessage(body)
	if !ok {
		return false
	}
	commandPath := strings.SplitN(path, "?", 2)[0]
	command := "guest-" + commandPath[strings.LastIndex(commandPath, "/")+1:]
	message = strings.TrimPrefix(message, fmt.Sprintf("VM %d qmp command '%s' failed - ", vmid, command))
	switch message {
	case "QEMU guest agent is not running",
		"unsupported command: " + command,
		"The command " + command + " has not been found":
		return true
	}
	// Preserve the existing OpenBSD OS-info skip rather than repeatedly probing
	// its missing os-release file. Do not apply it to another guest command.
	if command == "guest-get-osinfo" {
		message = strings.TrimPrefix(message, "guest agent command failed: ")
		for _, filename := range []string{"/etc/os-release", "/usr/lib/os-release"} {
			if message == "Failed to open file '"+filename+"': No such file or directory" {
				return true
			}
		}
	}
	return false
}

func guestAgentErrorMessage(body []byte) (string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", false
	}
	if body[0] != '{' {
		return string(body), true // Still requires an exact, command-bound match.
	}
	fields, ok := guestAgentResponseFields(body, "data", "message", "errors")
	if !ok {
		return "", false
	}
	if data, exists := fields["data"]; exists && string(bytes.TrimSpace(data)) != "null" {
		return "", false
	}
	message, hasMessage := fields["message"]
	if nested, hasErrors := fields["errors"]; hasErrors {
		if hasMessage {
			return "", false // Conflicting or duplicate failure evidence.
		}
		errors, ok := guestAgentResponseFields(nested, "message")
		if !ok {
			return "", false
		}
		message = errors["message"]
	}
	var text string
	if json.Unmarshal(message, &text) != nil {
		return "", false
	}
	return strings.TrimSpace(text), true
}

// Reject ambiguous envelopes (including duplicate keys and trailing values)
// instead of letting a favourable final field override other error evidence.
// Without an allowlist, arbitrary unique configuration fields are permitted.
func guestAgentResponseFields(body []byte, allowed ...string) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, false
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		name, ok := key.(string)
		if !ok {
			return nil, false
		}
		permitted := len(allowed) == 0
		for _, candidate := range allowed {
			permitted = permitted || name == candidate
		}
		if _, duplicate := fields[name]; !permitted || duplicate {
			return nil, false
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, false
		}
		fields[name] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	return fields, true
}
