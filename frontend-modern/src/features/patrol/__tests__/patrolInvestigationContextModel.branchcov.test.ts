import { describe, expect, it } from 'vitest';
import type { ApprovalRequest, InvestigationRecord } from '@/api/ai';
import type { ResourceCorrelation } from '@/types/aiIntelligence';
import type { PatrolRunRecord } from '@/api/patrol';
import type { UnifiedFinding } from '@/stores/aiIntelligence';

import {
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantFindingHandoffActions,
  buildPatrolAssistantFindingHandoffFromUnifiedFinding,
  buildPatrolAssistantFindingHandoffInputFromUnifiedFinding,
  buildPatrolAssistantProposedFixBriefingInput,
  buildPatrolAssistantProposedFixBriefingInputFromApproval,
  buildPatrolConfigurationFailureHandoff,
  buildPatrolInvestigationContextSummary,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRunAssistantHandoff,
} from '../patrolInvestigationContextModel';

// Branch-coverage companion to patrolInvestigationContextModel.test.ts.
// These cases target the residual uncovered arms (empty/missing collections,
// null/undefined optionals, error/failure states, alternate enum/variant
// arms) identified via V8 branch coverage. They do not duplicate the happy
// paths already pinned by the dev test.

describe('buildPatrolInvestigationContextSummary (branch coverage)', () => {
  it('uses the plural recent-change and singular policy-resource wording', () => {
    expect(
      buildPatrolInvestigationContextSummary({
        recentChangesCount: 2,
        correlations: null,
        policyPosture: {
          total_resources: 1,
          sensitivity_counts: {},
          routing_counts: {},
        },
      }),
    ).toEqual({
      recentChangeCount: 2,
      correlationCount: 0,
      governedResourceCount: 1,
      hasContext: true,
      summaryText: '2 recent changes · 1 policy-covered resource',
    });
  });

  it('clamps non-finite and negative counts to zero with no context parts', () => {
    expect(
      buildPatrolInvestigationContextSummary({
        recentChangesCount: Number.NEGATIVE_INFINITY,
        correlations: { count: -3, correlations: [] },
        policyPosture: {
          total_resources: Number.NaN,
          sensitivity_counts: {},
          routing_counts: {},
        },
      }),
    ).toEqual({
      recentChangeCount: 0,
      correlationCount: 0,
      governedResourceCount: 0,
      hasContext: false,
      summaryText: '',
    });
  });

  it('clamps a non-finite correlation count to the list length fallback', () => {
    expect(
      buildPatrolInvestigationContextSummary({
        correlations: {
          count: Number.POSITIVE_INFINITY,
          correlations: [
            {
              source_id: 's',
              target_id: 't',
              event_pattern: 'p',
              occurrences: 1,
            } as unknown as ResourceCorrelation,
          ],
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
});

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

describe('buildPatrolAssistantProposedFixBriefingInput (branch coverage)', () => {
  it('returns undefined for null and undefined sources', () => {
    expect(buildPatrolAssistantProposedFixBriefingInput(null)).toBeUndefined();
    expect(buildPatrolAssistantProposedFixBriefingInput(undefined)).toBeUndefined();
  });

  it('reads an explicit numeric commandCount and a null destructive flag', () => {
    expect(
      buildPatrolAssistantProposedFixBriefingInput({
        description: 'd',
        commandCount: 3,
        destructive: undefined,
      }),
    ).toEqual({
      description: 'd',
      riskLevel: '',
      targetHost: '',
      rationale: '',
      commandCount: 3,
      destructive: null,
    });
  });

  it('treats a source with neither commandCount nor commands as zero commands', () => {
    expect(buildPatrolAssistantProposedFixBriefingInput({ description: 'd' })).toMatchObject({
      commandCount: 0,
    });
  });

  it('returns undefined when every field is empty and the fix is not destructive', () => {
    expect(
      buildPatrolAssistantProposedFixBriefingInput({
        description: '',
        riskLevel: '',
        targetHost: '',
        rationale: '',
        commands: [],
        destructive: false,
      }),
    ).toBeUndefined();
  });

  it('keeps a fix whose only signal is that it is destructive', () => {
    expect(buildPatrolAssistantProposedFixBriefingInput({ destructive: true })).toEqual({
      description: '',
      riskLevel: '',
      targetHost: '',
      rationale: '',
      commandCount: 0,
      destructive: true,
    });
  });

  it('falls back to snake_case risk_level and target_host fields', () => {
    expect(
      buildPatrolAssistantProposedFixBriefingInput({
        risk_level: 'high',
        target_host: 'node-1',
      }),
    ).toMatchObject({ riskLevel: 'high', targetHost: 'node-1' });
  });
});

describe('buildPatrolAssistantProposedFixBriefingInputFromApproval (branch coverage)', () => {
  it('derives a one-command fix from an approval carrying a command', () => {
    const briefing = buildPatrolAssistantProposedFixBriefingInputFromApproval({
      context: 'Restart nginx',
      riskLevel: 'high',
      targetName: 'node-1',
      command: 'systemctl restart nginx',
    } as unknown as ApprovalRequest);

    expect(briefing).toMatchObject({
      description: 'Restart nginx',
      riskLevel: 'high',
      targetHost: 'node-1',
      commandCount: 1,
    });
    expect(JSON.stringify(briefing)).not.toContain('systemctl restart nginx');
  });

  it('records zero commands when the approval has no command', () => {
    expect(
      buildPatrolAssistantProposedFixBriefingInputFromApproval({
        context: 'Plan only',
        targetName: 't1',
      } as unknown as ApprovalRequest),
    ).toMatchObject({ description: 'Plan only', commandCount: 0 });
  });

  it('returns undefined for a null approval', () => {
    expect(buildPatrolAssistantProposedFixBriefingInputFromApproval(null)).toBeUndefined();
  });
});

describe('buildPatrolAssistantApprovalBriefingInput (branch coverage)', () => {
  it('returns undefined for null and undefined approvals', () => {
    expect(buildPatrolAssistantApprovalBriefingInput(null)).toBeUndefined();
    expect(buildPatrolAssistantApprovalBriefingInput(undefined)).toBeUndefined();
  });

  it('reads the plan summary and preflight posture when message/intendedChange are absent', () => {
    const briefing = buildPatrolAssistantApprovalBriefingInput({
      id: 'ap1',
      status: 'pending',
      riskLevel: 'high',
      requestedAt: 't1',
      expiresAt: 't2',
      targetName: 'node-1',
      requestedBy: 'pulse_patrol',
      plan: { summary: 'plan summary here' },
      preflight: { intendedChange: 'restart', dryRunSummary: 'dry ok' },
    } as unknown as ApprovalRequest);

    expect(briefing).toMatchObject({
      id: 'ap1',
      actionId: '',
      actionApprovalPolicy: '',
      actionPlanExpiresAt: '',
      actionPlanMessage: 'plan summary here',
      actionPreflight: 'restart',
      actionDryRunSummary: 'dry ok',
      actionRequestedBy: 'pulse_patrol',
    });
  });
});

describe('buildPatrolAssistantFindingHandoffActions (branch coverage)', () => {
  it('marks a proposed-fix-only finding as not requiring approval', () => {
    const actions = buildPatrolAssistantFindingHandoffActions({
      id: 'f-fix-only',
      title: 't',
      resourceId: 'r1',
      resourceName: 'n1',
      resourceType: 'vm',
      proposedFix: {
        description: 'Restart',
        commandCount: 1,
        destructive: false,
        riskLevel: 'low',
        targetHost: 'h1',
      },
    });

    expect(actions).toHaveLength(1);
    expect(actions[0]).toMatchObject({
      approvalId: undefined,
      fixId: undefined,
      description: 'Restart',
      riskLevel: 'low',
      targetHost: 'h1',
      destructive: false,
      actionRequiresApproval: false,
    });
  });

  it('reads fix id, risk, host, and subject from a record proposed_fix', () => {
    const actions = buildPatrolAssistantFindingHandoffActions({
      id: 'f-rec-fix',
      resourceId: 'r1',
      resourceName: 'n1',
      resourceType: 'vm',
      investigationRecord: {
        id: 'rec1',
        finding_id: 'f-rec-fix',
        subject: {
          resource_id: 'r1',
          resource_name: 'n1',
          resource_type: 'vm',
          node: 'pve-1',
        },
        proposed_fix: {
          id: 'fix1',
          description: 'Restart svc',
          risk_level: 'high',
          target_host: 'h1',
          destructive: true,
          commands: ['c'],
        },
      } as unknown as InvestigationRecord,
    });

    expect(actions[0]).toMatchObject({
      recordId: 'rec1',
      fixId: 'fix1',
      description: 'Restart svc',
      riskLevel: 'high',
      targetHost: 'h1',
      destructive: true,
      targetResourceId: 'r1',
      targetResourceName: 'n1',
      targetResourceType: 'vm',
      targetNode: 'pve-1',
      actionRequiresApproval: false,
    });
  });

  it('does not treat an explicit "none" approval policy as requiring approval', () => {
    const actions = buildPatrolAssistantFindingHandoffActions({
      id: 'f-none-policy',
      title: 't',
      resourceId: 'r1',
      pendingApproval: {
        actionId: 'act-1',
        actionApprovalPolicy: 'none',
      },
      proposedFix: { description: 'Restart' },
    });

    expect(actions[0]).toMatchObject({
      actionId: 'act-1',
      actionApprovalPolicy: 'none',
      actionRequiresApproval: false,
    });
  });
});

describe('buildPatrolAssistantFindingHandoff (branch coverage)', () => {
  it('omits resource targeting when no resource can be resolved', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-nores',
      title: 'T',
      subject: 'S',
    });

    expect(handoff.context.targetType).toBeUndefined();
    expect(handoff.context.targetId).toBeUndefined();
    expect(handoff.context.handoffResources).toBeUndefined();
    expect(handoff.context.findingId).toBe('f-nores');
    expect(handoff.context.briefing).toBeUndefined();
  });

  it('derives the finding id from the investigation record when the input id is empty', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: '',
      title: 'T',
      subject: 'S',
      investigationRecord: {
        id: 'rec1',
        finding_id: 'rec-fid',
        subject: { resource_id: '' },
      } as unknown as InvestigationRecord,
    });

    expect(handoff.context.findingId).toBe('rec-fid');
    expect(handoff.context.context?.investigationRecordId).toBe('rec1');
    expect(handoff.context.handoffResources).toBeUndefined();
  });
});

