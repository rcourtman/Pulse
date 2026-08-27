import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const sectionSource = readFileSync(
  resolve(__dirname, '..', 'ResourceOperatorStateSection.tsx'),
  'utf-8',
);

const resourceDetailDrawerSource = readFileSync(
  resolve(__dirname, '..', 'ResourceDetailDrawer.tsx'),
  'utf-8',
);

const supportDisclosureSource = readFileSync(
  resolve(__dirname, '..', 'ResourceDetailDrawerSupportDisclosure.tsx'),
  'utf-8',
);

const guestManageSource = readFileSync(
  resolve(__dirname, '..', '..', 'Workloads', 'GuestDrawerManage.tsx'),
  'utf-8',
);

describe('ResourceOperatorStateSection', () => {
  it('exposes operator-set controls bound to the canonical API client', () => {
    // The section is the operator's window into the per-resource state
    // feature. It must offer the suppress/remediation toggles plus
    // Patrol priority and note fields, all routed through the canonical
    // resourceOperatorState client — no parallel fetch path that could
    // drift from the API contract.
    expect(sectionSource).toContain("from '@/api/resourceOperatorState'");
    expect(sectionSource).toContain('getResourceOperatorState');
    expect(sectionSource).toContain('setResourceOperatorState');
    expect(sectionSource).toContain('clearResourceOperatorState');
    expect(sectionSource).toContain('Normal monitoring');
    expect(sectionSource).toContain('Expected offline');
    expect(sectionSource).toContain('Mute all attention');
    expect(sectionSource).toContain('Retired from monitoring');
    expect(sectionSource).toContain('Never auto-remediate');
    expect(sectionSource).toContain('Patrol priority');
    expect(sectionSource).toContain('Operator note');
    expect(sectionSource).toContain('setCriticality');
    expect(sectionSource).toContain('setNote');
  });

  it('presents resource action policy as an optional limit on global Patrol autonomy', () => {
    expect(sectionSource).toContain('Automatic action limits');
    expect(sectionSource).toContain('follows your Patrol mode by default');
    expect(sectionSource).toContain("capability.autoAuthorization !== 'never'");
    expect(sectionSource).toContain('Select at least one eligible capability.');
    expect(sectionSource).toContain('Restrict to daily hours');
    expect(sectionSource).toContain('autoRemediationPolicy');
    expect(sectionSource).toContain('disabled={saving() || Boolean(autoPolicyValidationError())}');
  });

  it('keeps the section out of the parent Suspense fallback by using createNonSuspendingQuery', () => {
    // The drawer wraps its children in a page-level Suspense fallback;
    // a vanilla createResource here would flicker the fallback every
    // time the section's fetch resolves. Pin the helper choice so this
    // does not silently regress to createResource.
    expect(sectionSource).toContain('createNonSuspendingQuery');
    expect(sectionSource).not.toContain('createResource<');
  });

  it('requires explicit confirmation before flipping never-auto-remediate to true', () => {
    // NeverAutoRemediate is a safety override — flipping it on must
    // require explicit confirmation so the operator does not lock a
    // resource by accident. The release path (true → false) is
    // permissive because clearing a lock is the recoverable action.
    expect(sectionSource).toContain('confirmingLock');
    expect(sectionSource).toContain('Lock this resource against all automated remediation?');
    expect(sectionSource).toContain('handleNeverAutoRemediateToggle');
    // The confirmation must not block the disable path — the inversion
    // gate only fires when next=true and the current value is false.
    expect(sectionSource).toContain('if (next && !neverAutoRemediate())');
  });

  it('preserves persisted maintenance-window data on override save so window scheduling stays decoupled', () => {
    // Saving non-window overrides must not clobber a persisted
    // maintenance window. The save path reads the current persisted
    // window fields and forwards them through the PUT body (which
    // replaces the whole record), while priority/note come from the
    // local edit state.
    expect(sectionSource).toContain('maintenanceStartAt: current?.maintenanceStartAt');
    expect(sectionSource).toContain('maintenanceEndAt: current?.maintenanceEndAt');
    expect(sectionSource).toContain('maintenanceRecurrence: current?.maintenanceRecurrence');
    expect(sectionSource).toContain("maintenanceScope: current?.maintenanceScope ?? 'resource'");
    expect(sectionSource).toContain('maintenanceReason: current?.maintenanceReason');
    expect(sectionSource).toContain('criticality: criticality()');
    expect(sectionSource).toContain('note: noteForSave()');
  });

  it('offers the canonical Patrol priority options and dirty-tracks priority plus note edits', () => {
    // Criticality is constrained by the API contract. The drawer must
    // expose exactly the canonical values and compare both priority and
    // trimmed note against persisted state so Save/Discard appear for
    // those edits too.
    expect(sectionSource).toContain('<option value="">Default</option>');
    expect(sectionSource).toContain('<option value="high">High</option>');
    expect(sectionSource).toContain('<option value="medium">Medium</option>');
    expect(sectionSource).toContain('<option value="low">Low</option>');
    expect(sectionSource).toContain('const persistedCriticality = current?.criticality ??');
    expect(sectionSource).toContain('const persistedNote = current?.note ??');
    expect(sectionSource).toContain('criticality() !== persistedCriticality');
    expect(sectionSource).toContain('note().trim() !== persistedNote');
  });

  it('keeps priority and note edit-state when scheduling or clearing maintenance windows', () => {
    // PUT replaces the whole record, so the maintenance-window actions
    // must forward the local priority/note signals rather than the last
    // persisted values. Otherwise editing a note and then scheduling a
    // window would silently lose the note.
    expect(sectionSource).toContain("scheduleKind() === 'once' ? start!.toISOString() : undefined");
    expect(sectionSource).toContain("scheduleKind() === 'recurring'");
    expect(sectionSource).toContain('maintenanceRecurrence:');
    expect(sectionSource).toContain('maintenanceScope: scheduleScope()');
    expect(sectionSource).toContain('criticality: criticality()');
    expect(sectionSource).toContain('note: noteForSave()');
    expect(sectionSource).not.toContain('criticality: current?.criticality');
    expect(sectionSource).not.toContain('note: current?.note');
  });

  it('exposes a maintenance-window scheduler with HTML5 datetime-local inputs and quick presets', () => {
    // The scheduler is the operator's path to setting a maintenance
    // window without curling the API. Pin the wiring: datetime-local
    // inputs, validation that end > start, optional reason field, and
    // 1h/4h/24h quick presets.
    expect(sectionSource).toContain('schedulerOpen');
    expect(sectionSource).toContain('handleScheduleSave');
    expect(sectionSource).toContain('handleClearMaintenanceWindow');
    expect(sectionSource).toContain('applyPresetDuration');
    expect(sectionSource).toContain('type="datetime-local"');
    expect(sectionSource).toContain('scheduleValidationError');
    // Quick presets — the three most common operator durations.
    expect(sectionSource).toContain('<For each={[1, 4, 24]}>');
    expect(sectionSource).toContain('applyPresetDuration(hours)');
    // Both directions of the datetime conversion live in helpers so the
    // scheduler stays free of inline date arithmetic.
    expect(sectionSource).toContain('formatLocalForInput');
    expect(sectionSource).toContain('parseLocalFromInput');
  });

  it('validates client-side that the scheduled end is strictly after start', () => {
    // The server validates the same constraint and returns 400 with
    // operator_state_invalid; pinning client-side validation keeps the
    // operator out of an avoidable round-trip. The Save button is gated
    // on the validation memo so a stale form state cannot be submitted.
    expect(sectionSource).toContain('End must be after start.');
    expect(sectionSource).toContain('disabled={saving() || Boolean(scheduleValidationError())}');
  });

  it('distinguishes a future-scheduled window from an active one in the badge surface', () => {
    // Active: now is within [start, end). Future-scheduled: start > now.
    // Each surfaces a distinct badge so operators see "scheduled"
    // before the window opens, "active" while it covers now, and
    // nothing once it ends.
    expect(sectionSource).toContain('scheduledMaintenanceWindow');
    expect(sectionSource).toContain('Maintenance window scheduled.');
    expect(sectionSource).toContain('Auto-acknowledgement will');
    expect(sectionSource).toContain("start{' '}");
  });

  it('exposes Edit window and Cancel window controls when a window exists', () => {
    // The compact view (form closed) must offer both editing the
    // window and clearing it without reopening the form. Cancel
    // window must clear ONLY the window fields, preserving the
    // toggles.
    expect(sectionSource).toContain('Schedule window');
    expect(sectionSource).toContain('Edit window');
    expect(sectionSource).toContain('Cancel window');
    // handleClearMaintenanceWindow must preserve toggles by reading
    // the current edit-state signals rather than nulling everything.
    expect(sectionSource).toContain('monitoringMode: monitoringMode()');
    expect(sectionSource).toContain('lifecycleState: lifecycleState()');
    expect(sectionSource).toContain(
      "intentionallyOffline: monitoringMode() === 'expected_offline'",
    );
    expect(sectionSource).toContain('neverAutoRemediate: neverAutoRemediate()');
    expect(sectionSource).toContain('maintenanceStartAt: undefined,');
    expect(sectionSource).toContain('maintenanceEndAt: undefined,');
    expect(sectionSource).toContain('maintenanceRecurrence: undefined,');
    expect(sectionSource).toContain("maintenanceScope: 'resource',");
  });

  it('offers recurring timezone-aware windows and explicit descendant scope', () => {
    expect(sectionSource).toContain("createSignal<'once' | 'recurring'>");
    expect(sectionSource).toContain('Days the window starts');
    expect(sectionSource).toContain('scheduleTimezone');
    expect(sectionSource).toContain('startMinute: timeToMinute(scheduleRecurringStart())');
    expect(sectionSource).toContain('endMinute: timeToMinute(scheduleRecurringEnd())');
    expect(sectionSource).toContain('resource_and_descendants');
    expect(sectionSource).toContain('Recurring maintenance configured.');
  });

  it('keeps drawer disclosure and operator controls touch-safe on phones', () => {
    expect(supportDisclosureSource).toContain('inline-flex min-h-11 shrink-0 items-center');
    expect(sectionSource).toContain('density="compact"');
    expect(sectionSource).toContain('<FormSelect');
    expect(sectionSource).toContain('<FormTextarea');
    expect(sectionSource).toContain('min-h-11 min-w-11 px-1.5 py-0.5');
    expect(sectionSource).toContain(
      'flex flex-col items-stretch justify-between gap-3 border-t border-border-subtle pt-2 sm:flex-row sm:items-center',
    );
    expect(sectionSource).toContain('min-h-11 self-start rounded border border-border');
    expect(sectionSource).toContain(
      'min-h-11 px-2.5 py-1 text-xs font-medium text-white bg-blue-600',
    );
  });

  it('renders a maintenance-window-active badge when the persisted window covers now', () => {
    // The section must gate the active badge on the
    // now-falls-within-window check, not just on the presence of a
    // window — a future-scheduled window surfaces under a separate
    // "scheduled" badge instead.
    expect(sectionSource).toContain('activeMaintenanceWindow');
    expect(sectionSource).toContain('Maintenance window active.');
    expect(sectionSource).toContain('if (now < start || now >= end) return null;');
  });

  it('attributes the persisted state with set-by and set-at metadata', () => {
    // The audit-attribution comes from server-side population (setAt /
    // setBy populated from the authenticated identity). The section
    // must surface both so operators can see "I set this 3 days ago"
    // when revisiting a resource.
    expect(sectionSource).toContain('persisted()?.setBy');
    expect(sectionSource).toContain('persisted()?.setAt');
  });
});

