package vmware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCollectInventoryEnrichesSignals(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{})
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	snapshot, err := client.CollectInventory(context.Background())
	if err != nil {
		t.Fatalf("CollectInventory: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected inventory snapshot")
	}
	if len(snapshot.Hosts) != 1 || len(snapshot.VMs) != 1 || len(snapshot.Datastores) != 1 || len(snapshot.Networks) != 1 {
		t.Fatalf("unexpected inventory sizes: hosts=%d vms=%d datastores=%d networks=%d", len(snapshot.Hosts), len(snapshot.VMs), len(snapshot.Datastores), len(snapshot.Networks))
	}

	host := snapshot.Hosts[0]
	if host.OverallStatus != "yellow" {
		t.Fatalf("host overall status = %q, want yellow", host.OverallStatus)
	}
	if host.ClusterName != "Prod Compute" || host.DatacenterName != "DC1" {
		t.Fatalf("expected host placement enrichment, got cluster=%q datacenter=%q", host.ClusterName, host.DatacenterName)
	}
	if host.ClusterHAEnabled == nil || !*host.ClusterHAEnabled || host.ClusterDRSEnabled == nil || *host.ClusterDRSEnabled {
		t.Fatalf("expected host cluster service enrichment HA=true DRS=false, got ha=%+v drs=%+v", host.ClusterHAEnabled, host.ClusterDRSEnabled)
	}
	if len(host.DatastoreNames) != 1 || host.DatastoreNames[0] != "nvme-primary" {
		t.Fatalf("expected host datastore names, got %+v", host.DatastoreNames)
	}
	if host.Metrics == nil || host.Metrics.CPUPercent == nil || *host.Metrics.CPUPercent != 21.4 {
		t.Fatalf("expected host cpu metrics, got %+v", host.Metrics)
	}
	if len(host.TriggeredAlarms) != 1 || host.TriggeredAlarms[0].Name != "Host connection degraded" {
		t.Fatalf("expected resolved host alarm metadata, got %+v", host.TriggeredAlarms)
	}
	if len(host.RecentTasks) != 1 || host.RecentTasks[0].Name != "Reconnect host" {
		t.Fatalf("expected host recent task summary, got %+v", host.RecentTasks)
	}
	if len(host.RecentEvents) != 1 || host.RecentEvents[0].Type != "HostConnectionStateEvent" {
		t.Fatalf("expected host recent event summary, got %+v", host.RecentEvents)
	}

	vm := snapshot.VMs[0]
	if vm.Metrics == nil || vm.Metrics.MemoryPercent == nil || *vm.Metrics.MemoryPercent != 57.5 {
		t.Fatalf("expected VM memory metrics, got %+v", vm.Metrics)
	}
	if len(vm.TriggeredAlarms) != 1 || vm.TriggeredAlarms[0].OverallStatus != "red" {
		t.Fatalf("expected VM alarm signals, got %+v", vm.TriggeredAlarms)
	}
	if vm.RuntimeHostName != "esxi-01.lab.local" || vm.ResourcePoolName != "Tier 1" {
		t.Fatalf("expected VM placement enrichment, got host=%q pool=%q", vm.RuntimeHostName, vm.ResourcePoolName)
	}
	if vm.ClusterHAEnabled == nil || !*vm.ClusterHAEnabled || vm.ClusterDRSEnabled == nil || *vm.ClusterDRSEnabled {
		t.Fatalf("expected VM cluster service enrichment HA=true DRS=false, got ha=%+v drs=%+v", vm.ClusterHAEnabled, vm.ClusterDRSEnabled)
	}
	if vm.InstanceUUID != "vm-instance-201" || vm.BIOSUUID != "vm-bios-201" {
		t.Fatalf("expected VM identity enrichment, got instance=%q bios=%q", vm.InstanceUUID, vm.BIOSUUID)
	}
	if vm.GuestHostname != "app-01.internal" || len(vm.GuestIPAddresses) != 1 || vm.GuestIPAddresses[0] != "10.0.0.21" {
		t.Fatalf("expected VM guest identity enrichment, got host=%q ips=%v", vm.GuestHostname, vm.GuestIPAddresses)
	}
	if vm.Hardware == nil {
		t.Fatalf("expected VM hardware enrichment")
	}
	if vm.Hardware.Version != "VMX_20" || vm.Hardware.UpgradePolicy != "AFTER_CLEAN_SHUTDOWN" || vm.Hardware.UpgradeStatus != "PENDING" {
		t.Fatalf("unexpected VM hardware upgrade context: %+v", vm.Hardware)
	}
	if vm.Hardware.GuestOS != "UBUNTU_64" {
		t.Fatalf("expected VM hardware guest OS, got %+v", vm.Hardware.GuestOS)
	}
	if vm.CPUCount != 6 || vm.Hardware.CPUCoresPerSocket == nil || *vm.Hardware.CPUCoresPerSocket != 3 {
		t.Fatalf("expected VM CPU topology from VM detail, got cpu=%d hardware=%+v", vm.CPUCount, vm.Hardware)
	}
	if vm.MemorySizeMiB != 12288 || vm.Hardware.MemoryHotAddLimitMiB == nil || *vm.Hardware.MemoryHotAddLimitMiB != 24576 {
		t.Fatalf("expected VM memory detail from VM detail, got memory=%d hardware=%+v", vm.MemorySizeMiB, vm.Hardware)
	}
	if len(vm.Hardware.BootDevices) != 2 || vm.Hardware.BootDevices[0].Disks[0] != "2000" || vm.Hardware.BootDevices[1].NIC != "4000" {
		t.Fatalf("expected VM boot device order, got %+v", vm.Hardware.BootDevices)
	}
	if vm.Tools == nil {
		t.Fatalf("expected VM tools enrichment")
	}
	if vm.Tools.RunState != "RUNNING" || vm.Tools.VersionStatus != "CURRENT" || vm.Tools.Version != "12.4.0" {
		t.Fatalf("unexpected VM tools status: %+v", vm.Tools)
	}
	if vm.Tools.AutoUpdateSupported == nil || !*vm.Tools.AutoUpdateSupported {
		t.Fatalf("expected VM tools auto-update support, got %+v", vm.Tools.AutoUpdateSupported)
	}
	if vm.Tools.InstallAttemptCount == nil || *vm.Tools.InstallAttemptCount != 1 {
		t.Fatalf("expected VM tools install attempt count, got %+v", vm.Tools.InstallAttemptCount)
	}
	if vm.Tools.GuestRebootRequested == nil || !*vm.Tools.GuestRebootRequested || len(vm.Tools.GuestRebootComponents) != 1 || vm.Tools.GuestRebootComponents[0] != "drivers" {
		t.Fatalf("expected VM tools reboot request context, got %+v", vm.Tools)
	}
	if len(vm.NetworkAdapters) != 1 {
		t.Fatalf("expected one VM network adapter, got %+v", vm.NetworkAdapters)
	}
	adapter := vm.NetworkAdapters[0]
	if adapter.NIC != "4000" || adapter.Label != "Network adapter 1" || adapter.Type != "VMXNET3" {
		t.Fatalf("unexpected VM network adapter identity: %+v", adapter)
	}
	if adapter.MACAddress != "00:50:56:aa:bb:cc" || adapter.NetworkName != "VM Network" || adapter.State != "CONNECTED" {
		t.Fatalf("unexpected VM network adapter backing/state: %+v", adapter)
	}
	if adapter.PCISlotNumber == nil || *adapter.PCISlotNumber != 160 {
		t.Fatalf("expected VM network adapter PCI slot, got %+v", adapter.PCISlotNumber)
	}
	if !adapter.StartConnected || !adapter.AllowGuestControl || !adapter.WakeOnLANEnabled {
		t.Fatalf("expected VM network adapter connection flags, got %+v", adapter)
	}
	if len(vm.VirtualDisks) != 1 {
		t.Fatalf("expected one VM virtual disk, got %+v", vm.VirtualDisks)
	}
	disk := vm.VirtualDisks[0]
	if disk.Disk != "2000" || disk.Label != "Hard disk 1" || disk.Type != "SCSI" {
		t.Fatalf("unexpected VM virtual disk identity: %+v", disk)
	}
	if disk.SCSIBus == nil || *disk.SCSIBus != 0 || disk.SCSIUnit == nil || *disk.SCSIUnit != 1 {
		t.Fatalf("expected VM virtual disk SCSI address, got bus=%+v unit=%+v", disk.SCSIBus, disk.SCSIUnit)
	}
	if disk.BackingType != "VMDK_FILE" || disk.VMDKFile != "[nvme-primary] app-01/app-01.vmdk" || disk.DatastoreName != "nvme-primary" {
		t.Fatalf("unexpected VM virtual disk backing: %+v", disk)
	}
	if disk.CapacityBytes == nil || *disk.CapacityBytes != 107374182400 {
		t.Fatalf("expected VM virtual disk capacity, got %+v", disk.CapacityBytes)
	}
	if vm.SnapshotCount != 2 {
		t.Fatalf("vm snapshot count = %d, want 2", vm.SnapshotCount)
	}
	if vm.CurrentSnapshotID != "snapshot-202" {
		t.Fatalf("vm current snapshot id = %q, want snapshot-202", vm.CurrentSnapshotID)
	}
	if len(vm.SnapshotTree) != 1 || vm.SnapshotTree[0].Name != "pre-upgrade" || len(vm.SnapshotTree[0].Children) != 1 {
		t.Fatalf("expected VM snapshot tree with one child, got %+v", vm.SnapshotTree)
	}
	if !vm.SnapshotTree[0].Quiesced || !vm.SnapshotTree[0].Children[0].Current {
		t.Fatalf("expected quiesced root and current child snapshot, got %+v", vm.SnapshotTree)
	}
	if len(vm.RecentTasks) != 1 || vm.RecentTasks[0].State != "success" {
		t.Fatalf("expected VM recent task info, got %+v", vm.RecentTasks)
	}
	if len(vm.RecentEvents) != 1 || vm.RecentEvents[0].Type != "VmMessageEvent" {
		t.Fatalf("expected VM recent event info, got %+v", vm.RecentEvents)
	}

	datastore := snapshot.Datastores[0]
	if datastore.OverallStatus != "yellow" {
		t.Fatalf("datastore overall status = %q, want yellow", datastore.OverallStatus)
	}
	if datastore.Accessible == nil || !*datastore.Accessible {
		t.Fatalf("expected datastore accessibility enrichment, got %+v", datastore.Accessible)
	}
	if len(datastore.HostNames) != 1 || datastore.HostNames[0] != "esxi-01.lab.local" {
		t.Fatalf("expected datastore host attachment enrichment, got %+v", datastore.HostNames)
	}
	if len(datastore.VMNames) != 1 || datastore.VMNames[0] != "app-01" {
		t.Fatalf("expected datastore VM attachment enrichment, got %+v", datastore.VMNames)
	}
	if len(datastore.RecentTasks) != 1 || datastore.RecentTasks[0].Name != "Refresh datastore" {
		t.Fatalf("expected datastore recent task info, got %+v", datastore.RecentTasks)
	}
	if len(datastore.RecentEvents) != 1 || datastore.RecentEvents[0].Type != "DatastoreRenamedEvent" {
		t.Fatalf("expected datastore recent event info, got %+v", datastore.RecentEvents)
	}

	network := snapshot.Networks[0]
	if network.OverallStatus != "green" {
		t.Fatalf("network overall status = %q, want green", network.OverallStatus)
	}
	if network.DatacenterName != "DC1" || network.FolderName != "Networks" {
		t.Fatalf("expected network placement enrichment, got datacenter=%q folder=%q", network.DatacenterName, network.FolderName)
	}
	if len(network.HostNames) != 1 || network.HostNames[0] != "esxi-01.lab.local" {
		t.Fatalf("expected network host attachment enrichment, got %+v", network.HostNames)
	}
	if len(network.VMNames) != 1 || network.VMNames[0] != "app-01" {
		t.Fatalf("expected network VM attachment enrichment, got %+v", network.VMNames)
	}
	if len(network.RecentEvents) != 1 || network.RecentEvents[0].Type != "NetworkEvent" {
		t.Fatalf("expected network recent event info, got %+v", network.RecentEvents)
	}
}

