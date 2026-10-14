package securityutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// ErrInvalidProxmoxSession deliberately contains no response or credential
// material. A malformed 200 is not a provider's credential refusal and must
// not authorise a second login encoding or replace the last complete session.
var ErrInvalidProxmoxSession = errors.New("invalid authentication response: expected a complete ticket and CSRF token")

// ParseProxmoxSession validates a complete, already byte-bounded PVE/PBS login
// body before either client publishes it. Metadata such as username and cap is
// allowed, but duplicate keys and case-insensitive credential overrides are
// not. This establishes response integrity, not server acceptance of a ticket.
func ParseProxmoxSession(body []byte) (ticket, csrf string, err error) {
	if !utf8.Valid(body) {
		return "", "", ErrInvalidProxmoxSession
	}
	envelope, ok := proxmoxSessionObject(body, "data", "errors")
	if !ok {
		return "", "", ErrInvalidProxmoxSession
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return "", "", ErrInvalidProxmoxSession
	}
	session, ok := proxmoxSessionObject(envelope["data"], "ticket", "CSRFPreventionToken")
	if !ok || json.Unmarshal(session["ticket"], &ticket) != nil || json.Unmarshal(session["CSRFPreventionToken"], &csrf) != nil ||
		!proxmoxSessionValue(ticket, true) || !proxmoxSessionValue(csrf, false) {
		return "", "", ErrInvalidProxmoxSession
	}
	return ticket, csrf, nil
}

func proxmoxSessionObject(body []byte, canonical ...string) (map[string]json.RawMessage, bool) {
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
		if _, duplicate := fields[name]; duplicate {
			return nil, false
		}
		for _, expected := range canonical {
			if name != expected && strings.EqualFold(name, expected) {
				return nil, false
			}
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

func proxmoxSessionValue(value string, cookie bool) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		// Credentials are opaque: do not trim, repair or log them. Require a
		// visible ASCII header value and, for the unquoted ticket cookie, the
		// cookie-octet subset. Real ticket delimiters and base64 stay supported.
		if b <= ' ' || b >= 0x7f || (cookie && (b == '"' || b == ',' || b == ';' || b == '\\')) {
			return false
		}
	}
	return true
}
