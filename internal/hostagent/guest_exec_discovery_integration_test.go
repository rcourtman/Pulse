package hostagent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rs/zerolog"
)

type guestSafetyDiscoveryExecutor struct{ server *agentexec.Server }

func (e guestSafetyDiscoveryExecutor) ExecuteCommand(ctx context.Context, id string, p servicediscovery.ExecuteCommandPayload) (*servicediscovery.CommandResultPayload, error) {
	r, err := e.server.ExecuteCommand(ctx, id, agentexec.ExecuteCommandPayload{RequestID: p.RequestID, Command: p.Command, TargetType: p.TargetType, TargetID: p.TargetID, Timeout: p.Timeout, Trusted: true})
	if err != nil || r == nil {
		return nil, err
	}
	return &servicediscovery.CommandResultPayload{RequestID: r.RequestID, Success: r.Success, Stdout: r.Stdout, Stderr: r.Stderr, ExitCode: r.ExitCode, Error: r.Error}, nil
}
func (e guestSafetyDiscoveryExecutor) GetConnectedAgents() []servicediscovery.ConnectedAgent {
	var out []servicediscovery.ConnectedAgent
	for _, a := range e.server.GetConnectedAgents() {
		out = append(out, servicediscovery.ConnectedAgent{AgentID: a.AgentID, Hostname: a.Hostname})
	}
	return out
}
func (e guestSafetyDiscoveryExecutor) IsAgentConnected(id string) bool {
	return e.server.IsAgentConnected(id)
}

// Real server/session/registration/agent/Discovery paths, with a fake non-QGA
// config reader and fake provider process. This is NOT native PVE acceptance.
func testGuestExecRealServerAgentDiscoveryAdmission(t *testing.T) {
	platforms := []string{runtime.GOOS, "linux", "darwin", "windows"}
	seen := make(map[string]bool)
	for _, platform := range platforms {
		if seen[platform] {
			continue
		}
		seen[platform] = true
		t.Run(platform, func(t *testing.T) {
			testGuestExecRealServerAgentDiscoveryPlatform(t, platform)
		})
	}
}

