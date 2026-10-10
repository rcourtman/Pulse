package hostagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

func TestGuestExecConfigLockDictionary(t *testing.T) {
	for _, tc := range []struct {
		name, config, lock string
		valid              bool
	}{
		{"unlocked", "cores: 2\nname: home-assistant\n", "", true},
		{"unicode name", "name: café\nmemory: 8192\n", "", true},
		{"hyphenated PVE key", "name: vm\namd-sev: type=sev\n", "", true},
		{"backup", "name: vm\nlock: backup\n", "backup", true},
		{"other operation", "lock: migrate\n", "migrate", true},
		{"comment", "# lock: backup\nname: vm\n", "", true},
		{"erased duplicate", "lock: backup\nlock:\n", "", false},
		{"duplicate unlocked", "cores: 2\ncores: 2\n", "", false},
		{"case conflict", "lock: backup\nLOCK:\n", "", false},
		{"empty lock", "name: vm\nlock:\n", "", false},
		{"snapshot", "name: vm\n[snapshot]\nlock: backup\n", "", false},
		{"empty", "", "", false},
		{"only comments", "# no config\n", "", false},
		{"malformed", "lock backup\n", "", false},
		{"command error", "ERROR: VM unavailable\n", "", false},
		{"bad key", "name: vm\n lock : backup\n", "", false},
		{"non utf8", "name: \xff\n", "", false},
		{"overflow", "name: " + strings.Repeat("x", guestExecConfigLimit), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lock, valid := guestExecConfigLock([]byte(tc.config))
			if valid != tc.valid || lock != tc.lock {
				t.Fatalf("lock=%q valid=%v", lock, valid)
			}
		})
	}
}

func TestGuestExecCommandPreflightAndPostflight(t *testing.T) {
	for _, tc := range []struct {
		name        string
		configs     []string
		failRead    int
		wantCalls   int
		wantSuccess bool
		wantReason  string
	}{
		{"healthy", []string{"name: vm\n", "name: vm\n"}, -1, 1, true, ""},
		{"backup before", []string{"lock: backup\n"}, -1, 0, false, agentexec.GuestExecVMLocked},
		{"migration before", []string{"lock: migrate\n"}, -1, 0, false, agentexec.GuestExecVMLocked},
		{"unverified owner", []string{""}, 0, 0, false, agentexec.GuestExecLockUnverified},
		{"ambiguous lock", []string{"lock: backup\nlock:\n"}, -1, 0, false, agentexec.GuestExecLockUnverified},
		{"backup in flight", []string{"name: vm\n", "lock: backup\n"}, -1, 1, false, agentexec.GuestExecVMLocked},
		{"owner lost in flight", []string{"name: vm\n", ""}, 1, 1, false, agentexec.GuestExecLockUnverified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reads, calls atomic.Int32
			g := newGuestExecGuard(func(_ context.Context, vmid string) ([]byte, error) {
				if vmid != "105" {
					t.Errorf("config read escaped canonical identity: %s", vmid)
				}
				i := int(reads.Add(1)) - 1
				if i == tc.failRead {
					return nil, errors.New("private config/provider text")
				}
				if i >= len(tc.configs) {
					t.Errorf("unexpected config retry %d", i)
					return nil, errors.New("retry")
				}
				return []byte(tc.configs[i]), nil
			})
			c := &CommandClient{guestExecAdmission: g}
			oldExec := execCommandContext
			execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
				calls.Add(1)
				if name != localGuestExecQM || !reflect.DeepEqual(args, []string{"guest", "exec", "105", "--", "sh", "-c", "echo original"}) {
					t.Errorf("changed command/identity: %s %#v", name, args)
				}
				return guestExecTestCommand(t, ctx, "original-result", "")
			}
			t.Cleanup(func() { execCommandContext = oldExec })
			payload := testApprovedCommandPayload(t, c, executeCommandPayload{RequestID: "scan", Command: "echo original", TargetType: " VM ", TargetID: "000105", Trusted: true})
			got := c.executeCommand(context.Background(), payload)
			if got.Success != tc.wantSuccess || int(calls.Load()) != tc.wantCalls {
				t.Fatalf("result=%#v calls=%d", got, calls.Load())
			}
			if tc.wantReason != "" && got.Error != agentexec.GuestExecDeferred(tc.wantReason).Error() {
				t.Fatalf("error=%q", got.Error)
			}
			if tc.wantCalls == 0 && (got.Stdout != "" || got.Stderr != "") {
				t.Fatal("preflight published guest output")
			}
			wantOutput := `{"exited":1,"exitcode":0,"out-data":"original-result"}`
			if tc.wantSuccess {
				wantOutput = "original-result"
			}
			if tc.wantCalls == 1 && (got.Stdout != wantOutput || got.ExitCode != 0) {
				t.Fatalf("lost actual process completion: %#v", got)
			}
			if strings.Contains(got.Error, "private") {
				t.Fatal("provider/config data leaked")
			}
			// A lock clearing after a preflight rejection is not an automatic
			// retry. A postflight unknown keeps even a NEW request in cooldown.
			if tc.wantCalls == 1 && !got.Success {
				before := reads.Load()
				next := c.executeCommand(context.Background(), payload)
				if next.Success || next.Error != agentexec.GuestExecDeferred(agentexec.GuestExecCooldown).Error() || reads.Load() != before || calls.Load() != 1 {
					t.Fatalf("uncertainty caused replay: %#v", next)
				}
			}
		})
	}
}

