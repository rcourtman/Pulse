//go:build linux

package hostagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

var (
	collectorTypedActionEUID       = os.Geteuid
	collectorTypedActionSelfCgroup = func() ([]byte, error) { return os.ReadFile("/proc/self/cgroup") }
	collectorTypedActionReconcile  = reconcileTypedActionUnitsUnder
	collectorTypedActionHasProxmox = func() bool {
		for _, tool := range []string{"pct", "qm"} {
			if _, err := typedActionResolveTarget(typedActionCatalogProxmox, tool); err == nil {
				return true
			}
		}
		return false
	}
)

// prepareCollectorTypedActions decides whether this collector can run a typed
// command. It must be root, because the unit and its result file are root's.
// It must be running as its own systemd service, because the unit is bound to
// that service and dies with it. And the probe must pass, which proves PID 1
// started, contained and collected a unit built exactly like a real one.
func prepareCollectorTypedActions(ctx context.Context, supervisor typedActionSupervisor) (string, error) {
	if !collectorTypedActionHasProxmox() {
		return "", errCollectorTypedActionsNotApplicable
	}
	if collectorTypedActionEUID() != 0 {
		return agentexec.ProxmoxGuestStartUnavailableContainment, errors.New("the Pulse agent is not running as root")
	}
	data, err := collectorTypedActionSelfCgroup()
	if err != nil {
		return agentexec.ProxmoxGuestStartUnavailableNotSystemdService, fmt.Errorf("read own cgroup: %w", err)
	}
	if unit := systemdUnitFromCgroup(string(data)); unit != supervisor.serviceUnit {
		return agentexec.ProxmoxGuestStartUnavailableNotSystemdService, fmt.Errorf("the Pulse agent is not running as %s (found %q)", supervisor.serviceUnit, unit)
	}
	if err := collectorTypedActionReconcile(ctx, supervisor); err != nil {
		return agentexec.ProxmoxGuestStartUnavailableContainment, err
	}
	return "", nil
}

// systemdUnitFromCgroup names the systemd unit a process runs in, from the
// content of /proc/<pid>/cgroup. It reads the unified hierarchy, or the named
// systemd hierarchy on a host that still has one. The unit is the cgroup
// itself, so a process systemd did not start as that unit never matches.
func systemdUnitFromCgroup(content string) string {
	unified, named := "", ""
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		switch parts[1] {
		case "":
			unified = parts[2]
		case "name=systemd":
			named = parts[2]
		}
	}
	cgroup := named
	if cgroup == "" {
		cgroup = unified
	}
	if cgroup == "" || !strings.HasPrefix(cgroup, "/") {
		return ""
	}
	return path.Base(cgroup)
}