func TestClientCollectInventoryPreservesBaseInventoryWhenOptionalEnrichmentDegrades(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{
		denyHostOverallStatus:  true,
		unavailableVMGuestInfo: true,
		denyClusterInventory:   true,
	})
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	snapshot, err := client.CollectInventory(context.Background())
	if err != nil {
		t.Fatalf("CollectInventory: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected inventory snapshot")
	}
	if len(snapshot.Hosts) != 1 || len(snapshot.VMs) != 1 || len(snapshot.Datastores) != 1 || len(snapshot.Networks) != 1 {
		t.Fatalf("unexpected inventory sizes: hosts=%d vms=%d datastores=%d networks=%d", len(snapshot.Hosts), len(snapshot.VMs), len(snapshot.Datastores), len(snapshot.Networks))
	}
	// The unavailableVMGuestInfo knob now degrades two REST reads: the
	// guest identity endpoint (topology stage) and the guest local
	// filesystem endpoint (signals stage). That plus the per-stage
	// permission denials gives four issues.
	if len(snapshot.EnrichmentIssues) != 4 {
		t.Fatalf("expected 4 enrichment issues, got %+v", snapshot.EnrichmentIssues)
	}
	seen := make(map[string]InventoryEnrichmentIssue, len(snapshot.EnrichmentIssues))
	for _, issue := range snapshot.EnrichmentIssues {
		key := issue.Stage + "/" + issue.EntityType + "/" + issue.Category
		seen[key] = issue
	}
	if issue, ok := seen["signals/host/permission"]; !ok {
		t.Fatalf("expected signals/host permission issue, got %+v", snapshot.EnrichmentIssues)
	} else if !strings.Contains(issue.Message, "overall status") {
		t.Fatalf("unexpected signals/host permission message: %+v", issue)
	}
	if issue, ok := seen["signals/vm/unavailable"]; !ok {
		t.Fatalf("expected signals/vm unavailable issue for guest local filesystem, got %+v", snapshot.EnrichmentIssues)
	} else if !strings.Contains(issue.Message, "guest local filesystem") {
		t.Fatalf("unexpected signals/vm unavailable message: %+v", issue)
	}
	if issue, ok := seen["topology/cluster/permission"]; !ok {
		t.Fatalf("expected topology/cluster permission issue, got %+v", snapshot.EnrichmentIssues)
	} else if !strings.Contains(issue.Message, "cluster inventory") {
		t.Fatalf("unexpected topology/cluster message: %+v", issue)
	}
	if issue, ok := seen["topology/vm/unavailable"]; !ok {
		t.Fatalf("expected topology/vm unavailable issue for guest identity, got %+v", snapshot.EnrichmentIssues)
	} else if !strings.Contains(issue.Message, "guest identity") {
		t.Fatalf("unexpected topology/vm unavailable message: %+v", issue)
	}

	host := snapshot.Hosts[0]
	if host.OverallStatus != "" {
		t.Fatalf("expected missing host overall status after degraded read, got %q", host.OverallStatus)
	}
	if host.Metrics == nil || host.Metrics.CPUPercent == nil || *host.Metrics.CPUPercent != 21.4 {
		t.Fatalf("expected host metrics to survive degraded signal read, got %+v", host.Metrics)
	}
	if host.ClusterName != "Prod Compute" || host.DatacenterName != "DC1" {
		t.Fatalf("expected host placement enrichment to survive degraded signal read, got cluster=%q datacenter=%q", host.ClusterName, host.DatacenterName)
	}
	if host.ClusterHAEnabled != nil || host.ClusterDRSEnabled != nil {
		t.Fatalf("expected cluster services to stay empty after degraded cluster inventory read, got ha=%+v drs=%+v", host.ClusterHAEnabled, host.ClusterDRSEnabled)
	}

	vm := snapshot.VMs[0]
	if vm.GuestHostname != "" || len(vm.GuestIPAddresses) != 0 {
		t.Fatalf("expected guest identity to stay empty after degraded topology read, got host=%q ips=%v", vm.GuestHostname, vm.GuestIPAddresses)
	}
	if vm.Hardware == nil || vm.Hardware.Version != "VMX_20" {
		t.Fatalf("expected hardware enrichment to survive degraded guest read, got %+v", vm.Hardware)
	}
	if vm.Tools == nil || vm.Tools.RunState != "RUNNING" {
		t.Fatalf("expected tools enrichment to survive degraded guest read, got %+v", vm.Tools)
	}
	if len(vm.NetworkAdapters) != 1 || vm.NetworkAdapters[0].NetworkName != "VM Network" {
		t.Fatalf("expected network adapter enrichment to survive degraded guest read, got %+v", vm.NetworkAdapters)
	}
	if len(vm.VirtualDisks) != 1 || vm.VirtualDisks[0].DatastoreName != "nvme-primary" {
		t.Fatalf("expected virtual disk enrichment to survive degraded guest read, got %+v", vm.VirtualDisks)
	}
	if vm.RuntimeHostName != "esxi-01.lab.local" || vm.ResourcePoolName != "Tier 1" {
		t.Fatalf("expected other topology enrichment to survive degraded guest read, got host=%q pool=%q", vm.RuntimeHostName, vm.ResourcePoolName)
	}
	if vm.ClusterHAEnabled != nil || vm.ClusterDRSEnabled != nil {
		t.Fatalf("expected VM cluster services to stay empty after degraded cluster inventory read, got ha=%+v drs=%+v", vm.ClusterHAEnabled, vm.ClusterDRSEnabled)
	}
}

