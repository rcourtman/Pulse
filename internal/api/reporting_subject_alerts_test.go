package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// Exercise the actual enrichment entry point with both open and resolved
// alerts. A matching path (including a metrics alias) must not override the
// journal's recorded-hardware ownership; an earlier path must not hide it.
func TestReportPhysicalDiskAlertsFollowRecordedHardware(t *testing.T) {
	const fullWWN = "5000c500a1b2c3d4"
	type diskSpec struct{ id, node, path, serial, wwn string }
	base := diskSpec{"physical-disk-subject", "pve1", "/dev/sda", "SER-A", ""}
	wwnOnly := diskSpec{"physical-disk-subject", "pve1", "/dev/sda", "", "naa." + fullWWN}
	placeholder := wwnOnly
	placeholder.serial = "unknown"
	twin := diskSpec{"physical-disk-other", "pve2", "/dev/sdb", "SER-A", "naa.5000c500000000b2"}
	for _, tc := range []struct {
		name                    string
		subject                 diskSpec
		others                  []diskSpec
		node, path, serial, wwn string
		metricsAlias            bool
		want                    bool
	}{
		{name: "serial", subject: base, node: "pve1", path: "/dev/sda", serial: "ser-a", want: true},
		{name: "replacement", subject: base, node: "pve1", path: "/dev/sda", serial: "SER-OLD"},
		{name: "path metrics alias cannot bypass replacement", subject: base, node: "pve1", path: "/dev/sda", serial: "SER-OLD", metricsAlias: true},
		{name: "earlier path", subject: base, node: "pve1", path: "/dev/sdz", serial: "SER-A", want: true},
		{name: "earlier node", subject: base, node: "pve2", path: "/dev/sdz", serial: "SER-A", want: true},
		{name: "WWN only framed", subject: wwnOnly, node: "pve1", path: "/dev/sda", wwn: "wwn-0x" + fullWWN, want: true},
		{name: "WWN only moved", subject: wwnOnly, node: "pve2", path: "/dev/sdz", wwn: "eui." + fullWWN, want: true},
		{name: "agent WWN fields", subject: wwnOnly, node: "pve1", path: "/dev/sda", wwn: "5-c50-a1b2c3d4", want: true},
		{name: "serial to WWN cross source", subject: wwnOnly, node: "pve1", path: "/dev/sda", serial: fullWWN, want: true},
		{name: "placeholder same WWN", subject: placeholder, node: "pve1", path: "/dev/sda", serial: "UNKNOWN", wwn: "0x" + fullWWN, want: true},
		{name: "placeholder different WWN", subject: placeholder, node: "pve1", path: "/dev/sda", serial: "UNKNOWN", wwn: "0x5000c500000000b2"},
		{name: "truncated WWN is not identity", subject: wwnOnly, node: "pve1", path: "/dev/sda", wwn: "0x5000c500a1b2"},
		{name: "unknown cannot claim identified disk", subject: base, node: "pve1", path: "/dev/sda", serial: "unknown"},
		{name: "zero cannot claim identified disk", subject: base, node: "pve1", path: "/dev/sda", serial: "00000000"},
		{name: "shared serial is ambiguous even at path", subject: base, others: []diskSpec{twin}, node: "pve1", path: "/dev/sda", serial: "SER-A"},
		{name: "WWN decides shared serial", subject: diskSpec{base.id, base.node, base.path, base.serial, "naa." + fullWWN}, others: []diskSpec{twin}, node: "pve2", path: "/dev/sdb", serial: "SER-A", wwn: "0x" + fullWWN, want: true},
		{name: "WWN decides other twin despite subject path", subject: diskSpec{base.id, base.node, base.path, base.serial, "naa." + fullWWN}, others: []diskSpec{twin}, node: "pve1", path: "/dev/sda", serial: "SER-A", wwn: twin.wwn},
		{name: "identity less exact path", subject: diskSpec{base.id, base.node, base.path, "unknown", ""}, node: "pve1", path: "/dev/sda", serial: "N/A", want: true},
		{name: "identity less cannot move by path", subject: diskSpec{base.id, base.node, base.path, "", ""}, node: "pve1", path: "/dev/sdz"},
		{name: "identity less path ambiguous", subject: diskSpec{base.id, base.node, base.path, "", ""}, others: []diskSpec{{twin.id, base.node, base.path, "", ""}}, node: "pve1", path: "/dev/sda"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
			start := now.Add(-24 * time.Hour)
			snapshot := reportingEnrichmentSnapshot{}
			for _, spec := range append([]diskSpec{tc.subject}, tc.others...) {
				snapshot.Resources = append(snapshot.Resources, unifiedresources.Resource{
					ID: spec.id, Type: unifiedresources.ResourceTypePhysicalDisk,
					Proxmox:      &unifiedresources.ProxmoxData{Instance: "lab:west", NodeName: spec.node},
					PhysicalDisk: &unifiedresources.PhysicalDiskMeta{DevPath: spec.path, Serial: spec.serial, WWN: spec.wwn},
				})
			}
			ref := unifiedresources.ProxmoxPhysicalDiskAlertResourceID("lab:west", tc.node, tc.path)
			identifiers := unifiedresources.ProxmoxPhysicalDiskAlertIdentifiers(ref)
			alert := models.Alert{ID: identifiers[0], ResourceID: ref, Type: "disk-health", Level: "critical", StartTime: start.Add(time.Hour),
				Metadata: map[string]any{unifiedresources.MetadataDiskSerial: tc.serial, unifiedresources.MetadataDiskWWN: tc.wwn}}
			snapshot.ActiveAlerts = []models.Alert{alert}
			resolved := alert
			resolved.ID, resolved.Type = identifiers[1], "disk-wearout"
			snapshot.RecentlyResolved = []models.ResolvedAlert{
				{Alert: resolved, ResolvedTime: start.Add(time.Hour)},
				{Alert: resolved, ResolvedTime: start}, // exact boundaries excluded
				{Alert: resolved, ResolvedTime: now},
			}
			// Even a path-shaped alert on the subject's metrics alias is not
			// a PVE alert unless its canonical alert identifier agrees.
			unrelated := alert
			unrelated.ID = "unrelated-alert"
			snapshot.ActiveAlerts = append(snapshot.ActiveAlerts, unrelated)
			req := reporting.MetricReportRequest{ResourceType: "disk", ResourceID: tc.subject.id}
			if tc.metricsAlias {
				req.MetricsResourceID = ref
			}
			NewReportingHandlers(nil, nil).enrichReportRequest(context.Background(), "default", &req, snapshot, start, now)
			wantCount := 0
			if tc.want {
				wantCount = 2
			}
			if len(req.Alerts) != wantCount {
				t.Fatalf("alerts = %+v, want %d owned open/in-window rows", req.Alerts, wantCount)
			}
			if tc.want && (req.Alerts[0].ResolvedTime != nil || req.Alerts[1].ResolvedTime == nil || !req.Alerts[1].ResolvedTime.Equal(start.Add(time.Hour))) {
				t.Fatalf("open/resolved state not preserved: %+v", req.Alerts)
			}
		})
	}
}
