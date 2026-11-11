package hostagent

import (
	"context"
	"errors"
	"fmt"
)

type typedActionCatalog string

const (
	typedActionCatalogPackage        typedActionCatalog = "package"
	typedActionCatalogProxmox        typedActionCatalog = "proxmox"
	typedActionCatalogProxmoxHandoff typedActionCatalog = "proxmox-handoff"
	typedActionCatalogProbe          typedActionCatalog = "probe"
)

const (
	typedActionLauncherCommand               = "__pulse_typed_action_launcher"
	typedActionLauncherDescendantExit        = 200
	typedActionLauncherInspectionFailureExit = 201
	typedActionLauncherStartFailureExit      = 202
	typedActionLauncherSupervisorFailureExit = 203
)

var (
	errTypedActionContainmentUnavailable   = errors.New("typed action systemd containment unavailable")
	errTypedActionContainmentIndeterminate = errors.New("typed action containment is indeterminate")
)

type typedActionCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

// typedActionSupervisor is the long-running Pulse service a transient typed
// action unit belongs to. The unit is bound to that service, named with its
// prefix and reports through its private state directory. Every other
// containment property is the same for both supervisors.
type typedActionSupervisor struct {
	serviceUnit string
	unitPrefix  string
	stateDir    func() (string, error)
	// allow narrows the closed catalog further. Nil keeps the whole catalog.
	allow func(typedActionCatalog, string, []string) error
}

const (
	typedActionRunnerServiceUnit    = "pulse-agent-runner.service"
	typedActionCollectorServiceUnit = "pulse-agent.service"
	// The collector names its units apart from the runner's, so neither
	// supervisor's start-up reconciliation can stop the other's work.
	typedActionCollectorUnitPrefix = "pulse-agent-collector-action-"
)

// runTypedActionCommand is the single subprocess boundary for fixed-catalog
// privileged actions. Linux runs the closed invocation inside a transient
// systemd service and does not return until PID 1 has collected that unit.
func runTypedActionCommand(ctx context.Context, env []string, catalog typedActionCatalog, name string, args ...string) typedActionCommandResult {
	if err := validateTypedActionInvocation(catalog, name, args); err != nil {
		return typedActionCommandResult{exitCode: -1, err: err}
	}
	return runTypedActionCommandPlatform(ctx, env, catalog, name, args...)
}

// validateCollectorTypedActionInvocation is the whole of what a collector with
// commands enabled may run through typed containment: the containment probe,
// a guest status read, and a guest start. Stop, shutdown, reboot and every
// package command stay with the action runner.
func validateCollectorTypedActionInvocation(catalog typedActionCatalog, name string, args []string) error {
	if err := validateTypedActionInvocation(catalog, name, args); err != nil {
		return err
	}
	switch catalog {
	case typedActionCatalogProbe:
		return nil
	case typedActionCatalogProxmox:
		if len(args) == 2 && args[0] == "status" {
			return nil
		}
	case typedActionCatalogProxmoxHandoff:
		if len(args) == 2 && args[0] == "start" {
			return nil
		}
	}
	return fmt.Errorf("typed action invocation is outside the collector's %s catalog", catalog)
}

func validateTypedActionInvocation(catalog typedActionCatalog, name string, args []string) error {
	switch catalog {
	case typedActionCatalogPackage:
		if name == "dpkg" && len(args) == 1 && args[0] == "--audit" {
			return nil
		}
		if name == "apt-get" {
			switch fmt.Sprint(args) {
			case "[-s -o Debug::NoLocking=1 upgrade]", "[update]", "[-y --no-remove -o Dpkg::Options::=--force-confold upgrade]", "[clean]":
				return nil
			}
		}
	case typedActionCatalogProxmox:
		if (name == "qm" || name == "pct") && len(args) == 2 && isProxmoxContainedLifecycleInvocation(name, args[0]) && isPositiveDecimalID(args[1]) {
			return nil
		}
	case typedActionCatalogProxmoxHandoff:
		if (name == "qm" || name == "pct") && len(args) == 2 && isProxmoxHandoffLifecycleInvocation(name, args[0]) && isPositiveDecimalID(args[1]) {
			return nil
		}
	case typedActionCatalogProbe:
		if name == "true" && len(args) == 0 {
			return nil
		}
	}
	return fmt.Errorf("typed action invocation is outside the closed %s catalog", catalog)
}

func isProxmoxContainedLifecycleInvocation(tool, verb string) bool {
	switch verb {
	case "status", "stop", "shutdown":
		return true
	case "reboot":
		return tool == "qm"
	default:
		return false
	}
}

func isProxmoxHandoffLifecycleInvocation(tool, verb string) bool {
	switch verb {
	case "start":
		return true
	case "reboot":
		return tool == "pct"
	default:
		return false
	}
}

func isPositiveDecimalID(value string) bool {
	if value == "" || len(value) > 10 || value == "0" || value[0] == '0' {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
