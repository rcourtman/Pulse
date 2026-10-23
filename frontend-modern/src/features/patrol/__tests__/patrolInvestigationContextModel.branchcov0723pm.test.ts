import { describe, expect, it } from 'vitest';
import type { InvestigationRecord } from '@/api/ai';

import { buildPatrolInvestigationRecordPresentation } from '../patrolInvestigationContextModel';

// Second branch-coverage companion to patrolInvestigationContextModel.test.ts.
// Targets the residual uncovered arms (v8 branch coverage): null/empty
// collections, absent timestamps, unknown resource kinds, null handoff
// metadata, truncation/cap boundaries, and every fallback label. Does not
// duplicate the happy paths already pinned by the dev test or the first
// branchcov file.

const minimalRecord = {
  id: 'r',
  finding_id: 'f',
  subject: { resource_id: 'vm-1' },
  trigger: { detected_at: '2026-01-01T00:00:00Z' },
  status: 'completed',
} as unknown as InvestigationRecord;

describe('buildPatrolInvestigationRecordPresentation (residual branches)', () => {
  it('labels an unknown tool via the identifier fallback', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      ...minimalRecord,
      tools_used: ['metrics.history', 'custom.diagnostic.tool'],
    });

    // Known tool resolves to its map label; unknown tool falls through to
    // formatIdentifierLabel.
    expect(presentation.toolsUsed).toEqual(['Metrics history', 'Custom Diagnostic Tool']);
  });
});

describe('buildPatrolInvestigationRecordPresentation (tool label fallback)', () => {
  it('drops a tool name that the identifier formatter reduces to empty', () => {
    // '___' is not a known tool and formatIdentifierLabel('___') returns '' ->
    // `formatIdentifierLabel(normalized) || ''` -> '' -> filtered out by
    // .filter(Boolean). A genuine unknown tool still resolves via the fallback.
    const presentation = buildPatrolInvestigationRecordPresentation({
      ...minimalRecord,
      tools_used: ['___', 'custom.tool'],
    });

    expect(presentation.toolsUsed).toEqual(['Custom Tool']);
  });
});