func testGuestExecRealServerAgentDiscoveryPlatform(t *testing.T, platform string) {
	s := agentexec.NewServer(func(token, agent, host string) bool {
		return token == "fixture" && agent == "node-agent" && host == "node"
	})
	ts := httptest.NewServer(http.HandlerFunc(s.HandleWebSocket))
	defer ts.Close()
	logger := zerolog.Nop()
	c := NewCommandClient(Config{PulseURL: ts.URL, APIToken: "fixture", StateDir: t.TempDir(), Logger: &logger}, "node-agent", "node", platform, "6")
	var locked, lockDuringHandoff, failedGuest atomic.Bool
	var qgaCalls, configReads atomic.Int32
	locked.Store(true)
	c.guestExecAdmission = newGuestExecGuard(func(_ context.Context, vmid string) ([]byte, error) {
		configReads.Add(1)
		if vmid != "105" {
			t.Errorf("wrong VM read: %s", vmid)
		}
		if locked.Load() {
			return []byte("name: vm\nlock: backup\n"), nil
		}
		return []byte("name: vm\n"), nil
	})
	oldExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == localGuestExecQM {
			qgaCalls.Add(1)
			if len(args) != 7 || args[2] != "105" {
				t.Errorf("wrong VM handoff: %#v", args)
			}
			if lockDuringHandoff.Load() {
				locked.Store(true)
			}
		}
		reply := `{"exited":1,"exitcode":0,"out-data":"safe-observation"}`
		if name == localGuestExecQM && failedGuest.Load() {
			reply = `{"exited":1,"exitcode":7,"out-data":"misleading service identity","err-data":"failed probe"}`
		}
		return exec.CommandContext(ctx, "sh", "-c", "printf '%s' "+shellQuote(reply))
	}
	t.Cleanup(func() { execCommandContext = oldExec })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	defer func() {
		cancel()
		_ = c.Close()
		s.Shutdown()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("agent did not stop")
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for !s.IsAgentConnected("node-agent") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !s.IsAgentConnected("node-agent") {
		t.Fatal("agent not connected")
	}
	scanner := servicediscovery.NewDeepScanner(guestSafetyDiscoveryExecutor{s})
	for _, rt := range []servicediscovery.ResourceType{servicediscovery.ResourceTypeVM, servicediscovery.ResourceTypeDockerVM} {
		id := "105"
		if rt == servicediscovery.ResourceTypeDockerVM {
			id = "105:app"
		}
		got, err := scanner.Scan(context.Background(), servicediscovery.DiscoveryRequest{ResourceType: rt, ResourceID: id, TargetID: "node-agent"})
		if err == nil || !agentexec.IsGuestExecDeferred(err.Error()) || got == nil || len(got.CommandOutputs) != 0 || qgaCalls.Load() != 0 {
			t.Fatalf("locked scan escaped: %+v %v calls=%d", got, err, qgaCalls.Load())
		}
	}
	locked.Store(false)
	if runtime.GOOS != "linux" || platform != "linux" {
		// PVE admission belongs to a Linux host agent. A platform label must
		// not make a non-Linux executable advertise that capability, and a
		// Linux executable labelled as another platform must still be refused.
		// Exercise the real registration and server gate, not a skipped test
		// or an artificial capability override just to reach the happy path.
		for _, rt := range []servicediscovery.ResourceType{servicediscovery.ResourceTypeVM, servicediscovery.ResourceTypeDockerVM} {
			id := "105"
			if rt == servicediscovery.ResourceTypeDockerVM {
				id = "105:app"
			}
			got, err := scanner.Scan(context.Background(), servicediscovery.DiscoveryRequest{ResourceType: rt, ResourceID: id, TargetID: "node-agent"})
			if err == nil || err.Error() != agentexec.GuestExecDeferred(agentexec.GuestExecGuardUnavailable).Error() || got == nil || len(got.CommandOutputs) != 0 || qgaCalls.Load() != 0 || configReads.Load() != 0 {
				t.Fatalf("unsupported platform reached guest admission: %+v %v calls=%d reads=%d", got, err, qgaCalls.Load(), configReads.Load())
			}
		}
		if !s.IsAgentConnected("node-agent") {
			t.Fatal("platform deferral dropped session liveness")
		}
		return
	}
	got, err := scanner.ScanVM(context.Background(), "node-agent", "node", "105")
	want := int32(len(servicediscovery.GetCommandsForResource(servicediscovery.ResourceTypeVM)))
	if err != nil || got == nil || int32(len(got.CommandOutputs)) != want || qgaCalls.Load() != want {
		t.Fatalf("healthy explicit scan changed: %+v %v calls=%d", got, err, qgaCalls.Load())
	}
	for _, output := range got.CommandOutputs {
		if output != "safe-observation" {
			t.Fatalf("CLI wrapper became discovery evidence: %q", output)
		}
	}
	failedGuest.Store(true)
	got, err = scanner.ScanVM(context.Background(), "node-agent", "node", "105")
	if !errors.Is(err, servicediscovery.ErrNoCommandEvidence) || got == nil || len(got.CommandOutputs) != 0 || qgaCalls.Load() != 2*want {
		t.Fatalf("known failed guest became discovery evidence or uncertainty: %+v %v calls=%d", got, err, qgaCalls.Load())
	}
	failedGuest.Store(false)
	got, err = scanner.ScanVM(context.Background(), "node-agent", "node", "105")
	if err != nil || got == nil || int32(len(got.CommandOutputs)) != want || qgaCalls.Load() != 3*want {
		t.Fatalf("explicit healthy scan could not resume: %+v %v calls=%d", got, err, qgaCalls.Load())
	}
	lockDuringHandoff.Store(true)
	got, err = scanner.ScanVM(context.Background(), "node-agent", "node", "105")
	if err == nil || !agentexec.IsGuestExecDeferred(err.Error()) || got == nil || len(got.CommandOutputs) != 0 || qgaCalls.Load() != 3*want+1 {
		t.Fatalf("in-flight deferral was lost/retried: %+v %v calls=%d", got, err, qgaCalls.Load())
	}
	// Host probes do not use this QGA guard, even while VM admission is paused.
	if host, err := scanner.ScanHost(context.Background(), "node-agent", "node"); err != nil || host == nil || len(host.CommandOutputs) == 0 || qgaCalls.Load() != 3*want+1 {
		t.Fatalf("host control changed: %+v %v", host, err)
	}
	if !s.IsAgentConnected("node-agent") {
		t.Fatal("deferral dropped session liveness")
	}
	if configReads.Load() != 2+3*want*2+2 {
		t.Fatalf("config retries/unexpected reads: %d", configReads.Load())
	}
	if !strings.Contains(err.Error(), "operation lock") {
		t.Fatalf("lost lock reason: %v", err)
	}
}
