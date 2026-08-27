import { Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { Toggle } from '@/components/shared/Toggle';
import { FormSelect } from '@/components/shared/FormSelect';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { notificationStore } from '@/stores/notifications';
import {
  type ResourceCriticality,
  type ResourceLifecycleState,
  type ResourceMonitoringMode,
  type ResourceOperatorState,
  type ResourceOperatorStateInput,
  clearResourceOperatorState,
  getResourceOperatorState,
  setResourceOperatorState,
} from '@/api/resourceOperatorState';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { formatRelativeTime } from '@/utils/format';
import type { ResourceCapability } from '@/types/resource';
import { describeResourceInventoryOwnership } from '@/utils/resourceMonitoringPolicy';

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
  resourceType?: string;
  platformType?: string;
  capabilities?: ResourceCapability[];
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
  const [monitoringMode, setMonitoringMode] = createSignal<ResourceMonitoringMode>('normal');
  const [lifecycleState, setLifecycleState] = createSignal<ResourceLifecycleState>('active');
  const [neverAutoRemediate, setNeverAutoRemediate] = createSignal(false);
  const [criticality, setCriticality] = createSignal<ResourceCriticality>('');
  const [note, setNote] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [confirmingLock, setConfirmingLock] = createSignal(false);
  const [autoRemediationEnabled, setAutoRemediationEnabled] = createSignal(false);
  const [autoCapabilities, setAutoCapabilities] = createSignal<string[]>([]);
  const [autoWindowEnabled, setAutoWindowEnabled] = createSignal(false);
  const [autoWindowStart, setAutoWindowStart] = createSignal('00:00');
  const [autoWindowEnd, setAutoWindowEnd] = createSignal('23:59');
  const [autoWindowTimezone, setAutoWindowTimezone] = createSignal('UTC');

  const eligibleAutoCapabilities = createMemo(() =>
    (props.capabilities ?? []).filter(
      (capability) => capability.autoAuthorization && capability.autoAuthorization !== 'never',
    ),
  );
  const inventoryOwnership = createMemo(() =>
    describeResourceInventoryOwnership(props.resourceType, props.platformType),
  );

  const minuteToTime = (minute: number): string => {
    const normalized = Math.max(0, Math.min(1439, minute));
    return `${String(Math.floor(normalized / 60)).padStart(2, '0')}:${String(normalized % 60).padStart(2, '0')}`;
  };

  const timeToMinute = (value: string): number => {
    const [hours, minutes] = value.split(':').map(Number);
    return hours * 60 + minutes;
  };

  const capabilityDisplayName = (name: string): string =>
    name
      .split('_')
      .filter(Boolean)
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(' ');

  // Maintenance-window scheduler state. The form is closed by default;
  // `Schedule maintenance window` opens it pre-filled with sensible
  // defaults (start = now, end = +1h). Datetime-local inputs use the
  // browser's local timezone display but the API exchanges ISO 8601
  // (UTC offset preserved) — formatLocalForInput / parseLocalFromInput
  // handle the conversion.
  const [schedulerOpen, setSchedulerOpen] = createSignal(false);
  const [scheduleKind, setScheduleKind] = createSignal<'once' | 'recurring'>('once');
  const [scheduleStart, setScheduleStart] = createSignal('');
  const [scheduleEnd, setScheduleEnd] = createSignal('');
  const [scheduleWeekdays, setScheduleWeekdays] = createSignal<string[]>([
    'monday',
    'tuesday',
    'wednesday',
    'thursday',
    'friday',
    'saturday',
    'sunday',
  ]);
  const [scheduleRecurringStart, setScheduleRecurringStart] = createSignal('02:00');
  const [scheduleRecurringEnd, setScheduleRecurringEnd] = createSignal('03:00');
  const [scheduleTimezone, setScheduleTimezone] = createSignal(
    Intl.DateTimeFormat().resolvedOptions().timeZone ?? 'UTC',
  );
  const [scheduleScope, setScheduleScope] = createSignal<'resource' | 'resource_and_descendants'>(
    'resource',
  );
  const [scheduleReason, setScheduleReason] = createSignal('');

  const maintenanceWeekdays = [
    ['monday', 'Mon'],
    ['tuesday', 'Tue'],
    ['wednesday', 'Wed'],
    ['thursday', 'Thu'],
    ['friday', 'Fri'],
    ['saturday', 'Sat'],
    ['sunday', 'Sun'],
  ] as const;

  // Hydrate edit state from persisted record on first load and on resource change.
  createEffect(() => {
    const current = persisted();
    if (current === undefined) return;
    setMonitoringMode(
      current?.monitoringMode ?? (current?.intentionallyOffline ? 'expected_offline' : 'normal'),
    );
    setLifecycleState(current?.lifecycleState ?? 'active');
    setNeverAutoRemediate(current?.neverAutoRemediate ?? false);
    setCriticality(current?.criticality ?? '');
    setNote(current?.note ?? '');
    const autoPolicy = current?.autoRemediationPolicy;
    setAutoRemediationEnabled(autoPolicy?.enabled ?? false);
    setAutoCapabilities(autoPolicy?.capabilityNames ?? []);
    setAutoWindowEnabled(Boolean(autoPolicy?.window));
    setAutoWindowStart(minuteToTime(autoPolicy?.window?.startMinute ?? 0));
    setAutoWindowEnd(minuteToTime(autoPolicy?.window?.endMinute ?? 1439));
    setAutoWindowTimezone(
      autoPolicy?.window?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone ?? 'UTC',
    );
    setConfirmingLock(false);
  });

  const isDirty = createMemo(() => {
    const current = persisted();
    const persistedMonitoringMode =
      current?.monitoringMode ?? (current?.intentionallyOffline ? 'expected_offline' : 'normal');
    const persistedLifecycleState = current?.lifecycleState ?? 'active';
    const persistedLocked = current?.neverAutoRemediate ?? false;
    const persistedCriticality = current?.criticality ?? '';
    const persistedNote = current?.note ?? '';
    const persistedAuto = current?.autoRemediationPolicy ?? {
      enabled: false,
      capabilityNames: [],
    };
    const selected = [...autoCapabilities()].sort();
    // capabilityNames can arrive as null from rows persisted before the
    // policy column was always written (issue #1621) — never assume [].
    const persistedSelected = [...(persistedAuto.capabilityNames ?? [])].sort();
    const windowChanged = autoWindowEnabled()
      ? !persistedAuto.window ||
        persistedAuto.window.timezone !== autoWindowTimezone() ||
        persistedAuto.window.startMinute !== timeToMinute(autoWindowStart()) ||
        persistedAuto.window.endMinute !== timeToMinute(autoWindowEnd())
      : Boolean(persistedAuto.window);
    return (
      monitoringMode() !== persistedMonitoringMode ||
      lifecycleState() !== persistedLifecycleState ||
      neverAutoRemediate() !== persistedLocked ||
      criticality() !== persistedCriticality ||
      note().trim() !== persistedNote ||
      autoRemediationEnabled() !== persistedAuto.enabled ||
      JSON.stringify(selected) !== JSON.stringify(persistedSelected) ||
      windowChanged
    );
  });

  const noteForSave = () => note().trim() || undefined;

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
        monitoringMode: monitoringMode(),
        lifecycleState: lifecycleState(),
        intentionallyOffline: monitoringMode() === 'expected_offline',
        neverAutoRemediate: neverAutoRemediate(),
        autoRemediationPolicy: {
          enabled: autoRemediationEnabled(),
          capabilityNames: autoCapabilities(),
          ...(autoWindowEnabled()
            ? {
                window: {
                  timezone: autoWindowTimezone(),
                  startMinute: timeToMinute(autoWindowStart()),
                  endMinute: timeToMinute(autoWindowEnd()),
                },
              }
            : {}),
        },
        // Preserve any maintenance-window data the API currently holds
        // — window scheduling is a separate action and clobbering it
        // on save would surprise the operator.
        maintenanceStartAt: current?.maintenanceStartAt,
        maintenanceEndAt: current?.maintenanceEndAt,
        maintenanceRecurrence: current?.maintenanceRecurrence,
        maintenanceScope: current?.maintenanceScope ?? 'resource',
        maintenanceReason: current?.maintenanceReason,
        criticality: criticality(),
        note: noteForSave(),
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
    setMonitoringMode(
      current?.monitoringMode ?? (current?.intentionallyOffline ? 'expected_offline' : 'normal'),
    );
    setLifecycleState(current?.lifecycleState ?? 'active');
    setNeverAutoRemediate(current?.neverAutoRemediate ?? false);
    setCriticality(current?.criticality ?? '');
    setNote(current?.note ?? '');
    const autoPolicy = current?.autoRemediationPolicy;
    setAutoRemediationEnabled(autoPolicy?.enabled ?? false);
    setAutoCapabilities(autoPolicy?.capabilityNames ?? []);
    setAutoWindowEnabled(Boolean(autoPolicy?.window));
    setAutoWindowStart(minuteToTime(autoPolicy?.window?.startMinute ?? 0));
    setAutoWindowEnd(minuteToTime(autoPolicy?.window?.endMinute ?? 1439));
    setAutoWindowTimezone(
      autoPolicy?.window?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone ?? 'UTC',
    );
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
      setMonitoringMode('normal');
      setLifecycleState('active');
      setNeverAutoRemediate(false);
      setCriticality('');
      setNote('');
      setAutoRemediationEnabled(false);
      setAutoCapabilities([]);
      setAutoWindowEnabled(false);
      notificationStore.success('Operator overrides cleared');
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to clear operator overrides',
      );
    } finally {
      setSaving(false);
    }
  };

  const toggleAutoCapability = (name: string, checked: boolean) => {
    setAutoCapabilities((current) =>
      checked ? [...new Set([...current, name])] : current.filter((value) => value !== name),
    );
  };

  const autoPolicyValidationError = createMemo(() => {
    if (!autoRemediationEnabled()) return '';
    if (autoCapabilities().length === 0) return 'Select at least one eligible capability.';
    if (autoWindowEnabled() && autoWindowStart() === autoWindowEnd()) {
      return 'The automatic-action window must have different start and end times.';
    }
    return '';
  });

  // Compute whether a maintenance window covers `now` so the section
  // can badge it. Distinct from `scheduledMaintenanceWindow` — both
  // active and future windows are surfaced, with different copy.
  const activeMaintenanceWindow = createMemo(() => {
    const current = persisted();
    if (current?.maintenanceWindowActive && current.maintenanceActiveEndAt) return current;
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

  const hasAnyMaintenanceWindow = createMemo(() =>
    Boolean(
      persisted()?.maintenanceRecurrence ||
      (persisted()?.maintenanceStartAt && persisted()?.maintenanceEndAt),
    ),
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
    setScheduleScope(current?.maintenanceScope ?? 'resource');
    if (current?.maintenanceRecurrence) {
      setScheduleKind('recurring');
      setScheduleWeekdays(current.maintenanceRecurrence.weekdays);
      setScheduleRecurringStart(minuteToTime(current.maintenanceRecurrence.startMinute));
      setScheduleRecurringEnd(minuteToTime(current.maintenanceRecurrence.endMinute));
      setScheduleTimezone(current.maintenanceRecurrence.timezone);
      setScheduleReason(current.maintenanceReason ?? '');
    } else if (current?.maintenanceStartAt && current?.maintenanceEndAt) {
      setScheduleKind('once');
      const start = new Date(current.maintenanceStartAt);
      const end = new Date(current.maintenanceEndAt);
      if (!Number.isNaN(start.getTime())) setScheduleStart(formatLocalForInput(start));
      if (!Number.isNaN(end.getTime())) setScheduleEnd(formatLocalForInput(end));
      setScheduleReason(current.maintenanceReason ?? '');
    } else {
      setScheduleKind('once');
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

  const toggleMaintenanceWeekday = (weekday: string) => {
    setScheduleWeekdays((current) =>
      current.includes(weekday)
        ? current.filter((candidate) => candidate !== weekday)
        : [...current, weekday],
    );
  };

  const scheduleValidationError = createMemo(() => {
    if (scheduleKind() === 'recurring') {
      if (scheduleWeekdays().length === 0) return 'Select at least one day.';
      if (!scheduleTimezone().trim()) return 'Timezone is required.';
      if (scheduleRecurringStart() === scheduleRecurringEnd()) {
        return 'Recurring start and end times must differ.';
      }
      return null;
    }
    const start = parseLocalFromInput(scheduleStart());
    const end = parseLocalFromInput(scheduleEnd());
    if (!start || !end) return 'Both start and end are required.';
    if (end.getTime() <= start.getTime()) return 'End must be after start.';
    return null;
  });

  const handleScheduleSave = async () => {
    const start = parseLocalFromInput(scheduleStart());
    const end = parseLocalFromInput(scheduleEnd());
    if (scheduleKind() === 'once' && (!start || !end)) {
      notificationStore.error('Both start and end are required.');
      return;
    }
    if (scheduleKind() === 'once' && end!.getTime() <= start!.getTime()) {
      notificationStore.error('Maintenance end must be strictly after start.');
      return;
    }
    setSaving(true);
    try {
      const input: ResourceOperatorStateInput = {
        // Keep the toggle state intact when scheduling — operator
        // editing one facet must not lose work on the other.
        monitoringMode: monitoringMode(),
        lifecycleState: lifecycleState(),
        intentionallyOffline: monitoringMode() === 'expected_offline',
        neverAutoRemediate: neverAutoRemediate(),
        autoRemediationPolicy: {
          enabled: autoRemediationEnabled(),
          capabilityNames: autoCapabilities(),
          ...(autoWindowEnabled()
            ? {
                window: {
                  timezone: autoWindowTimezone(),
                  startMinute: timeToMinute(autoWindowStart()),
                  endMinute: timeToMinute(autoWindowEnd()),
                },
              }
            : {}),
        },
        maintenanceStartAt: scheduleKind() === 'once' ? start!.toISOString() : undefined,
        maintenanceEndAt: scheduleKind() === 'once' ? end!.toISOString() : undefined,
        maintenanceRecurrence:
          scheduleKind() === 'recurring'
            ? {
                timezone: scheduleTimezone().trim(),
                weekdays: scheduleWeekdays(),
                startMinute: timeToMinute(scheduleRecurringStart()),
                endMinute: timeToMinute(scheduleRecurringEnd()),
              }
            : undefined,
        maintenanceScope: scheduleScope(),
        maintenanceReason: scheduleReason().trim() || undefined,
        criticality: criticality(),
        note: noteForSave(),
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
      const input: ResourceOperatorStateInput = {
        monitoringMode: monitoringMode(),
        lifecycleState: lifecycleState(),
        intentionallyOffline: monitoringMode() === 'expected_offline',
        neverAutoRemediate: neverAutoRemediate(),
        autoRemediationPolicy: {
          enabled: autoRemediationEnabled(),
          capabilityNames: autoCapabilities(),
          ...(autoWindowEnabled()
            ? {
                window: {
                  timezone: autoWindowTimezone(),
                  startMinute: timeToMinute(autoWindowStart()),
                  endMinute: timeToMinute(autoWindowEnd()),
                },
              }
            : {}),
        },
        maintenanceStartAt: undefined,
        maintenanceEndAt: undefined,
        maintenanceRecurrence: undefined,
        maintenanceScope: 'resource',
        maintenanceReason: undefined,
        criticality: criticality(),
        note: noteForSave(),
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
    <section
      class="rounded-md border border-border bg-surface p-4 space-y-3"
      aria-label="Operator overrides"
    >
      <header class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-semibold text-base-content">Operator overrides</h3>
          <p class="text-xs text-muted">
            Control Alerts, Patrol, lifecycle, priority, and automated remediation from one
            persisted resource policy.
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
          <span class="font-semibold">Maintenance window active.</span> Findings raised on this
          resource
          <Show when={activeMaintenanceWindow()!.maintenanceScope === 'resource_and_descendants'}>
            {' '}
            and its descendants
          </Show>{' '}
          are auto-acknowledged until{' '}
          {formatRelativeTime(
            activeMaintenanceWindow()!.maintenanceActiveEndAt ??
              activeMaintenanceWindow()!.maintenanceEndAt!,
            { compact: true },
          )}
          .
          <Show when={activeMaintenanceWindow()!.maintenanceReason}>
            <span class="block mt-0.5">Reason: {activeMaintenanceWindow()!.maintenanceReason}</span>
          </Show>
        </div>
      </Show>

      <Show when={scheduledMaintenanceWindow() && !activeMaintenanceWindow()}>
        <div class="rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
          <span class="font-semibold">Maintenance window scheduled.</span> Auto-acknowledgement will
          start{' '}
          {formatRelativeTime(scheduledMaintenanceWindow()!.maintenanceStartAt!, { compact: true })}{' '}
          and end{' '}
          {formatRelativeTime(scheduledMaintenanceWindow()!.maintenanceEndAt!, { compact: true })}.
          <Show when={scheduledMaintenanceWindow()!.maintenanceReason}>
            <span class="block mt-0.5">
              Reason: {scheduledMaintenanceWindow()!.maintenanceReason}
            </span>
          </Show>
        </div>
      </Show>

      <Show when={persisted()?.maintenanceRecurrence && !activeMaintenanceWindow()}>
        <div class="rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
          <span class="font-semibold">Recurring maintenance configured.</span>{' '}
          {persisted()!
            .maintenanceRecurrence!.weekdays.map((weekday) => weekday.slice(0, 3))
            .join(', ')}{' '}
          from {minuteToTime(persisted()!.maintenanceRecurrence!.startMinute)} to{' '}
          {minuteToTime(persisted()!.maintenanceRecurrence!.endMinute)}{' '}
          {persisted()!.maintenanceRecurrence!.timezone}.
          <Show when={persisted()?.maintenanceScope === 'resource_and_descendants'}>
            <span class="block mt-0.5 font-medium">
              This resource and all canonical descendants are covered.
            </span>
          </Show>
          <Show when={persisted()!.maintenanceReason}>
            <span class="block mt-0.5">Reason: {persisted()!.maintenanceReason}</span>
          </Show>
        </div>
      </Show>

      <div class="grid grid-cols-1 gap-3 pt-2 border-t border-border-subtle sm:grid-cols-[minmax(0,12rem)_minmax(0,1fr)]">
        <FormSelect
          label="Patrol priority"
          density="compact"
          value={criticality()}
          onChange={(e) => setCriticality(e.currentTarget.value as ResourceCriticality)}
          disabled={saving()}
          help="Orders this resource among same-severity Patrol findings."
          helpClass="text-[11px] leading-tight"
        >
          <option value="">Default</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </FormSelect>

        <FormTextarea
          label="Operator note"
          density="compact"
          value={note()}
          onInput={(e) => setNote(e.currentTarget.value)}
          placeholder="e.g. Production database. Page before rebooting"
          disabled={saving()}
          maxLength={500}
        />
      </div>

      {/* Maintenance window scheduler. The form is closed by default;
          opening it pre-fills with sensible defaults (start = now, end
          = +1h) or with the persisted window when one exists. */}
      <Show when={!schedulerOpen()}>
        <div class="flex flex-col items-stretch justify-between gap-3 border-t border-border-subtle pt-2 sm:flex-row sm:items-center">
          <div class="min-w-0 flex-1">
            <label class="text-sm font-medium text-base-content">Maintenance window</label>
            <p class="text-[11px] text-muted mt-0.5 leading-tight">
              Suspend findings on this resource for a defined window. Useful for scheduled upgrades,
              planned downtime, or reboots where Pulse should stay quiet until the window closes.
            </p>
          </div>
          <Show
            when={hasAnyMaintenanceWindow()}
            fallback={
              <button
                type="button"
                onClick={handleOpenScheduler}
                disabled={saving()}
                class="min-h-11 self-start rounded border border-border px-2.5 py-1 text-xs font-medium text-base-content hover:bg-surface-hover disabled:opacity-50 sm:min-h-0"
              >
                Schedule window
              </button>
            }
          >
            <div class="flex items-center gap-2 self-start">
              <button
                type="button"
                onClick={handleOpenScheduler}
                disabled={saving()}
                class="min-h-11 px-2.5 py-1 text-xs font-medium text-base-content border border-border rounded hover:bg-surface-hover disabled:opacity-50 sm:min-h-0"
              >
                Edit window
              </button>
              <button
                type="button"
                onClick={handleClearMaintenanceWindow}
                disabled={saving()}
                class="min-h-11 px-2.5 py-1 text-xs font-medium text-amber-700 border border-amber-200 rounded hover:bg-amber-50 dark:text-amber-300 dark:border-amber-800 dark:hover:bg-amber-900 disabled:opacity-50 sm:min-h-0"
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
          <div class="inline-flex gap-0.5 rounded border border-border bg-surface p-0.5 text-xs">
            <button
              type="button"
              onClick={() => setScheduleKind('once')}
              class={`rounded px-2.5 py-1 ${scheduleKind() === 'once' ? 'bg-blue-600 text-white' : 'text-muted hover:bg-surface-hover'}`}
            >
              One time
            </button>
            <button
              type="button"
              onClick={() => setScheduleKind('recurring')}
              class={`rounded px-2.5 py-1 ${scheduleKind() === 'recurring' ? 'bg-blue-600 text-white' : 'text-muted hover:bg-surface-hover'}`}
            >
              Recurring
            </button>
          </div>

          <Show when={scheduleKind() === 'once'}>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <label class="block">
                <span class="text-[11px] text-muted">Start</span>
                <input
                  type="datetime-local"
                  value={scheduleStart()}
                  onInput={(e) => setScheduleStart(e.currentTarget.value)}
                  class="mt-0.5 min-h-11 w-full text-xs rounded border border-border bg-surface px-2 py-1 text-base-content focus:outline-none focus:ring-1 focus:ring-blue-400 sm:min-h-0"
                  disabled={saving()}
                />
              </label>
              <label class="block">
                <span class="text-[11px] text-muted">End</span>
                <input
                  type="datetime-local"
                  value={scheduleEnd()}
                  onInput={(e) => setScheduleEnd(e.currentTarget.value)}
                  class="mt-0.5 min-h-11 w-full text-xs rounded border border-border bg-surface px-2 py-1 text-base-content focus:outline-none focus:ring-1 focus:ring-blue-400 sm:min-h-0"
                  disabled={saving()}
                />
              </label>
            </div>
            <div class="flex items-center gap-1 text-[11px]">
              <span class="text-muted">Quick presets:</span>
              <For each={[1, 4, 24]}>
                {(hours) => (
                  <button
                    type="button"
                    onClick={() => applyPresetDuration(hours)}
                    disabled={saving()}
                    class="min-h-11 min-w-11 px-1.5 py-0.5 rounded border border-border hover:bg-surface-hover disabled:opacity-50 sm:min-h-0 sm:min-w-0"
                  >
                    {hours}h
                  </button>
                )}
              </For>
            </div>
          </Show>

          <Show when={scheduleKind() === 'recurring'}>
            <div class="space-y-2">
              <fieldset>
                <legend class="text-[11px] text-muted">Days the window starts</legend>
                <div class="mt-1 flex flex-wrap gap-1">
                  <For each={maintenanceWeekdays}>
                    {([value, label]) => (
                      <button
                        type="button"
                        aria-pressed={scheduleWeekdays().includes(value)}
                        onClick={() => toggleMaintenanceWeekday(value)}
                        class={`min-h-9 rounded border px-2 text-xs ${scheduleWeekdays().includes(value) ? 'border-blue-600 bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-200' : 'border-border text-muted hover:bg-surface-hover'}`}
                      >
                        {label}
                      </button>
                    )}
                  </For>
                </div>
              </fieldset>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
                <label class="block">
                  <span class="text-[11px] text-muted">Start</span>
                  <input
                    type="time"
                    value={scheduleRecurringStart()}
                    onInput={(event) => setScheduleRecurringStart(event.currentTarget.value)}
                    class="mt-0.5 min-h-11 w-full rounded border border-border bg-surface px-2 text-xs text-base-content sm:min-h-0"
                  />
                </label>
                <label class="block">
                  <span class="text-[11px] text-muted">End</span>
                  <input
                    type="time"
                    value={scheduleRecurringEnd()}
                    onInput={(event) => setScheduleRecurringEnd(event.currentTarget.value)}
                    class="mt-0.5 min-h-11 w-full rounded border border-border bg-surface px-2 text-xs text-base-content sm:min-h-0"
                  />
                </label>
                <label class="block">
                  <span class="text-[11px] text-muted">Timezone</span>
                  <input
                    type="text"
                    value={scheduleTimezone()}
                    onInput={(event) => setScheduleTimezone(event.currentTarget.value)}
                    class="mt-0.5 min-h-11 w-full rounded border border-border bg-surface px-2 text-xs text-base-content sm:min-h-0"
                  />
                </label>
              </div>
              <p class="text-[11px] text-muted">
                End times earlier than start times continue into the following day.
              </p>
            </div>
          </Show>

          <FormSelect
            label="Applies to"
            density="compact"
            value={scheduleScope()}
            onChange={(event) =>
              setScheduleScope(event.currentTarget.value as 'resource' | 'resource_and_descendants')
            }
            help="Descendant scope follows Pulse's canonical inventory hierarchy and covers resources added beneath this one later."
            helpClass="text-[11px] leading-tight"
          >
            <option value="resource">This resource only</option>
            <option value="resource_and_descendants">This resource and descendants</option>
          </FormSelect>

          <label class="block">
            <span class="text-[11px] text-muted">Reason (optional)</span>
            <input
              type="text"
              value={scheduleReason()}
              onInput={(e) => setScheduleReason(e.currentTarget.value)}
              placeholder="e.g. Q3 storage upgrade, kernel patch reboot"
              class="mt-0.5 min-h-11 w-full text-xs rounded border border-border bg-surface px-2 py-1 text-base-content focus:outline-none focus:ring-1 focus:ring-blue-400 sm:min-h-0"
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
              class="min-h-11 px-2.5 py-1 text-xs font-medium text-muted hover:bg-surface-hover rounded transition-colors disabled:opacity-50 sm:min-h-0"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleScheduleSave}
              disabled={saving() || Boolean(scheduleValidationError())}
              class="min-h-11 px-2.5 py-1 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 rounded transition-colors sm:min-h-0"
            >
              {saving() ? 'Saving…' : 'Save window'}
            </button>
          </div>
        </div>
      </Show>

      <Show when={eligibleAutoCapabilities().length > 0}>
        <div class="space-y-3 border-t border-border-subtle pt-3" aria-label="Automatic actions">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <label class="text-sm font-medium text-base-content">Automatic action limits</label>
              <p class="mt-0.5 text-[11px] leading-tight text-muted">
                This resource follows your Patrol mode by default. Turn this on only to limit
                automatic work to selected actions or daily hours. Live safety checks and
                verification always apply.
              </p>
            </div>
            <Toggle
              checked={autoRemediationEnabled()}
              onChange={(event) => setAutoRemediationEnabled(event.currentTarget.checked)}
              disabled={saving() || neverAutoRemediate()}
            />
          </div>

          <Show when={autoRemediationEnabled()}>
            <fieldset class="space-y-2 rounded border border-border bg-surface-alt/40 px-3 py-2.5">
              <legend class="px-1 text-xs font-semibold text-base-content">
                Actions allowed by this limit
              </legend>
              <For each={eligibleAutoCapabilities()}>
                {(capability) => (
                  <label class="flex items-start gap-2 text-xs text-base-content">
                    <input
                      type="checkbox"
                      checked={autoCapabilities().includes(capability.name)}
                      onChange={(event) =>
                        toggleAutoCapability(capability.name, event.currentTarget.checked)
                      }
                      disabled={saving()}
                      class="mt-0.5"
                    />
                    <span class="min-w-0">
                      <span class="block font-medium">
                        {capabilityDisplayName(capability.name)}
                      </span>
                      <span class="block text-muted">
                        {capability.description || 'Backend-governed resource action'}
                      </span>
                      <span class="mt-0.5 block text-muted">
                        {capability.autoAuthorization === 'low_risk'
                          ? 'Available in Safe auto-fix and Autopilot'
                          : 'Available only in unlocked Autopilot'}
                      </span>
                    </span>
                  </label>
                )}
              </For>

              <label class="flex items-center justify-between gap-3 border-t border-border-subtle pt-2">
                <span>
                  <span class="block text-xs font-medium text-base-content">
                    Restrict to daily hours
                  </span>
                  <span class="block text-[11px] text-muted">
                    Leave off to allow the selected actions at any time.
                  </span>
                </span>
                <Toggle
                  checked={autoWindowEnabled()}
                  onChange={(event) => setAutoWindowEnabled(event.currentTarget.checked)}
                  disabled={saving()}
                />
              </label>

              <Show when={autoWindowEnabled()}>
                <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
                  <label class="block text-[11px] text-muted">
                    <span class="block">Start</span>
                    <input
                      type="time"
                      value={autoWindowStart()}
                      onInput={(event) => setAutoWindowStart(event.currentTarget.value)}
                      disabled={saving()}
                      class="mt-0.5 w-full rounded border border-border bg-surface px-2 py-1 text-xs text-base-content"
                    />
                  </label>
                  <label class="block text-[11px] text-muted">
                    <span class="block">End</span>
                    <input
                      type="time"
                      value={autoWindowEnd()}
                      onInput={(event) => setAutoWindowEnd(event.currentTarget.value)}
                      disabled={saving()}
                      class="mt-0.5 w-full rounded border border-border bg-surface px-2 py-1 text-xs text-base-content"
                    />
                  </label>
                  <label class="block text-[11px] text-muted">
                    <span class="block">Timezone</span>
                    <input
                      type="text"
                      value={autoWindowTimezone()}
                      onInput={(event) => setAutoWindowTimezone(event.currentTarget.value)}
                      disabled={saving()}
                      class="mt-0.5 w-full rounded border border-border bg-surface px-2 py-1 text-xs text-base-content"
                    />
                  </label>
                </div>
              </Show>
              <Show when={autoPolicyValidationError()}>
                <p class="text-[11px] text-red-700 dark:text-red-400">
                  {autoPolicyValidationError()}
                </p>
              </Show>
            </fieldset>
          </Show>
        </div>
      </Show>

      <div class="grid grid-cols-1 gap-3 border-t border-border-subtle pt-3 sm:grid-cols-2">
        <FormSelect
          label="Monitoring"
          density="compact"
          value={monitoringMode()}
          onChange={(event) =>
            setMonitoringMode(event.currentTarget.value as ResourceMonitoringMode)
          }
          disabled={saving() || lifecycleState() === 'retired'}
          help="Expected offline hides availability noise only. Mute all stops Alerts and Patrol while keeping this resource visible."
          helpClass="text-[11px] leading-tight"
        >
          <option value="normal">Normal monitoring</option>
          <option value="expected_offline">Expected offline</option>
          <option value="muted">Mute all attention</option>
        </FormSelect>

        <FormSelect
          label="Lifecycle"
          density="compact"
          value={lifecycleState()}
          onChange={(event) =>
            setLifecycleState(event.currentTarget.value as ResourceLifecycleState)
          }
          disabled={saving()}
          help={inventoryOwnership().retirementDescription}
          helpClass="text-[11px] leading-tight"
        >
          <option value="active">Active</option>
          <option value="retired">Retired from monitoring</option>
        </FormSelect>
      </div>

      <Show when={lifecycleState() === 'retired'}>
        <div class="rounded border border-border bg-surface-alt px-3 py-2 text-xs text-base-content">
          Retired resources remain in {inventoryOwnership().ownerLabel} inventory and keep their
          Pulse history. All alert attention and automated remediation are disabled until the
          lifecycle returns to Active.
        </div>
      </Show>

      <div class="flex items-start justify-between gap-3 pt-2 border-t border-border-subtle">
        <div class="min-w-0 flex-1">
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
          disabled={saving() || lifecycleState() === 'retired'}
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
            disabled={saving() || Boolean(autoPolicyValidationError())}
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
