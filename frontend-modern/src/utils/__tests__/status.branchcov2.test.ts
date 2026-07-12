/**
 * Branch-coverage tests for status.ts — second pass.
 *
 * Focused on branches of the named indicator/label helpers that the sibling
 * status.test.ts does not yet reach: the `?? `/`||` coalescing arms, the
 * `connection || status` label-priority ternaries, the unknown/empty fallthrough
 * arms, and the badge-tone class table lookup (including its `|| muted` guard).
 *
 * Fixtures use `Partial<Node>`/`Partial<...>` exactly like the public types the
 * functions accept; a single deliberately-malformed badge-variant input is cast
 * via `as unknown as StatusIndicatorVariant` to exercise the defensive `||`.
 */
import { describe, expect, it } from 'vitest';

import type { StatusIndicatorVariant } from '@/utils/status';
import {
  formatStatusLabel,
  getCanonicalStatusLabel,
  getStatusIndicatorBadgeToneClasses,
  isNodeOnline,
  getNodeStatusIndicator,
  getDockerHostStatusIndicator,
  getDockerContainerStatusIndicator,
  getDockerServiceStatusIndicator,
  getAgentStatusIndicator,
  getPBSStatusIndicator,
  getReplicationJobStatusIndicator,
} from '@/utils/status';

describe('formatStatusLabel (branch coverage)', () => {
  it('returns the fallback when value is null', () => {
    // `if (!value)` arm — null is falsy.
    expect(formatStatusLabel(null, 'N/A')).toBe('N/A');
  });

  it('returns the fallback for a whitespace-only value (post-trim guard)', () => {
    // `if (!normalized)` arm — trim() yields '' which is falsy.
    expect(formatStatusLabel('   ', 'Unavailable')).toBe('Unavailable');
  });

  it('uses the default "Unknown" fallback when none is provided for empty input', () => {
    expect(formatStatusLabel('')).toBe('Unknown');
  });

  it('capitalizes the first character without lowercasing the rest of a multi-word value', () => {
    // capitalize arm: only charAt(0).toUpperCase() is applied; the tail keeps case.
    expect(formatStatusLabel('sync IN progress')).toBe('Sync IN progress');
  });
});

describe('getCanonicalStatusLabel (branch coverage)', () => {
  it('returns the default fallback for undefined and the custom fallback for whitespace', () => {
    // `if (!normalized)` arm via normalize('') on undefined and whitespace.
    expect(getCanonicalStatusLabel(undefined)).toBe('Unknown');
    expect(getCanonicalStatusLabel('   ', 'Pending')).toBe('Pending');
  });

  it('resolves every canonical label in STATUS_LABELS through the lowercase lookup', () => {
    // STATUS_LABELS[normalized] hit arm, including mixed-case normalization.
    expect(getCanonicalStatusLabel('ONLINE')).toBe('Online');
    expect(getCanonicalStatusLabel('Degraded')).toBe('Degraded');
    expect(getCanonicalStatusLabel('PAUSED')).toBe('Paused');
    expect(getCanonicalStatusLabel('Stopped')).toBe('Stopped');
    expect(getCanonicalStatusLabel('UNKNOWN')).toBe('Unknown');
  });

  it('falls back to the trimmed raw value (original case preserved) when the status is not canonical', () => {
    // `|| raw` arm — `raw` is the pre-lowercase trim(), so case is preserved.
    expect(getCanonicalStatusLabel('CustomState')).toBe('CustomState');
    expect(getCanonicalStatusLabel('waiting-for-quorum')).toBe('waiting-for-quorum');
  });
});

