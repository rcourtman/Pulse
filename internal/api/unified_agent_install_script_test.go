package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedInstallScriptFreeBSDRcDUsesSupervisorPidfile(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "install.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}

	script := string(data)
	if strings.Contains(script, "/usr/sbin/daemon -r -p ${pidfile}") {
		t.Fatal("FreeBSD rc.d templates must not use the child pidfile as the service pidfile with daemon -r")
	}

	wantDaemon := "/usr/sbin/daemon -r -P ${supervisor_pidfile} -p ${child_pidfile} -f ${command} ${command_args}"
	if got := strings.Count(script, wantDaemon); got != 2 {
		t.Fatalf("expected both FreeBSD rc.d templates to use supervisor and child pidfiles, got %d", got)
	}

	for _, required := range []string{
		`supervisor_pidfile="/var/run/${name}.pid"`,
		`child_pidfile="/var/run/${name}.child.pid"`,
		`kill "${supervisor_pid}"`,
		`kill "${child_pid}"`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("install script missing %q", required)
		}
	}
}
