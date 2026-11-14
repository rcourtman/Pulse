package cloudcp

import (
	"context"
	"fmt"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

type tenantRuntimeTransaction struct {
	service                   *tenantRuntimeRolloutService
	lease                     *lifecycle.Lease
	tenant                    *registry.Tenant
	previous                  *cpDocker.RuntimeContainerInfo
	dataDir, canonicalName    string
	healthTimeout, healthPoll time.Duration
	record                    lifecycle.Recovery
}

func (t *tenantRuntimeTransaction) held(err error) error {
	return fmt.Errorf("%w; automatic recovery stopped; lifecycle hold retained for tenant %s (previous=%s new=%s backup=%s snapshot=%s)", err, t.tenant.ID, t.record.PreviousContainerID, t.record.NewContainerID, t.record.BackupContainerName, t.record.SnapshotDir)
}

func (t *tenantRuntimeTransaction) requireStopped(ctx context.Context, id string) error {
	info, err := t.service.docker.Inspect(ctx, id)
	if err != nil {
		return fmt.Errorf("confirm stopped container %s: %w", id, err)
	}
	if info == nil || info.ID != id || !info.Stopped || info.Running || info.Restarting || info.Paused {
		return fmt.Errorf("container %s is not confirmed stopped", id)
	}
	return nil
}

// abort always has its own bounded context, while retaining the lifecycle
// lease. A canceled request must not skip compensation or release a partial
// restore to the health monitor. Every failed prerequisite stops the sequence.
func (t *tenantRuntimeTransaction) abort(ctx context.Context, cause error, restore bool) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	defer cancel()
	t.record.Phase = "rollback"
	if err := t.lease.MarkRecovery(t.record); err != nil {
		return t.held(fmt.Errorf("%w; record rollback: %v", cause, err))
	}
	if restore {
		if err := t.removeReplacement(ctx); err != nil {
			return t.held(fmt.Errorf("%w; remove replacement: %v", cause, err))
		}
		if t.previous != nil {
			if err := t.requireStopped(ctx, t.previous.ID); err != nil {
				return t.held(fmt.Errorf("%w; %v", cause, err))
			}
		}
		if err := t.service.synchronizer.Restore(ctx, t.record.SnapshotDir, t.dataDir); err != nil {
			return t.held(fmt.Errorf("%w; restore tenant data: %v", cause, err))
		}
	}
	if err := t.resumePrevious(ctx); err != nil {
		return t.held(fmt.Errorf("%w; recover previous runtime: %v", cause, err))
	}
	if err := t.lease.Complete(); err != nil {
		return t.held(fmt.Errorf("%w; complete rollback: %v", cause, err))
	}
	return fmt.Errorf("%w; rollback completed (snapshot=%s)", cause, t.record.SnapshotDir)
}

func (t *tenantRuntimeTransaction) removeReplacement(ctx context.Context) error {
	d := t.service.docker
	canonical, err := d.Inspect(ctx, t.canonicalName)
	if err != nil && !cpDocker.IsNotFound(err) {
		return err
	}
	id := t.record.NewContainerID
	if canonical != nil {
		if t.previous != nil && canonical.ID == t.previous.ID {
			return fmt.Errorf("previous runtime unexpectedly occupies the replacement name")
		}
		if id != "" && canonical.ID != id {
			return fmt.Errorf("replacement identity differs from canonical container")
		}
		// A create call can have reached the daemon without returning an ID.
		// The canonical name was vacated under this same lifecycle lease.
		id = canonical.ID
	}
	if id == "" {
		return nil
	}
	if t.previous != nil && id == t.previous.ID {
		return fmt.Errorf("replacement identity is the previous runtime")
	}
	info, err := d.Inspect(ctx, id)
	if cpDocker.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info == nil || info.ID != id {
		return fmt.Errorf("replacement identity is unknown")
	}
	t.record.NewContainerID = id
	if err := t.lease.MarkRecovery(t.record); err != nil {
		return err
	}
	if err := d.Stop(ctx, id); err != nil {
		return err
	}
	if err := t.requireStopped(ctx, id); err != nil {
		return err
	}
	if err := d.Remove(ctx, id); err != nil {
		return err
	}
	if info, err := d.Inspect(ctx, id); !cpDocker.IsNotFound(err) {
		return fmt.Errorf("replacement removal is unconfirmed (present=%t): %v", info != nil, err)
	}
	if info, err := d.Inspect(ctx, t.canonicalName); !cpDocker.IsNotFound(err) {
		return fmt.Errorf("canonical name is not confirmed vacant (present=%t): %v", info != nil, err)
	}
	return nil
}

func (t *tenantRuntimeTransaction) resumePrevious(ctx context.Context) error {
	if t.previous == nil {
		updated := *t.tenant
		updated.HealthCheckOK = false
		now := t.service.now()
		updated.LastHealthCheck = &now
		return t.service.registry.Update(&updated)
	}
	d := t.service.docker
	info, err := d.Inspect(ctx, t.previous.ID)
	if err != nil {
		return err
	}
	if info == nil || info.ID != t.previous.ID {
		return fmt.Errorf("previous runtime identity is unknown")
	}
	if info.Name != t.canonicalName {
		if info.Name != t.record.BackupContainerName && info.Name != t.previous.Name {
			return fmt.Errorf("previous runtime has an unexpected name")
		}
		if err := t.requireStopped(ctx, info.ID); err != nil {
			return err
		}
		if occupied, err := d.Inspect(ctx, t.canonicalName); !cpDocker.IsNotFound(err) {
			return fmt.Errorf("cannot reclaim canonical name (occupied=%t): %v", occupied != nil, err)
		}
		if err := d.Rename(ctx, info.ID, t.canonicalName); err != nil {
			return err
		}
	}
	canonical, err := d.Inspect(ctx, t.canonicalName)
	if err != nil {
		return err
	}
	if canonical == nil || canonical.ID != t.previous.ID {
		return fmt.Errorf("previous runtime is not canonical")
	}
	// A failed pre-swap capture must not start a previously stopped runtime.
	if t.previous.Running {
		if !canonical.Running {
			if err := t.requireStopped(ctx, canonical.ID); err != nil {
				return err
			}
			if err := d.Start(ctx, canonical.ID); err != nil {
				return err
			}
		}
		healthy, healthErr := t.service.waitForHealth(ctx, canonical.ID, t.healthTimeout, t.healthPoll)
		if err := t.service.persistTenantRuntimeState(t.tenant, canonical, healthy); err != nil {
			return err
		}
		return healthErr
	}
	if err := t.requireStopped(ctx, canonical.ID); err != nil {
		return err
	}
	return t.service.persistTenantRuntimeState(t.tenant, canonical, false)
}
