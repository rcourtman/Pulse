package hostagent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

// collectorTypedActions is the typed containment a collector with commands
// enabled owns. It exists so that collector can start a stopped Proxmox guest
// the same way the action runner does: one fixed command in a transient
// systemd unit, with a durable receipt. The unit is bound to the collector's
// own service and reports through the collector's own state directory. The
// collector's catalog is the containment probe, a guest status read and a
// guest start.
//
// Nothing runs until prepare has passed once in this process. prepare stops
// this collector's stale units, so it must never run while one of them could
// be in flight, and the ready flag is what guarantees that.
type collectorTypedActions struct {
	stateDir string
	logger   zerolog.Logger
	prepare  func(context.Context, typedActionSupervisor) (string, error)
	run      func(context.Context, typedActionSupervisor, []string, typedActionCatalog, string, ...string) typedActionCommandResult
	// wait is how long a registration waits for a preparation. Containment
	// waits for systemd without a limit of its own, and the command session
	// must not wait with it: everything else the session carries still works.
	wait time.Duration

	mu          sync.Mutex
	ready       bool
	unavailable string
	preparing   chan struct{}
	loggedCause string
}

const (
	collectorTypedActionPrepareTimeout = 30 * time.Second
	collectorTypedActionPrepareWait    = 15 * time.Second
)

// errCollectorTypedActionsNotApplicable is the answer on a host that is not a
// Proxmox node. There is no guest to start there, so the agent reports
// nothing and logs nothing.
var errCollectorTypedActionsNotApplicable = errors.New("this host is not a Proxmox node")

func newCollectorTypedActions(stateDir string, logger zerolog.Logger) *collectorTypedActions {
	return &collectorTypedActions{
		stateDir: strings.TrimSpace(stateDir),
		logger:   logger,
		prepare:  prepareCollectorTypedActions,
		run:      runTypedActionCommandUnder,
		wait:     collectorTypedActionPrepareWait,
	}
}

func (c *collectorTypedActions) supervisor() typedActionSupervisor {
	stateDir := c.stateDir
	return typedActionSupervisor{
		serviceUnit: typedActionCollectorServiceUnit,
		unitPrefix:  typedActionCollectorUnitPrefix,
		stateDir:    func() (string, error) { return stateDir, nil },
		allow:       validateCollectorTypedActionInvocation,
	}
}

// ensure reports whether typed containment is ready, preparing it if this
// process has not yet done so. One preparation runs at a time. A failed one is
// tried again on the next call, which is the next registration. A preparation
// that outlasts the wait keeps going in the background and the caller
// registers without guest start. A later registration does not wait for it
// again: it reads what the preparation has found so far.
func (c *collectorTypedActions) ensure(ctx context.Context) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	if c.ready {
		c.mu.Unlock()
		return true
	}
	if c.prepare == nil {
		c.unavailable = agentexec.ProxmoxGuestStartUnavailableContainment
		c.mu.Unlock()
		return false
	}
	done := c.preparing
	started := done == nil
	if started {
		done = make(chan struct{})
		c.preparing = done
		go c.runPrepare(done)
	}
	c.mu.Unlock()

	if started {
		wait := c.wait
		if wait <= 0 {
			wait = collectorTypedActionPrepareWait
		}
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-done:
		case <-ctx.Done():
		case <-timer.C:
			c.logger.Warn().Dur("waited", wait).
				Msg("Typed action containment is still being prepared. This session registers without Proxmox guest start")
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ready && c.preparing != nil && c.unavailable == "" {
		c.unavailable = agentexec.ProxmoxGuestStartUnavailableContainment
	}
	return c.ready
}

func (c *collectorTypedActions) runPrepare(done chan struct{}) {
	prepareCtx, cancel := context.WithTimeout(context.Background(), collectorTypedActionPrepareTimeout)
	reason, err := c.prepare(prepareCtx, c.supervisor())
	cancel()

	c.mu.Lock()
	switch {
	case err == nil:
		c.ready, c.unavailable = true, ""
	case errors.Is(err, errCollectorTypedActionsNotApplicable):
		c.unavailable = ""
	default:
		if reason = agentexec.NormalizeProxmoxGuestStartUnavailable(reason); reason == "" {
			reason = agentexec.ProxmoxGuestStartUnavailableContainment
		}
		c.unavailable = reason
	}
	ready, unavailable := c.ready, c.unavailable
	// The same cause is logged once, not at every reconnect.
	cause := ""
	if err != nil && unavailable != "" {
		cause = unavailable + ": " + err.Error()
	}
	logCause := cause != "" && cause != c.loggedCause
	c.loggedCause = cause
	c.preparing = nil
	c.mu.Unlock()
	close(done)

	switch {
	case ready:
		c.logger.Info().Msg("Typed action containment is ready: this Pulse agent can start a stopped Proxmox guest after an admin approves it")
	case logCause:
		c.logger.Warn().Err(err).Str("reason", unavailable).
			Msg("This Pulse agent cannot start Proxmox guests: typed action containment is not available")
	}
}

func (c *collectorTypedActions) isReady() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ready
}

// registration is what the collector tells Pulse when it registers: the
// protocol version when it can start guests, or the closed reason it cannot.
func (c *collectorTypedActions) registration() (version int, unavailable string) {
	if c == nil {
		return 0, ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ready {
		return agentexec.ProxmoxGuestStartProtocolVersion, ""
	}
	return 0, c.unavailable
}

func (c *collectorTypedActions) runCommand(ctx context.Context, env []string, catalog typedActionCatalog, name string, args ...string) typedActionCommandResult {
	if !c.isReady() || c.run == nil {
		return typedActionCommandResult{exitCode: -1, err: errTypedActionContainmentUnavailable}
	}
	return c.run(ctx, c.supervisor(), env, catalog, name, args...)
}
