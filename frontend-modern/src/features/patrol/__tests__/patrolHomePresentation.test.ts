import { describe, expect, it } from 'vitest';
import type { PatrolObjective } from '@/api/patrol';
import type { AttentionItem, AttentionItemDetail } from '@/api/patrolAttention';
import type { UnifiedFinding } from '@/stores/aiIntelligence';
import {
  getAttentionHandoverClose,
  getLinkedPatrolFindings,
  getPatrolAttentionDecisionReason,
  getPatrolFindingsOnSubject,
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

describe('getAttentionHandoverClose', () => {
  const transition = (
    overrides: Partial<AttentionItemDetail['timeline'][number]>,
  ): AttentionItemDetail['timeline'][number] => ({
    id: 'transition-1',
    operationalRecordId: 'record-1',
    from: 'open',
    to: 'resolved',
    at: '2026-08-14T08:00:00Z',
    cause: 'recovery_evidence',
    causeKey: 'memory:pve1',
    evidenceIds: ['evidence-1'],
    ...overrides,
  });
  const reason =
    'Alert moved to pve1 (Host Agent). This is not a recovery: check the agent for the current reading.';

  it('returns the handover account only for a record closed by an ownership transfer', () => {
    const resolved = attentionItem({ state: 'resolved' });
    expect(
      getAttentionHandoverClose({
        item: resolved,
        timeline: [transition({ cause: 'ownership_transferred', reason })],
      }),
    ).toBe(reason);
    expect(
      getAttentionHandoverClose({
        item: resolved,
        timeline: [transition({ cause: 'ownership_transferred' })],
      }),
    ).toBe('This alert moved to another owner when it closed. It did not recover.');
    expect(getAttentionHandoverClose({ item: resolved, timeline: [transition({})] })).toBeNull();
  });

  it('follows the latest close, so a later recovery or reopen wins', () => {
    const handover = transition({ cause: 'ownership_transferred', reason });
    const reopened = transition({
      id: 'transition-2',
      from: 'resolved',
      to: 'open',
      at: '2026-08-14T09:00:00Z',
      cause: 'detector_decision',
    });
    const recovered = transition({ id: 'transition-3', at: '2026-08-14T10:00:00Z' });
    expect(
      getAttentionHandoverClose({
        item: attentionItem({ state: 'resolved' }),
        timeline: [handover, reopened, recovered],
      }),
    ).toBeNull();
    expect(
      getAttentionHandoverClose({
        item: attentionItem({ state: 'open' }),
        timeline: [handover, reopened],
      }),
    ).toBeNull();
  });
});

describe('getPatrolFindingsOnSubject', () => {
  const finding = (overrides: Partial<UnifiedFinding> = {}): UnifiedFinding => ({
    id: 'finding-1',
    source: 'ai-patrol',
    resourceId: 'app-container-a1bf6ef7c728a8db',
    resourceName: 'pulse-live-web',
    resourceType: 'app-container',
    category: 'reliability',
    severity: 'warning',
    title: 'Web server in container pulse-live-web stopped answering',
    description: 'The health check has been refused for 40 minutes.',
    detectedAt: '2026-10-24T07:00:00Z',
    status: 'active',
    ...overrides,
  });

  it('lists the open findings on the unified resource the alert item names', () => {
    // The backend resolves the alert's own reference ("docker:<host>/<container>")
    // to the unified container the findings carry.
    const item = attentionItem({ subjectResourceId: 'app-container-a1bf6ef7c728a8db' });
    const onSubject = finding();
    const mirroring = finding({
      id: 'finding-2',
      mirrorsAlertId: 'record-1',
      mirrorsAlertType: 'availability',
    });
    const findings = [
      onSubject,
      mirroring,
      finding({ id: 'resolved', status: 'resolved' }),
      finding({ id: 'dismissed', status: 'dismissed', dismissedReason: 'not_an_issue' }),
      finding({ id: 'elsewhere', resourceId: 'app-container-0594de6a76cb71fe' }),
    ];

    expect(getPatrolFindingsOnSubject(item, findings).map(({ id }) => id)).toEqual([
      'finding-1',
      'finding-2',
    ]);
    expect(getLinkedPatrolFindings(item, findings).map(({ id }) => id)).toEqual(['finding-2']);
    expect(getPatrolFindingsOnSubject(attentionItem({ subjectResourceId: ' ' }), findings)).toEqual(
      [],
    );
  });

  it('links a finding only to the alert it restates when alerts share a resource and kind', () => {
    // Two filesystem alerts on one host name the same unified host and kind.
    const data = attentionItem({
      id: 'record-data',
      operationalRecordId: 'record-data',
      subjectResourceId: 'agent-6e63a900bdf8d8b7',
      kind: 'disk',
    });
    const backup = attentionItem({
      id: 'record-backup',
      operationalRecordId: 'record-backup',
      subjectResourceId: 'agent-6e63a900bdf8d8b7',
      kind: 'disk',
    });
    const backupFull = finding({
      id: 'backup-full',
      resourceId: 'agent-6e63a900bdf8d8b7',
      category: 'capacity',
      title: 'Disk /backup is 92% full',
      mirrorsAlertId: 'record-backup',
      mirrorsAlertType: 'disk',
    });

    expect(getLinkedPatrolFindings(backup, [backupFull]).map(({ id }) => id)).toEqual([
      'backup-full',
    ]);
    expect(getLinkedPatrolFindings(data, [backupFull])).toEqual([]);
    // The other filesystem's finding is still an open finding on the host.
    expect(getPatrolFindingsOnSubject(data, [backupFull]).map(({ id }) => id)).toEqual([
      'backup-full',
    ]);
  });
});
