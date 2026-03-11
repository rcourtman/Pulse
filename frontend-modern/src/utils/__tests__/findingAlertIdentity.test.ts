import { describe, expect, it } from 'vitest';
import { getFindingAlertIdentifier, hasTriggeringAlert } from '@/utils/findingAlertIdentity';

describe('findingAlertIdentity', () => {
  it('prefers canonical alertIdentifier when present', () => {
    expect(
      getFindingAlertIdentifier({
        alertIdentifier: 'instance:node:100::metric/cpu',
        alertId: 'legacy-alert-id',
      }),
    ).toBe('instance:node:100::metric/cpu');
  });

  it('falls back to compatibility alertId when canonical identifier is absent', () => {
    expect(
      getFindingAlertIdentifier({
        alertId: 'legacy-alert-id',
      }),
    ).toBe('legacy-alert-id');
  });

  it('treats blank values as missing', () => {
    expect(getFindingAlertIdentifier({ alertIdentifier: '  ', alertId: '' })).toBeUndefined();
    expect(hasTriggeringAlert({ alertIdentifier: '  ', alertId: '' })).toBe(false);
  });

  it('detects when a finding was triggered by an alert', () => {
    expect(hasTriggeringAlert({ alertIdentifier: 'instance:node:100::metric/cpu' })).toBe(true);
    expect(hasTriggeringAlert({ alertId: 'legacy-alert-id' })).toBe(true);
  });
});
