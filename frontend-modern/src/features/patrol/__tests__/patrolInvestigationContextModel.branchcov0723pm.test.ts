import { describe, expect, it } from 'vitest';
import type { RemediationPlan } from '@/api/ai';
import type { InvestigationRecord } from '@/api/ai';
import type { PatrolRunRecord } from '@/api/patrol';

import {
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantFindingHandoffActions,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantModelContext,
  buildPatrolRunAssistantHandoff,
  patrolAssistantFindingHandoffRequiresApprovalMode,
} from '../patrolInvestigationContextModel';

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

describe('buildPatrolRunAssistantHandoff (residual branches)', () => {
  it('falls back every identity field when the run carries no id, type, or status', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: '',
      type: '',
      status: '',
    } as unknown as PatrolRunRecord);

    // runId || undefined, normalizeText(run.type) || undefined,
    // normalizeText(run.status) || undefined -> all collapse to undefined.
    expect(handoff.context.targetId).toBeUndefined();
    expect(handoff.context.handoffMetadata?.runId).toBeUndefined();
    expect(handoff.context.context?.runId).toBeUndefined();
    expect(handoff.context.context?.runType).toBeUndefined();
    expect(handoff.context.context?.status).toBeUndefined();
    // run.status is absent, so the || 'unknown' arm is taken and surfaces on
    // both the context and the handoff metadata.
    expect(handoff.context.context?.effectiveStatus).toBe('unknown');
    expect(handoff.context.handoffMetadata?.runStatus).toBe('unknown');
  });

  it('renders the coverage-facts fallback list from non-resource check counts', () => {
    // resources_checked is 0 with no scope ids -> getPatrolRunCoverageSummary
    // returns '' -> the fallback briefing-string list is used. Each non-zero
    // *_checked count then contributes its fact string.
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-cov',
      type: 'full',
      status: 'healthy',
      resources_checked: 0,
      nodes_checked: 1,
      guests_checked: 1,
      docker_checked: 1,
      storage_checked: 1,
      hosts_checked: 1,
      truenas_checked: 1,
      kubernetes_checked: 1,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.briefing?.statusLabel).toContain('1 nodes');
    expect(handoff.context.briefing?.statusLabel).toContain('1 VMs');
    expect(handoff.context.briefing?.statusLabel).toContain('1 containers');
    expect(handoff.context.briefing?.statusLabel).toContain('1 storage resources');
    expect(handoff.context.briefing?.statusLabel).toContain('1 agents');
    expect(handoff.context.briefing?.statusLabel).toContain('1 TrueNAS systems');
    expect(handoff.context.briefing?.statusLabel).toContain('1 Kubernetes resources');
  });

  it('singularizes each outcome fact when exactly one of each was observed', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-one',
      type: 'full',
      status: 'healthy',
      new_findings: 1,
      existing_findings: 1,
      resolved_findings: 1,
      rejected_findings: 1,
      auto_fix_count: 1,
      error_count: 1,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.briefing?.evidence?.[0]).toBe(
      '1 new finding · 1 existing finding · 1 resolved finding · 1 rejected finding · 1 auto-remediation · 1 error',
    );
  });

  it('pluralizes each outcome fact when more than one was observed', () => {
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-many',
      type: 'full',
      status: 'healthy',
      new_findings: 2,
      existing_findings: 2,
      resolved_findings: 2,
      rejected_findings: 2,
      auto_fix_count: 2,
      error_count: 2,
    } as unknown as PatrolRunRecord);

    expect(handoff.context.briefing?.evidence?.[0]).toBe(
      '2 new findings · 2 existing findings · 2 resolved findings · 2 rejected findings · 2 auto-remediations · 2 errors',
    );
  });

  it('singularizes and pluralizes the triage-flag effort fact', () => {
    const one = buildPatrolRunAssistantHandoff({
      id: 'run-eff1',
      type: 'full',
      status: 'healthy',
      triage_flags: 1,
    } as unknown as PatrolRunRecord);
    expect(one.context.briefing?.detailLines).toContain('1 triage flag');

    const two = buildPatrolRunAssistantHandoff({
      id: 'run-eff2',
      type: 'full',
      status: 'healthy',
      triage_flags: 2,
    } as unknown as PatrolRunRecord);
    expect(two.context.briefing?.detailLines).toContain('2 triage flags');
  });

  it('caps run handoff resources at eight and leaves the type unset for multi-type scopes', () => {
    // scope_resource_types length != 1 -> resource type collapses to ''.
    // Nine distinct ids -> the ninth is skipped once resources.size hits the cap.
    const ids = Array.from({ length: 9 }, (_, i) => `res-${i + 1}`);
    const handoff = buildPatrolRunAssistantHandoff({
      id: 'run-cap',
      type: 'scoped',
      status: 'healthy',
      scope_resource_ids: ids,
      scope_resource_types: ['vm', 'host'],
    } as unknown as PatrolRunRecord);

    expect(handoff.context.context?.handoffResourceCount).toBe(8);
  });
});

