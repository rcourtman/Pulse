package hostagent

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os/exec"
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
		SocketPath:  "/run/pulse-agent/helper.sock",
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
	agent := &Agent{
		collector: collector,
		privilegedTelemetry: &fakePrivilegedTelemetry{
			smartErr: errors.New("helper unavailable"),
		},
		logger: zerolog.Nop(),
	}

	if got := agent.collectSMARTData(context.Background(), nil, nil); got != nil {
		t.Fatalf("SMART after helper failure = %+v, want nil", got)
	}
	if localCalls != 0 {
		t.Fatalf("local SMART fallback calls = %d, want 0", localCalls)
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
		collector:           collector,
		privilegedTelemetry: helper,
		logger:              zerolog.Nop(),
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
}
