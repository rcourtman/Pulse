package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
)

// --- Mock GuestProber ---

type mockGuestProber struct {
	agents  map[string]string                // hostname -> agentID
	results map[string]map[string]PingResult // agentID -> ip -> result
	err     error
}

func (m *mockGuestProber) GetAgentForHost(hostname string) (string, bool) {
	id, ok := m.agents[hostname]
	return id, ok
}

func (m *mockGuestProber) PingGuests(ctx context.Context, agentID string, ips []string) (map[string]PingResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if results, ok := m.results[agentID]; ok {
		filtered := make(map[string]PingResult)
		for _, ip := range ips {
			if r, found := results[ip]; found {
				filtered[ip] = r
			}
		}
		return filtered, nil
	}
	return map[string]PingResult{}, nil
}

// --- Mock Discovery Store helpers ---

func setupTestDiscoveryStore(t *testing.T, discoveries []*servicediscovery.ResourceDiscovery) *servicediscovery.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := servicediscovery.NewStore(dir)
	if err != nil {
		t.Fatalf("failed to create discovery store: %v", err)
	}
	for _, d := range discoveries {
		if err := store.Save(d); err != nil {
			t.Fatalf("failed to save discovery: %v", err)
		}
	}
	return store
}

// --- Tests ---

func TestGatherGuestIntelligence_DiscoveryMatching(t *testing.T) {
	store := setupTestDiscoveryStore(t, []*servicediscovery.ResourceDiscovery{
		{
			ID:           "vm:pve1:100",
			ResourceType: servicediscovery.ResourceTypeVM,
			HostID:       "pve1",
			ResourceID:   "100",
			ServiceName:  "PostgreSQL 15",
			ServiceType:  "postgres",
		},
		{
			ID:           "lxc:pve1:101",
			ResourceType: servicediscovery.ResourceTypeSystemContainer,
			HostID:       "pve1",
			ResourceID:   "101",
			ServiceName:  "Nginx",
			ServiceType:  "nginx",
		},
	})

	ps := NewPatrolService(nil, nil)
	ps.SetDiscoveryStore(store)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "db-server", Node: "pve1", Status: "running"},
			{ID: "qemu/200", VMID: 200, Name: "unknown-vm", Node: "pve1", Status: "running"},
		},
		Containers: []models.Container{
			{ID: "lxc/101", VMID: 101, Name: "web-proxy", Node: "pve1", Status: "running"},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)

	// VM 100 should match discovery
	if gi := intel["qemu/100"]; gi == nil {
		t.Fatal("expected intel for qemu/100")
	} else {
		if gi.ServiceName != "PostgreSQL 15" {
			t.Errorf("qemu/100 ServiceName = %q, want %q", gi.ServiceName, "PostgreSQL 15")
		}
		if gi.ServiceType != "postgres" {
			t.Errorf("qemu/100 ServiceType = %q, want %q", gi.ServiceType, "postgres")
		}
		if gi.Name != "db-server" {
			t.Errorf("qemu/100 Name = %q, want %q", gi.Name, "db-server")
		}
		if gi.GuestType != "vm" {
			t.Errorf("qemu/100 GuestType = %q, want %q", gi.GuestType, "vm")
		}
	}

	// VM 200 has no discovery match
	if gi := intel["qemu/200"]; gi == nil {
		t.Fatal("expected intel for qemu/200")
	} else {
		if gi.ServiceName != "" {
			t.Errorf("qemu/200 ServiceName = %q, want empty", gi.ServiceName)
		}
	}

	// Container 101 should match discovery
	if gi := intel["lxc/101"]; gi == nil {
		t.Fatal("expected intel for lxc/101")
	} else {
		if gi.ServiceName != "Nginx" {
			t.Errorf("lxc/101 ServiceName = %q, want %q", gi.ServiceName, "Nginx")
		}
		if gi.GuestType != "system-container" {
			t.Errorf("lxc/101 GuestType = %q, want %q", gi.GuestType, "system-container")
		}
	}
}

