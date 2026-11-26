package proxmox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

// Keep the wire status separate from the existing diagnostic text. In
// particular, a proxy's error body can quote a different HTTP status.
type apiResponseError struct {
	statusCode           int
	guestCommandRejected bool
	guestFailureReason   string
	cause                error
}

func (e *apiResponseError) Error() string { return e.cause.Error() }
func (e *apiResponseError) Unwrap() error { return e.cause }

const guestAgentReasonCommandDisabled = "agent-command-disabled"

// GuestAgentErrorReason preserves observed HTTP/command evidence for callers
// without treating numbers or instructions quoted in an error body as status.
// It returns only existing fixed disk-status reasons, never provider text.
func GuestAgentErrorReason(err error) string {
	if reason := GuestAgentDeferredReason(err); reason != "" {
		return reason
	}
	var response *apiResponseError
	if !errors.As(err, &response) {
		return ""
	}
	switch response.statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "permission-denied"
	case http.StatusInternalServerError:
		if response.guestCommandRejected {
			if response.guestFailureReason == guestAgentReasonCommandDisabled {
				return "agent-error"
			}
			return response.guestFailureReason
		}
	}
	return "agent-error"
}

// IsGuestAgentCommandDisabled reports whether a Proxmox HTTP 500 explicitly
// rejected the requested guest-agent command as disabled for that VM instance.
func IsGuestAgentCommandDisabled(err error) bool {
	var response *apiResponseError
	return errors.As(err, &response) &&
		response.statusCode == http.StatusInternalServerError &&
		response.guestCommandRejected &&
		response.guestFailureReason == guestAgentReasonCommandDisabled
}

