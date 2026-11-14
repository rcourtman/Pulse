package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

func TestSavedTrueNASOutboundPolicyBindsWarmAndColdPolls(t *testing.T) {
	mock := newTrueNASMockServer(t, "saved-nas")
	defer mock.Close()
	mtp, store := newTestTenantPersistence(t)
	instance := trueNASInstanceForServer(t, "saved-connection", mock.URL(), true)
	if err := store.SaveTrueNASConfig([]config.TrueNASInstance{instance}); err != nil {
		t.Fatal(err)
	}
	poller := NewTrueNASPoller(mtp, time.Nanosecond, nil)
	defer poller.Stop()
	poller.syncConnections()
	poller.pollAll(context.Background())
	before := poller.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID]
	records := poller.GetCurrentRecordsForOrg("default")
	if before.Poll == nil || before.Poll.LastSuccessAt == nil || before.Poll.LastError != nil || len(records) == 0 || mock.RequestCount() == 0 {
		t.Fatal("instance-managed control did not collect")
	}
	requests := mock.RequestCount()
	// Same host/credentials, with newly saved tenant provenance. It must
	// invalidate the warm client, survive a fresh persistence read and fail
	// without deleting the last valid inventory or advancing its freshness.
	instance.OutboundPolicy = securityutil.ConnectionOutboundTenantLAN
	if err := store.SaveTrueNASConfig([]config.TrueNASInstance{instance}); err != nil {
		t.Fatal(err)
	}
	poller.syncConnections()
	poller.pollAll(context.Background())
	after := poller.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID]
	if mock.RequestCount() != requests {
		t.Fatal("warm tenant poll reached loopback")
	}
	if after.Poll.LastError == nil || !strings.Contains(after.Poll.LastError.Message, securityutil.ErrTenantOutboundDestination.Error()) || after.Poll.LastSuccessAt == nil || !after.Poll.LastSuccessAt.Equal(*before.Poll.LastSuccessAt) {
		t.Fatalf("failure lost truth/freshness: %+v", after.Poll)
	}
	if !reflect.DeepEqual(records, poller.GetCurrentRecordsForOrg("default")) {
		t.Fatal("failed observation discarded saved inventory")
	}
	freshMTP := config.NewMultiTenantPersistence(mtp.BaseDataDir())
	cold := NewTrueNASPoller(freshMTP, time.Nanosecond, nil)
	defer cold.Stop()
	cold.syncConnections()
	cold.pollAll(context.Background())
	status := cold.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID].Poll
	if mock.RequestCount() != requests || status.LastError == nil || status.LastSuccessAt != nil {
		t.Fatalf("cold tenant poll reached loopback or invented success: %+v", status)
	}
	loaded, err := store.LoadTrueNASConfig()
	if err != nil || len(loaded) != 1 || loaded[0].OutboundPolicy != instance.OutboundPolicy {
		t.Fatal("poll modified saved policy")
	}
	// An explicit instance-managed save can restore its ordinary local
	// monitoring. No retry silently strips the tenant restriction.
	instance.OutboundPolicy = ""
	if err := store.SaveTrueNASConfig([]config.TrueNASInstance{instance}); err != nil {
		t.Fatal(err)
	}
	poller.syncConnections()
	poller.pollAll(context.Background())
	recovered := poller.ConnectionSummaries("default", []config.TrueNASInstance{instance})[instance.ID].Poll
	if mock.RequestCount() <= requests || recovered.LastError != nil || recovered.LastSuccessAt == nil {
		t.Fatalf("instance-managed recovery failed: %+v", recovered)
	}
}

func TestSavedVMwareOutboundPolicyBindsWarmAndColdPolls(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	mtp, store := newTestTenantPersistence(t)
	instance := config.NewVMwareVCenterInstance()
	instance.ID = "saved-vc"
	instance.Host = server.URL
	instance.Username = "fixture"
	instance.Password = "synthetic"
	instance.InsecureSkipVerify = true
	if err := store.SaveVMwareConfig([]config.VMwareVCenterInstance{instance}); err != nil {
		t.Fatal(err)
	}
	poller := NewVMwarePoller(mtp, time.Nanosecond)
	defer poller.Stop()
	// The initial provider is a deterministic inventory fixture. The failed
	// successor and cold start below use the actual native API provider/client.
	poller.newProvider = func(config.VMwareVCenterInstance) (vmwarePollerProvider, error) {
		return vmware.NewProvider(vmware.InventorySnapshot{ConnectionID: instance.ID, ConnectionName: "Saved VC", VCenterHost: instance.Host, CollectedAt: time.Now().UTC(), Hosts: []vmware.InventoryHost{{Host: "host-1", Name: "Saved host", ConnectionState: "CONNECTED", PowerState: "POWERED_ON"}}}), nil
	}
	poller.syncConnections()
	poller.pollAll(context.Background())
	before := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{instance})[instance.ID].Poll
	records := poller.GetCurrentRecordsForOrg("default")
	if before == nil || before.LastSuccessAt == nil || len(records) == 0 {
		t.Fatal("initial fixture did not collect")
	}
	instance.OutboundPolicy = securityutil.ConnectionOutboundTenantLAN
	if err := store.SaveVMwareConfig([]config.VMwareVCenterInstance{instance}); err != nil {
		t.Fatal(err)
	}
	poller.newProvider = poller.defaultProvider
	poller.syncConnections()
	poller.pollAll(context.Background())
	after := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{instance})[instance.ID].Poll
	if hits.Load() != 0 || after.LastError == nil || after.LastError.Message != "refresh vmware inventory: This VMware destination requires an instance administrator." || after.LastSuccessAt == nil || !after.LastSuccessAt.Equal(*before.LastSuccessAt) {
		t.Fatalf("warm policy/freshness failure: hits=%d status=%+v", hits.Load(), after)
	}
	if !reflect.DeepEqual(records, poller.GetCurrentRecordsForOrg("default")) {
		t.Fatal("failed native read discarded inventory")
	}
	cold := NewVMwarePoller(config.NewMultiTenantPersistence(mtp.BaseDataDir()), time.Nanosecond)
	defer cold.Stop()
	cold.syncConnections()
	cold.pollAll(context.Background())
	status := cold.ConnectionSummaries("default", []config.VMwareVCenterInstance{instance})[instance.ID].Poll
	if hits.Load() != 0 || status.LastError == nil || status.LastError.Message != "refresh vmware inventory: This VMware destination requires an instance administrator." || status.LastSuccessAt != nil {
		t.Fatalf("cold policy/freshness failure: %+v", status)
	}
	loaded, err := store.LoadVMwareConfig()
	if err != nil || len(loaded) != 1 || loaded[0].OutboundPolicy != instance.OutboundPolicy {
		t.Fatal("poll changed saved policy")
	}
	// Only the explicit instance-managed save restores the original dial.
	instance.OutboundPolicy = ""
	if err := store.SaveVMwareConfig([]config.VMwareVCenterInstance{instance}); err != nil {
		t.Fatal(err)
	}
	cold.syncConnections()
	cold.pollAll(context.Background())
	if hits.Load() == 0 {
		t.Fatal("instance-managed control no longer reaches its local appliance")
	}
}
