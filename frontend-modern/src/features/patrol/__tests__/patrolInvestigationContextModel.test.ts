import { describe, expect, it } from 'vitest';

import { buildPatrolInvestigationContextSummary } from '../patrolInvestigationContextModel';

describe('patrolInvestigationContextModel', () => {
  it('builds the canonical investigation context summary from recent changes, correlations, and policy posture', () => {
    expect(
      buildPatrolInvestigationContextSummary({
        recentChangesCount: 1,
        correlations: {
          count: 2,
          correlations: [],
        },
        policyPosture: {
          total_resources: 4,
          sensitivity_counts: {},
          routing_counts: {},
        },
      }),
    ).toEqual({
      recentChangeCount: 1,
      correlationCount: 2,
      governedResourceCount: 4,
      hasContext: true,
      summaryText: '1 recent change · 2 correlations · 4 governed resources',
    });
  });

  it('falls back to correlation list length and suppresses empty context parts', () => {
    expect(
      buildPatrolInvestigationContextSummary({
        recentChangesCount: 0,
        correlations: {
          correlations: [
            {
              source_id: 'a',
              source_name: 'A',
              source_type: 'host',
              target_id: 'b',
              target_name: 'B',
              target_type: 'vm',
              event_pattern: 'cpu -> restart',
              occurrences: 1,
              avg_delay: 30,
              confidence: 0.7,
              last_seen: '2026-03-01T00:00:00Z',
              description: 'desc',
            },
          ],
          count: Number.NaN,
        },
        policyPosture: {
          total_resources: 0,
          sensitivity_counts: {},
          routing_counts: {},
        },
      }),
    ).toEqual({
      recentChangeCount: 0,
      correlationCount: 1,
      governedResourceCount: 0,
      hasContext: true,
      summaryText: '1 correlation',
    });
  });

  it('returns an empty context summary when no secondary investigation signals exist', () => {
    expect(
      buildPatrolInvestigationContextSummary({
        recentChangesCount: undefined,
        correlations: null,
        policyPosture: null,
      }),
    ).toEqual({
      recentChangeCount: 0,
      correlationCount: 0,
      governedResourceCount: 0,
      hasContext: false,
      summaryText: '',
    });
  });
});
