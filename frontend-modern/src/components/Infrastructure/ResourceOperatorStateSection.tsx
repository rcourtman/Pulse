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
 *   - Schedule a maintenance window during which all findings on the
 *     resource get auto-acknowledged with cause=maintenance_window
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

  // Maintenance-window scheduler state. The form is closed by default;
  // `Schedule maintenance window` opens it pre-filled with sensible
  // defaults (start = now, end = +1h). Datetime-local inputs use the
  // browser's local timezone display but the API exchanges ISO 8601
  // (UTC offset preserved) — formatLocalForInput / parseLocalFromInput
  // handle the conversion.
  const [schedulerOpen, setSchedulerOpen] = createSignal(false);
  const [scheduleStart, setScheduleStart] = createSignal('');
  const [scheduleEnd, setScheduleEnd] = createSignal('');
  const [scheduleReason, setScheduleReason] = createSignal('');

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

  // Compute whether a maintenance window covers `now` so the section
  // can badge it. Distinct from `scheduledMaintenanceWindow` — both
  // active and future windows are surfaced, with different copy.
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

  // Future-scheduled window — set but not yet started. The section
  // surfaces this differently from an active window so the operator
  // sees "scheduled" rather than "active".
  const scheduledMaintenanceWindow = createMemo(() => {
    const current = persisted();
    if (!current?.maintenanceStartAt || !current?.maintenanceEndAt) return null;
    const now = Date.now();
    const start = Date.parse(current.maintenanceStartAt);
    if (Number.isNaN(start)) return null;
    if (start <= now) return null;
    return current;
  });

  const hasAnyMaintenanceWindow = createMemo(
    () => Boolean(activeMaintenanceWindow() || scheduledMaintenanceWindow()),
  );

  // Datetime-local input format is "YYYY-MM-DDTHH:mm" in the browser's
  // local timezone. Both directions convert through Date so the API
  // round-trip stays in ISO 8601 / UTC offset.
  const formatLocalForInput = (date: Date): string => {
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
  };

  const parseLocalFromInput = (value: string): Date | null => {
    if (!value) return null;
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  };

  const handleOpenScheduler = () => {
    const now = new Date();
    const oneHourFromNow = new Date(now.getTime() + 60 * 60 * 1000);
    const current = persisted();
    // Pre-fill from the persisted window when one exists; otherwise
    // default to "starting now, ending in one hour" — the most common
    // shape for a quick maintenance.
    if (current?.maintenanceStartAt && current?.maintenanceEndAt) {
      const start = new Date(current.maintenanceStartAt);
      const end = new Date(current.maintenanceEndAt);
      if (!Number.isNaN(start.getTime())) setScheduleStart(formatLocalForInput(start));
      if (!Number.isNaN(end.getTime())) setScheduleEnd(formatLocalForInput(end));
      setScheduleReason(current.maintenanceReason ?? '');
    } else {
      setScheduleStart(formatLocalForInput(now));
      setScheduleEnd(formatLocalForInput(oneHourFromNow));
      setScheduleReason('');
    }
    setSchedulerOpen(true);
  };

  const applyPresetDuration = (hoursFromStart: number) => {
    const start = parseLocalFromInput(scheduleStart()) ?? new Date();
    const end = new Date(start.getTime() + hoursFromStart * 60 * 60 * 1000);
    setScheduleEnd(formatLocalForInput(end));
  };

  const scheduleValidationError = createMemo(() => {
    const start = parseLocalFromInput(scheduleStart());
    const end = parseLocalFromInput(scheduleEnd());
    if (!start || !end) return 'Both start and end are required.';
    if (end.getTime() <= start.getTime()) return 'End must be after start.';
    return null;
  });

  const handleScheduleSave = async () => {
    const start = parseLocalFromInput(scheduleStart());
    const end = parseLocalFromInput(scheduleEnd());
    if (!start || !end) {
      notificationStore.error('Both start and end are required.');
      return;
    }
    if (end.getTime() <= start.getTime()) {
      notificationStore.error('Maintenance end must be strictly after start.');
      return;
    }
    setSaving(true);
    try {
      const current = persisted();
      const input: ResourceOperatorStateInput = {
        // Keep the toggle state intact when scheduling — operator
        // editing one facet must not lose work on the other.
        intentionallyOffline: intentionallyOffline(),
        neverAutoRemediate: neverAutoRemediate(),
        maintenanceStartAt: start.toISOString(),
        maintenanceEndAt: end.toISOString(),
        maintenanceReason: scheduleReason().trim() || undefined,
        criticality: current?.criticality,
        note: current?.note,
      };
      await setResourceOperatorState(props.resourceId, input);
      await query.refetch();
      setSchedulerOpen(false);
      notificationStore.success('Maintenance window saved');
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to save maintenance window',
      );
    } finally {
      setSaving(false);
    }
  };

  const handleClearMaintenanceWindow = async () => {
    setSaving(true);
    try {
      const current = persisted();
      const input: ResourceOperatorStateInput = {
        intentionallyOffline: intentionallyOffline(),
        neverAutoRemediate: neverAutoRemediate(),
        maintenanceStartAt: undefined,
        maintenanceEndAt: undefined,
        maintenanceReason: undefined,
        criticality: current?.criticality,
        note: current?.note,
      };
      await setResourceOperatorState(props.resourceId, input);
      await query.refetch();
      setSchedulerOpen(false);
      notificationStore.success('Maintenance window cleared');
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to clear maintenance window',
      );
    } finally {
      setSaving(false);
    }
  };

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

      <Show when={scheduledMaintenanceWindow() && !activeMaintenanceWindow()}>
        <div class="rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
          <span class="font-semibold">Maintenance window scheduled.</span>{' '}
          Auto-acknowledgement will start{' '}
          {formatRelativeTime(scheduledMaintenanceWindow()!.maintenanceStartAt!, { compact: true })}{' '}
          and end{' '}
          {formatRelativeTime(scheduledMaintenanceWindow()!.maintenanceEndAt!, { compact: true })}.
          <Show when={scheduledMaintenanceWindow()!.maintenanceReason}>
            <span class="block mt-0.5">Reason: {scheduledMaintenanceWindow()!.maintenanceReason}</span>
          </Show>
        </div>
      </Show>

      {/* Maintenance window scheduler. The form is closed by default;
          opening it pre-fills with sensible defaults (start = now, end
          = +1h) or with the persisted window when one exists. */}
      <Show when={!schedulerOpen()}>
        <div class="flex items-center justify-between gap-3 pt-2 border-t border-border-subtle">
          <div class="flex-1">
            <label class="text-sm font-medium text-base-content">Maintenance window</label>
            <p class="text-[11px] text-muted mt-0.5 leading-tight">
              Suspend findings on this resource for a defined window. Useful for scheduled
              upgrades, planned downtime, or reboots where Pulse should stay quiet until the
              window closes.
            </p>
          </div>
          <Show
            when={hasAnyMaintenanceWindow()}
            fallback={
              <button
                type="button"
                onClick={handleOpenScheduler}
                disabled={saving()}
                class="px-2.5 py-1 text-xs font-medium text-base-content border border-border rounded hover:bg-surface-hover disabled:opacity-50"
              >
                Schedule window
              </button>
            }
          >
            <div class="flex items-center gap-2">
              <button
                type="button"
                onClick={handleOpenScheduler}
                disabled={saving()}
                class="px-2.5 py-1 text-xs font-medium text-base-content border border-border rounded hover:bg-surface-hover disabled:opacity-50"
              >
                Edit window
              </button>
              <button
                type="button"
                onClick={handleClearMaintenanceWindow}
                disabled={saving()}
                class="px-2.5 py-1 text-xs font-medium text-amber-700 border border-amber-200 rounded hover:bg-amber-50 dark:text-amber-300 dark:border-amber-800 dark:hover:bg-amber-900 disabled:opacity-50"
              >
                Cancel window
              </button>
            </div>
          </Show>
        </div>
      </Show>

      <Show when={schedulerOpen()}>
        <div class="rounded border border-border bg-surface-alt/40 px-3 py-3 space-y-2">
          <div class="text-xs font-semibold text-base-content">Schedule maintenance window</div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <label class="block">
              <span class="text-[11px] text-muted">Start</span>
              <input
                type="datetime-local"
                value={scheduleStart()}
                onInput={(e) => setScheduleStart(e.currentTarget.value)}
                class="mt-0.5 w-full text-xs rounded border border-border bg-surface px-2 py-1 text-base-content focus:outline-none focus:ring-1 focus:ring-blue-400"
                disabled={saving()}
              />
            </label>
            <label class="block">
              <span class="text-[11px] text-muted">End</span>
              <input
                type="datetime-local"
                value={scheduleEnd()}
                onInput={(e) => setScheduleEnd(e.currentTarget.value)}
                class="mt-0.5 w-full text-xs rounded border border-border bg-surface px-2 py-1 text-base-content focus:outline-none focus:ring-1 focus:ring-blue-400"
                disabled={saving()}
              />
            </label>
          </div>

          <div class="flex items-center gap-1 text-[11px]">
            <span class="text-muted">Quick presets:</span>
            <button
              type="button"
              onClick={() => applyPresetDuration(1)}
              disabled={saving()}
              class="px-1.5 py-0.5 rounded border border-border hover:bg-surface-hover disabled:opacity-50"
            >
              1h
            </button>
            <button
              type="button"
              onClick={() => applyPresetDuration(4)}
              disabled={saving()}
              class="px-1.5 py-0.5 rounded border border-border hover:bg-surface-hover disabled:opacity-50"
            >
              4h
            </button>
            <button
              type="button"
              onClick={() => applyPresetDuration(24)}
              disabled={saving()}
              class="px-1.5 py-0.5 rounded border border-border hover:bg-surface-hover disabled:opacity-50"
            >
              24h
            </button>
          </div>

          <label class="block">
            <span class="text-[11px] text-muted">Reason (optional)</span>
            <input
              type="text"
              value={scheduleReason()}
              onInput={(e) => setScheduleReason(e.currentTarget.value)}
              placeholder="e.g. Q3 storage upgrade, kernel patch reboot"
              class="mt-0.5 w-full text-xs rounded border border-border bg-surface px-2 py-1 text-base-content focus:outline-none focus:ring-1 focus:ring-blue-400"
              disabled={saving()}
              maxLength={200}
            />
          </label>

          <Show when={scheduleValidationError()}>
            <p class="text-[11px] text-red-700 dark:text-red-400">{scheduleValidationError()}</p>
          </Show>

          <div class="flex items-center justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => setSchedulerOpen(false)}
              disabled={saving()}
              class="px-2.5 py-1 text-xs font-medium text-muted hover:bg-surface-hover rounded transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleScheduleSave}
              disabled={saving() || Boolean(scheduleValidationError())}
              class="px-2.5 py-1 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 rounded transition-colors"
            >
              {saving() ? 'Saving…' : 'Save window'}
            </button>
          </div>
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
