import { describe, expect, it } from 'vitest';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import {
  getStorageRowAlertPresentation,
  type StorageRowAlertPresentation,
} from '@/features/storageBackups/storageRowAlertPresentation';

// ---------------------------------------------------------------------------
// Fixture builders — mirror the alert-state shape used by the sibling
// storageRowAlertPresentation.test.ts so casts and import paths match. The
// default describes a healthy row (no alerts, null severity); each case
// overrides the relevant flags to drive a single branch of
// getStorageRowAlertPresentation.
// ---------------------------------------------------------------------------

const makeAlertState = (overrides: Partial<StorageAlertRowState> = {}): StorageAlertRowState => ({
  hasAlert: false,
  alertCount: 0,
  severity: null,
  hasUnacknowledgedAlert: false,
  unacknowledgedCount: 0,
  acknowledgedCount: 0,
  hasAcknowledgedOnlyAlert: false,
  ...overrides,
});

type RowOptions = Parameters<typeof getStorageRowAlertPresentation>[0];

const makeOptions = (overrides: Partial<RowOptions> = {}): RowOptions => ({
  alertState: makeAlertState(),
  parentNodeOnline: true,
  isExpanded: false,
  isResourceHighlighted: false,
  ...overrides,
});

// The two BASE_ROW_CLASSES the module always seeds, joined exactly as the
// module joins them (with a single space). Used to assert the "no extra
// classes" arm returns ONLY the base styling.
const BASE_ROW_CLASS = 'transition-all duration-200 hover:bg-surface-hover';

// ===========================================================================
// getStorageRowAlertPresentation — uncovered branches
// ===========================================================================
//
// The sibling storageRowAlertPresentation.test.ts already covers:
//   - showAlertHighlight true + severity 'critical' (red bg + critical accent)
//   - the acknowledged-only arm (severity 'warning', acknowledged classes)
// This file targets the remaining branch arms enumerated below.

