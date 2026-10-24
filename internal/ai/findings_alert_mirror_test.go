package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestFindingMirrorsAlert_SameResourceSameCondition(t *testing.T) {
	stopped := &Finding{
		ID:         proxmoxGuestStoppedFindingIDPrefix + "pve1/qemu/101",
		Key:        proxmoxGuestStoppedFindingKey,
		Category:   FindingCategoryReliability,
		ResourceID: "pve:vm:101",
		Title:      "Proxmox VM database stopped",
	}
	poweredOff := alertMirrorCandidate{ID: "powered-off-pve:vm:101", ResourceID: "pve:vm:101", Type: "powered-off"}
	if !findingMirrorsAlert(stopped, poweredOff) {
		t.Fatal("guest stopped finding did not mirror the powered-off alert on the same resource")
	}

	otherResource := alertMirrorCandidate{ID: "powered-off-pve:vm:102", ResourceID: "pve:vm:102", Type: "powered-off"}
	if findingMirrorsAlert(stopped, otherResource) {
		t.Fatal("matched an alert on a different resource")
	}

	memoryAlert := alertMirrorCandidate{ID: "memory-pve:vm:101", ResourceID: "pve:vm:101", Type: "memory"}
	if findingMirrorsAlert(stopped, memoryAlert) {
		t.Fatal("matched an alert with a different condition on the same resource")
	}
}

func TestFindingMirrorsAlert_ExplicitAlertIdentifierWins(t *testing.T) {
	f := &Finding{
		ID:              "llm-finding",
		Category:        FindingCategoryGeneral,
		ResourceID:      "docker:host/web",
		Title:           "Something Patrol noticed",
		AlertIdentifier: "docker-container-state-docker:host/web",
	}
	alert := alertMirrorCandidate{ID: "docker-container-state-docker:host/web", ResourceID: "docker:other", Type: "docker-container-state"}
	if !findingMirrorsAlert(f, alert) {
		t.Fatal("explicit AlertIdentifier link was not honoured")
	}
}

