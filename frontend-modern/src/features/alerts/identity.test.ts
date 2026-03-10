import { describe, expect, it } from 'vitest';

import {
  getAlertIdentifiers,
  getCanonicalAlertId,
  getLegacyAlertId,
  matchesAlertIdentifier,
} from './identity';

describe('alert identity helpers', () => {
  it('treats id as the canonical alert identifier', () => {
    expect(getCanonicalAlertId({ id: 'resource-a::spec-b' })).toBe('resource-a::spec-b');
  });

  it('returns a trimmed legacy identifier when present', () => {
    expect(getLegacyAlertId({ legacyId: ' legacy-alert-1 ' })).toBe('legacy-alert-1');
    expect(getLegacyAlertId({ legacyId: '' })).toBeUndefined();
  });

  it('returns canonical and legacy identifiers without duplicates', () => {
    expect(getAlertIdentifiers({ id: 'canonical-alert-1', legacyId: 'legacy-alert-1' })).toEqual([
      'canonical-alert-1',
      'legacy-alert-1',
    ]);
    expect(getAlertIdentifiers({ id: 'canonical-alert-1', legacyId: 'canonical-alert-1' })).toEqual(
      ['canonical-alert-1'],
    );
  });

  it('matches either canonical or legacy identifiers', () => {
    const alert = { id: 'canonical-alert-1', legacyId: 'legacy-alert-1' };
    expect(matchesAlertIdentifier(alert, 'canonical-alert-1')).toBe(true);
    expect(matchesAlertIdentifier(alert, 'legacy-alert-1')).toBe(true);
    expect(matchesAlertIdentifier(alert, 'missing-alert')).toBe(false);
  });
});
