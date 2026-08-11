package monitoring

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubPVEClientContainerMetadata struct {
	stubPVEClient

	config map[string]interface{}

	configCalls int
	statusCalls int
}

func (s *stubPVEClientContainerMetadata) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	s.configCalls++
	return s.config, nil
}

func (s *stubPVEClientContainerMetadata) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	s.statusCalls++
	return nil, nil
}

func TestEnrichContainerMetadata_DetectsOCIForStoppedContainer(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{}
	client := &stubPVEClientContainerMetadata{
		config: map[string]interface{}{
			"entrypoint": "/bin/sh",
			"ostype":     "unmanaged",
			"cmode":      "console",
			"lxc":        "lxc.signal.halt: SIGTERM",
		},
	}

	container := &models.Container{
		VMID:   300,
		Name:   "oci-alpine",
		Status: "stopped",
		Type:   "lxc",
	}

	monitor.enrichContainerMetadata(context.Background(), client, "delly", "delly", container)

	if client.configCalls != 1 {
		t.Fatalf("expected 1 config call, got %d", client.configCalls)
	}
	if client.statusCalls != 0 {
		t.Fatalf("expected 0 status calls for stopped container, got %d", client.statusCalls)
	}
	if !container.IsOCI {
		t.Fatalf("expected container.IsOCI true, got false")
	}
	if container.Type != "oci" {
		t.Fatalf("expected container.Type oci, got %q", container.Type)
	}
}

func TestIssue1477ConfigOnlyMountsReachUnifiedResourceWire(t *testing.T) {
	t.Parallel()

	// The exact production path: stock PVE answers the LXC status query with
	// an empty diskinfo map, the container config is the only record of the
	// mpX mount, and the enriched container is projected through the unified
	// resource registry into the wire shape the frontend consumes.
	monitor := &Monitor{}
	client := &stubPVEClientContainerMetadata{
		config: map[string]interface{}{
			"rootfs": "local-lvm:vm-106-disk-0,size=59G",
			"mp0":    "tank:subvol-106-disk-1,mp=/srv/archive,size=20G",
		},
	}
	container := models.Container{
		ID:       "pve1-106",
		VMID:     106,
		Name:     "edge-proxy-01",
		Node:     "pve1",
		Instance: "pve1",
		Type:     "lxc",
		Status:   "running",
		Disk: models.Disk{
			Total: 63350767616,
			Used:  23530764893,
			Free:  39820002723,
			Usage: 37.14,
		},
	}
	emptyDiskInfoStatus := &proxmox.Container{
		DiskInfo: map[string]proxmox.ContainerDiskUsage{},
	}

	monitor.enrichContainerMetadata(
		context.Background(), client, "pve1", "pve1", &container, emptyDiskInfoStatus,
	)

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{Containers: []models.Container{container}})
	resources := registry.ListByType(unifiedresources.ResourceTypeSystemContainer)
	if len(resources) != 1 {
		t.Fatalf("expected 1 system-container resource, got %d", len(resources))
	}
	facet := resources[0].Proxmox
	if facet == nil || len(facet.Disks) != 2 {
		t.Fatalf("proxmox facet disks = %+v, want rootfs + config-only mount", facet)
	}

	payload, err := json.Marshal(resources[0])
	if err != nil {
		t.Fatalf("marshal resource: %v", err)
	}
	var wire struct {
		Proxmox struct {
			Disks []map[string]interface{} `json:"disks"`
		} `json:"proxmox"`
	}
	if err := json.Unmarshal(payload, &wire); err != nil {
		t.Fatalf("unmarshal resource: %v", err)
	}
	if len(wire.Proxmox.Disks) != 2 {
		t.Fatalf("wire disks = %+v, want 2 rows", wire.Proxmox.Disks)
	}

	var rootfs, archive map[string]interface{}
	for _, disk := range wire.Proxmox.Disks {
		switch disk["mountpoint"] {
		case "/":
			rootfs = disk
		case "/srv/archive":
			archive = disk
		}
	}
	if rootfs == nil || rootfs["usage"] != 37.14 || rootfs["used"] != float64(23530764893) {
		t.Fatalf("live rootfs row must survive on the wire, got %+v", rootfs)
	}
	if archive == nil {
		t.Fatal("config-only mount missing from the wire payload")
	}
	if archive["usage"] != float64(-1) {
		t.Fatalf("wire usage = %v, want the -1 unknown sentinel", archive["usage"])
	}
	if archive["total"] != float64(21474836480) {
		t.Fatalf("wire total = %v, want 20 GiB from size=", archive["total"])
	}
	if archive["filesystem"] != "mp0" || archive["device"] != "tank:subvol-106-disk-1" {
		t.Fatalf("wire row must keep mp key and device, got %+v", archive)
	}
	if _, present := archive["used"]; present {
		t.Fatalf("wire used must stay absent for unknown usage (omitempty contract), got %+v", archive)
	}
}
