import { describe, expect, it } from 'vitest';
import {
  MAX_PROTECTION_POSTURE_BATCH_SIZE,
  buildProtectionPostureBatchURL,
  normalizeProtectionPostureResourceIDs,
} from '@/hooks/useProtectionPostures';

describe('useProtectionPostures transport', () => {
  it('deduplicates and sorts resource IDs for a stable bounded cache key and request', () => {
    expect(normalizeProtectionPostureResourceIDs(['vm:b', ' vm:a ', '', 'vm:b'])).toEqual([
      'vm:a',
      'vm:b',
    ]);

    const url = new URL(
      buildProtectionPostureBatchURL(['vm:b', 'vm:a', 'vm:b']),
      'https://pulse.invalid',
    );
    expect(url.pathname).toBe('/api/recovery/postures');
    expect(url.searchParams.getAll('resourceId')).toEqual(['vm:a', 'vm:b']);
    expect(url.searchParams.get('limit')).toBe(String(MAX_PROTECTION_POSTURE_BATCH_SIZE));
  });

  it('rejects a batch larger than the server contract', () => {
    const resourceIDs = Array.from(
      { length: MAX_PROTECTION_POSTURE_BATCH_SIZE + 1 },
      (_, index) => `vm:${index}`,
    );
    expect(() => buildProtectionPostureBatchURL(resourceIDs)).toThrow(/limited to 200/);
  });
});
