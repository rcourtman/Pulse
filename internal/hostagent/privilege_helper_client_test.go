package hostagent

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

type fakePrivilegedTelemetry struct {
	smart        []DiskSMART
	smartErr     error
	proxmox      *agentshost.ProxmoxLXCInventory
	proxmoxErr   error
	smartCalls   int
	proxmoxCalls int
}

func (f *fakePrivilegedTelemetry) SMARTSnapshot(context.Context) ([]DiskSMART, error) {
	f.smartCalls++
	return f.smart, f.smartErr
}

func (f *fakePrivilegedTelemetry) ProxmoxLXCFilesystems(context.Context) (*agentshost.ProxmoxLXCInventory, error) {
	f.proxmoxCalls++
	return f.proxmox, f.proxmoxErr
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