describe('buildPatrolAssistantFindingHandoff (residual branches)', () => {
  it('collapses findingId to undefined in both top-level and nested context when no id exists', () => {
    const handoff = buildPatrolAssistantFindingHandoff({ title: 'T', subject: 'S' });

    expect(handoff.context.findingId).toBeUndefined();
    expect(handoff.context.context?.findingId).toBeUndefined();
    expect(handoff.context.context?.investigationRecordId).toBeUndefined();
  });

  it('resolves the resource name from the record subject and from the subject string', () => {
    // resourceName absent -> record subject.resource_name used.
    const fromRecord = buildPatrolAssistantFindingHandoff({
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      investigationRecord: {
        id: 'rec',
        finding_id: 'f',
        subject: { resource_id: 'r1', resource_name: 'From Record', resource_type: 'vm' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
      } as unknown as InvestigationRecord,
    });
    expect(fromRecord.context.handoffResources?.[0]).toMatchObject({
      id: 'r1',
      name: 'From Record',
      type: 'vm',
    });

    // resourceName and record absent -> falls back to the subject string; type
    // and node collapse to undefined.
    const fromSubject = buildPatrolAssistantFindingHandoff({
      title: 'T',
      subject: 'SubjectName',
      resourceId: 'r2',
    });
    expect(fromSubject.context.handoffResources?.[0]).toMatchObject({
      id: 'r2',
      name: 'SubjectName',
      type: undefined,
      node: undefined,
    });
  });

  it('uses the explicit resource name and type when provided', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      resourceName: 'Explicit',
      resourceType: 'host',
    });
    expect(handoff.context.handoffResources?.[0]).toMatchObject({
      id: 'r1',
      name: 'Explicit',
      type: 'host',
    });
  });

  it('falls back to default finding/subject titles in the model context', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      title: '',
      subject: '',
      resourceId: 'r1',
    });

    expect(handoff.context.handoffContext).toContain('Finding: Patrol finding');
    expect(handoff.context.handoffContext).toContain('Subject: affected resource');
  });

  it('renders rollback entries and an approved (non-pending) approval posture', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-1',
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      regressionCount: 1,
      pendingApproval: { id: 'pa-1', status: 'approved', riskLevel: 'high' },
      investigationRecord: {
        id: 'rec',
        finding_id: 'f-1',
        subject: { resource_id: 'r1' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
        rollback: ['Restore prior config'],
      } as unknown as InvestigationRecord,
    });

    // Rollback entries are mapped (the record has rollback summaries).
    expect(handoff.context.handoffContext).toContain('Rollback 1: Restore prior config');
    // The model context renders the approval via labelled context lines
    // (Approval / Approval Status / Approval Risk), not the inline parts used
    // by the assessment finding context line.
    expect(handoff.context.handoffContext).toContain('Approval: pa-1');
    expect(handoff.context.handoffContext).toContain('Approval Status: approved');
    expect(handoff.context.handoffContext).toContain('Approval Risk: high');
    // Regression singular wording.
    expect(handoff.context.handoffContext).toContain('regressed 1 time');
  });

  it('includes destructive and rationale action-artifact facts when the fix carries them', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-1',
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      proposedFix: {
        description: 'Restart',
        rationale: 'wedged process',
        destructive: true,
        commandCount: 1,
      },
    });

    expect(handoff.context.handoffContext).toContain('destructive action artifact');
    expect(handoff.context.handoffContext).toContain('rationale wedged process');
  });
});

describe('buildPatrolAssistantFindingHandoffActions (residual branches)', () => {
  it('leaves finding/resource targeting undefined for an action with sparse finding fields', () => {
    const [action] = buildPatrolAssistantFindingHandoffActions({
      pendingApproval: { actionId: 'act-1' },
    });

    expect(action).toMatchObject({
      actionId: 'act-1',
      findingId: undefined,
      targetResourceId: undefined,
      targetResourceType: undefined,
      fixId: undefined,
      description: undefined,
    });
  });
});

describe('patrolAssistantFindingHandoffRequiresApprovalMode (residual branches)', () => {
  it('requires approval when the record carries an approval id', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'rec',
          finding_id: 'f',
          subject: { resource_id: 'r1' },
          trigger: { detected_at: '2026-01-01T00:00:00Z' },
          status: 'completed',
          approval_id: 'ap-1',
        } as unknown as InvestigationRecord,
      }),
    ).toBe(true);
  });

  it('requires approval when only the record outcome is a governed action outcome', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'rec',
          finding_id: 'f',
          subject: { resource_id: 'r1' },
          trigger: { detected_at: '2026-01-01T00:00:00Z' },
          status: 'completed',
          outcome: 'fix_executed',
        } as unknown as InvestigationRecord,
      }),
    ).toBe(true);
  });

  it('does not require approval for a benign outcome', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'rec',
          finding_id: 'f',
          subject: { resource_id: 'r1' },
          trigger: { detected_at: '2026-01-01T00:00:00Z' },
          status: 'completed',
          outcome: 'needs_attention',
        } as unknown as InvestigationRecord,
      }),
    ).toBe(false);
  });
});

