package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type clusterOptionsPVEClient struct {
	PVEClientInterface
	tagStyle string
}

func (c clusterOptionsPVEClient) GetClusterOptions(context.Context) (*proxmox.ClusterOptions, error) {
	return &proxmox.ClusterOptions{TagStyle: c.tagStyle}, nil
}

// Proxmox tag colours are per PVE instance and may be case-sensitive. A later
// refresh that clears one instance's colour map must not leave that instance's
// colours behind in the aggregate map shown to the UI.
func TestRefreshPVETagColorsKeepsStylePerInstance(t *testing.T) {
	m := &Monitor{state: models.NewState()}
	ctx := context.Background()

	m.refreshPVETagColors(ctx, "pve-a", clusterOptionsPVEClient{tagStyle: "case-sensitive=1,color-map=Production:00ff00"})
	m.refreshPVETagColors(ctx, "pve-b", clusterOptionsPVEClient{tagStyle: "color-map=backup:112233"})

	snapshot := m.state.GetSnapshot()
	styleA := snapshot.PVETagStyles["pve-a"]
	if !styleA.CaseSensitive || styleA.Colors["Production"] != "#00ff00" {
		t.Fatalf("pve-a style = %+v, want case-sensitive Production=#00ff00", styleA)
	}
	if styleB := snapshot.PVETagStyles["pve-b"]; styleB.CaseSensitive || styleB.Colors["backup"] != "#112233" {
		t.Fatalf("pve-b style = %+v, want case-insensitive backup=#112233", styleB)
	}

	m.refreshPVETagColors(ctx, "pve-a", clusterOptionsPVEClient{tagStyle: ""})

	snapshot = m.state.GetSnapshot()
	if len(snapshot.PVETagStyles["pve-a"].Colors) != 0 {
		t.Fatalf("pve-a style = %+v, want cleared colours", snapshot.PVETagStyles["pve-a"])
	}
	if _, stale := snapshot.PVETagColors["Production"]; stale {
		t.Fatalf("aggregate colours = %+v, want pve-a Production removed", snapshot.PVETagColors)
	}
	if got := snapshot.PVETagColors["backup"]; got != "#112233" {
		t.Fatalf("aggregate colours = %+v, want pve-b backup kept", snapshot.PVETagColors)
	}
}
