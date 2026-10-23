import { describe, expect, it } from 'vitest';
import type { InvestigationRecord } from '@/api/ai';

import { buildPatrolInvestigationRecordPresentation } from '../patrolInvestigationContextModel';

// Branch-coverage companion to patrolInvestigationContextModel.test.ts.
// These cases target the residual uncovered arms (empty/missing collections,
// null/undefined optionals, error/failure states, alternate enum/variant
// arms) identified via V8 branch coverage. They do not duplicate the happy
// paths already pinned by the dev test.

describe('buildPatrolInvestigationRecordPresentation (branch coverage)', () => {
  it('returns the empty presentation for null and undefined records', () => {
    const empty = {
      hasRecord: false,
      statusLabel: '',
      evidenceSummaries: [],
      verificationSummaries: [],
      rollbackSummaries: [],
      toolsUsed: [],
    };
    expect(buildPatrolInvestigationRecordPresentation(null)).toEqual(empty);
    expect(buildPatrolInvestigationRecordPresentation(undefined)).toEqual(empty);
  });

  it('falls back to the default status label and omits optionals on a minimal record', () => {
    const record = {
      id: 'r-min',
      finding_id: 'f-min',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: '',
    } as unknown as InvestigationRecord;

    expect(buildPatrolInvestigationRecordPresentation(record)).toEqual({
      hasRecord: true,
      statusLabel: 'Investigation recorded',
      outcomeLabel: undefined,
      confidenceLabel: undefined,
      conclusion: '',
      impact: '',
      recommendedAction: '',
      evidenceSummaries: [],
      verificationSummaries: [],
      rollbackSummaries: [],
      toolsUsed: [],
      proposedFix: undefined,
      error: '',
    });
  });

  it('reads evidence summary, kind, and id fallbacks in priority order', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-ev',
      finding_id: 'f-ev',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'completed',
      evidence: [{ kind: 'metrics' }, { id: 'ev-2' }, { summary: 'has summary' }],
    } as unknown as InvestigationRecord);

    expect(presentation.evidenceSummaries).toEqual(['metrics', 'ev-2', 'has summary']);
  });

  it('keeps a proposed fix that has only a command summary but no description', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-fix-cmd',
      finding_id: 'f-fix-cmd',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'completed',
      proposed_fix: {
        id: 'fix-cmd',
        description: '',
        commands: ['c1', 'c2'],
        destructive: false,
      },
    } as unknown as InvestigationRecord);

    expect(presentation.proposedFix).toMatchObject({
      description: '',
      commandSummary: '2 commands recorded for approval context',
      destructive: false,
    });
  });

  it('keeps a proposed fix that has only a rationale but no description', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-fix-rat',
      finding_id: 'f-fix-rat',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'completed',
      proposed_fix: {
        id: 'fix-rat',
        description: '',
        rationale: 'because',
        destructive: false,
      },
    } as unknown as InvestigationRecord);

    expect(presentation.proposedFix).toMatchObject({
      rationale: 'because',
      commandSummary: undefined,
    });
  });

  it('drops a proposed fix that carries only a risk label (no actionable text)', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-fix-risk',
      finding_id: 'f-fix-risk',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'completed',
      proposed_fix: {
        id: 'fix-risk',
        description: '',
        risk_level: 'high',
        destructive: false,
      },
    } as unknown as InvestigationRecord);

    expect(presentation.proposedFix).toBeUndefined();
  });

  it('words a single recorded command in the singular', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-fix-one',
      finding_id: 'f-fix-one',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'completed',
      proposed_fix: { id: 'fix-one', description: '', commands: ['c1'], destructive: true },
    } as unknown as InvestigationRecord);

    expect(presentation.proposedFix).toMatchObject({
      commandSummary: '1 command recorded for approval context',
      destructive: true,
    });
  });

  it('drops a proposed fix with no description, commands, rationale, or risk', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-fix-empty',
      finding_id: 'f-fix-empty',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'completed',
      proposed_fix: { id: 'fix-empty', description: '  ', commands: [], destructive: false },
    } as unknown as InvestigationRecord);

    expect(presentation.proposedFix).toBeUndefined();
  });

  it('surfaces the raw investigation error text when the backend records one', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'r-err',
      finding_id: 'f-err',
      subject: { resource_id: 'vm-1' },
      trigger: { detected_at: '2026-01-01T00:00:00Z' },
      status: 'failed',
      error: 'investigation crashed',
    } as unknown as InvestigationRecord);

    expect(presentation.error).toBe('investigation crashed');
  });
});
