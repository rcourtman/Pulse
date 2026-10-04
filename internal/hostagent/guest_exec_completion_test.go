package hostagent

import "testing"

func TestGuestExecTerminalDictionary(t *testing.T) {
	for _, response := range []string{
		`{"exited":1,"exitcode":0,"out-data":"observation"}`,
		`{"exited":true,"exitcode":1,"err-data":"command failed"}`,
		`{"exited":1,"signal":15}`, // terminal signal is not an unknown handoff
	} {
		if !guestExecCompleted(response) {
			t.Fatalf("terminal dictionary not retained: %s", response)
		}
	}
	for _, response := range []string{`null`, `[]`, `{"exited":"true"}`, `{"exited":1.0}`, `{"exited":2}`, `{"exited":false}`, `{"exited":1,"out-data":"a","out-data":"b"}`} {
		if guestExecCompleted(response) {
			t.Fatalf("ambiguous terminal dictionary admitted: %s", response)
		}
	}
}
