package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type proxmoxGuestMonitorResolver func(string) *monitoring.Monitor

type proxmoxGuestMonitoringObserver struct {
	resolveMonitor proxmoxGuestMonitorResolver
}

func newProxmoxGuestMonitoringObserver(resolveMonitor proxmoxGuestMonitorResolver) proxmoxGuestPostconditionObserver {
	if resolveMonitor == nil {
		return nil
	}
	return proxmoxGuestMonitoringObserver{resolveMonitor: resolveMonitor}
}

func (o proxmoxGuestMonitoringObserver) ObserveProxmoxGuest(ctx context.Context, resourceID, instance, node string, vmid int, kind proxmoxGuestKind) (proxmoxGuestPostconditionObservation, error) {
	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" {
		orgID = "default"
	}
	monitor := o.resolveMonitor(orgID)
	if monitor == nil {
		return proxmoxGuestPostconditionObservation{}, fmt.Errorf("monitor unavailable for organization %q", orgID)
	}
	observation, err := monitor.ObserveProxmoxGuest(ctx, instance, node, vmid, string(kind))
	receivedAt := time.Now().UTC()
	if err != nil {
		return proxmoxGuestPostconditionObservation{}, err
	}
	return proxmoxGuestPostconditionObservation{
		ObserverID:  "proxmox-api:" + orgID + ":" + strings.TrimSpace(instance),
		TrustDomain: "proxmox-control-plane:" + orgID + ":" + strings.TrimSpace(instance),
		Method:      "proxmox_api_guest_status_current",
		Snapshot: proxmoxGuestLifecycleSnapshot{
			Instance:   observation.Instance,
			Node:       observation.Node,
			VMID:       observation.VMID,
			Kind:       proxmoxGuestKind(observation.Kind),
			Status:     observation.Status,
			Uptime:     observation.Uptime,
			ObservedAt: observation.ObservedAt,
		},
		ReceivedAt: receivedAt,
	}, nil
}