describe('getStatusIndicatorBadgeToneClasses (branch coverage)', () => {
  it('returns the exact tone class string for every declared variant', () => {
    // Direct table-hit arm for all five variants, with full string assertion
    // (the sibling test only uses toContain for four of them).
    expect(getStatusIndicatorBadgeToneClasses('success')).toBe(
      'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
    );
    expect(getStatusIndicatorBadgeToneClasses('warning')).toBe(
      'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    );
    expect(getStatusIndicatorBadgeToneClasses('danger')).toBe(
      'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
    );
    expect(getStatusIndicatorBadgeToneClasses('info')).toBe(
      'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
    );
    expect(getStatusIndicatorBadgeToneClasses('muted')).toBe(
      'bg-surface-alt text-base-content',
    );
  });

  it('falls back to the muted tone class for a variant not present in the table', () => {
    // `|| STATUS_INDICATOR_BADGE_TONE_CLASSES.muted` defensive arm — driven by a
    // deliberately-malformed variant cast to satisfy the nominal type.
    const invalid = 'chartreuse' as unknown as StatusIndicatorVariant;
    expect(getStatusIndicatorBadgeToneClasses(invalid)).toBe(
      'bg-surface-alt text-base-content',
    );
  });
});

describe('isNodeOnline (branch coverage)', () => {
  it('treats an undefined uptime as zero via the `?? 0` coalescing (returns false)', () => {
    // `(node.uptime ?? 0) <= 0` arm when uptime is absent.
    expect(isNodeOnline({ status: 'online' })).toBe(false);
  });

  it('returns true when connectionHealth is absent (normalize("") is neither offline nor error)', () => {
    // Happy-path fall-through: connection is undefined -> normalize('') -> ''.
    expect(isNodeOnline({ status: 'online', uptime: 1000, connectionHealth: undefined })).toBe(
      true,
    );
  });

  it('normalizes connectionHealth case-insensitively before the offline/error check', () => {
    // normalize() lowercases, so 'OFFLINE' / 'ERROR' hit the reject arm.
    expect(isNodeOnline({ status: 'online', uptime: 1000, connectionHealth: 'OFFLINE' })).toBe(
      false,
    );
    expect(isNodeOnline({ status: 'online', uptime: 1000, connectionHealth: 'Error' })).toBe(
      false,
    );
  });

  it('rejects a whitespace-only connectionHealth neither matches offline nor error, but is still treated as connected', () => {
    // '   ' normalizes to '' — not rejected, so an otherwise-healthy node is online.
    expect(isNodeOnline({ status: 'online', uptime: 1000, connectionHealth: '   ' })).toBe(true);
  });
});

describe('getNodeStatusIndicator (branch coverage)', () => {
  it('flags danger from an offline connectionHealth even when status is online', () => {
    // OFFLINE_HEALTH_STATUSES.has(connection) arm; label prefers connection.
    const result = getNodeStatusIndicator({
      status: 'online',
      uptime: 1000,
      connectionHealth: 'disconnected',
    });
    expect(result).toEqual({ variant: 'danger', label: 'Disconnected' });
  });

  it('prefers connection over status when both are offline for the danger label', () => {
    // `formatStatusLabel(connection || status, 'Offline')` — connection wins.
    const result = getNodeStatusIndicator({
      status: 'offline',
      uptime: 0,
      connectionHealth: 'timeout',
    });
    expect(result).toEqual({ variant: 'danger', label: 'Timeout' });
  });

  it('falls back to the status for the danger label when connection is absent', () => {
    // connection '' -> `'' || status` -> status drives the label.
    const result = getNodeStatusIndicator({ status: 'unreachable', uptime: 1000 });
    expect(result).toEqual({ variant: 'danger', label: 'Unreachable' });
  });

  it('flags warning from a degraded connectionHealth that is not in the offline set', () => {
    // DEGRADED_HEALTH_STATUSES.has(connection) arm; label prefers connection.
    const result = getNodeStatusIndicator({
      status: 'online',
      uptime: 1000,
      connectionHealth: 'syncing',
    });
    expect(result).toEqual({ variant: 'warning', label: 'Syncing' });
  });

  it('returns muted default indicator when status is neither offline, degraded, nor online', () => {
    // Final `return defaultIndicator` arm — 'paused' is in no health-status set
    // and isNodeOnline is false because status !== 'online'.
    const result = getNodeStatusIndicator({ status: 'paused', uptime: 1000 });
    expect(result).toEqual({ variant: 'muted', label: 'Unknown' });
  });
});

describe('getDockerHostStatusIndicator (branch coverage)', () => {
  it('returns warning for a degraded status via the host object', () => {
    // DEGRADED_HEALTH_STATUSES.has(status) arm.
    expect(getDockerHostStatusIndicator({ status: 'degraded' })).toEqual({
      variant: 'warning',
      label: 'Degraded',
    });
  });

  it('returns a muted indicator with a formatted label for an unknown non-empty status string', () => {
    // `status ? { muted, label }` truthy arm — 'paused' is in no status set and
    // not a connected-health status, so it falls to the muted-with-label branch.
    expect(getDockerHostStatusIndicator('paused')).toEqual({
      variant: 'muted',
      label: 'Paused',
    });
  });

  it('returns the muted default indicator when no status is available', () => {
    // `: defaultIndicator` arm — empty normalized status is falsy.
    expect(getDockerHostStatusIndicator(undefined)).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
    expect(getDockerHostStatusIndicator({ status: undefined })).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
  });
});

describe('getDockerContainerStatusIndicator (branch coverage)', () => {
  it('falls to the warning branch when a running container has a non-healthy, non-unhealthy health', () => {
    // The running-success guard requires (!health || health === 'healthy');
    // 'starting' is truthy and not 'healthy', so it skips success and falls to
    // the trailing `if (state)` warning arm.
    expect(getDockerContainerStatusIndicator({ state: 'running', health: 'starting' })).toEqual({
      variant: 'warning',
      label: 'Running',
    });
  });

  it('labels an error state with a capitalized version of the state name', () => {
    // ERROR_CONTAINER_STATES.has(state) arm where health !== 'unhealthy', so the
    // label is formatStatusLabel(state, 'Error').
    expect(getDockerContainerStatusIndicator({ state: 'oomkilled' })).toEqual({
      variant: 'danger',
      label: 'Oomkilled',
    });
  });

  it('prioritizes the unhealthy label over a stopped state', () => {
    // 'created' is in STOPPED_CONTAINER_STATES, but the unhealthy check comes
    // first and forces the 'Unhealthy' label.
    expect(getDockerContainerStatusIndicator({ state: 'created', health: 'unhealthy' })).toEqual({
      variant: 'danger',
      label: 'Unhealthy',
    });
  });

  it('returns a warning indicator from a health-only signal when state is empty', () => {
    // `if (!state && health)` arm — state absent, health present.
    expect(getDockerContainerStatusIndicator({ state: undefined, health: 'starting' })).toEqual({
      variant: 'warning',
      label: 'Starting',
    });
  });

  it('returns a warning indicator for a custom state that is in no state set', () => {
    // `if (state)` trailing warning arm — 'deploying' is not running, not in the
    // error set, not in the stopped set.
    expect(getDockerContainerStatusIndicator({ state: 'deploying' })).toEqual({
      variant: 'warning',
      label: 'Deploying',
    });
  });

  it('returns the muted default indicator when neither state nor health is present', () => {
    // Final `return defaultIndicator` arm.
    expect(getDockerContainerStatusIndicator({})).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
  });
});

describe('getDockerServiceStatusIndicator (branch coverage)', () => {
  it('coalesces undefined task counts to zero and reports "No tasks"', () => {
    // `service.desiredTasks ?? 0` and `service.runningTasks ?? 0` defaulting arms.
    expect(getDockerServiceStatusIndicator({})).toEqual({
      variant: 'muted',
      label: 'No tasks',
    });
  });

  it('uses the singular "task" form when exactly one task is running with none desired', () => {
    // `running === 1 ? '' : 's'` singular arm inside the desired<=0 warning.
    expect(getDockerServiceStatusIndicator({ desiredTasks: 0, runningTasks: 1 })).toEqual({
      variant: 'warning',
      label: 'Running 1 task',
    });
  });

  it('reports healthy when running exceeds desired (>= arm, strictly-greater path)', () => {
    // `if (running >= desired)` arm with running > desired.
    expect(getDockerServiceStatusIndicator({ desiredTasks: 2, runningTasks: 5 })).toEqual({
      variant: 'success',
      label: 'Healthy',
    });
  });
});

describe('getAgentStatusIndicator (branch coverage)', () => {
  it('returns success with the "Online" label for a running agent', () => {
    // `status === RUNNING_STATUS` arm of the online/running check.
    expect(getAgentStatusIndicator({ status: 'running' })).toEqual({
      variant: 'success',
      label: 'Online',
    });
  });

  it('labels offline statuses from the OFFLINE set with a capitalized status', () => {
    // OFFLINE arm with a non-'offline' member to exercise formatStatusLabel.
    expect(getAgentStatusIndicator({ status: 'unreachable' })).toEqual({
      variant: 'danger',
      label: 'Unreachable',
    });
  });

  it('labels degraded statuses from the DEGRADED set with a capitalized status', () => {
    // DEGRADED arm with a non-'degraded' member.
    expect(getAgentStatusIndicator({ status: 'syncing' })).toEqual({
      variant: 'warning',
      label: 'Syncing',
    });
  });

  it('returns a muted indicator with a formatted label for an unknown non-empty status', () => {
    // `status ? { muted, label }` truthy arm — 'idle' is in no status set.
    expect(getAgentStatusIndicator({ status: 'idle' })).toEqual({
      variant: 'muted',
      label: 'Idle',
    });
  });

  it('returns the muted default indicator when agent status is empty', () => {
    // `: defaultIndicator` arm — normalized status is '' (falsy).
    expect(getAgentStatusIndicator({ status: '' })).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
  });
});

describe('getPBSStatusIndicator (branch coverage)', () => {
  it('flags warning from a degraded connectionHealth with a non-canonical status', () => {
    // DEGRADED arm driven by connection alone (status 'paused' is not healthy/
    // online and not in the degraded set, so without connection it would fall
    // through to defaultIndicator). Label prefers connection.
    expect(
      getPBSStatusIndicator({ status: 'paused', connectionHealth: 'maintenance' }),
    ).toEqual({ variant: 'warning', label: 'Maintenance' });
  });

  it('prefers connection over status when both are degraded for the warning label', () => {
    // `formatStatusLabel(connection || status, 'Degraded')` — connection wins.
    expect(
      getPBSStatusIndicator({ status: 'degraded', connectionHealth: 'recovering' }),
    ).toEqual({ variant: 'warning', label: 'Recovering' });
  });

  it('returns the muted default indicator for a status that is neither offline, healthy/online, nor degraded', () => {
    // Final `return defaultIndicator` arm.
    expect(getPBSStatusIndicator({ status: 'paused' })).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
  });
});

describe('getReplicationJobStatusIndicator (branch coverage)', () => {
  it('coalesces status from the state field when status is absent', () => {
    // `job.status || job.state` arm — status undefined falls through to state.
    expect(getReplicationJobStatusIndicator({ state: 'syncing' })).toEqual({
      variant: 'warning',
      label: 'Syncing',
    });
    expect(getReplicationJobStatusIndicator({ state: 'idle' })).toEqual({
      variant: 'success',
      label: 'Idle',
    });
  });

  it('flags danger from lastSyncStatus alone when status is empty, labelling from lastStatus', () => {
    // lastStatus.includes('error') arm; status is '' so `status || lastStatus`
    // yields lastStatus for the label.
    expect(getReplicationJobStatusIndicator({ lastSyncStatus: 'backup-error' })).toEqual({
      variant: 'danger',
      label: 'Backup-error',
    });
  });

  it('prioritizes the error check over the sync check for a status containing both', () => {
    // Order of checks: error is evaluated before sync.
    expect(getReplicationJobStatusIndicator({ status: 'sync-error' })).toEqual({
      variant: 'danger',
      label: 'Sync-error',
    });
  });

  it('returns the success/idle indicator for an empty status with no lastSyncStatus', () => {
    // Trailing success arm with status '' -> formatStatusLabel('', 'Idle') -> 'Idle'.
    expect(getReplicationJobStatusIndicator({})).toEqual({
      variant: 'success',
      label: 'Idle',
    });
  });
});
