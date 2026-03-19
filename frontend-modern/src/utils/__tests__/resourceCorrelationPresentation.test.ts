import { describe, expect, it } from 'vitest';

import {
  formatResourceCorrelationEndpoint,
  formatResourceCorrelationHeadline,
  formatResourceCorrelationPattern,
  formatResourceCorrelationSummary,
  formatResourceCorrelationSummaryText,
  sortResourceCorrelations,
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

  it('sorts correlations by confidence and recency', () => {
    const sorted = sortResourceCorrelations([
      {
        source_id: 'storage-2',
        source_name: 'Storage 2',
        source_type: 'storage',
        target_id: 'vm-99',
        target_name: 'VM 99',
        target_type: 'vm',
        event_pattern: 'storage -> vm',
        occurrences: 1,
        avg_delay: 1_000_000_000,
        confidence: 0.75,
        last_seen: '2026-03-18T11:00:00Z',
        description: 'Lower-confidence, older correlation',
      } as const,
      {
        source_id: 'storage-1',
        source_name: 'Storage 1',
        source_type: 'storage',
        target_id: 'vm-42',
        target_name: 'VM 42',
        target_type: 'vm',
        event_pattern: 'storage -> vm',
        occurrences: 2,
        avg_delay: 2_000_000_000,
        confidence: 0.75,
        last_seen: '2026-03-18T12:00:00Z',
        description: 'Lower-confidence, newer correlation',
      } as const,
      {
        source_id: 'storage-3',
        source_name: 'Storage 3',
        source_type: 'storage',
        target_id: 'vm-77',
        target_name: 'VM 77',
        target_type: 'vm',
        event_pattern: 'storage -> vm',
        occurrences: 3,
        avg_delay: 3_000_000_000,
        confidence: 0.9,
        last_seen: '2026-03-17T12:00:00Z',
        description: 'Higher-confidence correlation',
      } as const,
    ]);

    expect(sorted.map((item) => item.source_id)).toEqual(['storage-3', 'storage-1', 'storage-2']);
  });

  it('formats canonical correlation summary text', () => {
    expect(
      formatResourceCorrelationSummaryText({
        dependenciesCount: 2,
        dependentsCount: 1,
        correlationsCount: 3,
      }),
    ).toBe('2 dependencies · 1 dependent · 3 correlations');
    expect(
      formatResourceCorrelationSummaryText({
        dependenciesCount: 0,
        dependentsCount: 0,
        correlationsCount: 0,
        summaryText: 'custom summary',
      }),
    ).toBe('custom summary');
  });
});
