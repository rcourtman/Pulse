package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
