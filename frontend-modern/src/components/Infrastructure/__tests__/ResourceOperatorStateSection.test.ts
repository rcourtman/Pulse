import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const sectionSource = readFileSync(
  resolve(__dirname, '..', 'ResourceOperatorStateSection.tsx'),
  'utf-8',
);

const overviewTabSource = readFileSync(
  resolve(__dirname, '..', 'ResourceDetailDrawerOverviewTab.tsx'),
  'utf-8',
);

describe('ResourceOperatorStateSection', () => {
  it('exposes the two operator-set toggles bound to the canonical API client', () => {
    // The section is the operator's window into the per-resource state
    // feature. It must offer both toggles (intentionally offline + never
    // auto-remediate) and route them through the canonical
    // resourceOperatorState client — no parallel fetch path that could
    // drift from the API contract.
    expect(sectionSource).toContain("from '@/api/resourceOperatorState'");
    expect(sectionSource).toContain('getResourceOperatorState');
    expect(sectionSource).toContain('setResourceOperatorState');
    expect(sectionSource).toContain('clearResourceOperatorState');
    expect(sectionSource).toContain('Intentionally offline');
    expect(sectionSource).toContain('Never auto-remediate');
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

  it('preserves persisted maintenance-window data on toggle save so the two facets stay decoupled', () => {
    // Toggling intentionallyOffline or neverAutoRemediate must not
    // clobber a persisted maintenance window. The toggle save path
    // reads the current persisted window fields and forwards them
    // through the PUT body (which replaces the whole record).
    expect(sectionSource).toContain('maintenanceStartAt: current?.maintenanceStartAt');
    expect(sectionSource).toContain('maintenanceEndAt: current?.maintenanceEndAt');
    expect(sectionSource).toContain('maintenanceReason: current?.maintenanceReason');
    expect(sectionSource).toContain('criticality: current?.criticality');
    expect(sectionSource).toContain('note: current?.note');
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
    expect(sectionSource).toContain('applyPresetDuration(1)');
    expect(sectionSource).toContain('applyPresetDuration(4)');
    expect(sectionSource).toContain('applyPresetDuration(24)');
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
    expect(sectionSource).toContain('Auto-acknowledgement will start');
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
    expect(sectionSource).toContain('intentionallyOffline: intentionallyOffline()');
    expect(sectionSource).toContain('neverAutoRemediate: neverAutoRemediate()');
    expect(sectionSource).toContain('maintenanceStartAt: undefined,');
    expect(sectionSource).toContain('maintenanceEndAt: undefined,');
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

describe('ResourceDetailDrawerOverviewTab integration', () => {
  it('renders ResourceOperatorStateSection alongside ResourceActionHistory', () => {
    // The operator-set state and the action audit history are
    // conceptually paired — what the operator decided to suppress, and
    // what Pulse actually did. They belong on the same drawer surface
    // so the operator can read both stories together.
    expect(overviewTabSource).toContain("from './ResourceOperatorStateSection'");
    expect(overviewTabSource).toContain('<ResourceOperatorStateSection resourceId={resource.id} />');
    // Section must precede the action-history block so the override
    // explains the actions that follow, not vice versa.
    const operatorIndex = overviewTabSource.indexOf('<ResourceOperatorStateSection');
    const historyIndex = overviewTabSource.indexOf('<ResourceActionHistory');
    expect(operatorIndex).toBeGreaterThan(0);
    expect(historyIndex).toBeGreaterThan(0);
    expect(operatorIndex).toBeLessThan(historyIndex);
  });
});
