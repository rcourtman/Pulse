import { describe, expect, it } from 'vitest';

import { normalizeIntelligenceSummary } from '@/stores/aiIntelligenceSummaryModel';

describe('aiIntelligenceSummaryModel', () => {
  it('normalizes recent change count and governed policy posture fallbacks', () => {
    const summary = normalizeIntelligenceSummary({
      timestamp: '2026-03-01T00:00:00Z',
      overall_health: {
        score: 87,
        grade: 'B',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      findings_count: {
        critical: 1,
        warning: 2,
        watch: 0,
        info: 4,
        total: 7,
      },
      predictions_count: 3,
      recent_changes_count: Number.NaN,
      recent_changes: [
        {
          id: 'change-1',
          observedAt: '2026-03-01T00:00:00Z',
          resourceId: 'vm-100',
          kind: 'config_update',
          sourceType: 'pulse_diff',
          confidence: 'high',
        },
      ],
      policy_posture: {
        total_resources: Number.NaN,
        sensitivity_counts: {
          public: Number.NaN,
        },
        routing_counts: {
          'cloud-summary': 2,
          'local-only': 1,
        },
      },
      learning: {
        resources_with_knowledge: 4,
        total_notes: 11,
        resources_with_baselines: 3,
        patterns_detected: 2,
        correlations_learned: 1,
        incidents_tracked: 5,
      },
    });

    expect(summary.recent_changes_count).toBe(1);
    expect(summary.recent_changes).toHaveLength(1);
    expect(summary.policy_posture).toEqual({
      total_resources: 3,
      sensitivity_counts: {},
      routing_counts: {
        'cloud-summary': 2,
        'local-only': 1,
      },
    });
  });

  it('keeps the canonical summary count when the backend already supplies it', () => {
    const summary = normalizeIntelligenceSummary({
      timestamp: '2026-03-01T00:00:00Z',
      overall_health: {
        score: 92,
        grade: 'A',
        trend: 'improving',
        factors: [],
        prediction: 'Improving',
      },
      findings_count: {
        critical: 0,
        warning: 1,
        watch: 0,
        info: 2,
        total: 3,
      },
      predictions_count: 1,
      recent_changes_count: 4,
      recent_changes: [
        {
          id: 'change-2',
          observedAt: '2026-03-01T00:00:00Z',
          resourceId: 'vm-200',
          kind: 'restart',
          sourceType: 'platform_event',
          confidence: 'medium',
        },
      ],
      learning: {
        resources_with_knowledge: 3,
        total_notes: 8,
        resources_with_baselines: 2,
        patterns_detected: 1,
        correlations_learned: 1,
        incidents_tracked: 2,
      },
    });

    expect(summary.recent_changes_count).toBe(4);
    expect(summary.recent_changes).toHaveLength(1);
    expect(summary.policy_posture).toBeUndefined();
  });
});
