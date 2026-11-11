import { describe, expect, it } from 'vitest';
import type { InvestigationRecord } from '@/api/ai';
import type { AttentionItem } from '@/api/patrolAttention';
import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type { PatrolActionReference } from '@/types/actionAudit';
import { getPatrolAttentionFindingRead } from '../patrolHomePresentation';

const item = {
  id: 'attention-1',
  operationalRecordId: 'alert-1',
  subjectResourceId: 'app-container-4c434064ff5fc355',
  subjectResourceName: 'nextcloud',
} as AttentionItem;

const finding = (overrides: Partial<UnifiedFinding> = {}): UnifiedFinding => ({
  id: 'finding-1',
  source: 'ai-patrol',
  resourceId: 'app-container-4c434064ff5fc355',
  resourceName: 'nextcloud',
  resourceType: 'app-container',
  category: 'reliability',
  severity: 'critical',
  title: 'Nextcloud is failing its health check',
  description: 'First look: the app may be hung.',
  detectedAt: '2026-11-11T21:22:00Z',
  status: 'active',
  ...overrides,
});

const action = (state: PatrolActionReference['state']): PatrolActionReference =>
  ({
    action_id: 'act-1',
    resource_id: 'app-container-e4d4620ea8858185',
    capability_name: 'start',
    state,
    plan: { actionId: 'act-1' },
  }) as PatrolActionReference;

const record = (overrides: Partial<InvestigationRecord> = {}): InvestigationRecord => ({
  id: 'record-1',
  finding_id: 'finding-1',
  subject: { resource_id: 'app-container-4c434064ff5fc355' },
  trigger: { detected_at: '2026-11-11T21:22:00Z' },
  status: 'completed',
  evidence: [],
  verification: [],
  rollback: [],
  tools_used: [],
  started_at: '2026-11-11T21:22:30Z',
  conclusion: '### Conclusion\nNEEDS_ATTENTION: nextcloud-db is stopped. Start it.',
  ...overrides,
});

describe('getPatrolAttentionFindingRead', () => {
  it('says nothing when Patrol has no open finding on the item', () => {
    expect(getPatrolAttentionFindingRead(item, [])).toBeNull();
    expect(
      getPatrolAttentionFindingRead(item, [finding({ resourceId: 'app-container-other' })]),
    ).toBeNull();
    expect(getPatrolAttentionFindingRead(item, [finding({ status: 'resolved' })])).toBeNull();
    expect(
      getPatrolAttentionFindingRead(item, [finding({ dismissedReason: 'expected_behavior' })]),
    ).toBeNull();
  });

  it('reads the first look for a finding nobody investigated', () => {
    expect(getPatrolAttentionFindingRead(item, [finding()])).toMatchObject({
      state: 'found',
      label: 'Patrol finding',
      text: 'First look: the app may be hung.',
    });
  });

  it('says an investigation is running instead of repeating the first look', () => {
    const read = getPatrolAttentionFindingRead(item, [finding({ investigationStatus: 'running' })]);
    expect(read).toMatchObject({ state: 'investigating', label: 'Patrol is investigating' });
    expect(read?.text).not.toContain('First look');
  });

  it("leads with the investigation's conclusion once it has one", () => {
    const read = getPatrolAttentionFindingRead(item, [
      finding({ investigationStatus: 'completed', investigationRecord: record() }),
    ]);
    expect(read).toMatchObject({
      state: 'concluded',
      text: 'nextcloud-db is stopped. Start it.',
    });
    expect(read?.action).toBeUndefined();
  });

  it('carries a fix that waits on a person, with a label that says which step', () => {
    const pending = getPatrolAttentionFindingRead(item, [
      finding({ investigationRecord: record({ action: action('pending_approval') }) }),
    ]);
    expect(pending).toMatchObject({ state: 'action_waiting', label: 'Fix ready to approve' });
    expect(pending?.action?.action_id).toBe('act-1');

    expect(
      getPatrolAttentionFindingRead(item, [
        finding({ investigationRecord: record({ action: action('approved') }) }),
      ]),
    ).toMatchObject({ state: 'action_waiting', label: 'Fix ready to run' });
  });

  it('does not put the first look beside a fix it did not lead to', () => {
    // The action is planned before the investigation writes its conclusion.
    const read = getPatrolAttentionFindingRead(item, [
      finding({
        investigationStatus: 'running',
        investigationRecord: record({
          conclusion: 'First look: the app may be hung.',
          action: action('pending_approval'),
        }),
      }),
    ]);
    expect(read).toMatchObject({ state: 'action_waiting', label: 'Fix ready to approve' });
    expect(read?.text).toBe('Patrol has prepared a fix. The review says what it changes and why.');
  });

  it('offers no action once it ran, expired or was rejected', () => {
    for (const state of ['completed', 'failed', 'expired', 'rejected', 'executing'] as const) {
      const read = getPatrolAttentionFindingRead(item, [
        finding({ investigationRecord: record({ action: action(state) }) }),
      ]);
      expect(read?.state).toBe('concluded');
      expect(read?.action).toBeUndefined();
    }
  });

  it('puts the finding with a waiting fix ahead of the others on the resource', () => {
    const read = getPatrolAttentionFindingRead(item, [
      finding({ id: 'finding-a' }),
      finding({
        id: 'finding-b',
        investigationRecord: record({ action: action('pending_approval') }),
      }),
    ]);
    expect(read?.finding.id).toBe('finding-b');
  });

  it('counts a finding that restates the alert once, even when it is also on the subject', () => {
    const read = getPatrolAttentionFindingRead(item, [finding({ mirrorsAlertId: 'alert-1' })]);
    expect(read?.finding.id).toBe('finding-1');
  });
});