describe('ResourceDetailDrawer manage-tab integration', () => {
  it('renders ResourceOperatorStateSection alongside ResourceActionHistory', () => {
    // The operator-set state and the action audit history are
    // conceptually paired — what the operator decided to suppress, and
    // what Pulse actually did. They belong on the same drawer surface
    // so the operator can read both stories together.
    expect(resourceDetailDrawerSource).toContain("from './ResourceOperatorStateSection'");
    expect(resourceDetailDrawerSource).toContain('capabilities={props.resource.capabilities}');
    // Section must precede the action-history block so the override
    // explains the actions that follow, not vice versa.
    const operatorIndex = resourceDetailDrawerSource.indexOf('<ResourceOperatorStateSection');
    const historyIndex = resourceDetailDrawerSource.indexOf('<ResourceActionHistory');
    expect(operatorIndex).toBeGreaterThan(0);
    expect(historyIndex).toBeGreaterThan(0);
    expect(operatorIndex).toBeLessThan(historyIndex);
  });

  it('renders operator overrides for every resource regardless of capability eligibility', () => {
    // Issue #1622: the section used to be gated on the resource exposing
    // an auto-authorizable capability, which hid intentionally-offline /
    // never-auto-remediate exactly when they matter most (a stopped
    // container only exposes `start`, which is never auto-authorized).
    // The overrides don't depend on capabilities — only the
    // automatic-actions block does, and it self-gates inside the section
    // via eligibleAutoCapabilities.
    expect(resourceDetailDrawerSource).toContain('resourceId={props.resource.id}');
    expect(resourceDetailDrawerSource).not.toContain('shouldRenderOperatorStateSection');
    expect(sectionSource).toContain('<Show when={eligibleAutoCapabilities().length > 0}>');
  });
});

describe('Proxmox guest drawer integration', () => {
  it('exposes the same canonical policy with provider ownership context', () => {
    expect(guestManageSource).toContain(
      "from '@/components/Infrastructure/ResourceOperatorStateSection'",
    );
    expect(guestManageSource).toContain('<ResourceOperatorStateSection');
    expect(guestManageSource).toContain('resourceId={props.resourceId}');
    expect(guestManageSource).toContain("platformType={props.guest.platformType || 'proxmox'}");
  });
});