func TestGatherGuestIntelligence_DiscoveryInstanceFallback(t *testing.T) {
	// Discovery stored with Instance as hostID (different from Node)
	store := setupTestDiscoveryStore(t, []*servicediscovery.ResourceDiscovery{
		{
			ID:           "vm:my-instance:100",
			ResourceType: servicediscovery.ResourceTypeVM,
			HostID:       "my-instance",
			ResourceID:   "100",
			ServiceName:  "Redis",
			ServiceType:  "redis",
		},
	})

	ps := NewPatrolService(nil, nil)
	ps.SetDiscoveryStore(store)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "cache", Node: "pve1", Instance: "my-instance", Status: "running"},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	if gi := intel["qemu/100"]; gi == nil || gi.ServiceName != "Redis" {
		t.Errorf("expected Redis from instance fallback, got %+v", gi)
	}
}

func TestGatherGuestIntelligence_Reachability(t *testing.T) {
	prober := &mockGuestProber{
		agents: map[string]string{"pve1": "agent-1"},
		results: map[string]map[string]PingResult{
			"agent-1": {
				"10.0.0.1": {Reachable: true},
				"10.0.0.2": {Reachable: false},
			},
		},
	}

	ps := NewPatrolService(nil, nil)
	ps.SetGuestProber(prober)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-up", Node: "pve1", Status: "running", IPAddresses: []string{"10.0.0.1"}},
			{ID: "qemu/101", VMID: 101, Name: "vm-down", Node: "pve1", Status: "running", IPAddresses: []string{"10.0.0.2"}},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)

	if gi := intel["qemu/100"]; gi == nil || gi.Reachable == nil || !*gi.Reachable {
		t.Error("expected qemu/100 to be reachable")
	}
	if gi := intel["qemu/101"]; gi == nil || gi.Reachable == nil || *gi.Reachable {
		t.Error("expected qemu/101 to be unreachable")
	}
}

func TestGatherGuestIntelligence_StoppedGuestsNotPinged(t *testing.T) {
	prober := &mockGuestProber{
		agents:  map[string]string{"pve1": "agent-1"},
		results: map[string]map[string]PingResult{},
	}

	ps := NewPatrolService(nil, nil)
	ps.SetGuestProber(prober)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "stopped-vm", Node: "pve1", Status: "stopped", IPAddresses: []string{"10.0.0.1"}},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	if gi := intel["qemu/100"]; gi == nil {
		t.Fatal("expected intel entry for stopped VM")
	} else if gi.Reachable != nil {
		t.Error("stopped VM should not have reachability checked (Reachable should be nil)")
	}
}

func TestGatherGuestIntelligence_NoAgentOnNode(t *testing.T) {
	prober := &mockGuestProber{
		agents: map[string]string{}, // no agents connected
	}

	ps := NewPatrolService(nil, nil)
	ps.SetGuestProber(prober)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Status: "running", IPAddresses: []string{"10.0.0.1"}},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	if gi := intel["qemu/100"]; gi == nil {
		t.Fatal("expected intel entry")
	} else if gi.Reachable != nil {
		t.Error("should show - (nil) when no agent available")
	}
}

func TestGatherGuestIntelligence_NoIPAddress(t *testing.T) {
	prober := &mockGuestProber{
		agents: map[string]string{"pve1": "agent-1"},
	}

	ps := NewPatrolService(nil, nil)
	ps.SetGuestProber(prober)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "no-ip-vm", Node: "pve1", Status: "running", IPAddresses: nil},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	if gi := intel["qemu/100"]; gi == nil {
		t.Fatal("expected intel entry")
	} else if gi.Reachable != nil {
		t.Error("should show - (nil) when guest has no IP")
	}
}

func TestGatherGuestIntelligence_ProberError(t *testing.T) {
	prober := &mockGuestProber{
		agents: map[string]string{"pve1": "agent-1"},
		err:    fmt.Errorf("connection refused"),
	}

	ps := NewPatrolService(nil, nil)
	ps.SetGuestProber(prober)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Status: "running", IPAddresses: []string{"10.0.0.1"}},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	// Should not crash, guest should still be in map but without reachability
	if gi := intel["qemu/100"]; gi == nil {
		t.Fatal("expected intel entry despite prober error")
	} else if gi.Reachable != nil {
		t.Error("should show nil reachability when prober fails")
	}
}

