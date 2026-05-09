import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { Toggle } from '@/components/shared/Toggle';
import { notificationStore } from '@/stores/notifications';
import {
  type ResourceOperatorState,
  type ResourceOperatorStateInput,
  clearResourceOperatorState,
  getResourceOperatorState,
  setResourceOperatorState,
} from '@/api/resourceOperatorState';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { formatRelativeTime } from '@/utils/format';

/**
 * ResourceOperatorStateSection surfaces the operator-set per-resource
 * intent (`/api/resources/{id}/operator-state`) on the resource detail
 * drawer so operators can:
 *   - Mark a resource as intentionally offline (suppress
 *     "X is offline" findings)
 *   - Lock the resource against automated remediation (action broker
 *     refuses dispatch with resource_remediation_locked:)
 *   - See whether a maintenance window is currently active (read-only;
 *     scheduling lives in a follow-up slice that owns the date-picker
 *     UX)
 *
 * The section stays compact and out of the way until the operator has
 * something to say about the resource — fresh-install resources see a
 * collapsed "Operator overrides" hint with no toggles in the active
 * state.
 */
interface ResourceOperatorStateSectionProps {
  resourceId: string;
}

export const ResourceOperatorStateSection: Component<ResourceOperatorStateSectionProps> = (
  props,
) => {
  // Fetch the persisted state via the non-suspending helper so the
  // drawer's parent Suspense boundary does not flicker the page-level
  // fallback while operator-state is in flight. null means "no entry"
  // (the default no-state posture).
  const query = createNonSuspendingQuery<ResourceOperatorState | null, string>({
    source: () => props.resourceId || null,
    fetcher: async (id: string) => {
      try {
        return await getResourceOperatorState(id);
      } catch (err) {
        notificationStore.error(
          err instanceof Error ? err.message : 'Failed to load operator state',
        );
        return null;
      }
    },
    initialValue: null,
    cacheKey: (id: string) => `resource-operator-state:${id}`,
  });
  const persisted = query.value;

  // Local edit state, hydrated from the persisted record. Operators can
  // toggle either flag and the section dirty-tracks until they hit Save
  // or Discard.
  const [intentionallyOffline, setIntentionallyOffline] = createSignal(false);
  const [neverAutoRemediate, setNeverAutoRemediate] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [confirmingLock, setConfirmingLock] = createSignal(false);

  // Hydrate edit state from persisted record on first load and on resource change.
  createEffect(() => {
    const current = persisted();
    if (current === undefined) return;
    setIntentionallyOffline(current?.intentionallyOffline ?? false);
    setNeverAutoRemediate(current?.neverAutoRemediate ?? false);
    setConfirmingLock(false);
  });

  const isDirty = createMemo(() => {
    const current = persisted();
    const persistedOffline = current?.intentionallyOffline ?? false;
    const persistedLocked = current?.neverAutoRemediate ?? false;
    return (
      intentionallyOffline() !== persistedOffline ||
      neverAutoRemediate() !== persistedLocked
    );
  });

  // The lock toggle is a safety override — confirm before flipping to
  // true. Flipping back to false from true is just a release and does
  // not need confirmation.
  const handleNeverAutoRemediateToggle = (next: boolean) => {
    if (next && !neverAutoRemediate()) {
      setConfirmingLock(true);
      return;
    }
    setNeverAutoRemediate(next);
  };

  const confirmLockToggle = () => {
    setNeverAutoRemediate(true);
    setConfirmingLock(false);
  };

  const cancelLockToggle = () => {
    setConfirmingLock(false);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const current = persisted();
      const input: ResourceOperatorStateInput = {
        intentionallyOffline: intentionallyOffline(),
        neverAutoRemediate: neverAutoRemediate(),
        // Preserve any maintenance-window data the API currently holds
        // — this slice owns toggles only; window scheduling is a
        // separate slice and clobbering it on save would surprise the
        // operator.
        maintenanceStartAt: current?.maintenanceStartAt,
        maintenanceEndAt: current?.maintenanceEndAt,
        maintenanceReason: current?.maintenanceReason,
        criticality: current?.criticality,
        note: current?.note,
      };
      await setResourceOperatorState(props.resourceId, input);
      // Refresh from server so the section displays the persisted
      // attribution (setAt / setBy populated server-side).
      await query.refetch();
      notificationStore.success('Operator overrides saved');
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to save operator overrides',
      );
    } finally {
      setSaving(false);
    }
  };

  const handleDiscard = () => {
    const current = persisted();
    setIntentionallyOffline(current?.intentionallyOffline ?? false);
    setNeverAutoRemediate(current?.neverAutoRemediate ?? false);
    setConfirmingLock(false);
  };

  const handleClear = async () => {
    setSaving(true);
    try {
      await clearResourceOperatorState(props.resourceId);
      // Refresh from server — DELETE leaves no entry, so refetch will
      // resolve to null and the section will return to the no-state
      // posture.
      await query.refetch();
      setIntentionallyOffline(false);
      setNeverAutoRemediate(false);
      notificationStore.success('Operator overrides cleared');
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to clear operator overrides',
      );
    } finally {
      setSaving(false);
    }
  };

  // Maintenance window is read-only this slice. Compute whether one is
  // currently active so the section can badge it.
  const activeMaintenanceWindow = createMemo(() => {
    const current = persisted();
    if (!current?.maintenanceStartAt || !current?.maintenanceEndAt) return null;
    const now = Date.now();
    const start = Date.parse(current.maintenanceStartAt);
    const end = Date.parse(current.maintenanceEndAt);
    if (Number.isNaN(start) || Number.isNaN(end)) return null;
    if (now < start || now >= end) return null;
    return current;
  });

  return (
    <section class="rounded-md border border-border bg-surface p-4 space-y-3" aria-label="Operator overrides">
      <header class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-semibold text-base-content">Operator overrides</h3>
          <p class="text-xs text-muted">
            Tell Pulse how to treat this resource — suppress expected noise, or lock it against
            automated remediation.
          </p>
        </div>
        <Show when={persisted()?.setBy || persisted()?.setAt}>
          <span class="text-[11px] text-muted">
            <Show when={persisted()?.setBy}>
              <span>Set by {persisted()!.setBy} </span>
            </Show>
            <Show when={persisted()?.setAt}>
              <span>{formatRelativeTime(persisted()!.setAt, { compact: true })}</span>
            </Show>
          </span>
        </Show>
      </header>

      <Show when={activeMaintenanceWindow()}>
        <div class="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-200">
          <span class="font-semibold">Maintenance window active.</span>{' '}
          Findings raised on this resource are auto-acknowledged until{' '}
          {formatRelativeTime(activeMaintenanceWindow()!.maintenanceEndAt!, { compact: true })}.
          <Show when={activeMaintenanceWindow()!.maintenanceReason}>
            <span class="block mt-0.5">Reason: {activeMaintenanceWindow()!.maintenanceReason}</span>
          </Show>
        </div>
      </Show>

      <div class="flex items-start justify-between gap-3">
        <div class="flex-1">
          <label class="text-sm font-medium text-base-content">Intentionally offline</label>
          <p class="text-[11px] text-muted mt-0.5 leading-tight">
            Suppress findings on this resource. Use when a workload is deprecated, a dev environment
            is shut down on purpose, or a host is archived.
          </p>
        </div>
        <Toggle
          checked={intentionallyOffline()}
          onChange={(e) => setIntentionallyOffline(e.currentTarget.checked)}
          disabled={saving()}
        />
      </div>

      <div class="flex items-start justify-between gap-3 pt-2 border-t border-border-subtle">
        <div class="flex-1">
          <label class="text-sm font-medium text-red-700 dark:text-red-400">
            Never auto-remediate
          </label>
          <p class="text-[11px] text-muted mt-0.5 leading-tight">
            Refuse all automated remediation against this resource, even with a valid approval. The
            action broker logs every refused dispatch as a Failed audit record. Use for resources
            where Pulse must not act under any circumstance.
          </p>
        </div>
        <Toggle
          checked={neverAutoRemediate()}
          onChange={(e) => handleNeverAutoRemediateToggle(e.currentTarget.checked)}
          disabled={saving()}
        />
      </div>

      <Show when={confirmingLock()}>
        <div class="rounded border border-red-300 bg-red-50 px-3 py-2.5 text-xs text-red-900 dark:border-red-800 dark:bg-red-950 dark:text-red-100">
          <p class="font-semibold">Lock this resource against all automated remediation?</p>
          <p class="mt-1 leading-relaxed">
            Pulse will refuse every dispatch targeting this resource, including approved actions
            from Patrol or the Assistant. Operators must clear the lock to allow remediation again.
          </p>
          <div class="mt-2 flex items-center gap-2">
            <button
              type="button"
              onClick={confirmLockToggle}
              class="rounded border border-red-400 bg-white px-2 py-1 text-xs font-medium text-red-900 hover:bg-red-100 dark:border-red-700 dark:bg-red-900 dark:text-red-100 dark:hover:bg-red-800"
            >
              Lock this resource
            </button>
            <button
              type="button"
              onClick={cancelLockToggle}
              class="rounded border border-border bg-surface px-2 py-1 text-xs font-medium text-muted hover:bg-surface-hover"
            >
              Cancel
            </button>
          </div>
        </div>
      </Show>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-border-subtle">
        <Show when={persisted() && !isDirty()}>
          <button
            type="button"
            onClick={handleClear}
            disabled={saving()}
            class="px-2.5 py-1 text-xs font-medium text-muted hover:text-base-content hover:bg-surface-hover rounded transition-colors disabled:opacity-50"
          >
            Clear all overrides
          </button>
        </Show>
        <Show when={isDirty()}>
          <button
            type="button"
            onClick={handleDiscard}
            disabled={saving()}
            class="px-2.5 py-1 text-xs font-medium text-muted hover:bg-surface-hover rounded transition-colors disabled:opacity-50"
          >
            Discard
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving()}
            class="px-2.5 py-1 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 rounded transition-colors"
          >
            {saving() ? 'Saving…' : 'Save overrides'}
          </button>
        </Show>
      </div>

    </section>
  );
};

export default ResourceOperatorStateSection;
