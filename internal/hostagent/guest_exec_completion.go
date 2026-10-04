package hostagent

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// qm can exit zero after handing off a guest PID without observing completion
// (for example when synchronous waiting expires). Only its complete terminal
// dictionary, not a local CLI exit, permits another QGA command. Keep the raw
// output in the ordinary command result; this decoder never logs guest data.
func guestExecCompleted(stdout string) bool {
	if len(stdout) == 0 || len(stdout) > maxCommandOutputSize {
		return false
	}
	d := json.NewDecoder(strings.NewReader(stdout))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	fields := make(map[string]json.RawMessage)
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return false
		}
		if _, duplicate := fields[name]; duplicate {
			return false
		}
		if name != "exited" && strings.EqualFold(name, "exited") {
			return false
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return false
		}
		fields[name] = value
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return false
	}
	if _, err := d.Token(); err != io.EOF {
		return false
	}
	// PVE/QGA's numeric true is 1; accept JSON true from compatible codecs.
	exited := bytes.TrimSpace(fields["exited"])
	return bytes.Equal(exited, []byte("1")) || bytes.Equal(exited, []byte("true"))
}
