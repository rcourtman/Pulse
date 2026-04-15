import { describe, expect, it } from 'vitest';

import {
  getPatrolSupportingContextCorrelationSummary,
  getPatrolSupportingContextToggleLabel,
  PATROL_SUPPORTING_CONTEXT_CHANGE_SUBTITLE,
  PATROL_SUPPORTING_CONTEXT_DESCRIPTION,
  PATROL_SUPPORTING_CONTEXT_EVIDENCE_LABEL,
  PATROL_SUPPORTING_CONTEXT_EVIDENCE_NOTE,
  PATROL_SUPPORTING_CONTEXT_POLICY_SUBTITLE,
  PATROL_SUPPORTING_CONTEXT_TITLE,
} from '../patrolSupportingContextPresentation';

describe('patrolSupportingContextPresentation', () => {
  it('exports the canonical supporting-context trust copy', () => {
    expect(PATROL_SUPPORTING_CONTEXT_TITLE).toBe('Supporting context');
    expect(PATROL_SUPPORTING_CONTEXT_DESCRIPTION).toBe(
      'Recent changes, learned correlations, and policy coverage that may explain findings or incomplete verification.',
    );
    expect(PATROL_SUPPORTING_CONTEXT_EVIDENCE_LABEL).toBe('How to read this');
    expect(PATROL_SUPPORTING_CONTEXT_EVIDENCE_NOTE).toBe(
      'Findings and run history are Patrol verification evidence. The cards below add explanatory context and do not count as a fresh full patrol.',
    );
    expect(PATROL_SUPPORTING_CONTEXT_CHANGE_SUBTITLE).toBe(
      'Observed from the canonical timeline in the last 24 hours.',
    );
    expect(PATROL_SUPPORTING_CONTEXT_POLICY_SUBTITLE).toBe(
      'Coverage posture for policy-covered resources.',
    );
  });

  it('builds canonical supporting-context action labels and correlation summaries', () => {
    expect(getPatrolSupportingContextToggleLabel(false)).toBe('View supporting context');
    expect(getPatrolSupportingContextToggleLabel(true)).toBe('Hide supporting context');
    expect(getPatrolSupportingContextCorrelationSummary(2)).toBe(
      '2 learned patterns · explanatory context',
    );
    expect(getPatrolSupportingContextCorrelationSummary(1)).toBe(
      '1 learned pattern · explanatory context',
    );
    expect(getPatrolSupportingContextCorrelationSummary(Number.NaN)).toBe(
      'Learned pattern context',
    );
  });
});
