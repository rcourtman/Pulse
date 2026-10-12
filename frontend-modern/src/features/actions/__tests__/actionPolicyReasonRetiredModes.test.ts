import { describe, expect, it } from 'vitest';
import { formatPolicyReason } from '../actionPresentation';

// Patrol's Safe auto-fix (assisted) and Autopilot (full) modes were retired.
// Action records planned before that still carry their reason codes, so the
// review surface keeps labelling them, in the past tense, as retired modes.
describe('formatPolicyReason for retired Patrol modes', () => {
  it.each([
    ['tenant_mode_assisted', 'Patrol was in Safe fixes (retired mode)'],
    ['tenant_mode_full', 'Patrol was in Autopilot (retired mode)'],
    ['tenant_full_mode_locked', 'Autopilot acknowledgement was not active (retired mode)'],
    ['tenant_full_mode_unlocked', 'Autopilot acknowledgement was active (retired mode)'],
  ] as const)('labels historical %s as a retired mode', (reason, label) => {
    expect(formatPolicyReason(reason)).toBe(label);
  });

  it('keeps the current modes in the present tense', () => {
    expect(formatPolicyReason('tenant_mode_monitor')).toBe('Patrol is in Watch only');
  });
});
