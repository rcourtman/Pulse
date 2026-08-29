package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type pendingUpdatesPVEClient struct {
	*stubPVEClient
	packages []proxmox.AptPackage
	err      error
}

func (c *pendingUpdatesPVEClient) GetNodePendingUpdates(context.Context, string) ([]proxmox.AptPackage, error) {
	return c.packages, c.err
}

func TestApplyNodePendingUpdatesEvidenceStates(t *testing.T) {
	t.Run("confirmed zero", func(t *testing.T) {
		monitor := &Monitor{}
		node := &models.Node{}
		client := &pendingUpdatesPVEClient{stubPVEClient: &stubPVEClient{}}

		monitor.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "pve1"}, "lab-pve1", "online", node)

		if node.PendingUpdates != 0 || node.PendingUpdatesStatus != "checked" || node.PendingUpdatesCheckedAt.IsZero() {
			t.Fatalf("expected checked zero evidence, got %+v", node)
		}
	})

	t.Run("permission unavailable", func(t *testing.T) {
		monitor := &Monitor{}
		node := &models.Node{}
		client := &pendingUpdatesPVEClient{
			stubPVEClient: &stubPVEClient{},
			err:           errors.New("authentication error: API error 403 (Forbidden)"),
		}

		monitor.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "pve1"}, "lab-pve1", "online", node)

		if node.PendingUpdatesStatus != "unavailable" || node.PendingUpdatesReason != "permission_denied" || !node.PendingUpdatesCheckedAt.IsZero() {
			t.Fatalf("expected unavailable permission evidence, got %+v", node)
		}
	})

	t.Run("cached value becomes stale", func(t *testing.T) {
		checkedAt := time.Now().Add(-pendingUpdatesCacheTTL - time.Minute)
		monitor := &Monitor{nodePendingUpdatesCache: map[string]pendingUpdatesCache{
			"lab-pve1": {count: 7, checkedAt: checkedAt},
		}}
		node := &models.Node{}
		client := &pendingUpdatesPVEClient{
			stubPVEClient: &stubPVEClient{},
			err:           errors.New("no healthy nodes available"),
		}

		monitor.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "pve1"}, "lab-pve1", "online", node)

		if node.PendingUpdates != 7 || node.PendingUpdatesStatus != "stale" || node.PendingUpdatesReason != "source_unavailable" || !node.PendingUpdatesCheckedAt.Equal(checkedAt) {
			t.Fatalf("expected stale retained evidence, got %+v", node)
		}
	})

	t.Run("offline is not checked", func(t *testing.T) {
		monitor := &Monitor{}
		node := &models.Node{}
		client := &pendingUpdatesPVEClient{stubPVEClient: &stubPVEClient{}}

		monitor.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "pve1"}, "lab-pve1", "offline", node)

		if node.PendingUpdatesStatus != "not_checked" || node.PendingUpdatesReason != "node_offline" {
			t.Fatalf("expected offline not-checked evidence, got %+v", node)
		}
	})
}