func TestClientTestConnectionReportsUnavailableOptionalSignalAsDegraded(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{
		omitHostOverallStatus: true,
	})
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	summary, err := client.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if summary == nil || !summary.Degraded {
		t.Fatalf("expected degraded connection summary, got %+v", summary)
	}
	if len(summary.Issues) != 1 ||
		summary.Issues[0].Stage != "signals" ||
		summary.Issues[0].EntityType != "host" ||
		summary.Issues[0].Category != "not_found" ||
		!strings.Contains(summary.Issues[0].Message, "overall status") {
		t.Fatalf("expected unavailable host signal diagnostic, got %+v", summary.Issues)
	}
}

func TestClientTestConnectionStillRejectsSignalAuthenticationFailure(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{
		unauthorizedHostOverallStatus: true,
	})
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "auth" {
		t.Fatalf("connection error category = %q, want auth", connectionErr.Category)
	}
}

func TestClientResolveVIJSONReleaseFallsBackToSupportedProbeFloor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/8.0.3/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"sessionManager": map[string]any{"value": "SessionManager"},
			"perfManager":    map[string]any{"value": "PerformanceManager"},
			"eventManager":   map[string]any{"value": "EventManager"},
		}); err != nil {
			t.Fatalf("encode service content: %v", err)
		}
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	release, refs, err := client.resolveVIJSONRelease(context.Background())
	if err != nil {
		t.Fatalf("resolveVIJSONRelease: %v", err)
	}
	if release != "8.0.3" {
		t.Fatalf("release = %q, want 8.0.3", release)
	}
	if refs.SessionManagerMoID != "SessionManager" || refs.PerfManagerMoID != "PerformanceManager" || refs.EventManagerMoID != "EventManager" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

