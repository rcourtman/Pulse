package hostagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type guestExecOutcome struct {
	stdout, stderr string
	exitCode       int
	error          string
}

// qm's exit is the transport outcome, not the guest's exit. Only a complete,
// unambiguous terminal dictionary supplies guest output and status. Unknown
// completion retains the raw CLI result at the caller and fences admission;
// known failure/truncation is not an unknown handoff. Never log guest data.
func decodeGuestExecOutcome(stdout string) (guestExecOutcome, bool) {
	var outcome guestExecOutcome
	if len(stdout) == 0 || len(stdout) > maxCommandOutputSize || !utf8.ValidString(stdout) {
		return outcome, false
	}
	d := json.NewDecoder(strings.NewReader(stdout))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return outcome, false
	}
	fields := make(map[string]json.RawMessage)
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return outcome, false
		}
		if _, duplicate := fields[name]; duplicate {
			return outcome, false
		}
		for _, canonical := range []string{"exited", "exitcode", "signal", "out-data", "err-data", "out-truncated", "err-truncated"} {
			if name != canonical && strings.EqualFold(name, canonical) {
				return outcome, false
			}
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return outcome, false
		}
		fields[name] = value
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return outcome, false
	}
	if _, err := d.Token(); err != io.EOF {
		return outcome, false
	}
	// PVE/QGA's numeric true is 1; accept JSON true from compatible codecs.
	exited := bytes.TrimSpace(fields["exited"])
	if !bytes.Equal(exited, []byte("1")) && !bytes.Equal(exited, []byte("true")) {
		return outcome, false
	}
	exit, hasExit := fields["exitcode"]
	signal, hasSignal := fields["signal"]
	if hasExit == hasSignal {
		return outcome, false
	}
	status, maximum := exit, 255
	if hasSignal {
		status, maximum = signal, 64 // Linux QGA execution only.
	}
	code, err := strconv.Atoi(string(bytes.TrimSpace(status)))
	if err != nil || code < 0 || code > maximum || (hasSignal && code == 0) {
		return outcome, false
	}
	outcome.exitCode = code
	if hasSignal {
		outcome.exitCode += 128
		outcome.error = fmt.Sprintf("guest command terminated by signal %d", code)
	} else if code != 0 {
		outcome.error = fmt.Sprintf("guest command exited with status %d", code)
	}
	for key, target := range map[string]*string{"out-data": &outcome.stdout, "err-data": &outcome.stderr} {
		if data, exists := fields[key]; exists {
			if bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, target) != nil {
				return guestExecOutcome{}, false
			}
		}
	}
	for _, key := range []string{"out-truncated", "err-truncated"} {
		if flag, exists := fields[key]; exists {
			switch string(bytes.TrimSpace(flag)) {
			case "true", "1":
				// Keep a known nonzero/signal outcome even when its output
				// is also incomplete; neither qualifies as success.
				if outcome.error == "" {
					outcome.error = "guest command output was truncated"
				}
			case "false", "0":
			default:
				return guestExecOutcome{}, false
			}
		}
	}
	return outcome, true
}

func guestExecCompleted(stdout string) bool {
	_, terminal := decodeGuestExecOutcome(stdout)
	return terminal
}
