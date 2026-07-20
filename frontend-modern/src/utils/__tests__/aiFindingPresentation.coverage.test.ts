import { describe, expect, it } from 'vitest';

import type { UnifiedFinding } from '@/stores/aiIntelligence';
import {
  doesFindingNeedAttention,
  getInvestigationConfidenceBadgeClasses,
  getInvestigationConfidenceBadgeTone,
  getInvestigationOutcomeSortOrder,
  hasFindingInvestigationHandoffPointer,
} from '@/utils/aiFindingPresentation';

// Mirrors the module-private fallback so the test asserts the real fallback
// behaviour rather than echoing a mock. These literals are the contract.
const DEFAULT_BADGE_CLASSES = 'border-border bg-surface-alt text-muted';

describe('getInvestigationConfidenceBadgeClasses', () => {
  it('returns the high-confidence (emerald) tier classes', () => {
    expect(getInvestigationConfidenceBadgeClasses('high')).toBe(
      'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
    );
  });

  it('returns the medium-confidence (neutral surface) tier classes', () => {
    expect(getInvestigationConfidenceBadgeClasses('medium')).toBe(
      'border-border bg-surface-alt text-base-content',
    );
  });

  it('returns the low-confidence (amber) tier classes', () => {
    expect(getInvestigationConfidenceBadgeClasses('low')).toBe(
      'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
    );
  });

  it('falls back to the default classes for an unrecognised confidence', () => {
    expect(getInvestigationConfidenceBadgeClasses('critical')).toBe(DEFAULT_BADGE_CLASSES);
  });

  it('falls back to the default classes for the empty string', () => {
    expect(getInvestigationConfidenceBadgeClasses('')).toBe(DEFAULT_BADGE_CLASSES);
  });

  it('distinguishes the medium tier from the unknown fallback by content', () => {
    // Medium keeps base-content text; the unknown fallback uses text-muted.
    expect(getInvestigationConfidenceBadgeClasses('medium')).toContain('text-base-content');
    expect(getInvestigationConfidenceBadgeClasses('unknown')).toContain('text-muted');
  });
});

describe('getInvestigationConfidenceBadgeTone (confidence tone map miss)', () => {
  it('maps each known confidence to its tone', () => {
    expect(getInvestigationConfidenceBadgeTone('high')).toBe('success');
    expect(getInvestigationConfidenceBadgeTone('medium')).toBe('neutral');
    expect(getInvestigationConfidenceBadgeTone('low')).toBe('warning');
  });

  it('falls back to the muted tone for an unknown confidence', () => {
    expect(getInvestigationConfidenceBadgeTone('very-sure')).toBe('muted');
  });

  it('falls back to the muted tone for the empty string', () => {
    expect(getInvestigationConfidenceBadgeTone('')).toBe('muted');
  });
});

describe('hasFindingInvestigationHandoffPointer', () => {
  const base = {
    investigationOutcome: undefined as UnifiedFinding['investigationOutcome'],
    investigationSessionId: undefined as string | undefined,
    lastInvestigatedAt: undefined as string | undefined,
  };

  it('is true when an investigation outcome is recorded', () => {
    expect(
      hasFindingInvestigationHandoffPointer({ ...base, investigationOutcome: 'fix_failed' }),
    ).toBe(true);
  });

  it('is true when a session id is present even without an outcome', () => {
    expect(
      hasFindingInvestigationHandoffPointer({ ...base, investigationSessionId: 'sess-123' }),
    ).toBe(true);
  });

  it('is true when only a last-investigated timestamp is present', () => {
    expect(
      hasFindingInvestigationHandoffPointer({
        ...base,
        lastInvestigatedAt: '2026-07-01T00:00:00Z',
      }),
    ).toBe(true);
  });

  it('is false when no handoff field is set', () => {
    expect(hasFindingInvestigationHandoffPointer(base)).toBe(false);
  });

  it('is false when every handoff field is an empty string rather than absent', () => {
    expect(
      hasFindingInvestigationHandoffPointer({
        investigationOutcome: '' as unknown as UnifiedFinding['investigationOutcome'],
        investigationSessionId: '',
        lastInvestigatedAt: '',
      }),
    ).toBe(false);
  });
});