describe('buildPatrolAssistantFindingHandoffInputFromUnifiedFinding (branch coverage)', () => {
  const unified = {
    id: 'uf-1',
    source: 'ai-patrol',
    resourceId: 'vm-1',
    resourceName: 'web',
    resourceType: 'vm',
    severity: 'critical',
    status: 'active',
    title: 'High CPU',
    description: 'desc',
    detectedAt: '2026-01-01T00:00:00Z',
    lastSeenAt: '2026-01-02T00:00:00Z',
    investigationStatus: 'completed',
    investigationOutcome: 'fix_queued',
    loopState: 'awaiting_approval',
    timesRaised: 2,
    regressionCount: 1,
    lastRegressionAt: '2026-01-01T00:00:00Z',
    remediationPlanId: 'rem-1',
  } as unknown as UnifiedFinding;

  it('maps unified finding presentation fields onto the handoff input', () => {
    const input = buildPatrolAssistantFindingHandoffInputFromUnifiedFinding(unified);
    expect(input).toMatchObject({
      id: 'uf-1',
      title: 'High CPU',
      subject: 'web (vm)',
      severity: 'critical',
      findingStatus: 'active',
      investigationStatus: 'completed',
      investigationOutcome: 'fix_queued',
      loopState: 'awaiting_approval',
      timesRaised: 2,
      regressionCount: 1,
      remediationId: 'rem-1',
      resourceId: 'vm-1',
      resourceName: 'web',
      resourceType: 'vm',
    });
  });

  it('threads approval and proposed-fix options through to the handoff', () => {
    const handoff = buildPatrolAssistantFindingHandoffFromUnifiedFinding(unified, {
      pendingApproval: { id: 'a1', status: 'pending' },
      proposedFix: { description: 'Restart', commandCount: 1 },
    });

    expect(handoff.context).toMatchObject({
      targetType: 'vm',
      targetId: 'vm-1',
      findingId: 'uf-1',
    });
    expect(handoff.context.context).toMatchObject({ pendingApprovalId: 'a1' });
    expect(handoff.context.handoffResources?.[0]).toMatchObject({
      id: 'vm-1',
      name: 'web',
      type: 'vm',
    });
    expect(handoff.context.handoffActions).toHaveLength(1);
  });
});

