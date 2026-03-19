import { describe, expect, it } from 'vitest';

import {
  formatResourceCorrelationEndpoint,
  formatResourceCorrelationHeadline,
  formatResourceCorrelationPattern,
  formatResourceCorrelationSummary,
} from '@/utils/resourceCorrelationPresentation';

describe('resourceCorrelationPresentation utils', () => {
  const correlation = {
    source_id: 'storage-1',
    source_name: 'Storage 1',
    source_type: 'storage',
    target_id: 'vm-42',
    target_name: 'VM 42',
    target_type: 'vm',
    event_pattern: 'disk_full -> restart',
    occurrences: 2,
    avg_delay: 5_000_000_000,
    confidence: 0.875,
    last_seen: '2026-03-18T12:00:00Z',
    description: 'Disk pressure often precedes restarts',
  } as const;

  it('formats correlation endpoints and headline labels', () => {
    expect(formatResourceCorrelationEndpoint(correlation, 'source')).toBe('Storage 1');
    expect(formatResourceCorrelationEndpoint(correlation, 'target')).toBe('VM 42');
    expect(formatResourceCorrelationHeadline(correlation)).toBe('Storage 1 → VM 42');
  });

  it('formats correlation patterns and summaries', () => {
    expect(formatResourceCorrelationPattern(correlation)).toBe('Disk Full → Restart');
    expect(formatResourceCorrelationSummary(correlation)).toBe(
      '2 occurrences · avg delay 5s · 88% confidence',
    );
  });
});