// Only specific, complete terminal failures from the command endpoint allow
// another command after an HTTP 500. An unexplained 5xx, even with a complete
// body, does not establish that a serial QGA command has finished.
func guestAgentTerminalFailureReason(path string, body []byte) string {
	_, vmid, ok := guestAgentPath(path)
	if !ok {
		return ""
	}
	message, ok := guestAgentErrorMessage(body)
	if !ok {
		return ""
	}
	commandPath := strings.SplitN(path, "?", 2)[0]
	command := "guest-" + commandPath[strings.LastIndex(commandPath, "/")+1:]
	// PVE's /agent/file-read endpoint runs guest-file-open before
	// guest-file-read, so a disabled or stopped agent rejects on guest-file-open.
	commands := []string{command}
	if command == "guest-file-read" {
		commands = []string{"guest-file-read", "guest-file-open"}
	}
	for _, qmpCommand := range commands {
		trimmed := strings.TrimPrefix(message, fmt.Sprintf("VM %d qmp command '%s' failed - ", vmid, qmpCommand))
		switch trimmed {
		case "QEMU guest agent is not running":
			return "agent-not-running"
		case "unsupported command: " + qmpCommand,
			"The command " + qmpCommand + " has not been found":
			return "agent-error"
		case "The command " + qmpCommand + " has been disabled for this instance":
			return guestAgentReasonCommandDisabled
		}
	}
	// Preserve the existing OpenBSD OS-info skip rather than repeatedly probing
	// its missing os-release file. Do not apply it to another guest command.
	if command == "guest-get-osinfo" {
		trimmed := strings.TrimPrefix(message, fmt.Sprintf("VM %d qmp command '%s' failed - ", vmid, command))
		trimmed = strings.TrimPrefix(trimmed, "guest agent command failed: ")
		for _, filename := range []string{"/etc/os-release", "/usr/lib/os-release"} {
			if trimmed == "Failed to open file '"+filename+"': No such file or directory" {
				return "agent-error"
			}
		}
	}
	return ""
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

// PVE guest-command success has one data dictionary. Preserve its existing
// method-specific decoding (including empty/null result, object-style rows,
// and partial row admission), but do not treat a proxy response, duplicate
// envelope or case-insensitive overwrite as a completed reply. This is not
// payload-to-command provenance or proof of native QGA completion/thaw.
func guestAgentSuccessResponse(body []byte) bool {
	_, valid := guestAgentSuccessData(body)
	return valid
}

func guestAgentSuccessData(body []byte) (map[string]json.RawMessage, bool) {
	if !utf8.Valid(body) {
		return nil, false
	}
	envelope, ok := guestAgentResponseFields(body, "data")
	if !ok {
		return nil, false
	}
	data, ok := guestAgentResponseFields(envelope["data"])
	if !ok {
		return nil, false
	}
	for name, value := range data {
		for _, canonical := range []string{"result", "content", "truncated"} {
			if name != canonical && strings.EqualFold(name, canonical) {
				return nil, false
			}
		}
		// A well-formed envelope can still contain an impossible guest reply.
		// The filesystem/network compatibility decoders otherwise turn scalar
		// results into empty inventories, and the file decoder reports a type
		// error only after admission has already been released. Neither proves
		// that the command finished. Keep this check before release, while
		// preserving absent/null fields and the existing row/content semantics.
		value = bytes.TrimSpace(value)
		switch name {
		case "result":
			if !bytes.Equal(value, []byte("null")) && value[0] != '{' && value[0] != '[' {
				return nil, false
			}
		case "content":
			if !bytes.Equal(value, []byte("null")) && value[0] != '"' {
				return nil, false
			}
		case "truncated":
			if !bytes.Equal(value, []byte("null")) && !bytes.Equal(value, []byte("true")) && !bytes.Equal(value, []byte("false")) {
				return nil, false
			}
		}
	}
	return data, true
}

// Valid field types alone do not make a reply compatible with the command
// sent. For example, a file-read body is valid JSON but cannot complete an
// outstanding filesystem request. Refuse known foreign reply fields before
// releasing shared admission. This detects contradictory shapes, not native
// payload provenance: shared fields (name/version), empty/null results and
// unknown metadata cannot identify the originating command and remain valid.
func guestAgentCommandResponse(path string, body []byte) bool {
	data, valid := guestAgentSuccessData(body)
	if !valid {
		return false
	}
	if _, _, valid := guestAgentPath(path); !valid {
		return false
	}
	command := strings.SplitN(path, "?", 2)[0]
	command = command[strings.LastIndex(command, "/")+1:]
	switch command {
	case "get-osinfo", "info", "get-fsinfo", "network-get-interfaces", "file-read":
	default:
		return false
	}
	if !guestAgentReplyFieldsCompatible(data, command) {
		return false
	}
	result := bytes.TrimSpace(data["result"])
	if len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return true
	}
	if command == "file-read" {
		return false // file-read returns content/truncated, not a result payload.
	}
	if result[0] == '{' {
		return guestAgentReplyDictionaryCompatible(result, command)
	}
	if command == "get-osinfo" || command == "info" {
		return false // Both information commands return dictionaries, not lists.
	}
	decoder := json.NewDecoder(bytes.NewReader(result))
	if start, err := decoder.Token(); err != nil || start != json.Delim('[') {
		return false
	}
	for decoder.More() {
		var row json.RawMessage
		if decoder.Decode(&row) != nil {
			return false
		}
		// Existing inventory decoders own malformed/partial rows. Only a
		// recognizable foreign dictionary contradicts the command here.
		row = bytes.TrimSpace(row)
		if len(row) > 0 && row[0] == '{' && !guestAgentReplyDictionaryCompatible(row, command) {
			return false
		}
	}
	end, err := decoder.Token()
	return err == nil && end == json.Delim(']')
}

// maxGuestAgentListEntries bounds each list a guest agent returns: filesystems,
// network interfaces and the addresses of one interface. The guest writes the
// reply and an entry can be two bytes long, while every entry is decoded into
// a record and a rejected one is also logged, so the byte limit of the
// response does not bound what a list costs.
const maxGuestAgentListEntries = 4096

var errGuestAgentListTooLong = fmt.Errorf("guest agent list exceeds %d entries", maxGuestAgentListEntries)