func TestGatherGuestIntelligence_NilProber(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	// No prober set

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Status: "running", IPAddresses: []string{"10.0.0.1"}},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	if gi := intel["qemu/100"]; gi == nil {
		t.Fatal("expected intel entry")
	} else if gi.Reachable != nil {
		t.Error("should show nil reachability when no prober set")
	}
}

func TestGatherGuestIntelligence_TemplatesSkipped(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/9000", VMID: 9000, Name: "template-vm", Node: "pve1", Status: "stopped", Template: true},
		},
		Containers: []models.Container{
			{ID: "lxc/9001", VMID: 9001, Name: "template-ct", Node: "pve1", Status: "stopped", Template: true},
		},
	}

	intel := ps.gatherGuestIntelligence(context.Background(), state)
	if _, ok := intel["qemu/9000"]; ok {
		t.Error("template VM should be skipped")
	}
	if _, ok := intel["lxc/9001"]; ok {
		t.Error("template container should be skipped")
	}
}

// --- parsePingOutput tests ---

func TestParsePingOutput_Normal(t *testing.T) {
	output := "REACH:10.0.0.1:UP\nREACH:10.0.0.2:DOWN\nREACH:10.0.0.3:UP\n"
	results := parsePingOutput(output)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !results["10.0.0.1"].Reachable {
		t.Error("10.0.0.1 should be reachable")
	}
	if results["10.0.0.2"].Reachable {
		t.Error("10.0.0.2 should be unreachable")
	}
	if !results["10.0.0.3"].Reachable {
		t.Error("10.0.0.3 should be reachable")
	}
}

func TestParsePingOutput_Empty(t *testing.T) {
	results := parsePingOutput("")
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty output, got %d", len(results))
	}
}

func TestParsePingOutput_NoiseLines(t *testing.T) {
	output := "bash: some warning\nREACH:10.0.0.1:UP\nsome other output\n"
	results := parsePingOutput(output)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results["10.0.0.1"].Reachable {
		t.Error("10.0.0.1 should be reachable")
	}
}

