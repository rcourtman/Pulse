package hostagent

import (
	"strings"
	"testing"
)

func TestCommandEnvironmentDropsInterpretationVariablesForReads(t *testing.T) {
	environ := []string{
		"GREP_OPTIONS=--file=/etc/shadow",
		"BASH_FUNC_grep%%=() {  /bin/cat /etc/shadow\n}",
		"BASH_FUNC_ls()=() {  id\n}",
		"BASH_ENV=/tmp/startup",
		"ENV=/tmp/startup",
		"SHELLOPTS=xtrace",
		"BASHOPTS=extglob",
		"PS4=$(id)",
		"BLKID_FILE=/etc/x",
		"APT_CONFIG=/etc/shadow",
		"PS_FORMAT=pid,environ",
		"LANG=C.UTF-8",
		"GREP_COLORS=mt=01;31",
		"PATH=/usr/bin",
	}
	interpretation := []string{"GREP_OPTIONS=", "BASH_FUNC_", "BASH_ENV=", "ENV=/tmp", "SHELLOPTS=", "BASHOPTS=", "PS4=", "BLKID_FILE=", "APT_CONFIG=", "PS_FORMAT="}

	read := strings.Join(commandEnvironment(environ, "linux", false), "\n")
	for _, withheld := range interpretation {
		if strings.Contains(read, withheld) {
			t.Fatalf("read command environment kept %q:\n%s", withheld, read)
		}
	}
	for _, kept := range []string{"LANG=C.UTF-8", "GREP_COLORS=mt=01;31", "PATH=" + commandSearchPath + ":/usr/bin"} {
		if !strings.Contains(read, kept) {
			t.Fatalf("read command environment lost %q:\n%s", kept, read)
		}
	}

	// Approved and trusted commands run as the operator or Pulse chose, with
	// the service environment as it is.
	approved := strings.Join(commandEnvironment(environ, "linux", true), "\n")
	for _, kept := range interpretation {
		if !strings.Contains(approved, kept) {
			t.Fatalf("approved command environment lost %q:\n%s", kept, approved)
		}
	}
}
