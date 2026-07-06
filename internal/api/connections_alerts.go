package api

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type aggregatorRuntimeSources struct {
	orgID         string
	vmwarePoller  *monitoring.VMwarePoller
	truenasPoller *monitoring.TrueNASPoller
}

func buildAggregatorInputsWithRuntimeSources(
	ctx context.Context,
	cfg *config.Config,
	persistence *config.ConfigPersistence,
	monitor *monitoring.Monitor,
	runtime aggregatorRuntimeSources,
) aggregatorInputs {
	inputs := aggregatorInputs{now: time.Now()}

	if cfg != nil {
		inputs.pveInstances = cfg.PVEInstances
		inputs.pbsInstances = cfg.PBSInstances
		inputs.pmgInstances = cfg.PMGInstances
		inputs.apiTokens = append([]config.APITokenRecord(nil), cfg.APITokens...)
	}

	if persistence != nil {
		if vmw, err := persistence.LoadVMwareConfig(); err == nil {
			inputs.vmwareInstances = vmw
		}
		if tn, err := persistence.LoadTrueNASConfig(); err == nil {
			inputs.truenasInstances = tn
		}
		if availability, err := persistence.LoadAvailabilityTargets(); err == nil {
			inputs.availabilityTargets = availability
		}
	}

	orgID := runtime.orgID
	if orgID == "" {
		orgID = "default"
	}
	if runtime.vmwarePoller != nil && len(inputs.vmwareInstances) > 0 {
		inputs.vmwareSummaries = runtime.vmwarePoller.ConnectionSummaries(orgID, inputs.vmwareInstances)
	}
	if runtime.truenasPoller != nil && len(inputs.truenasInstances) > 0 {
		inputs.truenasSummaries = runtime.truenasPoller.ConnectionSummaries(orgID, inputs.truenasInstances)
	}

	if monitor != nil {
		inputs.hosts = monitor.HostsSnapshot()
		inputs.agentDesiredConfigs = connectionAgentDesiredConfigFingerprints(monitor, inputs.hosts, inputs.apiTokens)
		inputs.instanceHealth = instanceHealthByKey(monitor.SchedulerHealth())
		inputs.availabilityStatuses = monitor.AvailabilityStatusSnapshot()
	} else {
		inputs.hosts = []models.Host{}
		inputs.instanceHealth = map[string]monitoring.InstanceHealth{}
		inputs.availabilityStatuses = map[string]monitoring.AvailabilityProbeStatus{}
	}
	if mock.IsMockEnabled() {
		mockTargets, mockStatuses := mockAvailabilityConnectionInputs()
		inputs.availabilityTargets = mergeAvailabilityTargets(inputs.availabilityTargets, mockTargets)
		inputs.availabilityStatuses = mergeAvailabilityStatuses(inputs.availabilityStatuses, mockStatuses)
	}
	inputs.expectedAgentVersion = currentAgentTargetVersion()
	_ = ctx
	return inputs
}

// connectionTypeForAlerts maps the wire ConnectionType to the alerts package's
// narrow platform set. Types that fall outside the platform set return false
// so the caller drops the row before invoking CheckConnection.
func connectionTypeForAlerts(t ConnectionType) (alerts.ConnectionType, bool) {
	switch t {
	case ConnectionTypePVE:
		return alerts.ConnectionTypePVE, true
	case ConnectionTypePBS:
		return alerts.ConnectionTypePBS, true
	case ConnectionTypePMG:
		return alerts.ConnectionTypePMG, true
	case ConnectionTypeVMware:
		return alerts.ConnectionTypeVMware, true
	case ConnectionTypeTrueNAS:
		return alerts.ConnectionTypeTrueNAS, true
	default:
		return "", false
	}
}

// snapshotConnectionsForAlerts translates the aggregator's rich Connection
// rows into the narrow alerts.ConnectionSnapshot view. Non-platform types
// (agent, availability, docker, kubernetes) are dropped because they have
// their own alert lifecycles.
func snapshotConnectionsForAlerts(connections []Connection) []alerts.ConnectionSnapshot {
	out := make([]alerts.ConnectionSnapshot, 0, len(connections))
	for _, conn := range connections {
		alertType, ok := connectionTypeForAlerts(conn.Type)
		if !ok {
			continue
		}

		snap := alerts.ConnectionSnapshot{
			ID:          conn.ID,
			Name:        conn.Name,
			Type:        alertType,
			State:       alerts.ConnectionState(conn.State),
			StateReason: conn.StateReason,
			Enabled:     conn.Enabled,
			LastSeen:    conn.LastSeen,
		}
		if conn.LastError != nil {
			snap.LastError = &alerts.ConnectionErrorSnapshot{
				At:       conn.LastError.At,
				Message:  conn.LastError.Message,
				Category: conn.LastError.Category,
			}
		}
		out = append(out, snap)
	}
	return out
}

func buildAlertConnectionSnapshotsWithRuntimeSources(
	ctx context.Context,
	cfg *config.Config,
	persistence *config.ConfigPersistence,
	monitor *monitoring.Monitor,
	runtime aggregatorRuntimeSources,
) []alerts.ConnectionSnapshot {
	inputs := buildAggregatorInputsWithRuntimeSources(ctx, cfg, persistence, monitor, runtime)
	return snapshotConnectionsForAlerts(buildConnections(inputs))
}
