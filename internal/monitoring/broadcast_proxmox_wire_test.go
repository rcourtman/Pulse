package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Frozen parent encoder: dynamic maps are deliberate. This oracle must not
// call the production payload/disk builders or source normalizer it checks.
func parentProxmoxPlatformJSON(resource *unifiedresources.Resource) json.RawMessage {
	p := resource.Proxmox
	if p == nil {
		return nil
	}
	var disks []map[string]interface{}
	for _, d := range p.Disks {
		usage := float64(0)
		if d.Total > 0 {
			usage = float64(d.Used) / float64(d.Total) * 100
		}
		disks = append(disks, map[string]interface{}{
			"total": d.Total, "used": d.Used, "free": d.Free, "usage": usage,
			"mountpoint": d.Mountpoint, "type": d.Filesystem, "device": d.Device,
		})
	}
	metric := func(pick func(*unifiedresources.ResourceMetrics) *unifiedresources.MetricValue) int64 {
		if resource.Metrics == nil || pick(resource.Metrics) == nil {
			return 0
		}
		return int64(math.Round(pick(resource.Metrics).Value))
	}
	payload := map[string]interface{}{
		"vmid": p.VMID, "node": p.NodeName, "instance": p.Instance, "cpus": p.CPUs, "template": p.Template,
		"networkIn":  metric(func(m *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return m.NetIn }),
		"networkOut": metric(func(m *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return m.NetOut }),
		"diskRead":   metric(func(m *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return m.DiskRead }),
		"diskWrite":  metric(func(m *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return m.DiskWrite }),
		"disks":      disks, "swapUsed": p.SwapUsed, "swapTotal": p.SwapTotal, "balloon": p.Balloon,
		"lastBackup": p.LastBackup, "backupInProgress": p.BackupInProgress,
		"ipAddresses": append([]string(nil), resource.Identity.IPAddresses...),
	}
	// Keep the parent's exact source normalization but an independent copy.
	var sources []string
	seen := map[string]bool{}
	for _, source := range resource.Sources {
		key := strings.TrimSpace(string(source))
		if key != "" && !seen[key] {
			seen[key] = true
			sources = append(sources, key)
		}
	}
	if len(sources) > 0 {
		payload["sources"] = sources
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return encoded
}

func broadcastProxmoxWireFixture() unifiedresources.Resource {
	return unifiedresources.Resource{
		ID: "fixture:pve1:101", Name: "wire-guest", Type: unifiedresources.ResourceTypeVM,
		Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourceAgent},
		Identity: unifiedresources.ResourceIdentity{IPAddresses: []string{"192.0.2.10", "2001:db8::10"}},
		Proxmox: &unifiedresources.ProxmoxData{VMID: 101, CPUs: 4, NodeName: "pve1", Instance: "fixture",
			SwapUsed: 1024, SwapTotal: 2048, Balloon: 4096, LastBackup: time.Date(2026, 10, 25, 10, 30, 0, 123456789, time.UTC),
			Disks: []unifiedresources.DiskInfo{
				{Device: "/dev/sda1", Mountpoint: "/", Filesystem: "ext4", Total: 10 << 30, Used: 4 << 30, Free: 6 << 30},
				{Device: "/dev/sdb1", Mountpoint: "/data", Filesystem: "xfs", Total: 20 << 30, Used: 5 << 30, Free: 15 << 30},
				{Device: "tmpfs", Mountpoint: "/run", Filesystem: "tmpfs", Total: 1 << 30, Used: 0, Free: 1 << 30},
			}},
		Metrics: &unifiedresources.ResourceMetrics{
			NetIn: &unifiedresources.MetricValue{Value: 10.6}, NetOut: &unifiedresources.MetricValue{Value: 8.2},
			DiskRead: &unifiedresources.MetricValue{Value: 7.5}, DiskWrite: &unifiedresources.MetricValue{Value: -6.5},
		},
	}
}