// Issue #1596: vCenter 8.0.3 answers the newer 9.0.0.0 release path with
// HTTP 500 rather than 404. The negotiation must fall through to the next
// release instead of aborting on the first probe.
func TestClientResolveVIJSONReleaseFallsThroughServerErrorProbe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/9.0.0.0/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	mux.HandleFunc("/sdk/vim25/8.0.3/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"sessionManager": map[string]any{"value": "SessionManager"},
			"perfManager":    map[string]any{"value": "PerformanceManager"},
			"eventManager":   map[string]any{"value": "EventManager"},
		}); err != nil {
			t.Fatalf("encode service content: %v", err)
		}
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	release, refs, err := client.resolveVIJSONRelease(context.Background())
	if err != nil {
		t.Fatalf("resolveVIJSONRelease: %v", err)
	}
	if release != "8.0.3" {
		t.Fatalf("release = %q, want 8.0.3", release)
	}
	if refs.SessionManagerMoID != "SessionManager" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

// Auth failures on the probe must still abort immediately: retrying other
// release strings cannot fix credentials.
func TestClientResolveVIJSONReleaseAbortsOnAuthProbe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, _, err = client.resolveVIJSONRelease(context.Background())
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T (%v)", err, err)
	}
	if connectionErr.Category != "auth" {
		t.Fatalf("connection error category = %q, want auth", connectionErr.Category)
	}
}

