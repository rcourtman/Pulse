package hostagent

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
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
	status.Record(privilegeHelperOperationContainerInventory, errors.New("container inventory unavailable"))
	module := status.moduleStatus()
	if module.Name != agentshost.ModuleNameTypedPrivilegeHelper || module.State != "degraded" || !module.Enabled {
		t.Fatalf("degraded helper module = %+v", module)
	}
	if !strings.Contains(module.LastError, "proxmox.lxc_filesystems: helper operation failed") ||
		!strings.Contains(module.LastError, "smart.snapshot: helper operation failed") ||
		!strings.Contains(module.LastError, "container.inventory: helper operation failed") {
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
	if module.State != "degraded" || !strings.Contains(module.LastError, privilegeHelperOperationContainerInventory) {
		t.Fatalf("container helper failure cleared with unrelated operations = %+v", module)
	}

	status.Record(privilegeHelperOperationContainerInventory, nil)
	module = status.ModuleStatus()
	if module.State != "running" || module.LastError != "" {
		t.Fatalf("complete helper recovery = %+v", module)
	}
}

func TestSharedPrivilegeHelperStatusReportsContainerFailureWithoutHostHelperCall(t *testing.T) {
	status := NewPrivilegeHelperStatus()
	status.Record(privilegeHelperOperationContainerInventory, &agenthelper.RemoteError{
		Code: agenthelper.ErrorProviderUnavailable, Message: "token=must-not-leak", RequestID: "secret-id",
	})
	agent := &Agent{privilegeHelperHealth: status}

	module := requirePrivilegeHelperModuleStatus(t, agent.currentModuleStatus())
	if module.State != "degraded" || module.LastError != "container.inventory: helper provider unavailable" {
		t.Fatalf("shared container helper status = %+v", module)
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
	status.Record(privilegeHelperOperationContainerInventory, &agenthelper.RemoteError{
		Code:      agenthelper.ErrorProviderUnavailable,
		Message:   "container provider token=container-secret",
		RequestID: "container-request-id",
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
		"container-secret",
		"container-request-id",
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

func TestCollectProxmoxPartialInventoryKeepsRowsAndDegradedHealth(t *testing.T) {
	helper := &fakePrivilegedTelemetry{proxmox: &agentshost.ProxmoxLXCInventory{
		Status: agentshost.ProxmoxLXCCollectionPartial, Containers: []agentshost.ProxmoxLXCContainer{{VMID: 100, Name: "web"}}, OmittedVMIDs: []int{102},
	}}
	status := NewPrivilegeHelperStatus()
	status.Record(privilegeHelperOperationSMART, errors.New("unrelated failure"))
	agent := &Agent{privilegedTelemetry: helper, privilegeHelperHealth: status, logger: zerolog.Nop()}
	got := agent.collectProxmoxLXCFilesystemsForReport(t.Context())
	if got == nil || len(got.Containers) != 1 || len(got.OmittedVMIDs) != 1 {
		t.Fatal("partial inventory was discarded")
	}
	module := status.ModuleStatus()
	if module.State != "degraded" || !strings.Contains(module.LastError, "inventory is incomplete") {
		t.Fatalf("module = %+v", module)
	}
	helper.proxmox = &agentshost.ProxmoxLXCInventory{Status: agentshost.ProxmoxLXCCollectionComplete}
	if agent.collectProxmoxLXCFilesystemsForReport(t.Context()) == nil {
		t.Fatal("complete recovery omitted")
	}
	module = status.ModuleStatus()
	if module.State != "degraded" || strings.Contains(module.LastError, "proxmox.lxc_filesystems") || !strings.Contains(module.LastError, privilegeHelperOperationSMART) {
		t.Fatalf("recovery cleared wrong health: %+v", module)
	}
}

func TestProxmoxHelperVersionCompatibilityIsNarrow(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		status    string
		omitted   []int
		wantErr   bool
		wantCalls int
	}{
		{"v2 partial", "", "partial", []int{102}, false, 1},
		{"older helper", agenthelper.ErrorUnsupportedOperation, "", nil, false, 2},
		{"provider failure", agenthelper.ErrorProviderUnavailable, "", nil, true, 1},
		{"peer denial", agenthelper.ErrorUnauthorizedPeer, "", nil, true, 1},
		{"missing v2 completeness", "", "", nil, true, 1},
		{"invalid v2 completeness", "", "complete", []int{102}, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := make(chan int, 3)
			client, err := agenthelper.NewClient(agenthelper.ClientConfig{
				SocketPath: filepath.Join(t.TempDir(), "helper.sock"), MaxDeadline: privilegeHelperOperationDeadline,
				DialContext: func(context.Context, string, string) (net.Conn, error) {
					clientConn, serverConn := net.Pipe()
					go func() {
						defer serverConn.Close()
						var header [4]byte
						if _, err := io.ReadFull(serverConn, header[:]); err != nil {
							return
						}
						b := make([]byte, binary.BigEndian.Uint32(header[:]))
						if _, err := io.ReadFull(serverConn, b); err != nil {
							return
						}
						var req agenthelper.Request
						if json.Unmarshal(b, &req) != nil {
							return
						}
						calls <- req.OperationVersion
						inventory := &agentshost.ProxmoxLXCInventory{Status: tt.status, Containers: []agentshost.ProxmoxLXCContainer{{VMID: 100, Name: "web"}}, OmittedVMIDs: tt.omitted}
						raw, _ := json.Marshal(struct {
							Inventory *agentshost.ProxmoxLXCInventory `json:"inventory"`
						}{inventory})
						response := agenthelper.Response{ProtocolVersion: 1, RequestID: req.RequestID, Operation: req.Operation, OperationVersion: req.OperationVersion, Success: true, Result: raw}
						if tt.code != "" && req.OperationVersion == 2 {
							response.Success = false
							response.Result = nil
							response.Error = &agenthelper.ResponseError{Code: tt.code}
						}
						b, _ = json.Marshal(response)
						frame := make([]byte, 4+len(b))
						binary.BigEndian.PutUint32(frame[:4], uint32(len(b)))
						copy(frame[4:], b)
						serverConn.Write(frame)
					}()
					return clientConn, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			inventory, err := (&privilegeHelperTelemetry{client: client}).ProxmoxLXCFilesystems(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("inventory=%+v, error=%v", inventory, err)
			}
			if len(calls) != tt.wantCalls {
				t.Fatalf("calls=%d, want %d", len(calls), tt.wantCalls)
			}
			for n := 0; n < tt.wantCalls; n++ {
				version := <-calls
				if version != 2-n {
					t.Fatalf("call %d version=%d", n, version)
				}
			}
		})
	}
}

func TestCollectProxmoxInvalidListCannotBecomeCompleteEmpty(t *testing.T) {
	header := "VMID Status Lock Name\n"
	boundary := header
	for n := 0; n < proxmoxLXCMaxContainers; n++ {
		boundary += fmt.Sprintf("%d running - web\n", 100+n)
	}
	if rows, err := parseProxmoxLXCRunningContainers(boundary); err != nil || len(rows) != proxmoxLXCMaxContainers {
		t.Fatalf("exact running-container boundary rejected: rows=%d err=%v", len(rows), err)
	}
	tests := []struct {
		name, output string
		valid        bool
	}{
		{"header only", header, true},
		{"all stopped", header + "100 stopped - web\n", true},
		{"empty output", "", false},
		{"unrecognised output", "pmxcfs unavailable\n", false},
		{"headerless rows", "100 running - web\n", false},
		{"malformed header", "VMID Status\n", false},
		{"invalid running identity", header + "99 running - web\n", false},
		{"invalid state", header + "100 unknown - web\n", false},
		{"duplicate identity", header + "100 running - web\n100 stopped - web\n", false},
		{"over running limit", boundary + "228 running - web\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := 0
			collector := &mockCollector{
				goos:       "linux",
				lookPathFn: func(string) (string, error) { return "/usr/sbin/pct", nil },
				commandCombinedOutputLimitedFn: func(_ context.Context, _ int, _ string, args ...string) (string, error) {
					queries++
					if strings.Join(args, " ") != "list" {
						t.Fatal("invalid or empty list must not start per-container queries")
					}
					return tt.output, nil
				},
			}
			result := (&Agent{logger: zerolog.Nop(), collector: collector}).collectProxmoxLXCFilesystemsResult(t.Context())
			if queries != 1 || !result.Applicable {
				t.Fatalf("collection did not use the one bounded list operation: %+v queries=%d", result, queries)
			}
			if tt.valid {
				if result.Degraded || result.Inventory == nil || result.Inventory.Status != "complete" || len(result.Inventory.Containers) != 0 {
					t.Fatalf("valid empty list=%+v", result)
				}
			} else if !result.Degraded || result.Inventory != nil {
				t.Fatalf("invalid list appeared complete: %+v", result)
			}
		})
	}
}

// The typed helper runs with PrivateNetwork=true, where pct and lxc-info cannot
// reach pmxcfs or the LXC monitor: their abstract Unix sockets belong to the
// host network namespace (#2511). This drives the collector the helper's
// provider runs, with every command failing as it does there: it must still
// return a complete inventory from configs, cgroups and /proc alone. A
// degraded result is what the provider turns into provider_unavailable.
func TestHelperProxmoxLXCFilesystemsNeedNoPctOrLXCSockets(t *testing.T) {
	h := newFakeProxmoxLXCHost()
	h.config(126, "hostname: qual2511\nrootfs: delly2-lvm:vm-126-disk-0,size=1G\nmp0: delly2-lvm:vm-126-disk-1,mp=/srv/data,size=1G\n")
	h.cgroup("126/ns", 3019253)
	h.proc(3019253, "3019253\t1", "/lxc/126/ns")
	h.config(127, "hostname: qual2511-priv\nrootfs: delly2-lvm:vm-127-disk-0,size=1G\n")
	h.cgroup("127/ns", 3021000)
	h.proc(3021000, "3021000\t1", "/lxc/127/ns")
	h.config(128, "hostname: stopped\nrootfs: delly2-lvm:vm-128-disk-0,size=1G\n")
	// Counters observed on a real PVE 9.2 node for the same layout.
	h.observations[3019253] = map[string]lxcObservation{
		"/":         lxcUsage(1020702720, 12021760, 938217472),
		"/srv/data": lxcUsage(1020702720, 290816, 949948416),
	}
	h.observations[3021000] = map[string]lxcObservation{"/": lxcUsage(1020702720, 12017664, 938221568)}

	collector, commands := h.collector(t)
	agent := &Agent{logger: zerolog.Nop(), collector: collector}
	result := agent.collectProxmoxLXCFilesystemsResult(context.Background())

	if !result.Applicable || result.Degraded || result.FailedContainers != 0 {
		t.Fatalf("collection = applicable %v, degraded %v, failed %d; want a complete inventory", result.Applicable, result.Degraded, result.FailedContainers)
	}
	if *commands != 0 {
		t.Fatalf("collection ran %d pct/lxc-info commands; none can work inside the helper", *commands)
	}
	got := result.Inventory.Containers
	if len(got) != 2 || got[0].VMID != 126 || got[0].Name != "qual2511" || got[1].VMID != 127 || got[1].Name != "qual2511-priv" {
		t.Fatalf("containers = %+v, want the two running ones by their pct names", got)
	}
	if len(got[0].Disks) != 2 || got[0].Disks[1].Mountpoint != "/srv/data" || got[0].Disks[1].UsedBytes != 290816 {
		t.Fatalf("126 disks = %+v, want rootfs and the mp0 mount with its own usage", got[0].Disks)
	}
}

func TestSocketFreeProxmoxLXCDiscoveryCannotTruncateCompleteInventory(t *testing.T) {
	for _, tc := range []struct {
		name     string
		running  int
		tail     string
		complete bool
	}{
		{"below limit", proxmoxLXCMaxContainers - 1, "", true},
		{"exact limit", proxmoxLXCMaxContainers, "", true},
		{"exact limit plus stopped", proxmoxLXCMaxContainers, "stopped", true},
		{"over limit", proxmoxLXCMaxContainers + 1, "", false},
		{"unknown after limit", proxmoxLXCMaxContainers, "unknown", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newFakeProxmoxLXCHost()
			for n := 0; n < tc.running; n++ {
				vmid, pid := 100+n, 1000+n
				h.config(vmid, fmt.Sprintf("hostname: ct%d\nrootfs: local:vm-%d-disk-0,size=1G\n", vmid, vmid))
				h.cgroup(strconv.Itoa(vmid)+"/ns", pid)
				h.proc(pid, fmt.Sprintf("%d\t1", pid), fmt.Sprintf("/lxc/%d/ns", vmid))
				h.observations[pid] = map[string]lxcObservation{"/": lxcUsage(4096, 1024, 3072)}
			}
			if tc.tail != "" {
				vmid := 100 + tc.running
				h.config(vmid, "hostname: tail\nrootfs: local:tail,size=1G\n")
				if tc.tail == "unknown" {
					h.cgroup(strconv.Itoa(vmid) + "/ns") // exists, but has no established init
				}
			}
			collector, commands := h.collector(t)
			agent := &Agent{logger: zerolog.Nop(), collector: collector}
			rows, ok := agent.discoverRunningProxmoxLXCContainers(t.Context())
			if ok != tc.complete || (ok && len(rows) != tc.running) || (!ok && rows != nil) || *commands != 0 {
				t.Fatalf("socket-free discovery: complete=%v rows=%d commands=%d", ok, len(rows), *commands)
			}
			result := agent.collectProxmoxLXCFilesystemsResult(t.Context())
			if tc.complete {
				if !result.Applicable || result.Degraded || result.Inventory == nil || result.Inventory.Status != "complete" || len(result.Inventory.Containers) != tc.running || *commands != 0 {
					t.Fatalf("complete boundary inventory=%+v commands=%d", result, *commands)
				}
				if last := result.Inventory.Containers[tc.running-1]; last.VMID != 99+tc.running || len(last.Disks) != 1 || last.Disks[0].UsedBytes != 1024 {
					t.Fatalf("boundary guest reading lost: %+v", last)
				}
			} else if !result.Applicable || !result.Degraded || result.Inventory != nil || *commands != 1 {
				t.Fatalf("unestablished node appeared complete instead of one failed pct fallback: %+v commands=%d", result, *commands)
			}
		})
	}
}