func TestParsePingOutput_MalformedLines(t *testing.T) {
	output := "REACH:\nREACH:10.0.0.1\nREACH:10.0.0.2:UP\n"
	results := parsePingOutput(output)

	// Only the well-formed line should parse
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestParsePingOutput_WhitespaceHandling(t *testing.T) {
	output := "  REACH:10.0.0.1:UP  \n\n  REACH:10.0.0.2:DOWN\n"
	results := parsePingOutput(output)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// --- formatReachable / formatService / reachableFromIntel tests ---

func TestFormatReachable(t *testing.T) {
	trueVal := true
	falseVal := false

	if got := formatReachable(nil); got != "-" {
		t.Errorf("formatReachable(nil) = %q, want %q", got, "-")
	}
	if got := formatReachable(&trueVal); got != "yes" {
		t.Errorf("formatReachable(true) = %q, want %q", got, "yes")
	}
	if got := formatReachable(&falseVal); got != "NO" {
		t.Errorf("formatReachable(false) = %q, want %q", got, "NO")
	}
}

func TestFormatService(t *testing.T) {
	if got := formatService(nil); got != "-" {
		t.Errorf("formatService(nil) = %q, want %q", got, "-")
	}
	if got := formatService(&GuestIntelligence{}); got != "-" {
		t.Errorf("formatService(empty) = %q, want %q", got, "-")
	}
	if got := formatService(&GuestIntelligence{ServiceName: "PostgreSQL 15"}); got != "PostgreSQL 15" {
		t.Errorf("formatService(PostgreSQL 15) = %q, want %q", got, "PostgreSQL 15")
	}
	// Truncation
	longName := "Very Long Service Name That Exceeds Limit"
	got := formatService(&GuestIntelligence{ServiceName: longName})
	if len(got) > 25 {
		t.Errorf("formatService should truncate to <= 25 chars, got %d: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated name should end with ..., got %q", got)
	}
}

func TestReachableFromIntel(t *testing.T) {
	if got := reachableFromIntel(nil); got != nil {
		t.Error("reachableFromIntel(nil) should be nil")
	}
	trueVal := true
	gi := &GuestIntelligence{Reachable: &trueVal}
	if got := reachableFromIntel(gi); got == nil || !*got {
		t.Error("reachableFromIntel should return pointer to true")
	}
}

// --- buildServiceHealthIssues tests ---

func TestBuildServiceHealthIssues_Empty(t *testing.T) {
	if got := buildServiceHealthIssues(nil); got != "" {
		t.Errorf("expected empty string for nil issues, got %q", got)
	}
	if got := buildServiceHealthIssues([]serviceHealthIssue{}); got != "" {
		t.Errorf("expected empty string for empty issues, got %q", got)
	}
}

func TestBuildServiceHealthIssues_WithIssues(t *testing.T) {
	issues := []serviceHealthIssue{
		{name: "mail-relay", service: "Postfix", node: "pve1"},
		{name: "unknown-vm", service: "VM", node: "pve2"},
	}
	got := buildServiceHealthIssues(issues)

	if !strings.Contains(got, "# Service Health Issues") {
		t.Error("expected section header")
	}
	if !strings.Contains(got, "mail-relay (Postfix on pve1): Running but UNREACHABLE") {
		t.Errorf("expected mail-relay issue, got:\n%s", got)
	}
	if !strings.Contains(got, "unknown-vm (VM on pve2): Running but UNREACHABLE") {
		t.Errorf("expected unknown-vm issue, got:\n%s", got)
	}
}

// --- DetectReachabilitySignals tests ---

func TestDetectReachabilitySignals_UnreachableGuest(t *testing.T) {
	falseVal := false
	trueVal := true
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Name: "db-server", GuestType: "vm", Reachable: &falseVal},
		"lxc/101":  {Name: "web-proxy", GuestType: "lxc", Reachable: &trueVal},
		"qemu/102": {Name: "no-check", GuestType: "vm", Reachable: nil},
	}

	signals := DetectReachabilitySignals(intel)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := signals[0]
	if s.SignalType != SignalGuestUnreachable {
		t.Errorf("SignalType = %q, want %q", s.SignalType, SignalGuestUnreachable)
	}
	if s.ResourceID != "qemu/100" {
		t.Errorf("ResourceID = %q, want %q", s.ResourceID, "qemu/100")
	}
	if s.ResourceName != "db-server" {
		t.Errorf("ResourceName = %q, want %q", s.ResourceName, "db-server")
	}
	if s.ResourceType != "vm" {
		t.Errorf("ResourceType = %q, want %q", s.ResourceType, "vm")
	}
	if s.SuggestedSeverity != "warning" {
		t.Errorf("SuggestedSeverity = %q, want %q", s.SuggestedSeverity, "warning")
	}
	if s.Category != "connectivity" {
		t.Errorf("Category = %q, want %q", s.Category, "connectivity")
	}
}

func TestDetectReachabilitySignals_AllReachable(t *testing.T) {
	trueVal := true
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Reachable: &trueVal},
		"lxc/101":  {Reachable: &trueVal},
	}

	signals := DetectReachabilitySignals(intel)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals when all reachable, got %d", len(signals))
	}
}

func TestDetectReachabilitySignals_NilIntel(t *testing.T) {
	signals := DetectReachabilitySignals(nil)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for nil intel, got %d", len(signals))
	}
}

func TestDetectReachabilitySignals_NoReachabilityData(t *testing.T) {
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Reachable: nil},
	}

	signals := DetectReachabilitySignals(intel)
	if len(signals) != 0 {
		t.Fatalf("expected 0 signals when reachability unchecked, got %d", len(signals))
	}
}

// --- Enriched seed context tests ---

