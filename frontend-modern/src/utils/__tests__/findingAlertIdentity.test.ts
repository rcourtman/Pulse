import { describe, expect, it } from 'vitest';
import { getFindingAlertIdentifier, hasTriggeringAlert } from '@/utils/findingAlertIdentity';

describe('findingAlertIdentity', () => {
  it('prefers canonical alertIdentifier when present', () => {
    expect(
      getFindingAlertIdentifier({
        alertIdentifier: 'instance:node:100::metric/cpu',
      }),
    ).toBe('instance:node:100::metric/cpu');
  });

  it('treats blank values as missing', () => {
    expect(getFindingAlertIdentifier({ alertIdentifier: '  ' })).toBeUndefined();
    expect(hasTriggeringAlert({ alertIdentifier: '  ' })).toBe(false);
  });

  it('detects when a finding was triggered by an alert', () => {
    expect(hasTriggeringAlert({ alertIdentifier: 'instance:node:100::metric/cpu' })).toBe(true);
  });
});