describe('buildPatrolConfigurationFailureHandoff (branch coverage)', () => {
  it('falls back to the default message and bare subject when message and code are empty', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({ message: '' });
    expect(handoff.context.briefing?.subject).toBe('Patrol mode could not be saved.');
    expect(handoff.context.briefing?.title).toBe('Patrol mode save failure attached');
    expect(handoff.context.briefing?.statusLabel).toBeUndefined();
    expect(handoff.context.briefing?.detailLines).toEqual([]);
  });

  it('derives the cause from blockedCause when readiness is absent', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      blockedCause: 'provider_unavailable',
    });

    expect(handoff.context.briefing?.statusLabel).toBe('provider_unavailable');
    expect(handoff.context.briefing?.detailLines).toEqual(['Cause: Provider Unavailable']);
    expect(handoff.context.context?.readinessCause).toBe('provider_unavailable');
  });

  it('renders the full requested-settings line for an enabled full-mode configuration', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      autonomyLevel: 'approval',
      fullModeUnlocked: true,
      investigationBudget: 5,
      investigationTimeoutSec: 30,
    });

    expect(handoff.context.briefing?.detailLines).toContain(
      'Requested settings: mode approval, automatic critical fixes enabled, budget 5, timeout 30s',
    );
  });

  it('surfaces provider and model when readiness carries only those fields', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      readiness: { provider: 'ollama', model: 'm1' },
    });

    expect(handoff.context.briefing?.detailLines).toEqual(['Provider: ollama', 'Model: m1']);
    expect(handoff.context.context?.provider).toBe('ollama');
    expect(handoff.context.context?.model).toBe('m1');
  });

  it('withholds sensitive keys and command-bearing values while showing safe details', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'm',
      details: {
        db_password: 'secret123',
        normal_field: 'visible value',
        cmd_value: 'operator ran sudo rm',
      },
    });

    expect(handoff.context.briefing?.evidence).toEqual([
      'Db Password: sensitive or command detail withheld',
      'Normal Field: visible value',
      'Cmd Value: sensitive or command detail withheld',
    ]);
    expect(JSON.stringify(handoff)).not.toContain('secret123');
    expect(JSON.stringify(handoff)).not.toContain('sudo rm');
    expect(JSON.stringify(handoff)).toContain('visible value');
  });
});