func TestClientResolveVIJSONReleaseClassifiesUnsupportedVersionFloor(t *testing.T) {
	server := httptest.NewTLSServer(http.NewServeMux())
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, _, err = client.resolveVIJSONRelease(context.Background())
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "unsupported_version" {
		t.Fatalf("connection error category = %q, want unsupported_version", connectionErr.Category)
	}
	if !strings.Contains(connectionErr.Message, "9.0.0.0") || !strings.Contains(connectionErr.Message, "8.0.1.0") {
		t.Fatalf("expected message to mention implemented probe floor, got %q", connectionErr.Message)
	}
}

type vmwareTestServerConfig struct {
	omitHostOverallStatus         bool
	denyHostOverallStatus         bool
	unauthorizedHostOverallStatus bool
	denyClusterInventory          bool
	unavailableVMGuestInfo        bool
}

func newVMwareTestServer(t *testing.T, cfg vmwareTestServerConfig) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, payload any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}

	mux.HandleFunc("/api/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, "automation-session")
	})

	mux.HandleFunc("/api/vcenter/host", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryHost{{
			Host:            "host-101",
			Name:            "esxi-01.lab.local",
			ConnectionState: "CONNECTED",
			PowerState:      "POWERED_ON",
			HostUUID:        "uuid-host-1",
		}})
	})
	mux.HandleFunc("/api/vcenter/vm", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryVM{{
			VM:            "vm-201",
			Name:          "app-01",
			PowerState:    "POWERED_ON",
			CPUCount:      4,
			MemorySizeMiB: 8192,
		}})
	})
	mux.HandleFunc("/api/vcenter/datastore", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryDatastore{{
			Datastore: "datastore-11",
			Name:      "nvme-primary",
			Type:      "VMFS",
			FreeSpace: 40,
			Capacity:  100,
		}})
	})
	mux.HandleFunc("/api/vcenter/network", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryNetwork{{
			Network: "network-101",
			Name:    "VM Network",
			Type:    "STANDARD_PORTGROUP",
		}})
	})
	mux.HandleFunc("/api/vcenter/cluster", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		if cfg.denyClusterInventory {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		writeJSON(w, []map[string]any{{
			"cluster":     "domain-c101",
			"name":        "Prod Compute",
			"ha_enabled":  true,
			"drs_enabled": false,
		}})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, map[string]any{
			"guest_os":             "UBUNTU_64",
			"power_state":          "POWERED_ON",
			"instant_clone_frozen": false,
			"identity": map[string]any{
				"bios_uuid":     "vm-bios-201",
				"instance_uuid": "vm-instance-201",
			},
			"hardware": map[string]any{
				"version":         "VMX_20",
				"upgrade_policy":  "AFTER_CLEAN_SHUTDOWN",
				"upgrade_version": "VMX_21",
				"upgrade_status":  "PENDING",
			},
			"boot": map[string]any{
				"type":             "EFI",
				"efi_legacy_boot":  false,
				"network_protocol": "IPV4",
				"delay":            5000,
				"retry":            true,
				"retry_delay":      10000,
				"enter_setup_mode": false,
			},
			"boot_devices": []map[string]any{
				{"type": "DISK", "disks": []string{"2000"}},
				{"type": "ETHERNET", "nic": "4000"},
			},
			"cpu": map[string]any{
				"count":              6,
				"cores_per_socket":   3,
				"hot_add_enabled":    true,
				"hot_remove_enabled": false,
			},
			"memory": map[string]any{
				"size_mib":                   12288,
				"hot_add_enabled":            true,
				"hot_add_increment_size_mib": 256,
				"hot_add_limit_mib":          24576,
			},
		})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/guest/identity", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		if cfg.unavailableVMGuestInfo {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]any{
			"family":     "LINUX",
			"host_name":  "app-01.internal",
			"ip_address": "10.0.0.21",
		})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/guest/local-filesystem", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		// VMware Tools reports guest filesystem usage; the unavailable knob
		// stands in for "Tools not running" which vCenter signals via 503.
		if cfg.unavailableVMGuestInfo {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]any{
			"/": map[string]any{
				"capacity":   int64(50 * 1024 * 1024 * 1024),
				"free_space": int64(20 * 1024 * 1024 * 1024),
				"filesystem": "ext4",
			},
			"/var": map[string]any{
				"capacity":   int64(20 * 1024 * 1024 * 1024),
				"free_space": int64(5 * 1024 * 1024 * 1024),
				"filesystem": "ext4",
			},
		})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/tools", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, map[string]any{
			"auto_update_supported": true,
			"install_attempt_count": 1,
			"version_number":        12352,
			"version":               "12.4.0",
			"upgrade_policy":        "MANUAL",
			"version_status":        "CURRENT",
			"install_type":          "OPEN_VM_TOOLS",
			"run_state":             "RUNNING",
			"guest_reboot_status": map[string]any{
				"reboot_requested":      true,
				"requesting_components": []string{"drivers"},
				"request_timestamp":     "2026-03-30T18:20:00Z",
			},
		})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/hardware/ethernet", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []map[string]any{{"nic": "4000"}})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/hardware/ethernet/4000", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, map[string]any{
			"label":                        "Network adapter 1",
			"type":                         "VMXNET3",
			"upt_compatibility_enabled":    true,
			"upt_v2_compatibility_enabled": false,
			"mac_type":                     "GENERATED",
			"mac_address":                  "00:50:56:aa:bb:cc",
			"pci_slot_number":              160,
			"wake_on_lan_enabled":          true,
			"backing": map[string]any{
				"type":                    "STANDARD_PORTGROUP",
				"network":                 "network-101",
				"network_name":            "VM Network",
				"distributed_switch_uuid": "",
				"distributed_port":        "",
			},
			"state":               "CONNECTED",
			"start_connected":     true,
			"allow_guest_control": true,
		})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/hardware/disk", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []map[string]any{{"disk": "2000"}})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/hardware/disk/2000", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, map[string]any{
			"label": "Hard disk 1",
			"type":  "SCSI",
			"scsi": map[string]any{
				"bus":  0,
				"unit": 1,
			},
			"backing": map[string]any{
				"type":      "VMDK_FILE",
				"vmdk_file": "[nvme-primary] app-01/app-01.vmdk",
			},
			"capacity": 107374182400,
		})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"sessionManager": map[string]any{"value": "SessionManager"},
			"perfManager":    map[string]any{"value": "PerformanceManager"},
			"eventManager":   map[string]any{"value": "EventManager"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/SessionManager/SessionManager/Login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("vmware-api-session-id", "vi-session")
		writeJSON(w, map[string]any{"value": "ok"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/perfCounter", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{
			{"key": 1, "groupInfo": map[string]any{"key": "cpu"}, "nameInfo": map[string]any{"key": "usage"}, "rollupType": "average"},
			{"key": 2, "groupInfo": map[string]any{"key": "mem"}, "nameInfo": map[string]any{"key": "usage"}, "rollupType": "average"},
			{"key": 3, "groupInfo": map[string]any{"key": "mem"}, "nameInfo": map[string]any{"key": "totalCapacity"}, "rollupType": "average"},
			{"key": 4, "groupInfo": map[string]any{"key": "net"}, "nameInfo": map[string]any{"key": "bytesRx"}, "rollupType": "average"},
			{"key": 5, "groupInfo": map[string]any{"key": "net"}, "nameInfo": map[string]any{"key": "bytesTx"}, "rollupType": "average"},
			{"key": 6, "groupInfo": map[string]any{"key": "disk"}, "nameInfo": map[string]any{"key": "read"}, "rollupType": "average"},
			{"key": 7, "groupInfo": map[string]any{"key": "disk"}, "nameInfo": map[string]any{"key": "write"}, "rollupType": "average"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryPerfProviderSummary", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"currentSupported": true,
			"refreshRate":      20,
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryAvailablePerfMetric", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		var request struct {
			Entity struct {
				Type string `json:"type"`
			} `json:"entity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode available perf request: %v", err)
		}
		switch request.Entity.Type {
		case "HostSystem":
			writeJSON(w, []map[string]any{
				{"counterId": 1, "instance": ""},
				{"counterId": 2, "instance": ""},
				{"counterId": 3, "instance": ""},
				{"counterId": 4, "instance": "vmnic0"},
				{"counterId": 5, "instance": "vmnic0"},
				{"counterId": 6, "instance": "naa.1"},
				{"counterId": 7, "instance": "naa.1"},
			})
		case "VirtualMachine":
			writeJSON(w, []map[string]any{
				{"counterId": 1, "instance": ""},
				{"counterId": 2, "instance": ""},
				{"counterId": 4, "instance": "4000"},
				{"counterId": 5, "instance": "4000"},
				{"counterId": 6, "instance": "2000"},
				{"counterId": 7, "instance": "2000"},
			})
		default:
			writeJSON(w, []map[string]any{})
		}
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryPerf", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		var request struct {
			QuerySpec []struct {
				Entity struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"entity"`
			} `json:"querySpec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode query perf request: %v", err)
		}
		if len(request.QuerySpec) != 1 {
			t.Fatalf("expected single query spec, got %d", len(request.QuerySpec))
		}
		switch request.QuerySpec[0].Entity.Type {
		case "HostSystem":
			writeJSON(w, []map[string]any{{
				"entity": map[string]any{"type": "HostSystem", "value": "host-101"},
				"value": []map[string]any{
					{"id": map[string]any{"counterId": 1, "instance": ""}, "value": []int64{2140}},
					{"id": map[string]any{"counterId": 2, "instance": ""}, "value": []int64{6320}},
					{"id": map[string]any{"counterId": 3, "instance": ""}, "value": []int64{40960}},
					{"id": map[string]any{"counterId": 4, "instance": "vmnic0"}, "value": []int64{1}},
					{"id": map[string]any{"counterId": 5, "instance": "vmnic0"}, "value": []int64{2}},
					{"id": map[string]any{"counterId": 6, "instance": "naa.1"}, "value": []int64{4}},
					{"id": map[string]any{"counterId": 7, "instance": "naa.1"}, "value": []int64{8}},
				},
			}})
		case "VirtualMachine":
			writeJSON(w, []map[string]any{{
				"entity": map[string]any{"type": "VirtualMachine", "value": "vm-201"},
				"value": []map[string]any{
					{"id": map[string]any{"counterId": 1, "instance": ""}, "value": []int64{3810}},
					{"id": map[string]any{"counterId": 2, "instance": ""}, "value": []int64{5750}},
					{"id": map[string]any{"counterId": 4, "instance": "4000"}, "value": []int64{3}},
					{"id": map[string]any{"counterId": 5, "instance": "4000"}, "value": []int64{4}},
					{"id": map[string]any{"counterId": 6, "instance": "2000"}, "value": []int64{5}},
					{"id": map[string]any{"counterId": 7, "instance": "2000"}, "value": []int64{6}},
				},
			}})
		default:
			writeJSON(w, []map[string]any{})
		}
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/EventManager/EventManager/QueryEvents", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		var request struct {
			Filter struct {
				Entity *struct {
					Entity struct {
						Type  string `json:"type"`
						Value string `json:"value"`
					} `json:"entity"`
					Recursion string `json:"recursion"`
				} `json:"entity"`
				MaxCount int `json:"maxCount"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode query events request: %v", err)
		}
		if request.Filter.Entity == nil {
			t.Fatal("expected event filter entity")
		}
		if request.Filter.Entity.Recursion != "self" {
			t.Fatalf("event recursion = %q, want self", request.Filter.Entity.Recursion)
		}
		if request.Filter.MaxCount != 3 {
			t.Fatalf("event maxCount = %d, want 3", request.Filter.MaxCount)
		}
		switch request.Filter.Entity.Entity.Type {
		case "HostSystem":
			writeJSON(w, []map[string]any{{
				"key":                  101,
				"userName":             "administrator@vsphere.local",
				"createdTime":          "2026-03-30T18:15:00Z",
				"fullFormattedMessage": "Host entered maintenance evaluation",
				"eventTypeId":          "HostConnectionStateEvent",
			}})
		case "VirtualMachine":
			writeJSON(w, []map[string]any{{
				"key":                  201,
				"userName":             "vpxuser",
				"createdTime":          "2026-03-30T18:12:00Z",
				"fullFormattedMessage": "Snapshot completed successfully",
				"eventTypeId":          "VmMessageEvent",
			}})
		case "Datastore":
			writeJSON(w, []map[string]any{{
				"key":                  301,
				"userName":             "storage-admin",
				"createdTime":          "2026-03-30T18:09:00Z",
				"fullFormattedMessage": "Datastore metadata refreshed",
				"eventTypeId":          "DatastoreRenamedEvent",
			}})
		case "Network":
			writeJSON(w, []map[string]any{{
				"key":                  401,
				"userName":             "network-admin",
				"createdTime":          "2026-03-30T18:07:00Z",
				"fullFormattedMessage": "Network metadata refreshed",
				"eventTypeId":          "NetworkEvent",
			}})
		default:
			writeJSON(w, []map[string]any{})
		}
	})

	if !cfg.omitHostOverallStatus {
		mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/overallStatus", func(w http.ResponseWriter, r *http.Request) {
			requireVISession(t, r)
			if cfg.unauthorizedHostOverallStatus {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if cfg.denyHostOverallStatus {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			writeJSON(w, "yellow")
		})
	}
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{
			"alarm":         map[string]any{"type": "Alarm", "value": "alarm-11"},
			"overallStatus": "yellow",
			"acknowledged":  false,
			"time":          "2026-03-30T18:12:00Z",
		}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Task", "value": "task-11"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "esxi-01.lab.local")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "ClusterComputeResource", "value": "domain-c101"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/datastore", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Datastore", "value": "datastore-11"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/overallStatus", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "green")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{
			"alarm":         map[string]any{"type": "Alarm", "value": "alarm-21"},
			"overallStatus": "red",
			"acknowledged":  false,
			"time":          "2026-03-30T18:13:00Z",
		}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Task", "value": "task-21"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/snapshot", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"currentSnapshot": map[string]any{"type": "VirtualMachineSnapshot", "value": "snapshot-202"},
			"rootSnapshotList": []map[string]any{{
				"snapshot":    map[string]any{"type": "VirtualMachineSnapshot", "value": "snapshot-201"},
				"name":        "pre-upgrade",
				"description": "Before application upgrade",
				"id":          101,
				"createTime":  "2026-03-28T18:15:00Z",
				"state":       "poweredOn",
				"quiesced":    true,
				"childSnapshotList": []map[string]any{{
					"snapshot":          map[string]any{"type": "VirtualMachineSnapshot", "value": "snapshot-202"},
					"name":              "post-migration-checkpoint",
					"id":                102,
					"createTime":        "2026-03-29T18:15:00Z",
					"state":             "poweredOn",
					"quiesced":          false,
					"childSnapshotList": []map[string]any{},
				}},
			}},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-v7"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/runtime", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"host": map[string]any{"type": "HostSystem", "value": "host-101"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/resourcePool", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "ResourcePool", "value": "resgroup-22"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/datastore", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Datastore", "value": "datastore-11"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/overallStatus", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "yellow")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Task", "value": "task-31"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "nvme-primary")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-s4"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/summary", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"accessible":         true,
			"multipleHostAccess": true,
			"maintenanceMode":    "normal",
			"url":                "ds:///vmfs/volumes/datastore-11/",
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/host", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "HostSystem", "value": "host-101"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/vm", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "VirtualMachine", "value": "vm-201"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/overallStatus", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "green")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "VM Network")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-n4"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/host", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "HostSystem", "value": "host-101"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Network/network-101/vm", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "VirtualMachine", "value": "vm-201"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ClusterComputeResource/domain-c101/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Prod Compute")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/ClusterComputeResource/domain-c101/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-h4"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-h4/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Prod Hosts")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-h4/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-v7/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Production VMs")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-v7/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-s4/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Shared Datastores")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-s4/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-n4/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Networks")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-n4/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datacenter/datacenter-1/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "DC1")
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ResourcePool/resgroup-22/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Tier 1")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/ResourcePool/resgroup-22/owner", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "ClusterComputeResource", "value": "domain-c101"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Alarm/alarm-11/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"name": "Host connection degraded"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Alarm/alarm-21/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"name": "VM replication fault"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Task/task-11/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"name":      map[string]any{"localizedMessage": "Reconnect host"},
			"state":     "running",
			"startTime": "2026-03-30T18:14:00Z",
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Task/task-21/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"name":          map[string]any{"localizedMessage": "Create snapshot"},
			"state":         "success",
			"descriptionId": "VirtualMachine.createSnapshot",
			"startTime":     "2026-03-30T18:10:00Z",
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Task/task-31/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"name":      map[string]any{"localizedMessage": "Refresh datastore"},
			"state":     "queued",
			"startTime": "2026-03-30T18:11:00Z",
		})
	})

	return httptest.NewTLSServer(mux)
}