func TestSeedResourceInventory_WithIntelligence(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	cfg.AnalyzeGuests = true

	trueVal := true
	falseVal := false
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Name: "db-server", ServiceName: "PostgreSQL 15", Reachable: &trueVal},
		"lxc/101":  {Name: "mail-relay", ServiceName: "Postfix", Reachable: &falseVal},
		"qemu/102": {Name: "dev-box", Reachable: nil},
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "db-server", Node: "pve1", Status: "running", CPU: 0.12, Memory: models.Memory{Usage: 65}, Disk: models.Disk{Usage: 45}},
			{ID: "qemu/102", VMID: 102, Name: "dev-box", Node: "pve2", Status: "running", CPU: 0.30, Memory: models.Memory{Usage: 70}, Disk: models.Disk{Usage: 60}},
		},
		Containers: []models.Container{
			{ID: "lxc/101", VMID: 101, Name: "mail-relay", Node: "pve1", Status: "running", CPU: 0.0, Memory: models.Memory{Usage: 10}, Disk: models.Disk{Usage: 8}},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), false, intel)

	// Check table headers
	if !strings.Contains(out, "| Service |") {
		t.Errorf("expected Service column header, got:\n%s", out)
	}
	if !strings.Contains(out, "| Reachable |") {
		t.Errorf("expected Reachable column header, got:\n%s", out)
	}

	// Check service names appear
	if !strings.Contains(out, "PostgreSQL 15") {
		t.Errorf("expected PostgreSQL 15 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Postfix") {
		t.Errorf("expected Postfix in output, got:\n%s", out)
	}

	// Check reachability values
	if !strings.Contains(out, "| yes |") {
		t.Errorf("expected 'yes' reachability, got:\n%s", out)
	}
	if !strings.Contains(out, "| NO |") {
		t.Errorf("expected 'NO' reachability, got:\n%s", out)
	}

	// Check health issues section
	if !strings.Contains(out, "# Service Health Issues") {
		t.Errorf("expected Service Health Issues section, got:\n%s", out)
	}
	if !strings.Contains(out, "mail-relay (Postfix on pve1): Running but UNREACHABLE") {
		t.Errorf("expected mail-relay unreachable issue, got:\n%s", out)
	}
}

func TestSeedResourceInventory_NilIntelligence(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	cfg.AnalyzeGuests = true

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Status: "running", CPU: 0.10, Memory: models.Memory{Usage: 50}, Disk: models.Disk{Usage: 30}},
		},
	}

	// nil intel should produce "-" for both columns
	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), false, nil)
	if !strings.Contains(out, "| - |") {
		t.Errorf("expected '-' for unknown service/reachability, got:\n%s", out)
	}
}

func TestSeedResourceInventory_QuietMode_AllReachable(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	cfg.AnalyzeGuests = true

	trueVal := true
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Reachable: &trueVal},
		"qemu/101": {Reachable: &trueVal},
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Status: "running"},
			{ID: "qemu/101", VMID: 101, Name: "vm-2", Node: "pve1", Status: "stopped"},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), true, intel)
	if !strings.Contains(out, "All reachable.") {
		t.Errorf("expected 'All reachable.' in quiet summary, got:\n%s", out)
	}
}

func TestSeedResourceInventory_QuietMode_Unreachable(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	cfg.AnalyzeGuests = true

	falseVal := false
	trueVal := true
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Reachable: &trueVal},
		"qemu/101": {Reachable: &falseVal},
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-ok", Node: "pve1", Status: "running"},
			{ID: "qemu/101", VMID: 101, Name: "vm-bad", Node: "pve1", Status: "running"},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), true, intel)
	if !strings.Contains(out, "1 UNREACHABLE: vm-bad") {
		t.Errorf("expected unreachable guest name in quiet summary, got:\n%s", out)
	}
}

func TestSeedResourceInventory_QuietMode_NoReachabilityData(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	cfg.AnalyzeGuests = true

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "vm-1", Node: "pve1", Status: "running"},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), true, nil)
	if strings.Contains(out, "reachable") {
		t.Errorf("should not mention reachability when no data available, got:\n%s", out)
	}
	if !strings.Contains(out, "no issues detected") {
		t.Errorf("expected standard quiet summary, got:\n%s", out)
	}
}

func TestSeedResourceInventory_HealthIssuesFallbackType(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	cfg.AnalyzeGuests = true

	falseVal := false
	intel := map[string]*GuestIntelligence{
		"qemu/100": {Name: "unknown-vm", GuestType: "vm", Reachable: &falseVal},
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "qemu/100", VMID: 100, Name: "unknown-vm", Node: "pve1", Status: "running", CPU: 0.10, Memory: models.Memory{Usage: 50}, Disk: models.Disk{Usage: 30}},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), false, intel)
	// When no discovery, should use "VM" not "-" in health issues
	if !strings.Contains(out, "unknown-vm (VM on pve1): Running but UNREACHABLE") {
		t.Errorf("expected VM type fallback in health issue, got:\n%s", out)
	}
}