describe('buildPatrolAssistantFindingBriefing (branch coverage)', () => {
  it('returns undefined when there are no finding facts, record, approval, or fix', () => {
    expect(buildPatrolAssistantFindingBriefing({ title: 'T', subject: 'S' })).toBeUndefined();
  });

  it('uses the commands-plus-approval safety note when the fix is not destructive', () => {
    const briefing = buildPatrolAssistantFindingBriefing({
      title: 'T',
      subject: 'S',
      pendingApproval: { id: 'a1', status: 'pending' },
      proposedFix: { commandCount: 1, destructive: false },
    });

    expect(briefing?.safetyNote).toBe(
      'Command details stay in approval context; execution requires the governed approval flow.',
    );
  });

  it('uses the commands-only safety note without an approval or destructive flag', () => {
    expect(
      buildPatrolAssistantFindingBriefing({
        title: 'T',
        subject: 'S',
        proposedFix: { commandCount: 1 },
      })?.safetyNote,
    ).toBe('Command details stay in approval context.');
  });

  it('uses the destructive-only safety note without commands', () => {
    expect(
      buildPatrolAssistantFindingBriefing({
        title: 'T',
        subject: 'S',
        proposedFix: { destructive: true },
      })?.safetyNote,
    ).toBe('Destructive actions require governed approval.');
  });

  it('surfaces record impact as a detail line when the backend records one', () => {
    const briefing = buildPatrolAssistantFindingBriefing({
      title: 'T',
      subject: 'S',
      investigationRecord: {
        id: 'r-imp',
        finding_id: 'f-imp',
        subject: { resource_id: 'vm-1' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
        impact: 'impacted',
      } as unknown as InvestigationRecord,
    });

    expect(briefing?.detailLines).toContain('Impact: impacted');
  });
});

describe('buildPatrolRunAssistantHandoff (branch coverage)', () => {
  it('builds a healthy-run handoff with coverage, outcomes, timing, and effort facts', () => {
    const run = {
      id: 'run-ok',
      type: 'full',
      trigger_reason: 'scheduled',
      status: 'healthy',
      scope_resource_ids: ['vm-1', 'vm-2'],
      scope_resource_types: ['vm'],
      resources_checked: 10,
      nodes_checked: 2,
      guests_checked: 3,
      docker_checked: 1,
      storage_checked: 0,
      hosts_checked: 1,
      truenas_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 2,
      existing_findings: 1,
      resolved_findings: 1,
      rejected_findings: 0,
      auto_fix_count: 0,
      error_count: 0,
      finding_ids: ['f1', 'f2'],
      findings_summary: '2 new findings',
      started_at: 't1',
      completed_at: 't2',
      duration_ms: 5000,
      tool_call_count: 4,
      triage_flags: 1,
      triage_skipped_llm: true,
      input_tokens: 100,
      output_tokens: 200,
      ai_analysis: 'clean analysis text',
    } as unknown as PatrolRunRecord;

    const handoff = buildPatrolRunAssistantHandoff(run);

    expect(handoff.context).toMatchObject({
      targetType: 'patrol-run',
      targetId: 'run-ok',
    });
    expect(handoff.context.handoffMetadata).toMatchObject({
      kind: 'patrol_run',
      runId: 'run-ok',
      runType: 'Patrol check',
      runStatus: 'healthy',
      runtimeFailure: false,
    });
    expect(handoff.context.context).toMatchObject({
      runType: 'full',
      triggerReason: 'scheduled',
      status: 'healthy',
      effectiveStatus: 'healthy',
      errorCount: 0,
      resourcesChecked: 10,
      findingSnapshotCount: 2,
      handoffResourceCount: 2,
    });
    expect(handoff.context.briefing?.actionLabel).toBe('Discuss Patrol run outcome');
    expect(handoff.context.briefing?.evidence).toContain('2 new findings');
  });

  it('leaves the finding snapshot count undefined when finding_ids is absent', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-no-snap',
      type: 'scoped',
      status: 'issues_found',
      error_count: 0,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.context?.findingSnapshotCount).toBeUndefined();
    expect(handoff.context.handoffMetadata?.runStatus).toBe('issues found');
  });
});