// guestAgentList splits a guest agent list into its entries and refuses a
// longer list before the entries beyond the bound are allocated. Like
// []json.RawMessage it leaves null as nil and fails for any other non-list.
type guestAgentList []json.RawMessage

func (list *guestAgentList) UnmarshalJSON(data []byte) error {
	*list = nil
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if start, err := decoder.Token(); err != nil || start != json.Delim('[') {
		return errors.New("guest agent result is not a list")
	}
	entries := guestAgentList{}
	for decoder.More() {
		if len(entries) == maxGuestAgentListEntries {
			return errGuestAgentListTooLong
		}
		var entry json.RawMessage
		if err := decoder.Decode(&entry); err != nil {
			return err
		}
		entries = append(entries, entry)
	}
	*list = entries
	return nil
}

// guestAgentFields returns the named fields of an object the guest wrote and
// decodes nothing else of it. A generic decode allocates a map or slice for
// every nested value, many times the bytes the response limit counted, while
// callers read a few fields. As in a generic decode, names match exactly and
// the last value of a repeated name counts.
func guestAgentFields(data []byte, names ...string) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return nil, false
	}
	fields := make(map[string]json.RawMessage, len(names))
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, false
		}
		if name, _ := key.(string); slices.Contains(names, name) {
			fields[name] = value
		}
	}
	return fields, true
}

// guestAgentScalarFields decodes those named fields of a guest's object that
// hold a string, number or boolean.
func guestAgentScalarFields(data []byte, names ...string) (map[string]interface{}, bool) {
	fields, ok := guestAgentFields(data, names...)
	if !ok {
		return nil, false
	}
	scalars := make(map[string]interface{}, len(fields))
	for name, value := range fields {
		if scalar := guestAgentScalar(value); scalar != nil {
			scalars[name] = scalar
		}
	}
	return scalars, true
}

// guestAgentScalar decodes a string, number or boolean. Null, an object and a
// list all yield nil, as none of them is a usable scalar reading.
func guestAgentScalar(data json.RawMessage) interface{} {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] == '{' || data[0] == '[' {
		return nil
	}
	var value interface{}
	if json.Unmarshal(data, &value) != nil {
		return nil
	}
	return value
}

func guestAgentReplyFieldsCompatible(fields map[string]json.RawMessage, command string) bool {
	for name, value := range fields {
		if !guestAgentReplyFieldCompatible(name, value, command) {
			return false
		}
	}
	return true
}

func guestAgentReplyDictionaryCompatible(body []byte, command string) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return false
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return false
		}
		name, valid := key.(string)
		var value json.RawMessage
		if !valid || decoder.Decode(&value) != nil || !guestAgentReplyFieldCompatible(name, value, command) {
			return false
		}
	}
	// Do not add another uniqueness/measurement policy inside a result. The
	// existing filesystem decoder rejects duplicate counters as unusable
	// completed readings and preserves valid peers. Inspect every field here
	// instead of last-key-wins, so a later null cannot erase foreign evidence.
	end, err := decoder.Token()
	return err == nil && end == json.Delim('}')
}

func guestAgentReplyFieldCompatible(name string, value json.RawMessage, command string) bool {
	// Null foreign fields carry no affirmative payload identity; keep
	// the established null-field and completed-empty semantics.
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return true
	}
	var owner string
	switch strings.ToLower(name) {
	case "content", "truncated":
		owner = "file-read"
	case "id", "kernel-release", "kernel-version", "machine", "pretty-name", "version-id":
		owner = "get-osinfo"
	case "qemu-ga", "supported-commands", "supported_commands":
		owner = "info"
	case "mountpoint", "total-bytes", "total-bytes-privileged", "used-bytes", "disk":
		owner = "get-fsinfo"
	case "hardware-address", "ip-addresses", "statistics", "has-ipv4-synth-gateway", "has-ipv6-synth-gateway":
		owner = "network-get-interfaces"
	case "error", "errors":
		return false
	}
	return owner == "" || owner == command
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
