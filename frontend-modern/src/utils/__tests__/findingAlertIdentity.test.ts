import { describe, expect, it } from 'vitest';
import {
  getFindingAlertIdentifier,
  hasTriggeringAlert,
  isAlertMirroredFinding,
} from '@/utils/findingAlertIdentity';

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

  it('demotes only active findings that Patrol stamped as mirroring an alert', () => {
    expect(isAlertMirroredFinding({ mirrorsAlertId: 'usage-storage-1', status: 'active' })).toBe(
      true,
    );
    expect(isAlertMirroredFinding({ mirrorsAlertId: '  ', status: 'active' })).toBe(false);
    expect(isAlertMirroredFinding({ status: 'active' })).toBe(false);
    // A remembered or resolved mirrored finding is Patrol history, not a duplicate item.
    expect(isAlertMirroredFinding({ mirrorsAlertId: 'usage-storage-1', status: 'dismissed' })).toBe(
      false,
    );
    expect(isAlertMirroredFinding({ mirrorsAlertId: 'usage-storage-1', status: 'resolved' })).toBe(
      false,
    );
  });
});