func TestBroadcastProxmoxPlatformWireMatchesParent(t *testing.T) {
	rng := rand.New(rand.NewSource(20261025))
	for scenario := 0; scenario < 256; scenario++ {
		resource := broadcastProxmoxWireFixture()
		p := resource.Proxmox
		p.VMID = rng.Intn(100000)
		p.CPUs = rng.Intn(256)
		p.Template = scenario%2 == 0
		p.BackupInProgress = scenario%3 == 0
		p.SwapTotal = rng.Int63()
		p.NodeName = fmt.Sprintf("pve <&> café\u2028 \"%d\"", scenario)
		p.Instance = "fixture\\lab\n\x00"
		for i := range p.Disks {
			p.Disks[i].Total = rng.Int63()
			p.Disks[i].Used = rng.Int63()
			p.Disks[i].Free = rng.Int63()
			p.Disks[i].Usage = math.NaN() // parent ignores this native scalar
		}
		switch scenario % 16 {
		case 0:
			resource.Proxmox = nil
		case 1:
			resource.Proxmox = &unifiedresources.ProxmoxData{}
		case 2:
			p.Disks = nil
			resource.Identity.IPAddresses = nil
		case 3:
			p.Disks = []unifiedresources.DiskInfo{}
			resource.Identity.IPAddresses = []string{}
		case 4:
			p.Disks[0].Total = 0
			p.Disks[1].Total = -1
		case 5:
			resource.Metrics = nil
		case 6:
			resource.Metrics = &unifiedresources.ResourceMetrics{}
		case 7:
			p.LastBackup = time.Time{}
		case 8:
			p.LastBackup = time.Date(2026, 10, 25, 10, 30, 0, 987654321, time.FixedZone("fixture", -7*3600))
		case 9:
			p.LastBackup = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) // encoding failure stays absent
		case 10:
			resource.Metrics.NetIn.Value = math.Inf(1)
			resource.Metrics.NetOut.Value = math.NaN()
		case 11:
			resource.Sources = nil
		case 12:
			resource.Sources = []unifiedresources.DataSource{"", "   ", "proxmox", " proxmox ", "agent", "agent"}
		case 13:
			resource.Sources = []unifiedresources.DataSource{"", "   "}
		case 14:
			p.Disks[0].Mountpoint = "<fixture>\n café \" \\ \u2028"
			resource.Identity.IPAddresses = []string{"", "<fixture>", "192.0.2.1", "192.0.2.1"}
		}
		for _, kind := range []string{"vm", "system-container", "oci-container"} {
			t.Run(fmt.Sprintf("%d/%s", scenario, kind), func(t *testing.T) {
				got := monitorPlatformData(&resource, kind, "unused")
				want := parentProxmoxPlatformJSON(&resource)
				if !bytes.Equal(got, want) || (got == nil) != (want == nil) {
					t.Fatalf("platform JSON changed: got %s, want %s", got, want)
				}
			})
		}
	}
}

func TestBroadcastProxmoxPlatformWireOwnsEncodedBytes(t *testing.T) {
	resource := broadcastProxmoxWireFixture()
	want := parentProxmoxPlatformJSON(&resource)
	got := monitorPlatformData(&resource, "vm", "unused")
	resource.Proxmox.Disks[0].Mountpoint = "changed"
	resource.Identity.IPAddresses[0] = "changed"
	resource.Sources[0] = "changed"
	if !bytes.Equal(got, want) {
		t.Fatal("caller changes altered an already encoded broadcast")
	}
	got[0] = '!'
	if next := monitorPlatformData(&resource, "vm", "unused"); !json.Valid(next) || bytes.Equal(next, want) {
		t.Fatal("encoded caller data shared writable bytes with the next projection")
	}
}

var broadcastPlatformJSONSink json.RawMessage

func TestBroadcastProxmoxPlatformWireAllocations(t *testing.T) {
	resource := broadcastProxmoxWireFixture()
	parent := testing.AllocsPerRun(100, func() { broadcastPlatformJSONSink = parentProxmoxPlatformJSON(&resource) })
	current := testing.AllocsPerRun(100, func() { broadcastPlatformJSONSink = monitorPlatformData(&resource, "vm", "unused") })
	t.Logf("guest plus three filesystems: parent %.0f allocations, current %.0f", parent, current)
	if current >= parent/2 {
		t.Fatalf("fixed records must eliminate at least half the dynamic-map allocations: parent %.0f, current %.0f", parent, current)
	}
}

func BenchmarkBroadcastProxmoxPlatformJSON(b *testing.B) {
	resource := broadcastProxmoxWireFixture()
	for _, tc := range []struct {
		name   string
		encode func(*unifiedresources.Resource) json.RawMessage
	}{
		{"parent-maps", parentProxmoxPlatformJSON},
		{"production", func(r *unifiedresources.Resource) json.RawMessage { return monitorPlatformData(r, "vm", "unused") }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				broadcastPlatformJSONSink = tc.encode(&resource)
			}
		})
	}
}