func TestGuestExecSerializationCancellationCapacityAndExplicitResumption(t *testing.T) {
	now := time.Now()
	g := newGuestExecGuard(func(context.Context, string) ([]byte, error) { return []byte("name: vm\n"), nil })
	g.now = func() time.Time { return now }
	release, err := g.acquire(context.Background(), "105")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := g.acquire(ctx, "105"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("same-target waiter: %v", err)
	}
	other, err := g.acquire(context.Background(), "106")
	if err != nil {
		t.Fatal(err)
	}
	other(false)
	release(true)
	if _, err := g.acquire(context.Background(), "105"); err == nil || err.Error() != agentexec.GuestExecDeferred(agentexec.GuestExecCooldown).Error() {
		t.Fatalf("no uncertainty fence: %v", err)
	}
	g.mu.Lock()
	now = now.Add(guestExecCooldown)
	g.mu.Unlock()
	resumed, err := g.acquire(context.Background(), "105")
	if err != nil {
		t.Fatal(err)
	}
	resumed(false)
	g.mu.Lock()
	if len(g.entries) != 0 {
		t.Fatal("completed entries retained")
	}
	for i := 0; i < maxGuestExecGuardEntries; i++ {
		g.entries["fixture-"+strings.Repeat("x", i%32)+time.Duration(i).String()] = guestExecEntry{until: now.Add(time.Minute)}
	}
	g.mu.Unlock()
	if _, err := g.acquire(context.Background(), "200"); err == nil || err.Error() != agentexec.GuestExecDeferred(agentexec.GuestExecCapacity).Error() {
		t.Fatalf("unbounded admission: %v", err)
	}
	g.mu.Lock()
	now = now.Add(time.Minute)
	g.mu.Unlock()
	cleaned, err := g.acquire(context.Background(), "200")
	if err != nil {
		t.Fatal(err)
	}
	cleaned(false)
}

// guestExecTestCommand runs this package's own test executable rather than a
// platform-specific shell. The wait mode acknowledges that the child actually
// started; command construction or elapsed wall time is not handoff evidence.
func guestExecTestCommand(t *testing.T, ctx context.Context, mode, address string) *exec.Cmd {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exec.CommandContext(ctx, self, "-test.run=^TestGuestExecControlledChild$", "--", "guest-exec-child", mode, address)
}

func TestGuestExecControlledChild(t *testing.T) {
	args := os.Args
	if len(args) < 4 || args[len(args)-3] != "guest-exec-child" {
		return // ordinary package invocation, not the controlled child
	}
	mode, address := args[len(args)-2], args[len(args)-1]
	if mode != "wait" {
		fmt.Printf(`{"exited":1,"exitcode":0,"out-data":%q}`, mode)
		os.Exit(0) // do not append the test runner's PASS to the QGA payload
	}
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		os.Exit(2)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "started\n"); err != nil {
		os.Exit(3)
	}
	// No timer completes this child. Only the production cancellation path
	// kills it; a handshake failure closes the parent endpoint for cleanup.
	var release [1]byte
	_, _ = conn.Read(release[:])
	os.Exit(4)
}

