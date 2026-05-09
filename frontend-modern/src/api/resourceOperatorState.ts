import { apiFetchJSON } from '@/utils/apiClient';

/**
 * Operator-set per-resource intent. Mirrors the canonical Go shape from
 * `internal/unifiedresources/resource_operator_state.go` with explicit JSON
 * field names so the TS surface stays decoupled from the storage type's
 * evolution. See the patrol-intelligence and ai-runtime subsystem contracts
 * for the suppression / refusal semantics each field drives.
 */
export interface ResourceOperatorState {
  canonicalId: string;
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
  /**
   * Maintenance window — when present and `now` falls within it, all
   * new findings on this resource get auto-acknowledged with
   * reason=expected_behavior + cause=maintenance_window. Both start
   * and end must be set together (server validates).
   */
  maintenanceStartAt?: string;
  maintenanceEndAt?: string;
  maintenanceReason?: string;
  /**
   * Optional operator hint that affects finding sort order. One of
   * `'high' | 'medium' | 'low' | ''` (empty = default).
   */
  criticality?: 'high' | 'medium' | 'low' | '';
  note?: string;
  setAt: string;
  setBy?: string;
}

/**
 * The PUT body shape — same as the read model but with attribution
 * stripped because the server populates `setAt` and `setBy` from the
 * authenticated identity, ignoring any client-supplied values.
 */
export type ResourceOperatorStateInput = Omit<
  ResourceOperatorState,
  'canonicalId' | 'setAt' | 'setBy'
>;

/**
 * Read the operator-set state for a resource. Resolves to null when
 * the server returns 404 (no entry recorded — the default no-state
 * posture). Throws on other errors.
 */
export async function getResourceOperatorState(
  resourceId: string,
): Promise<ResourceOperatorState | null> {
  try {
    return await apiFetchJSON<ResourceOperatorState>(
      `/api/resources/${encodeURIComponent(resourceId)}/operator-state`,
      { cache: 'no-store' },
    );
  } catch (err) {
    // The 404 response shape is `{ error: 'operator_state_not_set', ... }`.
    // Translating into null lets the caller treat "no state" as a clean
    // default rather than a thrown error.
    if (err && typeof err === 'object' && 'status' in err && (err as { status: number }).status === 404) {
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
  return apiFetchJSON<ResourceOperatorState>(
    `/api/resources/${encodeURIComponent(resourceId)}/operator-state`,
    {
      method: 'PUT',
      body: JSON.stringify(state),
      headers: { 'Content-Type': 'application/json' },
    },
  );
}

/**
 * Remove any operator-set state for the resource. Idempotent — resolves
 * cleanly whether or not an entry was present.
 */
export async function clearResourceOperatorState(resourceId: string): Promise<void> {
  await apiFetchJSON(
    `/api/resources/${encodeURIComponent(resourceId)}/operator-state`,
    { method: 'DELETE' },
  );
}
