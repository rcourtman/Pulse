import { describe, expect, it } from 'vitest';
import {
  ENVIRONMENT_LOCK_BADGE_CLASS,
  ENVIRONMENT_LOCK_BADGE_LABEL,
  ENVIRONMENT_LOCK_BUTTON_TITLE,
  getEnvironmentLockTitle,
} from '../environmentLockPresentation';

describe('environmentLockPresentation', () => {
  it('exports canonical environment lock badge semantics', () => {
    expect(ENVIRONMENT_LOCK_BADGE_LABEL).toBe('ENV');
    expect(ENVIRONMENT_LOCK_BADGE_CLASS).toContain('bg-amber-100');
    expect(getEnvironmentLockTitle('PULSE_TELEMETRY')).toBe(
      'Locked by environment variable PULSE_TELEMETRY',
    );
    expect(ENVIRONMENT_LOCK_BUTTON_TITLE).toBe('Locked by environment variable');
  });
});
