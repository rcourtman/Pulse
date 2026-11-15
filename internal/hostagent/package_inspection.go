package hostagent

import (
	"context"
	"errors"
	"slices"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// Collectors can report package state, but cannot use that inspector for an
// action preflight or mutation. The separately enrolled runner owns those.
type packageUpdateReporter interface {
	Snapshot(context.Context, bool) agentexec.HostPackageUpdateSnapshot
}

type packageUpdateInspector struct {
	manager *packageUpdateManager
}

func newPackageUpdateInspector(platform string, lease *packageManagerLease) *packageUpdateInspector {
	m := newPackageUpdateManager(platform, lease)
	m.run = runPackageInspectionInvocation
	m.lookPath = func(name string) (string, error) {
		if name != "apt-get" {
			return "", errors.New("package inspection accepts only apt-get")
		}
		return packageInspectionResolveAPT()
	}
	return &packageUpdateInspector{manager: m}
}

func (i *packageUpdateInspector) Snapshot(ctx context.Context, force bool) agentexec.HostPackageUpdateSnapshot {
	return i.manager.Snapshot(ctx, force)
}

// Keep the parser/cache/host lease shared, without exposing the manager's
// remediation methods. Even an accidental internal action call cannot turn
// this report-only command adapter into an executor or a containment fallback.
func runPackageInspectionInvocation(ctx context.Context, env []string, name string, args ...string) packageUpdateCommandResult {
	if len(env) != 0 || name != "apt-get" || !slices.Equal(args, []string{"-s", "-o", "Debug::NoLocking=1", "upgrade"}) {
		return packageUpdateCommandResult{exitCode: -1, err: errors.New("only the fixed APT report simulation is allowed")}
	}
	return runPackageInspectionCommand(ctx)
}
