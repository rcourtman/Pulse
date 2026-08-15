import { describe, expect, it } from 'vitest';
import type { PatrolObjective } from '@/api/patrol';
import type { AttentionItem } from '@/api/patrolAttention';
import {
  getPatrolAttentionDecisionReason,
  getPatrolObjectiveProtectionSummary,
  partitionPatrolAttention,
} from '../patrolHomePresentation';

const attentionItem = (overrides: Partial<AttentionItem> = {}): AttentionItem => ({
  id: 'attention-1',
  operationalRecordId: 'record-1',
  subjectResourceId: 'docker:host/service/jellyfin',
  subjectResourceName: 'Jellyfin',
  kind: 'availability',
  title: 'Jellyfin playback unavailable',
  plainLanguageSummary: 'Playback health checks are failing.',
  severity: 'warning',
  state: 'open',
  firstObservedAt: '2026-08-14T07:00:00Z',
  lastObservedAt: '2026-08-14T07:05:00Z',
  evidenceFreshness: 'fresh',
  evidenceCompleteness: 'complete',
  relatedResources: [],
  availableActions: [
    {
      targetResourceId: 'docker:host/service/jellyfin',
      capability: 'restart_service',
      kind: 'restart',
      label: 'Restart Jellyfin',
      mode: 'execute',
      risk: 'low',
      approval: 'not-required',
      eligibility: 'eligible',
      reasons: [],
      evidenceIds: ['evidence-1'],
      expectedPostcondition: 'Playback health checks return to normal.',
      verificationPolicy: 'availability',
      requiresApproval: false,
    },
  ],
  verificationState: 'pending',
  ...overrides,
});

const objective = (overrides: Partial<PatrolObjective> = {}): PatrolObjective => ({
  id: 'objective-1',
  brief: 'Keep Jellyfin playback healthy',
  scope: { resource_ids: [] },
  status: 'active',
  coverage: {
    state: 'covered',
    reason_code: 'observer_current',
    summary: 'Watching playback health locally.',
  },
  revision: 1,
  created_at: '2026-08-14T07:00:00Z',
  updated_at: '2026-08-14T07:00:00Z',
  ...overrides,
});

describe('patrol home presentation', () => {
  it('keeps every active issue visible as a decision in watch-only and ask-first modes', () => {
    const item = attentionItem();

    expect(getPatrolAttentionDecisionReason(item, 'monitor')).toMatch(/will not make changes/i);
    expect(getPatrolAttentionDecisionReason(item, 'approval')).toMatch(/requires your approval/i);
    expect(getPatrolAttentionDecisionReason(item, 'full', true)).toMatch(/will not make changes/i);
  });

  it('keeps safe eligible work quiet in assisted and full modes', () => {
    const item = attentionItem();

    expect(getPatrolAttentionDecisionReason(item, 'assisted')).toBeNull();
    expect(getPatrolAttentionDecisionReason(item, 'full')).toBeNull();
    expect(partitionPatrolAttention([item], 'full')).toEqual({ needsUser: [], quiet: [item] });
  });

  it('interrupts for approval, trust gaps, failed verification, and missing actions', () => {
    const approvalRequired = attentionItem({
      availableActions: [
        {
          ...attentionItem().availableActions[0],
          approval: 'required',
          requiresApproval: true,
        },
      ],
    });
    const staleEvidence = attentionItem({ evidenceFreshness: 'stale' });
    const failedVerification = attentionItem({ verificationState: 'failed' });
    const noAction = attentionItem({ availableActions: [] });

    for (const item of [approvalRequired, staleEvidence, failedVerification, noAction]) {
      expect(getPatrolAttentionDecisionReason(item, 'full')).not.toBeNull();
    }
  });

  it('does not interrupt when at least one eligible governed path can proceed', () => {
    const safeAction = attentionItem().availableActions[0];
    const item = attentionItem({
      availableActions: [
        { ...safeAction, capability: 'safe_cleanup' },
        {
          ...safeAction,
          capability: 'expand_disk',
          approval: 'required',
          requiresApproval: true,
        },
      ],
    });

    expect(getPatrolAttentionDecisionReason(item, 'assisted')).toBeNull();
  });

  it('summarizes protected, uncovered, and paused objectives without overstating health', () => {
    expect(getPatrolObjectiveProtectionSummary([])).toMatchObject({
      headline: 'No outcomes protected yet',
      tone: 'neutral',
    });
    expect(getPatrolObjectiveProtectionSummary([objective()])).toMatchObject({
      headline: 'Protecting 1 outcome',
      covered: 1,
      needsCoverage: 0,
      tone: 'success',
    });
    expect(
      getPatrolObjectiveProtectionSummary([
        objective(),
        objective({
          id: 'objective-2',
          coverage: {
            state: 'uncovered',
            reason_code: 'observer_missing',
            summary: 'Monitoring is being prepared.',
          },
        }),
        objective({ id: 'objective-3', status: 'paused' }),
      ]),
    ).toMatchObject({
      headline: '1 of 2 outcomes protected',
      covered: 1,
      needsCoverage: 1,
      paused: 1,
      tone: 'warning',
    });
  });
});