describe('buildPatrolRemediationPlanAssistant (residual branches)', () => {
  const planWithoutCommands: RemediationPlan = {
    id: 'plan-1',
    finding_id: 'f-1',
    resource_id: 'r-1',
    title: 'Restore service',
    description: 'Restart and verify.',
    risk_level: 'high',
    status: 'pending',
    created_at: '2026-01-01T00:00:00Z',
    steps: [],
  };

  it('falls back to default titles and the no-command safety note for an empty plan', () => {
    const modelContext = buildPatrolRemediationPlanAssistantModelContext({
      title: '',
      subject: '',
      plan: planWithoutCommands,
    });
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: '',
      subject: '',
      plan: planWithoutCommands,
    });

    expect(modelContext).toContain('Finding: Patrol finding on the affected resource');
    expect(briefing.subject).toBe('Patrol finding on affected resource');
    // No commands -> commandSummary undefined and the alternate safety note.
    expect(briefing.commandSummary).toBeUndefined();
    expect(briefing.safetyNote).toBe(
      'Assistant should decide remediation from evidence before any governed action.',
    );
    // No Governed Action Context line is emitted when commandSummary is absent.
    expect(modelContext).not.toContain('Governed Action Context:');
  });

  it('renders the artifact line, risk status, and governed-action context for a plan with commands', () => {
    const plan: RemediationPlan = {
      ...planWithoutCommands,
      steps: [{ order: 1, action: 'restart', command: 'systemctl restart x', risk_level: 'high' }],
    };
    const modelContext = buildPatrolRemediationPlanAssistantModelContext({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(modelContext).toContain('Governed Action Context: 1 command recorded');
    expect(modelContext).toContain('Treat this as approval state, not remediation guidance.');
  });

  it('omits the risk status segment and artifact line when the plan lacks them', () => {
    const plan = {
      id: 'plan-2',
      finding_id: 'f-1',
      resource_id: 'r-1',
      title: '',
      description: '',
      risk_level: '',
      status: '',
      created_at: '2026-01-01T00:00:00Z',
      steps: [],
    } as unknown as RemediationPlan;
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(briefing.statusLabel).toBeUndefined();
    expect(briefing.detailLines).toEqual([]);
  });

  it('counts rollback commands in the plan command summary', () => {
    const plan: RemediationPlan = {
      ...planWithoutCommands,
      steps: [
        { order: 1, action: 'a', rollback_command: 'systemctl stop x', risk_level: 'low' },
        { order: 2, action: 'b', rollback_command: 'systemctl stop y', risk_level: 'low' },
      ],
    };
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(briefing.commandSummary).toBe('2 rollback commands recorded');
  });
});

describe('buildPatrolAssistantFindingBriefing (residual branches)', () => {
  it('falls back to default finding/subject titles', () => {
    const briefing = buildPatrolAssistantFindingBriefing({
      title: '',
      subject: '',
      findingStatus: 'active',
    });

    expect(briefing?.subject).toBe('Patrol finding on affected resource');
    // severity/findingStatus are finding facts, not status parts in this
    // briefing, so an empty status stays undefined.
    expect(briefing?.statusLabel).toBeUndefined();
  });
});

describe('buildPatrolAssistantFindingHandoffActions (findingId source fallback)', () => {
  it('derives the action findingId from the investigation record when the finding has no id', () => {
    // finding.id absent -> normalizeText(finding.id) is '' (falsy) -> the
    // record's finding_id arm of `finding.id || record.finding_id || undefined`.
    const [action] = buildPatrolAssistantFindingHandoffActions({
      investigationRecord: {
        id: 'rec',
        finding_id: 'rec-finding',
        subject: { resource_id: 'r1' },
        trigger: { detected_at: '2026-01-01T00:00:00Z' },
        status: 'completed',
        proposed_fix: { id: 'fix-1', description: 'Restart service' },
      } as unknown as InvestigationRecord,
    });

    expect(action.findingId).toBe('rec-finding');
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

describe('buildPatrolRemediationPlanAssistantBriefing (non-array steps)', () => {
  it('treats a non-array steps field as empty when computing the command summary', () => {
    // Array.isArray(plan.steps) is false -> the `: []` arm -> no commands are
    // counted -> commandSummary stays undefined.
    const plan = {
      id: 'plan-1',
      finding_id: 'f-1',
      resource_id: 'r-1',
      title: 'Restore',
      description: 'desc',
      risk_level: 'low',
      status: 'pending',
      created_at: '2026-01-01T00:00:00Z',
      steps: 'not-an-array',
    } as unknown as RemediationPlan;

    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'T',
      subject: 'S',
      plan,
    });

    expect(briefing.commandSummary).toBeUndefined();
    expect(briefing.safetyNote).toBe(
      'Assistant should decide remediation from evidence before any governed action.',
    );
  });
});

describe('buildPatrolAssistantFindingHandoff (regression plural arm)', () => {
  it('pluralizes the regression count in the finding model context', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'f-1',
      title: 'T',
      subject: 'S',
      resourceId: 'r1',
      regressionCount: 2,
    });

    expect(handoff.context.handoffContext).toContain('regressed 2 times');
  });
});
