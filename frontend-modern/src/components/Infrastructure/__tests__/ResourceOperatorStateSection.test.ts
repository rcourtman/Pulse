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

  it('preserves persisted maintenance-window data on save so the toggle slice does not clobber the window slice', () => {
    // This slice owns toggles only; the maintenance-window scheduler
    // lands separately. If save sent only the toggle fields the server
    // would null out the window data on every save (PUT replaces). Pin
    // that the input passed to setResourceOperatorState carries the
    // current window fields through.
    expect(sectionSource).toContain('maintenanceStartAt: current?.maintenanceStartAt');
    expect(sectionSource).toContain('maintenanceEndAt: current?.maintenanceEndAt');
    expect(sectionSource).toContain('maintenanceReason: current?.maintenanceReason');
    expect(sectionSource).toContain('criticality: current?.criticality');
    expect(sectionSource).toContain('note: current?.note');
  });

  it('renders a maintenance-window-active badge when the persisted window covers now', () => {
    // Read-only display of the active window so operators see "this is
    // why findings are quiet right now" without being able to schedule
    // one yet (scheduler is a follow-up slice). The section must gate
    // the badge on the now-falls-within-window check, not just on the
    // presence of a window — a future-scheduled window should not show
    // as active.
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
