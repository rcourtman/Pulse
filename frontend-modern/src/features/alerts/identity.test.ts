import { describe, expect, it } from 'vitest';

import {
  getAlertIdentifiers,
  getCanonicalAlertId,
  matchesAlertIdentifier,
} from './identity';

describe('alert identity helpers', () => {
  it('treats id as the canonical alert identifier', () => {
    expect(getCanonicalAlertId({ id: 'resource-a::spec-b' })).toBe('resource-a::spec-b');
  });

  it('returns only the canonical alert identifier', () => {
    expect(getAlertIdentifiers({ id: 'canonical-alert-1' })).toEqual(['canonical-alert-1']);
  });

  it('matches only the canonical identifier', () => {
    const alert = { id: 'canonical-alert-1' };
    expect(matchesAlertIdentifier(alert, 'canonical-alert-1')).toBe(true);
    expect(matchesAlertIdentifier(alert, 'legacy-alert-1')).toBe(false);
    expect(matchesAlertIdentifier(alert, 'missing-alert')).toBe(false);
  });
});