describe('getStorageRowAlertPresentation branch coverage', () => {
  // ---- L30/L35 severity !== 'critical' (false arm) under showAlertHighlight -
  it('applies warning bg + warning accent for an unacknowledged warning alert', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'warning',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        }),
      }),
    );

    expect(result.rowClass).toContain('bg-yellow-50');
    expect(result.rowClass).toContain('dark:bg-yellow-950');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_#eab308]');
    // The critical classes must NOT appear on the warning arm.
    expect(result.rowClass).not.toContain('bg-red-50');
    expect(result.rowClass).not.toContain('shadow-[inset_4px_0_0_0_#ef4444]');
    expect(result.dataAlertState).toBe('unacknowledged');
    expect(result.dataAlertSeverity).toBe('warning');
  });

  // ---- L30/L35 severity !== 'critical' when severity is null but
  //      hasUnacknowledgedAlert is true (malformed alert state): the ternary
  //      falls to the warning-shaped else arm while dataAlertSeverity uses the
  //      `|| 'none'` fallback. -----------------------------------------------
  it('falls to the warning-shaped classes when severity is null but unacknowledged is true', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: null,
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        }),
      }),
    );

    // severity !== 'critical' (null is not 'critical') -> warning else arm.
    expect(result.rowClass).toContain('bg-yellow-50');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_#eab308]');
    // dataAlertSeverity fallback: null || 'none' -> 'none'.
    expect(result.dataAlertSeverity).toBe('none');
    expect(result.dataAlertState).toBe('unacknowledged');
  });

  // ---- L28 showAlertHighlight precedence over L39 isResourceHighlighted ----
  //      When both are true, the critical/warning classes win and the blue
  //      highlight classes must NOT be appended.
  it('prefers the unacknowledged-alert classes over resource highlighting', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        }),
        isResourceHighlighted: true,
      }),
    );

    expect(result.rowClass).toContain('bg-red-50');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_#ef4444]');
    expect(result.rowClass).not.toContain('bg-blue-50');
    expect(result.rowClass).not.toContain('ring-blue-300');
    // dataResourceHighlighted still reflects the input verbatim.
    expect(result.dataResourceHighlighted).toBe('true');
    expect(result.dataAlertState).toBe('unacknowledged');
  });

  // ---- L39 else-if isResourceHighlighted true arm (blue highlight) ---------
  it('applies the blue resource-highlight classes when only isResourceHighlighted is true', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        isResourceHighlighted: true,
      }),
    );

    expect(result.rowClass).toContain('bg-blue-50');
    expect(result.rowClass).toContain('dark:bg-blue-900');
    expect(result.rowClass).toContain('ring-1');
    expect(result.rowClass).toContain('ring-blue-300');
    expect(result.rowClass).toContain('dark:ring-blue-600');
    // No alert styling should leak in.
    expect(result.rowClass).not.toContain('bg-red-50');
    expect(result.rowClass).not.toContain('bg-yellow-50');
    expect(result.rowClass).not.toContain('shadow-[inset_4px_0_0_0_rgba(156,163,175,0.8)]');
    expect(result.dataResourceHighlighted).toBe('true');
    // No alerts -> dataAlertState 'none' (the L51-55 ternary final arm).
    expect(result.dataAlertState).toBe('none');
  });

  // ---- L39 isResourceHighlighted precedence over L41 hasAcknowledgedOnly ---
  //      The else-if chain means resource highlighting wins over acknowledged.
  //      dataAlertState is computed independently and still reports
  //      'acknowledged'.
  it('prefers resource-highlight classes over acknowledged-only styling, but still reports acknowledged state', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'warning',
          acknowledgedCount: 1,
          hasAcknowledgedOnlyAlert: true,
        }),
        isResourceHighlighted: true,
      }),
    );

    // Blue classes from the resource-highlight arm.
    expect(result.rowClass).toContain('bg-blue-50');
    expect(result.rowClass).toContain('ring-blue-300');
    // Acknowledged accent must NOT be present (else-if did not reach it).
    expect(result.rowClass).not.toContain('shadow-[inset_4px_0_0_0_rgba(156,163,175,0.8)]');
    // dataAlertState is independent of the class if-chain.
    expect(result.dataAlertState).toBe('acknowledged');
    expect(result.dataResourceHighlighted).toBe('true');
  });

  // ---- No arm of the if/else-if chain (all flags false) --------------------
  //      Only BASE_ROW_CLASSES remain; every data attr is the neutral default.
  it('returns only the base classes when no alert, highlight or expand flags are set', () => {
    const result: StorageRowAlertPresentation = getStorageRowAlertPresentation(makeOptions());

    expect(result.rowClass).toBe(BASE_ROW_CLASS);
    expect(result.dataAlertState).toBe('none');
    // severity null -> null || 'none' -> 'none' (L56 right arm).
    expect(result.dataAlertSeverity).toBe('none');
    expect(result.dataResourceHighlighted).toBe('false');
  });

  // ---- L45 isExpanded true arm ---------------------------------------------
  it('appends the expanded bg-surface-alt class when isExpanded is true', () => {
    const result = getStorageRowAlertPresentation(makeOptions({ isExpanded: true }));

    // Base classes are still present and the expanded class is appended.
    expect(result.rowClass).toContain('transition-all duration-200');
    expect(result.rowClass).toContain('hover:bg-surface-hover');
    expect(result.rowClass).toContain('bg-surface-alt');
    // The expanded class appears exactly once (no alert/acknowledged arm ran).
    const surfaceAltCount = (result.rowClass.match(/bg-surface-alt/g) ?? []).length;
    expect(surfaceAltCount).toBe(1);
  });

  // ---- L45 isExpanded stacks on top of an unacknowledged critical alert ----
  it('stacks the expanded class on top of the critical unacknowledged classes', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        }),
        isExpanded: true,
      }),
    );

    expect(result.rowClass).toContain('bg-red-50');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_#ef4444]');
    expect(result.rowClass).toContain('bg-surface-alt');
  });

  // ---- L45 isExpanded stacks on top of the acknowledged-only arm -----------
  it('stacks the expanded class on top of the acknowledged-only classes', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'warning',
          acknowledgedCount: 1,
          hasAcknowledgedOnlyAlert: true,
        }),
        isExpanded: true,
      }),
    );

    // Acknowledged accent + TWO bg-surface-alt (acknowledged arm + expanded).
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_rgba(156,163,175,0.8)]');
    const surfaceAltCount = (result.rowClass.match(/bg-surface-alt/g) ?? []).length;
    expect(surfaceAltCount).toBe(2);
  });

  // ---- L22-23 && short-circuit: hasUnacknowledgedAlert true but
  //      parentNodeOnline false -> showAlertHighlight false -----------------
  it('suppresses the unacknowledged highlight when the parent node is offline', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        }),
        parentNodeOnline: false,
      }),
    );

    // No alert classes — the highlight was gated by parentNodeOnline.
    expect(result.rowClass).not.toContain('bg-red-50');
    expect(result.rowClass).not.toContain('bg-yellow-50');
    expect(result.rowClass).not.toContain('shadow-[inset_4px_0_0_0_#ef4444]');
    // showAlertHighlight is false, so dataAlertState is NOT 'unacknowledged'.
    expect(result.dataAlertState).toBe('none');
    // severity is still surfaced verbatim via the || fallback left operand.
    expect(result.dataAlertSeverity).toBe('critical');
  });

  // ---- L24-25 && short-circuit: hasAcknowledgedOnlyAlert true but
  //      parentNodeOnline false -> local hasAcknowledgedOnlyAlert false -----
  it('suppresses the acknowledged-only styling when the parent node is offline', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'warning',
          acknowledgedCount: 1,
          hasAcknowledgedOnlyAlert: true,
        }),
        parentNodeOnline: false,
      }),
    );

    expect(result.rowClass).not.toContain('shadow-[inset_4px_0_0_0_rgba(156,163,175,0.8)]');
    expect(result.rowClass).not.toContain('bg-surface-alt');
    // Local hasAcknowledgedOnlyAlert is false -> dataAlertState 'none'.
    expect(result.dataAlertState).toBe('none');
  });

  // ---- L51-55 dataAlertState 'none' arm (showAlertHighlight false AND
  //      hasAcknowledgedOnlyAlert false) with a non-null severity to confirm
  //      dataAlertSeverity uses the truthy left operand of `||` -------------
  it('reports dataAlertSeverity verbatim for a truthy warning severity with no row-level alert', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({ severity: 'warning' }),
      }),
    );

    expect(result.dataAlertState).toBe('none');
    // 'warning' is truthy -> left operand of `||` is used (NOT 'none').
    expect(result.dataAlertSeverity).toBe('warning');
  });

  // ---- L56 `severity || 'none'` left operand with critical severity --------
  it('reports dataAlertSeverity as critical when severity is critical', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({ severity: 'critical' }),
      }),
    );

    expect(result.dataAlertSeverity).toBe('critical');
  });

  // ---- L57 isResourceHighlighted false -> 'false' (explicit boundary) ------
  it('reports dataResourceHighlighted as false when isResourceHighlighted is false', () => {
    const result = getStorageRowAlertPresentation(makeOptions({ isResourceHighlighted: false }));

    expect(result.dataResourceHighlighted).toBe('false');
  });

  // ---- Deliberately-malformed empty-string severity: '' is falsy, so the
  //      `|| 'none'` fallback fires even though it is technically a string. -
  it('falls back to dataAlertSeverity none when severity is an empty string', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          severity: '' as unknown as StorageAlertRowState['severity'],
        }),
      }),
    );

    // '' is falsy -> right operand of || -> 'none'.
    expect(result.dataAlertSeverity).toBe('none');
  });

  // ---- Full return shape contract on a representative unacknowledged case -
  it('returns the full StorageRowAlertPresentation shape for an unacknowledged warning with highlight and expand', () => {
    const result = getStorageRowAlertPresentation(
      makeOptions({
        alertState: makeAlertState({
          hasAlert: true,
          alertCount: 2,
          severity: 'warning',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 2,
        }),
        isExpanded: true,
        isResourceHighlighted: true,
      }),
    );

    // showAlertHighlight wins over isResourceHighlighted in the class chain.
    expect(result.rowClass).toContain('bg-yellow-50');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_#eab308]');
    expect(result.rowClass).toContain('bg-surface-alt');
    // The four return fields and their literal types.
    expect(result.dataAlertState).toBe('unacknowledged');
    expect(result.dataAlertSeverity).toBe('warning');
    expect(result.dataResourceHighlighted).toBe('true');
    expect(typeof result.rowClass).toBe('string');
  });
});
