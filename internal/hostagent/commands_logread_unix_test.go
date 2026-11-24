//go:build !windows

package hostagent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// A log read is approved by name, and whoever owns a directory under /var/log
// decides what its names lead to. The agent must read only what is really
// beneath /var/log, whatever the name says.
func TestCommandClient_executeCommand_LogReadsStayBeneathLogRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("runs the reads through the Linux sh, coreutils and grep")
	}
	base := t.TempDir()
	logs := filepath.Join(base, "log")
	for _, dir := range []string{"svc", "containers"} {
		if err := os.MkdirAll(filepath.Join(logs, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{
		"secret":              "TOKEN-OUTSIDE\n",
		"log/syslog":          "one\n",
		"log/svc/a.log":       "alpha\n",
		"log/svc/b.log":       "beta\n",
		"log/svc/.hidden.log": "hidden\n",
	} {
		if err := os.WriteFile(filepath.Join(base, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		"svc/swapped.log": filepath.Join(base, "secret"),
		"svc/up.log":      "../../secret",
		"svc/current":     "a.log",
		"out":             base,
		// Absolute links, the way kubelet links /var/log/containers to
		// /var/log/pods: followed only while they name a path under the root.
		"containers/a.log":    filepath.Join(logs, "svc", "a.log"),
		"containers/svc":      filepath.Join(logs, "svc"),
		"containers/hop.log":  filepath.Join(logs, "svc", "swapped.log"),
		"containers/loop.log": filepath.Join(logs, "containers", "loop.log"),
	} {
		if err := os.Symlink(target, filepath.Join(logs, link)); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(logs, "svc", "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}

	origRoot := logReadRootDir
	t.Cleanup(func() { logReadRootDir = origRoot })
	logReadRootDir = logs

	c := &CommandClient{}
	run := func(command string) commandResultPayload {
		t.Helper()
		result := c.executeCommand(context.Background(), executeCommandPayload{RequestID: "log-read", Command: command, Timeout: 10})
		if output := result.Stdout + result.Stderr; strings.Contains(output, "TOKEN-OUTSIDE") || strings.Contains(output, "/dev/fd/") {
			t.Fatalf("%q left /var/log or shows a descriptor path: %+v", command, result)
		}
		return result
	}

	for command, want := range map[string]string{
		"tail -n 5 /var/log/svc/a.log":               "alpha\n",
		"cat '/var/log/svc/current'":                 "alpha\n",
		"sudo cat /var/log/./syslog":                 "one\n",
		"grep -c a /var/log/svc/b.log":               "1\n",
		"cat /var/log/containers/a.log":              "alpha\n",
		"head -n 1 /var/log/containers/svc/b.log":    "beta\n",
		"grep -rH alpha /var/log/":                   "/var/log/svc/a.log:alpha\n",
		"head -n 1 /var/log/svc/?.log":               "==> /var/log/svc/a.log <==\nalpha\n\n==> /var/log/svc/b.log <==\nbeta\n",
		"grep -H . /var/log/sys* /var/log/svc/b.log": "/var/log/syslog:one\n/var/log/svc/b.log:beta\n",
	} {
		if result := run(command); !result.Success || result.Stdout != want {
			t.Errorf("%q = %+v, want stdout %q", command, result, want)
		}
	}

	for command, named := range map[string]string{
		"cat /var/log/svc/swapped.log":     "cat: /var/log/svc/swapped.log: ",
		"tail -n 5 /var/log/svc/up.log":    "tail: /var/log/svc/up.log: ",
		"head -n 5 /var/log/out/secret":    "head: /var/log/out/secret: ",
		"grep -r TOKEN /var/log/out":       "grep: /var/log/out: ",
		"cat /var/log/containers/hop.log":  "cat: /var/log/containers/hop.log: ",
		"cat /var/log/containers/loop.log": "cat: /var/log/containers/loop.log: ",
		"cat /var/log/svc/pipe":            "cat: /var/log/svc/pipe: not a regular file or directory",
		"cat /var/log/missing.log":         "cat: /var/log/missing.log: no such file or directory",
		"grep TOKEN /var/log/nothing*":     "grep: /var/log/nothing*: no such file or directory",
		"grep TOKEN /var/log/out/*/*.log":  "grep: /var/log/out/*/*.log: ",
	} {
		result := run(command)
		if result.Success || result.ExitCode != 1 || result.Stdout != "" || !strings.Contains(result.Stderr, named) {
			t.Errorf("%q = %+v, want exit 1 and stderr naming %q", command, result, named)
		}
	}

	// The files that do open are still read, and the read does not report success.
	result := run("cat /var/log/svc/*.log")
	if result.Success || result.ExitCode != 1 || result.Stdout != "alpha\nbeta\n" ||
		!strings.Contains(result.Stderr, "cat: /var/log/svc/swapped.log: ") || !strings.Contains(result.Stderr, "cat: /var/log/svc/up.log: ") {
		t.Errorf("glob over a directory with planted links = %+v", result)
	}
}

// Inside a guest the host cannot confine symlinks with os.Root, so unapproved
// guest log reads require approval, while approved or trusted guest log reads
// keep their names for execution inside the guest.
func TestCommandClient_executeCommand_GuestLogReadsRequireApprovalAndKeepNamesWhenTrusted(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	var ran []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		ran = append([]string{name}, args...)
		return exec.CommandContext(ctx, "true")
	}

	c := &CommandClient{}
	unapproved := c.executeCommand(context.Background(), executeCommandPayload{
		RequestID: "guest-log-read-unapproved", Command: "cat /var/log/syslog", TargetType: "container", TargetID: "141", Timeout: 10,
	})
	if unapproved.Success || !strings.Contains(unapproved.Error, "requires approval") || len(ran) != 0 {
		t.Fatalf("unapproved guest log read = %+v (ran=%q), want approval rejection without execution", unapproved, ran)
	}

	result := c.executeCommand(context.Background(), executeCommandPayload{
		RequestID: "guest-log-read-trusted", Command: "cat /var/log/syslog", TargetType: "container", TargetID: "141", Timeout: 10, Trusted: true,
	})
	want := []string{"sh", "-c", "pct exec 141 -- sh -c 'cat /var/log/syslog'"}
	if !result.Success || !slices.Equal(ran, want) {
		t.Fatalf("ran %q (%+v), want %q", ran, result, want)
	}
}
