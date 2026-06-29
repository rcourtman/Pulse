import { describe, expect, it } from 'vitest';

import { hasSmartWarning, smartWarningEvidence, smartWarningTitle } from '../smartEvidence';
import type { SMARTAttributes } from '@/types/api';

describe('smartEvidence', () => {
  it('flags critical SATA SMART counters', () => {
    const attrs: SMARTAttributes = {
      reallocatedSectors: 0,
      pendingSectors: 2,
      offlineUncorrectable: 1,
      udmaCrcErrors: 4,
    };

    expect(hasSmartWarning(attrs)).toBe(true);
    expect(smartWarningEvidence(attrs).map((entry) => entry.key)).toEqual([
      'pendingSectors',
      'offlineUncorrectable',
      'udmaCrcErrors',
    ]);
  });

  it('flags NVMe life and media counters', () => {
    const attrs: SMARTAttributes = {
      percentageUsed: 91,
      availableSpare: 19,
      mediaErrors: 1,
      unsafeShutdowns: 7,
    };

    expect(smartWarningEvidence(attrs).map((entry) => entry.key)).toEqual([
      'percentageUsed',
      'availableSpare',
      'mediaErrors',
    ]);
    expect(smartWarningTitle(attrs)).toContain('Available spare=19');
  });

  it('does not flag normal or informational counters', () => {
    const attrs: SMARTAttributes = {
      reallocatedSectors: 0,
      pendingSectors: 0,
      percentageUsed: 45,
      availableSpare: 99,
      mediaErrors: 0,
      unsafeShutdowns: 12,
    };

    expect(hasSmartWarning(attrs)).toBe(false);
    expect(smartWarningTitle(attrs)).toBe('SMART counters normal');
  });
});
