import { describe, expect, it } from 'vitest';

import type { UnifiedFinding } from '@/stores/aiIntelligence';
import {
  getFindingResourceCriticalitySortOrder,
  sortFindingsForAttentionQueue,
} from '@/utils/aiFindingPresentation';

function makeFinding(overrides: Partial<UnifiedFinding>): UnifiedFinding {
  return {
    id: overrides.id ?? 'finding',
    source: 'ai-patrol',
    resourceId: overrides.resourceId ?? 'vm:101',
    resourceName: overrides.resourceName ?? 'db-primary',
    resourceType: overrides.resourceType ?? 'vm',
    category: 'performance',
    severity: overrides.severity ?? 'warning',
    title: overrides.title ?? 'CPU saturated',
    description: overrides.description ?? 'CPU is high',
    detectedAt: overrides.detectedAt ?? '2026-06-30T08:00:00Z',
    lastSeenAt: overrides.lastSeenAt ?? overrides.detectedAt ?? '2026-06-30T08:00:00Z',
    status: overrides.status ?? 'active',
    investigationOutcome: overrides.investigationOutcome ?? 'fix_failed',
    ...overrides,
  };
}

describe('getFindingResourceCriticalitySortOrder', () => {
  it('orders explicit resource priority around the default posture', () => {
    expect(getFindingResourceCriticalitySortOrder('high')).toBeLessThan(
      getFindingResourceCriticalitySortOrder('medium'),
    );
    expect(getFindingResourceCriticalitySortOrder('medium')).toBeLessThan(
      getFindingResourceCriticalitySortOrder(undefined),
    );
    expect(getFindingResourceCriticalitySortOrder(undefined)).toBeLessThan(
      getFindingResourceCriticalitySortOrder('low'),
    );
  });
});

describe('sortFindingsForAttentionQueue', () => {
  it('uses resource criticality before recency for same-severity findings', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({
        id: 'low-newest',
        resourceCriticality: 'low',
        lastSeenAt: '2026-06-30T08:30:00Z',
      }),
      makeFinding({
        id: 'default-middle',
        lastSeenAt: '2026-06-30T08:20:00Z',
      }),
      makeFinding({
        id: 'high-oldest',
        resourceCriticality: 'high',
        lastSeenAt: '2026-06-30T08:10:00Z',
      }),
    ]);

    expect(sorted.map((finding) => finding.id)).toEqual([
      'high-oldest',
      'default-middle',
      'low-newest',
    ]);
  });

  it('does not allow resource criticality to outrank severity', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({
        id: 'warning-high',
        severity: 'warning',
        resourceCriticality: 'high',
      }),
      makeFinding({
        id: 'critical-low',
        severity: 'critical',
        resourceCriticality: 'low',
      }),
    ]);

    expect(sorted.map((finding) => finding.id)).toEqual(['critical-low', 'warning-high']);
  });
});