func TestFindingConditionClass_CoversAlertOwnedConditions(t *testing.T) {
	cases := []struct {
		name string
		f    *Finding
		want string
	}{
		{"container exited", &Finding{Category: FindingCategoryReliability, Title: `Container "vaultwarden" exited unexpectedly`}, mirrorConditionDown},
		{"restart loop", &Finding{Category: FindingCategoryReliability, Title: `Container "sftp-ingest" is restarting repeatedly`}, mirrorConditionRestartLoop},
		{"unhealthy container", &Finding{Category: FindingCategoryReliability, Title: `Container "jellyfin" is unhealthy`}, mirrorConditionContainerHealth},
		{"storage full", &Finding{Category: FindingCategoryCapacity, Title: `Storage "core-b-service-pool" is 88% full`}, mirrorConditionDiskCapacity},
		{"memory capacity", &Finding{Category: FindingCategoryCapacity, Title: `Memory headroom on node pve1 is nearly exhausted`}, mirrorConditionMemory},
		{"guest memory", &Finding{Category: FindingCategoryPerformance, Title: `Container "artifact-cache" memory usage at 92%`}, mirrorConditionMemory},
		{"cpu", &Finding{Category: FindingCategoryPerformance, Title: `Sustained CPU saturation on pve3`}, mirrorConditionCPU},
		{"snapshot age", &Finding{Category: FindingCategoryBackup, Title: `Snapshot "base" is 349 days old`}, mirrorConditionSnapshotAge},
		{"backup age", &Finding{Category: FindingCategoryBackup, Title: `No recent backup for VM 104`}, mirrorConditionBackupAge},
		{"pdm offline", &Finding{ID: PDMAlertFindingPrefix + ":dc/node/pve1", Category: FindingCategoryReliability, Title: `PDM: node "pve1" is offline`}, mirrorConditionDown},
		{"smart offline uncorrectable is disk health", &Finding{Category: FindingCategoryReliability, Title: `Disk sda reports 8 offline uncorrectable sectors`}, ""},
		{"smart-failure key is disk health", &Finding{Key: "smart-failure", Category: FindingCategoryReliability, Title: `Disk sdb is failing and may go offline`}, ""},
		{"mail gateway degraded is Patrol-owned", &Finding{Category: FindingCategoryReliability, Title: `Mail gateway "mail-gateway-us" is degraded`}, ""},
		{"security finding never mirrors", &Finding{Category: FindingCategorySecurity, Title: `Container runs as root and is offline`}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findingConditionClass(tc.f); got != tc.want {
				t.Fatalf("findingConditionClass() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAlertConditionClass_MapsAlertTypes(t *testing.T) {
	cases := map[string]string{
		"docker-container-state":        mirrorConditionDown,
		"powered-off":                   mirrorConditionDown,
		"offline":                       mirrorConditionDown,
		"connectivity":                  mirrorConditionDown,
		"docker-container-restart-loop": mirrorConditionRestartLoop,
		"docker-container-health":       mirrorConditionContainerHealth,
		"usage":                         mirrorConditionDiskCapacity,
		"disk":                          mirrorConditionDiskCapacity,
		"memory":                        mirrorConditionMemory,
		"cpu":                           mirrorConditionCPU,
		"snapshot-age":                  mirrorConditionSnapshotAge,
		"backup-age":                    mirrorConditionBackupAge,
		"temperature":                   mirrorConditionTemperature,
		"zfs-pool-state":                "",
		"":                              "",
	}
	for alertType, want := range cases {
		if got := alertConditionClass(alertType); got != want {
			t.Errorf("alertConditionClass(%q) = %q, want %q", alertType, got, want)
		}
	}
}

func TestMatchFindingsToAlerts_SkipsResolvedAndStormFindings(t *testing.T) {
	resolvedAt := time.Now()
	alerts := []alertMirrorCandidate{
		{ID: "docker-container-state-docker:h/a", ResourceID: "docker:h/a", Type: "docker-container-state"},
		{ID: "docker-container-state-docker:h/b", ResourceID: "docker:h/b", Type: "docker-container-state"},
	}
	findings := []*Finding{
		{ID: "live", Category: FindingCategoryReliability, ResourceID: "docker:h/a", Title: `Container "a" exited`},
		{ID: "gone", Category: FindingCategoryReliability, ResourceID: "docker:h/b", Title: `Container "b" exited`, ResolvedAt: &resolvedAt},
		{ID: stormFindingIDPrefix + "docker:h/a", Source: stormFindingSource, Category: FindingCategoryReliability, ResourceID: "docker:h/a", Title: "Multiple findings emitted"},
		nil,
	}
	got := matchFindingsToAlerts(findings, alerts)
	if len(got) != 1 {
		t.Fatalf("mirrors = %d, want 1: %+v", len(got), got)
	}
	mirror, ok := got["live"]
	if !ok {
		t.Fatalf("live finding was not matched: %+v", got)
	}
	if mirror.AlertID != "docker-container-state-docker:h/a" || mirror.AlertType != "docker-container-state" {
		t.Fatalf("mirror = %+v", mirror)
	}
}

func TestStampAlertMirrors_SetsAndClears(t *testing.T) {
	store := NewFindingsStore()
	store.Add(&Finding{ID: "f1", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, ResourceID: "docker:h/a", Title: `Container "a" exited`})
	store.Add(&Finding{ID: "f2", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, ResourceID: "docker:h/b", Title: `Container "b" exited`})

	changed := store.StampAlertMirrors(map[string]findingAlertMirror{
		"f1": {AlertID: "alert-a", AlertType: "docker-container-state"},
	})
	if changed != 1 {
		t.Fatalf("changed = %d, want 1", changed)
	}
	if f := store.Get("f1"); f.MirrorsAlertID != "alert-a" || f.MirrorsAlertType != "docker-container-state" {
		t.Fatalf("f1 was not stamped: %+v", f)
	}
	if f := store.Get("f2"); f.MirrorsAlertID != "" {
		t.Fatalf("f2 was stamped without a match: %+v", f)
	}

	// Same stamp again is a no-op.
	if changed := store.StampAlertMirrors(map[string]findingAlertMirror{"f1": {AlertID: "alert-a", AlertType: "docker-container-state"}}); changed != 0 {
		t.Fatalf("idempotent restamp changed = %d, want 0", changed)
	}

	// Alert resolved: the finding is released.
	if changed := store.StampAlertMirrors(map[string]findingAlertMirror{}); changed != 1 {
		t.Fatalf("clear changed = %d, want 1", changed)
	}
	if f := store.Get("f1"); f.MirrorsAlertID != "" || f.MirrorsAlertType != "" {
		t.Fatalf("f1 stamp was not cleared: %+v", f)
	}
}

// Alerts name resources by their producer's references while Patrol findings
// carry the unified resource ID. The patrol-end reconcile resolves each
// alert's reference through current inventory before matching, so a container
// health finding folds under the Docker health alert on that container and a
// guest-stopped finding under the guest's powered-off alert. A filesystem alert
// resolves only to the host its sibling filesystems share, so it never folds a
// finding about another filesystem.
func TestReconcileAlertMirrorsMatchesFindingsOnTheUnifiedResource(t *testing.T) {
	webID := strings.Repeat("ab12", 16)
	now := time.Now()
	snapshot := models.StateSnapshot{
		VMs:   []models.VM{{ID: "pve1:node1:100", VMID: 100, Name: "db", Node: "node1", Instance: "pve1", Status: "stopped", Type: "qemu", LastSeen: now}},
		Hosts: []models.Host{{ID: "host-src-1", Hostname: "backup-box", MachineID: "machine-1", Status: "online", LastSeen: now}},
		DockerHosts: []models.DockerHost{{ID: "dock-agent-1", Hostname: "docker-box", Status: "online", LastSeen: now,
			Containers: []models.DockerContainer{
				{ID: webID, Name: "web", State: "running", Health: "unhealthy"},
				{ID: strings.Repeat("cd34", 16), Name: "api", State: "running", Health: "unhealthy"},
			}}},
		ActiveAlerts: []models.Alert{
			{ID: "docker-container-health-web", ResourceID: alerts.DockerContainerResourceID("dock-agent-1", webID, "web"), Type: "docker-container-health"},
			{ID: "guest-powered-off-db", ResourceID: "pve1:node1:100", Type: "powered-off"},
			{ID: "host-disk-data", ResourceID: "agent:host-src-1/disk:data", Type: "disk"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	resourceIDByName := map[string]string{}
	for _, resource := range registry.List() {
		resourceIDByName[resource.Name] = resource.ID
	}

	ps := NewPatrolService(nil, &mockStateProvider{state: snapshot})
	for id, finding := range map[string]Finding{
		"web-unhealthy": {ResourceID: resourceIDByName["web"], ResourceType: "app-container", Title: "Container web is unhealthy"},
		"api-unhealthy": {ResourceID: resourceIDByName["api"], ResourceType: "app-container", Title: "Container api is unhealthy"},
		"db-stopped":    {ResourceID: resourceIDByName["db"], ResourceType: "vm", Title: "Proxmox VM db stopped"},
		"backup-full":   {ResourceID: resourceIDByName["backup-box"], ResourceType: "agent", Title: "Disk /backup on backup-box is 92% full", Category: FindingCategoryCapacity},
	} {
		if finding.ResourceID == "" {
			t.Fatalf("fixture resource for %s missing", id)
		}
		if finding.Category == "" {
			finding.Category = FindingCategoryReliability
		}
		finding.ID, finding.Severity = id, FindingSeverityWarning
		ps.findings.Add(&finding)
	}

	if changed := ps.reconcileAlertMirrors(); changed != 2 {
		t.Fatalf("reconcile stamped %d findings, want the web and db findings", changed)
	}
	for id, want := range map[string]string{"web-unhealthy": "docker-container-health-web", "db-stopped": "guest-powered-off-db", "api-unhealthy": "", "backup-full": ""} {
		if got := ps.findings.Get(id).MirrorsAlertID; got != want {
			t.Errorf("%s mirrors %q, want %q", id, got, want)
		}
	}
}
