import { apiFetchJSON } from '@/utils/apiClient';

/**
 * Operator-set per-resource intent. Mirrors the canonical Go shape from
 * `internal/unifiedresources/resource_operator_state.go` with explicit JSON
 * field names so the TS surface stays decoupled from the storage type's
 * evolution. See the patrol-intelligence and ai-runtime subsystem contracts
 * for the suppression / refusal semantics each field drives.
 */
export type ResourceCriticality = 'high' | 'medium' | 'low' | '';
export type ResourceMonitoringMode = 'normal' | 'expected_offline' | 'muted';
export type ResourceLifecycleState = 'active' | 'retired';
export type MaintenanceScope = 'resource' | 'resource_and_descendants';

export interface RecurringMaintenanceWindow {
  timezone: string;
  weekdays: string[];
  startMinute: number;
  endMinute: number;
}

export interface AutoRemediationWindow {
  timezone: string;
  startMinute: number;
  endMinute: number;
}

export interface AutoRemediationPolicy {
  enabled: boolean;
  /**
   * Nullable on the wire: rows persisted before the store normalized the
   * policy on read carry `"capabilityNames": null` (issue #1621). Treat
   * null the same as [].
   */
  capabilityNames: string[] | null;
  window?: AutoRemediationWindow;
}

export interface ResourceOperatorState {
  canonicalId: string;
  monitoringMode?: ResourceMonitoringMode;
  lifecycleState?: ResourceLifecycleState;
  /**
   * When true, new findings raised against this resource get
   * auto-acknowledged with reason=expected_behavior — Patrol stops
   * notifying about a resource the operator has marked
   * "intentionally offline" (e.g. a deprecated workload, dev environment
   * shut down on purpose, archived host).
   */
  intentionallyOffline: boolean;
  /**
   * When true, the action broker refuses to dispatch automated
   * remediation against this resource even with a valid approval and
   * matching plan hash. The refusal is persisted as a Failed audit
   * record with `resource_remediation_locked:` prefix on the error.
   */
  neverAutoRemediate: boolean;
  /** Optional narrowing policy; an empty policy inherits the tenant Patrol mode. */
  autoRemediationPolicy?: AutoRemediationPolicy;
  /**
   * Maintenance window — when present and `now` falls within it, all
   * new findings on this resource get auto-acknowledged with
   * reason=expected_behavior + cause=maintenance_window. Both start
   * and end must be set together (server validates).
   */
  maintenanceStartAt?: string;
  maintenanceEndAt?: string;
  maintenanceRecurrence?: RecurringMaintenanceWindow;
  maintenanceScope?: MaintenanceScope;
  maintenanceReason?: string;
  maintenanceWindowActive?: boolean;
  maintenanceActiveStartAt?: string;
  maintenanceActiveEndAt?: string;
  /**
   * Optional operator hint that affects finding sort order. One of
   * `'high' | 'medium' | 'low' | ''` (empty = default).
   */
  criticality?: ResourceCriticality;
  note?: string;
  setAt: string;
  setBy?: string;
}

interface ResourceOperatorStateLookup {
  configured: boolean;
  state?: ResourceOperatorState;
}

/**
 * The PUT body shape — same as the read model but with attribution
 * stripped because the server populates `setAt` and `setBy` from the
 * authenticated identity, ignoring any client-supplied values.
 */
export type ResourceOperatorStateInput = Omit<
  ResourceOperatorState,
  | 'canonicalId'
  | 'setAt'
  | 'setBy'
  | 'maintenanceWindowActive'
  | 'maintenanceActiveStartAt'
  | 'maintenanceActiveEndAt'
>;

const normalizeResourceOperatorState = (state: ResourceOperatorState): ResourceOperatorState => ({
  ...state,
  monitoringMode:
    state.monitoringMode || (state.intentionallyOffline ? 'expected_offline' : 'normal'),
  lifecycleState: state.lifecycleState || 'active',
  maintenanceScope: state.maintenanceScope || 'resource',
});

/**
 * Read the operator-set state for a resource. The lookup view represents an
 * unset record as a successful envelope so opening a newly discovered
 * resource does not generate a routine 404 in the browser. The 404 fallback
 * keeps the frontend compatible with an older server during a rolling update.
 */
export async function getResourceOperatorState(
  resourceId: string,
): Promise<ResourceOperatorState | null> {
  try {
    const result = await apiFetchJSON<ResourceOperatorStateLookup | ResourceOperatorState>(
      `/api/resources/${encodeURIComponent(resourceId)}/operator-state?view=lookup`,
      { cache: 'no-store' },
    );
    if ('configured' in result) {
      if (!result.configured || !result.state) return null;
      return normalizeResourceOperatorState(result.state);
    }
    // Rolling-update compatibility: a previous server returns the persisted
    // state directly when it exists.
    return normalizeResourceOperatorState(result);
  } catch (err) {
    // Previous servers express no saved state as 404
    // `{ error: 'operator_state_not_set', ... }`.
    if (
      err &&
      typeof err === 'object' &&
      'status' in err &&
      (err as { status: number }).status === 404
    ) {
      return null;
    }
    throw err;
  }
}

/**
 * Replace the operator-set state for a resource. The server populates
 * `setAt` and `setBy` server-side, so the input shape excludes them.
 * Returns the persisted record (read-after-write) so the caller can
 * see exactly what landed, including the attribution fields.
 */
export async function setResourceOperatorState(
  resourceId: string,
  state: ResourceOperatorStateInput,
): Promise<ResourceOperatorState> {
  const persisted = await apiFetchJSON<ResourceOperatorState>(
    `/api/resources/${encodeURIComponent(resourceId)}/operator-state`,
    {
      method: 'PUT',
      body: JSON.stringify(state),
      headers: { 'Content-Type': 'application/json' },
    },
  );
  return normalizeResourceOperatorState(persisted);
}

/**
 * Remove any operator-set state for the resource. Idempotent — resolves
 * cleanly whether or not an entry was present.
 */
export async function clearResourceOperatorState(resourceId: string): Promise<void> {
  await apiFetchJSON(`/api/resources/${encodeURIComponent(resourceId)}/operator-state`, {
    method: 'DELETE',
  });
}