func TestGuestExecCanceledBeforeAdmissionNeverDispatches(t *testing.T) {
	var reads, calls atomic.Int32
	g := newGuestExecGuard(func(context.Context, string) ([]byte, error) {
		reads.Add(1)
		return []byte("name: vm\n"), nil
	})
	c := &CommandClient{guestExecAdmission: g}
	oldExec := execCommandContext
	execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		calls.Add(1)
		return guestExecTestCommand(t, ctx, "fresh", "")
	}
	t.Cleanup(func() { execCommandContext = oldExec })
	payload := testApprovedCommandPayload(t, c, executeCommandPayload{Command: "echo original", TargetType: "vm", TargetID: "105", Trusted: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := c.executeCommand(ctx, payload)
	want := agentexec.GuestExecDeferred("the command was canceled before guest admission").Error()
	if got.Success || got.Error != want || got.ExitCode != -1 || got.Stdout != "" || got.Stderr != "" || reads.Load() != 0 || calls.Load() != 0 {
		t.Fatalf("pre-admission cancellation dispatched or published evidence: %#v reads=%d calls=%d", got, reads.Load(), calls.Load())
	}
	g.mu.Lock()
	entries := len(g.entries)
	g.mu.Unlock()
	if entries != 0 {
		t.Fatalf("pre-admission cancellation retained %d entries", entries)
	}
	// A separate, explicit request is allowed: nothing was handed off and
	// therefore no completion-unknown cooldown should have been created.
	got = c.executeCommand(context.Background(), payload)
	if !got.Success || got.Stdout != "fresh" || reads.Load() != 2 || calls.Load() != 1 {
		t.Fatalf("fresh request after pre-admission cancellation: %#v reads=%d calls=%d", got, reads.Load(), calls.Load())
	}
}

func TestGuestExecCanceledHandoffNeverRetriesAndRechecksOnExplicitResumption(t *testing.T) {
	var reads, calls atomic.Int32
	g := newGuestExecGuard(func(context.Context, string) ([]byte, error) { reads.Add(1); return []byte("name: vm\n"), nil })
	now := time.Now()
	g.now = func() time.Time { return now }
	c := &CommandClient{guestExecAdmission: g}
	oldExec := execCommandContext
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	// This deadline is only a failing-test watchdog. It never initiates the
	// expected cancellation and cannot turn pre-admission into a passing test.
	if err := listener.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls.Add(1)
		if name != localGuestExecQM || !reflect.DeepEqual(args, []string{"guest", "exec", "105", "--", "sh", "-c", "echo original"}) {
			t.Errorf("changed handoff command/identity: %s %#v", name, args)
		}
		return guestExecTestCommand(t, ctx, "wait", listener.Addr().String())
	}
	t.Cleanup(func() { execCommandContext = oldExec })
	payload := testApprovedCommandPayload(t, c, executeCommandPayload{Command: "echo original", TargetType: "vm", TargetID: "105", Trusted: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handoff := make(chan error, 1)
	reaped := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			err = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			if err == nil {
				var ready [8]byte
				_, err = io.ReadFull(conn, ready[:])
				if err == nil && string(ready[:]) != "started\n" {
					err = fmt.Errorf("unexpected child handshake: %q", ready)
				}
			}
		}
		// Even on failure cancel and join the execution; no orphaned test child.
		cancel()
		handoff <- err
		// Keep the peer open until executeCommand has reaped its child below.
		if err == nil {
			<-reaped
		}
	}()
	got := c.executeCommand(ctx, payload)
	close(reaped)
	if err := <-handoff; err != nil {
		t.Fatalf("command never acknowledged handoff: %v (result=%#v)", err, got)
	}
	if got.Success || got.Error != agentexec.GuestExecDeferred(agentexec.GuestExecCompletionUnknown).Error() || reads.Load() != 1 || calls.Load() != 1 {
		t.Fatalf("canceled result=%#v reads=%d calls=%d", got, reads.Load(), calls.Load())
	}
	next := c.executeCommand(context.Background(), payload)
	if next.Success || next.Error != agentexec.GuestExecDeferred(agentexec.GuestExecCooldown).Error() || calls.Load() != 1 || reads.Load() != 1 {
		t.Fatalf("canceled handoff replayed: %#v calls=%d reads=%d", next, calls.Load(), reads.Load())
	}
	g.mu.Lock()
	now = now.Add(guestExecCooldown)
	g.mu.Unlock()
	execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		calls.Add(1)
		return guestExecTestCommand(t, ctx, "resumed", "")
	}
	got = c.executeCommand(context.Background(), payload)
	if !got.Success || got.Stdout != "resumed" || reads.Load() != 3 || calls.Load() != 2 {
		t.Fatalf("explicit resumption failed: %#v", got)
	}
}

func TestGuestExecCanceledLockCheckCannotPublishSuccessfulEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var reads atomic.Int32
	g := newGuestExecGuard(func(context.Context, string) ([]byte, error) {
		reads.Add(1)
		cancel() // config completion races the caller's cancellation
		return []byte("name: vm\n"), nil
	})
	if err := g.verifyUnlocked(ctx, "105"); err == nil || !agentexec.IsGuestExecDeferred(err.Error()) {
		t.Fatalf("canceled read became unlocked evidence: %v", err)
	}
	if err := g.verifyUnlocked(ctx, "105"); err == nil || !agentexec.IsGuestExecDeferred(err.Error()) || reads.Load() != 1 {
		t.Fatalf("already canceled check lost deferral or reread: %v reads=%d", err, reads.Load())
	}
	c := &CommandClient{guestExecAdmission: g}
	payload := testApprovedCommandPayload(t, c, executeCommandPayload{Command: "echo original", TargetType: "vm", TargetID: "105", Trusted: true})
	if got := c.executeCommand(ctx, payload); got.Success || !agentexec.IsGuestExecDeferred(got.Error) || got.Stdout != "" || reads.Load() != 1 {
		t.Fatalf("canceled admission was not an explicit safety pause: %#v", got)
	}
}