describe('getInvestigationOutcomeSortOrder', () => {
  it.each([
    ['fix_verification_failed', 0],
    ['fix_failed', 0],
    ['fix_verification_unknown', 1],
    ['fix_rejected', 1],
    ['timed_out', 1],
    ['needs_attention', 1],
    ['cannot_fix', 1],
    ['fix_queued', 2],
  ] as const)('ranks outcome %s at %i', (outcome, expected) => {
    expect(getInvestigationOutcomeSortOrder(outcome)).toBe(expected);
  });

  it('defaults positive-track outcomes that are intentionally unmapped to 3', () => {
    expect(getInvestigationOutcomeSortOrder('resolved')).toBe(3);
    expect(getInvestigationOutcomeSortOrder('fix_executed')).toBe(3);
    expect(getInvestigationOutcomeSortOrder('fix_verified')).toBe(3);
  });

  it('defaults an unrecognised outcome to 3', () => {
    expect(getInvestigationOutcomeSortOrder('not-a-real-outcome')).toBe(3);
  });

  it('returns 3 for undefined (no outcome)', () => {
    expect(getInvestigationOutcomeSortOrder(undefined)).toBe(3);
  });

  it('returns 3 for the empty string (treated as no outcome)', () => {
    expect(getInvestigationOutcomeSortOrder('')).toBe(3);
  });

  it('ranks a failed verification before a queued fix', () => {
    expect(getInvestigationOutcomeSortOrder('fix_verification_failed')).toBeLessThan(
      getInvestigationOutcomeSortOrder('fix_queued'),
    );
  });
});

describe('doesFindingNeedAttention (ATTENTION_OUTCOMES set membership)', () => {
  const attentionOutcomes = [
    'fix_verification_failed',
    'fix_verification_unknown',
    'fix_failed',
    'fix_rejected',
    'timed_out',
    'needs_attention',
    'cannot_fix',
  ] as const;

  it.each(attentionOutcomes)(
    'flags an active finding with outcome %s as needing attention',
    (outcome) => {
      expect(
        doesFindingNeedAttention({
          id: 'f1',
          status: 'active',
          investigationOutcome: outcome,
        }),
      ).toBe(true);
    },
  );

  it('does not flag a non-attention outcome that is not awaiting approval', () => {
    // fix_executed is positive progress, not in the attention set.
    expect(
      doesFindingNeedAttention({
        id: 'f2',
        status: 'active',
        investigationOutcome: 'fix_executed',
      }),
    ).toBe(false);
  });

  it('does not flag resolved/verified outcomes', () => {
    expect(
      doesFindingNeedAttention({
        id: 'f3',
        status: 'active',
        investigationOutcome: 'fix_verified',
      }),
    ).toBe(false);
    expect(
      doesFindingNeedAttention({
        id: 'f4',
        status: 'active',
        investigationOutcome: 'resolved',
      }),
    ).toBe(false);
  });

  it('returns false when the finding is not active, regardless of outcome', () => {
    expect(
      doesFindingNeedAttention({
        id: 'f5',
        status: 'resolved',
        investigationOutcome: 'fix_failed',
      }),
    ).toBe(false);
  });

  it('returns false when an active finding has no investigation outcome', () => {
    expect(
      doesFindingNeedAttention({
        id: 'f6',
        status: 'active',
        investigationOutcome: undefined,
      }),
    ).toBe(false);
  });

  it('flags a queued fix that has no live approval', () => {
    expect(
      doesFindingNeedAttention(
        { id: 'f7', status: 'active', investigationOutcome: 'fix_queued' },
        [],
      ),
    ).toBe(true);
  });

  it('does not flag a queued fix that still has a live pending approval', () => {
    const now = Date.now();
    const approval = {
      status: 'pending' as const,
      toolId: 'investigation_fix',
      targetId: 'f7',
      expiresAt: new Date(now + 60_000).toISOString(),
    };
    expect(
      doesFindingNeedAttention({ id: 'f7', status: 'active', investigationOutcome: 'fix_queued' }, [
        approval,
      ]),
    ).toBe(false);
  });
});
