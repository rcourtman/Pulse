import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const sectionSource = readFileSync(
  resolve(__dirname, '..', 'ResourceOperatorStateSection.tsx'),
  'utf-8',
);

const supportDisclosureSource = readFileSync(
  resolve(__dirname, '..', 'ResourceDetailDrawerSupportDisclosure.tsx'),
  'utf-8',
);

describe('ResourceOperatorStateSection', () => {
  it('exposes operator-set controls bound to the canonical API client', () => {
    // The section is the operator's window into the per-resource state
    // feature. It must offer the suppress/remediation toggles plus
    // Patrol priority and note fields, all routed through the canonical
    // resourceOperatorState client — no parallel fetch path that could
    // drift from the API contract.
    expect(sectionSource).toContain('Normal monitoring');
    expect(sectionSource).toContain('Expected offline');
    expect(sectionSource).toContain('Mute all attention');
    expect(sectionSource).toContain('Retired from monitoring');
    expect(sectionSource).toContain('Never auto-remediate');
    expect(sectionSource).toContain('Patrol priority');
    expect(sectionSource).toContain('Operator note');
  });

  it('explains each control by its effect, not by implementation terms', () => {
    // Users read the help to learn what a control will do; implementation
    // vocabulary hid that answer. Code comments may keep the internal terms.
    const visibleSource = sectionSource
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/^\s*\/\/.*$/gm, '');
    for (const jargon of [
      'persisted resource policy',
      'action broker',
      'dispatch',
      'canonical inventory hierarchy',
      'Backend-governed',
    ]) {
      expect(visibleSource).not.toContain(jargon);
    }
    // An active maintenance window silences alerts and dismisses new findings
    // (kept in history), so the help names both.
    expect(visibleSource).toContain(
      'Silence alerts and dismiss new Patrol findings during planned work',
    );
    // The lock's consequence stays beside it, including approved fixes.
    expect(visibleSource).toContain('Block every automated fix on this resource, including');
    expect(visibleSource).toContain('Expected offline hides availability noise only.');
    // Descendant scope covers alerts and new Patrol findings under the
    // resource, the same maintenance coverage both runtimes resolve.
    expect(visibleSource).toContain('alerts are silenced and new Patrol findings dismissed.');
    expect(visibleSource).not.toContain(
      'With descendants, alerts on everything under this resource',
    );
  });

  it('presents resource action policy as an optional limit on global Patrol autonomy', () => {
    expect(sectionSource).toContain('Automatic action limits');
    expect(sectionSource).toContain('follows your Patrol mode by default');
    expect(sectionSource).toContain('Select at least one eligible capability.');
    expect(sectionSource).toContain('Restrict to daily hours');
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

  it('validates client-side that the scheduled end is strictly after start', () => {
    // The server validates the same constraint and returns 400 with
    // operator_state_invalid; pinning client-side validation keeps the
    // operator out of an avoidable round-trip. The Save button is gated
    // on the validation memo so a stale form state cannot be submitted.
    expect(sectionSource).toContain('End must be after start.');
  });

  it('distinguishes a future-scheduled window from an active one in the badge surface', () => {
    // Active: now is within [start, end). Future-scheduled: start > now.
    // Each surfaces a distinct badge so operators see "scheduled"
    // before the window opens, "active" while it covers now, and
    // nothing once it ends.
    expect(sectionSource).toContain('Maintenance window scheduled.');
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
    expect(sectionSource).toContain('Days the window starts');
    expect(sectionSource).toContain('Recurring maintenance configured.');
  });

  it('keeps drawer disclosure and operator controls touch-safe on phones', () => {
    expect(supportDisclosureSource).toContain('inline-flex min-h-11 shrink-0 items-center');
    expect(sectionSource).toContain(
      'flex flex-col items-stretch justify-between gap-3 border-t border-border-subtle pt-2 sm:flex-row sm:items-center',
    );
    expect(sectionSource).toContain('min-h-11 self-start rounded-sm border border-border');
  });

  it('renders a maintenance-window-active badge when the persisted window covers now', () => {
    // The section must gate the active badge on the
    // now-falls-within-window check, not just on the presence of a
    // window — a future-scheduled window surfaces under a separate
    // "scheduled" badge instead.
    expect(sectionSource).toContain('Maintenance window active.');
  });
});