func requireAutomationSession(t *testing.T, r *http.Request) {
	t.Helper()
	if got := strings.TrimSpace(r.Header.Get("vmware-api-session-id")); got != "automation-session" {
		t.Fatalf("automation session header = %q, want automation-session", got)
	}
}

func requireVISession(t *testing.T, r *http.Request) {
	t.Helper()
	if got := strings.TrimSpace(r.Header.Get("vmware-api-session-id")); got != "vi-session" {
		t.Fatalf("vi-json session header = %q, want vi-session", got)
	}
}

func TestClientCreateAutomationSessionClassifiesLegacyCISVCenter(t *testing.T) {
	// vSphere 6.x (#1585): /api/session fails because /api routes to the
	// JSON-RPC servlet, while the legacy /rest CIS session API works. The
	// failure must classify as unsupported_version, not a generic endpoint
	// error.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Missing Content-Type header", http.StatusInternalServerError)
	})
	legacyDeleted := false
	mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if _, _, ok := r.BasicAuth(); !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"value":"legacy-session"}`)); err != nil {
				t.Fatalf("write legacy session response: %v", err)
			}
		case http.MethodDelete:
			legacyDeleted = true
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.createAutomationSession(context.Background())
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "unsupported_version" {
		t.Fatalf("connection error category = %q, want unsupported_version", connectionErr.Category)
	}
	if !strings.Contains(connectionErr.Message, "8.0U1") {
		t.Fatalf("expected message to name the supported floor, got %q", connectionErr.Message)
	}
	if !legacyDeleted {
		t.Fatal("expected probe to delete the legacy session it created")
	}
}

func TestClientCreateAutomationSessionKeepsEndpointErrorWithoutLegacyCIS(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.createAutomationSession(context.Background())
	if err == nil {
		t.Fatal("expected endpoint error")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "endpoint" {
		t.Fatalf("connection error category = %q, want endpoint", connectionErr.Category)
	}
}
