package dockeragent

import (
	"encoding/json"
	"testing"
)

func TestContainerBlockIOCounterPresenceWireCompatibility(t *testing.T) {
	for _, tc := range []struct {
		wire        string
		read, write bool
	}{
		{`null`, false, false},
		{`{}`, false, false},
		{`{"readBytes":5000}`, true, false},
		{`{"writeBytes":7000}`, false, true},
		{`{"readBytes":5000,"writeBytes":7000}`, true, true},
		{`{"readBytesPresent":true,"writeBytesPresent":true}`, true, true},
		{`{"readBytes":5000,"readBytesPresent":false,"writeBytesPresent":true}`, false, true},
	} {
		t.Run(tc.wire, func(t *testing.T) {
			var io *ContainerBlockIO
			if err := json.Unmarshal([]byte(tc.wire), &io); err != nil {
				t.Fatal(err)
			}
			read, write := io.CounterPresence()
			if read != tc.read || write != tc.write {
				t.Fatalf("presence = %v/%v, want %v/%v", read, write, tc.read, tc.write)
			}
		})
	}
}
