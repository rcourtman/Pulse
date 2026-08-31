package hostagent

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

type fakePrivilegedTelemetry struct {
	health       error
	smart        []DiskSMART
	smartErr     error
	proxmox      *agentshost.ProxmoxLXCInventory
	proxmoxErr   error
	smartCalls   int
	proxmoxCalls int
}

func (f *fakePrivilegedTelemetry) Health(context.Context) error {
	return f.health
}

func (f *fakePrivilegedTelemetry) SMARTSnapshot(context.Context) ([]DiskSMART, error) {
	f.smartCalls++
	return f.smart, f.smartErr
}

func (f *fakePrivilegedTelemetry) ProxmoxLXCFilesystems(context.Context) (*agentshost.ProxmoxLXCInventory, error) {
	f.proxmoxCalls++
	return f.proxmox, f.proxmoxErr
}

func testPrivilegeHelperTelemetry(t *testing.T, result agenthelper.HealthResult) *privilegeHelperTelemetry {
	t.Helper()
	client, err := agenthelper.NewClient(agenthelper.ClientConfig{
		SocketPath:  filepath.Join(t.TempDir(), "helper.sock"),
		MaxDeadline: privilegeHelperOperationDeadline,
		NewRequestID: func() (string, error) {
			return "health-request", nil
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			clientConn, serverConn := net.Pipe()
			go func() {
				defer serverConn.Close()
				var header [4]byte
				if _, readErr := io.ReadFull(serverConn, header[:]); readErr != nil {
					return
				}
				requestBytes := make([]byte, binary.BigEndian.Uint32(header[:]))
				if _, readErr := io.ReadFull(serverConn, requestBytes); readErr != nil {
					return
				}
				var request agenthelper.Request
				if unmarshalErr := json.Unmarshal(requestBytes, &request); unmarshalErr != nil {
					return
				}
				resultBytes, marshalErr := json.Marshal(result)
				if marshalErr != nil {
					return
				}
				responseBytes, marshalErr := json.Marshal(agenthelper.Response{
					ProtocolVersion:  agenthelper.ProtocolVersion,
					RequestID:        request.RequestID,
					Operation:        request.Operation,
					OperationVersion: request.OperationVersion,
					Success:          true,
					Result:           resultBytes,
				})
				if marshalErr != nil {
					return
				}
				frame := make([]byte, 4+len(responseBytes))
				binary.BigEndian.PutUint32(frame[:4], uint32(len(responseBytes)))
				copy(frame[4:], responseBytes)
				_, _ = serverConn.Write(frame)
			}()
			return clientConn, nil
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return &privilegeHelperTelemetry{client: client}
}

func TestPrivilegeHelperHealthRequiresExactProtocolResponse(t *testing.T) {
	tests := []struct {
		name    string
		result  agenthelper.HealthResult
		wantErr bool
	}{
		{
			name:   "healthy",
			result: agenthelper.HealthResult{Status: "ok", ProtocolVersion: agenthelper.ProtocolVersion},
		},
		{
			name:    "unhealthy status",
			result:  agenthelper.HealthResult{Status: "degraded", ProtocolVersion: agenthelper.ProtocolVersion},
			wantErr: true,
		},
		{
			name:    "protocol mismatch",
			result:  agenthelper.HealthResult{Status: "ok", ProtocolVersion: agenthelper.ProtocolVersion + 1},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			telemetry := testPrivilegeHelperTelemetry(t, test.result)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := telemetry.Health(ctx)
			if (err != nil) != test.wantErr {
				t.Fatalf("Health error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestCollectSMARTDataUsesTypedHelperWithoutLocalFallback(t *testing.T) {
	originalLookPath := zpoolLookPath
	zpoolLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { zpoolLookPath = originalLookPath })

	localCalls := 0
	collector := &mockCollector{
		smartLocalFn: func(context.Context, []string, *agentshost.UnraidStorage) ([]DiskSMART, error) {
			localCalls++
			return nil, errors.New("local SMART must not run")
		},
	}
	helper := &fakePrivilegedTelemetry{smart: []DiskSMART{
		{Device: "/dev/sda", Health: "PASSED"},
		{Device: "/dev/sdb", Health: "PASSED"},
	}}
	agent := &Agent{
		collector:           collector,
		privilegedTelemetry: helper,
		logger:              zerolog.Nop(),
	}

	got := agent.collectSMARTData(context.Background(), []string{"sdb"}, nil)

	if localCalls != 0 {
		t.Fatalf("local SMART calls = %d, want 0", localCalls)
	}
	if helper.smartCalls != 1 {
		t.Fatalf("helper SMART calls = %d, want 1", helper.smartCalls)
	}
	if len(got) != 1 || got[0].Device != "/dev/sda" {
		t.Fatalf("filtered helper SMART = %+v", got)
	}
}

func TestCollectSMARTDataDoesNotWidenPrivilegeAfterHelperFailure(t *testing.T) {
	localCalls := 0
	collector := &mockCollector{
		smartLocalFn: func(context.Context, []string, *agentshost.UnraidStorage) ([]DiskSMART, error) {
			localCalls++
			return []DiskSMART{{Device: "/dev/fallback"}}, nil
		},
	}
	helper := &fakePrivilegedTelemetry{smartErr: errors.New("helper unavailable")}
	agent := &Agent{
		collector:             collector,
		privilegedTelemetry:   helper,
		privilegeHelperHealth: newPrivilegeHelperStatus(),
		logger:                zerolog.Nop(),
	}

	if got := agent.collectSMARTData(context.Background(), nil, nil); got != nil {
		t.Fatalf("SMART after helper failure = %+v, want nil", got)
	}
	if localCalls != 0 {
		t.Fatalf("local SMART fallback calls = %d, want 0", localCalls)
	}
	status := requirePrivilegeHelperModuleStatus(t, agent.currentModuleStatus())
	if status.State != "degraded" || !strings.Contains(status.LastError, privilegeHelperOperationSMART) {
		t.Fatalf("helper status after SMART failure = %+v", status)
	}

	helper.smartErr = nil
	if got := agent.collectSMARTData(context.Background(), nil, nil); got != nil {
		t.Fatalf("empty SMART recovery result = %+v, want nil", got)
	}
	status = requirePrivilegeHelperModuleStatus(t, agent.currentModuleStatus())
	if status.State != "running" || status.LastError != "" {
		t.Fatalf("helper status after SMART recovery = %+v", status)
	}
}

func TestCollectProxmoxLXCFilesystemsDoesNotWidenPrivilegeAfterHelperFailure(t *testing.T) {
	localCalls := 0
	collector := &mockCollector{
		goos: "linux",
		lookPathFn: func(string) (string, error) {
			localCalls++
			return "/usr/sbin/pct", nil
		},
	}
	helper := &fakePrivilegedTelemetry{proxmoxErr: errors.New("helper unavailable")}
	agent := &Agent{
		cfg:                   Config{EnableProxmox: true},
		collector:             collector,
		privilegedTelemetry:   helper,
		privilegeHelperHealth: newPrivilegeHelperStatus(),
		logger:                zerolog.Nop(),
	}

	if got := agent.collectProxmoxLXCFilesystemsForReport(context.Background()); got != nil {
		t.Fatalf("Proxmox inventory after helper failure = %+v, want nil", got)
	}
	if helper.proxmoxCalls != 1 {
		t.Fatalf("helper Proxmox calls = %d, want 1", helper.proxmoxCalls)
	}
	if localCalls != 0 {
		t.Fatalf("local Proxmox fallback calls = %d, want 0", localCalls)
	}
	status := requirePrivilegeHelperModuleStatus(t, agent.currentModuleStatus())
	if status.State != "degraded" || !strings.Contains(status.LastError, privilegeHelperOperationProxmoxFilesystems) {
		t.Fatalf("helper status after Proxmox failure = %+v", status)
	}
}

func TestCollectProxmoxLXCFilesystemsDoesNotDegradeNonProxmoxHost(t *testing.T) {
	lookups := 0
	collector := &mockCollector{
		goos: "linux",
		lookPathFn: func(string) (string, error) {
			lookups++
			return "", exec.ErrNotFound
		},
	}
	helper := &fakePrivilegedTelemetry{proxmoxErr: errPrivilegeHelperProxmoxInventoryUnavailable}
	agent := &Agent{
		collector:             collector,
		privilegedTelemetry:   helper,
		privilegeHelperHealth: newPrivilegeHelperStatus(),
		logger:                zerolog.Nop(),
	}

	if got := agent.collectProxmoxLXCFilesystemsForReport(context.Background()); got != nil {
		t.Fatalf("non-Proxmox inventory = %+v, want nil", got)
	}
	if helper.proxmoxCalls != 1 || lookups != 1 {
		t.Fatalf("helper calls = %d lookups = %d, want 1 each", helper.proxmoxCalls, lookups)
	}
	status := requirePrivilegeHelperModuleStatus(t, agent.currentModuleStatus())
	if status.State != "running" || status.LastError != "" {
		t.Fatalf("non-Proxmox helper status = %+v", status)
	}
}

func TestCollectProxmoxLXCFilesystemsDegradesWhenConfiguredProviderReturnsNoInventory(t *testing.T) {
	agent := &Agent{
		cfg:                   Config{EnableProxmox: true},
		collector:             &mockCollector{goos: "linux"},
		privilegedTelemetry:   &fakePrivilegedTelemetry{proxmoxErr: errPrivilegeHelperProxmoxInventoryUnavailable},
		privilegeHelperHealth: newPrivilegeHelperStatus(),
		logger:                zerolog.Nop(),
	}

	if got := agent.collectProxmoxLXCFilesystemsForReport(context.Background()); got != nil {
		t.Fatalf("configured Proxmox inventory = %+v, want nil", got)
	}
	status := requirePrivilegeHelperModuleStatus(t, agent.currentModuleStatus())
	if status.State != "degraded" || !strings.Contains(status.LastError, "no Proxmox LXC filesystem inventory") {
		t.Fatalf("configured Proxmox helper status = %+v", status)
	}
}

func TestPrivilegeHelperStatusTracksIndependentOperationsAndRecovery(t *testing.T) {
	status := newPrivilegeHelperStatus()
	fixed := time.Date(2026, 8, 31, 11, 30, 0, 0, time.UTC)
	status.now = func() time.Time { return fixed }

	status.record(privilegeHelperOperationProxmoxFilesystems, errors.New("inventory unavailable"))
	status.record(privilegeHelperOperationSMART, errors.New("smart unavailable"))
	module := status.moduleStatus()
	if module.Name != agentshost.ModuleNameTypedPrivilegeHelper || module.State != "degraded" || !module.Enabled {
		t.Fatalf("degraded helper module = %+v", module)
	}
	if !strings.Contains(module.LastError, "proxmox.lxc_filesystems: helper operation failed") ||
		!strings.Contains(module.LastError, "smart.snapshot: helper operation failed") {
		t.Fatalf("aggregated helper failure = %q", module.LastError)
	}
	if module.UpdatedAt != fixed {
		t.Fatalf("updatedAt = %v, want %v", module.UpdatedAt, fixed)
	}

	status.record(privilegeHelperOperationSMART, nil)
	module = status.moduleStatus()
	if module.State != "degraded" || strings.Contains(module.LastError, privilegeHelperOperationSMART) ||
		!strings.Contains(module.LastError, privilegeHelperOperationProxmoxFilesystems) {
		t.Fatalf("partial helper recovery = %+v", module)
	}

	status.record(privilegeHelperOperationProxmoxFilesystems, nil)
	module = status.moduleStatus()
	if module.State != "running" || module.LastError != "" {
		t.Fatalf("complete helper recovery = %+v", module)
	}
}

func TestPrivilegeHelperStatusNeverPersistsRawErrorDetails(t *testing.T) {
	status := newPrivilegeHelperStatus()
	status.record(
		privilegeHelperOperationSMART,
		errors.New("request failed token=must-not-leak path=/private/helper.sock"),
	)
	status.record(privilegeHelperOperationProxmoxFilesystems, &agenthelper.RemoteError{
		Code:      agenthelper.ErrorProviderUnavailable,
		Message:   "provider failed bearer=remote-secret path=/root/private",
		RequestID: "secret-request-id",
	})

	module := status.moduleStatus()
	if module.State != "degraded" || !strings.Contains(module.LastError, "helper operation failed") {
		t.Fatalf("classified helper status = %+v", module)
	}
	serialized, err := json.Marshal(module)
	if err != nil {
		t.Fatalf("marshal module status: %v", err)
	}
	for _, secret := range []string{
		"must-not-leak",
		"/private/helper.sock",
		"token=",
		"remote-secret",
		"/root/private",
		"secret-request-id",
	} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("raw helper error detail %q reached serialized report state: %s", secret, serialized)
		}
	}
}

func TestPrivilegeHelperStatusConcurrentRecordAndSnapshot(t *testing.T) {
	status := newPrivilegeHelperStatus()
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(2)
		go func(iteration int) {
			defer group.Done()
			if iteration%2 == 0 {
				status.record(privilegeHelperOperationSMART, errors.New("temporary failure"))
				return
			}
			status.record(privilegeHelperOperationSMART, nil)
		}(i)
		go func() {
			defer group.Done()
			_ = status.moduleStatus()
		}()
	}
	group.Wait()
	module := status.moduleStatus()
	if module.State != "running" && module.State != "degraded" {
		t.Fatalf("concurrent helper state = %+v", module)
	}
}

func requirePrivilegeHelperModuleStatus(t *testing.T, statuses []agentshost.ModuleStatus) agentshost.ModuleStatus {
	t.Helper()
	for _, status := range statuses {
		if status.Name == agentshost.ModuleNameTypedPrivilegeHelper {
			return status
		}
	}
	t.Fatalf("typed privilege helper module missing from %+v", statuses)
	return agentshost.ModuleStatus{}
}
