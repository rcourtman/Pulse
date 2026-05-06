import { describe, expect, it } from 'vitest';
import type { RemediationPlan } from '@/api/ai';

import {
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingPrompt,
  buildPatrolInvestigationContextSummary,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantPrompt,
} from '../patrolInvestigationContextModel';

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
      summaryText: '1 recent change · 2 correlations · 4 policy-covered resources',
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

  it('builds operator-facing Patrol record presentation without exposing raw commands', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'record-1',
      finding_id: 'finding-1',
      subject: { resource_id: 'vm-100', resource_name: 'web', resource_type: 'vm' },
      trigger: {
        title: 'High CPU usage',
        detected_at: '2026-05-06T12:00:00Z',
      },
      status: 'completed',
      outcome: 'fix_queued',
      confidence: 'high',
      conclusion: 'Backup job saturated CPU.',
      recommended_action: 'Approve a controlled restart after the backup completes.',
      evidence: [
        { kind: 'metrics', summary: 'CPU stayed above 95% for 10 minutes' },
        { kind: 'logs', summary: 'Backup process held IO wait.' },
      ],
      proposed_fix: {
        id: 'fix-1',
        description: 'Restart the workload service',
        commands: ['systemctl restart workload.service'],
        risk_level: 'medium',
        destructive: false,
        target_host: 'pve-1',
        rationale: 'The process is wedged after backup IO pressure.',
      },
      verification: ['CPU returned below 50%'],
      tools_used: ['metrics.history', 'ssh.exec'],
      started_at: '2026-05-06T12:00:00Z',
    });

    expect(presentation).toMatchObject({
      hasRecord: true,
      statusLabel: 'Completed',
      outcomeLabel: 'Fix Queued',
      confidenceLabel: 'High confidence',
      conclusion: 'Backup job saturated CPU.',
      evidenceSummaries: ['CPU stayed above 95% for 10 minutes', 'Backup process held IO wait.'],
      verificationSummaries: ['CPU returned below 50%'],
      toolsUsed: ['Metrics history', 'SSH exec'],
      proposedFix: {
        description: 'Restart the workload service',
        riskLabel: 'Medium',
        targetHost: 'pve-1',
        commandSummary: '1 command recorded for approval context',
      },
    });
    expect(JSON.stringify(presentation)).not.toContain('systemctl restart workload.service');
  });

  it('frames Assistant handoff around the structured Patrol record when one exists', () => {
    expect(
      buildPatrolAssistantFindingPrompt({
        title: 'High CPU usage',
        subject: 'web-server',
        description: 'CPU stayed above 95%.',
        investigationRecord: {
          id: 'record-1',
          finding_id: 'finding-1',
          subject: { resource_id: 'vm-100' },
          trigger: { detected_at: '2026-05-06T12:00:00Z' },
          status: 'completed',
          evidence: [],
          verification: [],
          tools_used: [],
          started_at: '2026-05-06T12:00:00Z',
        },
      }),
    ).toContain('Use that record as the main context before suggesting next actions.');
  });

  it('builds a drawer briefing for Assistant handoff without exposing raw commands', () => {
    const approvalRequestedAt = new Date(Date.now() - 60_000).toISOString();
    const approvalExpiresAt = new Date(Date.now() + 10 * 60_000).toISOString();

    const briefing = buildPatrolAssistantFindingBriefing({
      title: 'High CPU usage',
      subject: 'web-server',
      severity: 'critical',
      findingStatus: 'active',
      loopState: 'awaiting_approval',
      timesRaised: 4,
      regressionCount: 2,
      lastRegressionAt: '2026-05-06T12:06:00Z',
      remediationId: 'remediation-1',
      pendingApproval: {
        id: 'approval-1',
        status: 'pending',
        riskLevel: 'high',
        requestedAt: approvalRequestedAt,
        expiresAt: approvalExpiresAt,
        targetName: 'web-server',
      },
      investigationRecord: {
        id: 'record-1',
        finding_id: 'finding-1',
        subject: { resource_id: 'vm-100' },
        trigger: { detected_at: '2026-05-06T12:00:00Z' },
        status: 'completed',
        outcome: 'fix_queued',
        confidence: 'high',
        conclusion: 'Backup job saturated CPU.',
        recommended_action: 'Approve a controlled restart after the backup completes.',
        evidence: [{ kind: 'metrics', summary: 'CPU stayed above 95% for 10 minutes' }],
        proposed_fix: {
          id: 'fix-1',
          description: 'Restart the workload service',
          commands: ['systemctl restart workload.service'],
          risk_level: 'medium',
          destructive: true,
        },
        verification: ['CPU returned below 50%'],
        tools_used: [],
        started_at: '2026-05-06T12:00:00Z',
        approval_id: 'approval-1',
      },
    });

    expect(briefing).toEqual({
      sourceLabel: 'Pulse Patrol',
      title: 'Operator briefing attached',
      subject: 'High CPU usage on web-server',
      statusLabel: 'Completed · Fix Queued · High confidence',
      detailLines: [
        'Attention: active critical finding; regressed 2 times; last regression 2026-05-06T12:06:00Z; loop awaiting approval; approval approval-1; live approval pending; destructive proposed fix; fix queued for governed review',
        'Backup job saturated CPU.',
        'Approve a controlled restart after the backup completes.',
        `Decision: review live governed approval approval-1 before execution; approval pending; target web-server; expires ${approvalExpiresAt}; requested ${approvalRequestedAt}; proposed fix fix-1; risk high; destructive true`,
      ],
      evidence: ['CPU stayed above 95% for 10 minutes', 'Verified: CPU returned below 50%'],
      actionLabel: 'Restart the workload service',
      commandSummary: '1 command recorded for approval context',
      safetyNote:
        'Command details stay in approval context; destructive actions require governed approval.',
    });
    expect(JSON.stringify(briefing)).not.toContain('systemctl restart workload.service');
  });

  it('builds remediation plan Assistant handoff context without exposing raw commands', () => {
    const plan: RemediationPlan = {
      id: 'plan-1',
      finding_id: 'finding-1',
      resource_id: 'agent-1',
      title: 'Restore web service',
      description: 'Restart the service and verify health.',
      risk_level: 'high',
      status: 'pending',
      created_at: '2026-05-06T12:00:00Z',
      steps: [
        {
          order: 1,
          action: 'Restart web service',
          command: 'systemctl restart nginx',
          rollback_command: 'systemctl stop nginx',
          risk_level: 'high',
        },
        {
          order: 2,
          action: 'Check service health',
          command: 'systemctl status nginx',
          risk_level: 'low',
        },
      ],
    };

    const prompt = buildPatrolRemediationPlanAssistantPrompt({
      title: 'Nginx down',
      subject: 'node-1',
      plan,
    });
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'Nginx down',
      subject: 'node-1',
      plan,
    });

    expect(prompt).toContain('Pulse Patrol generated a governed remediation plan');
    expect(prompt).toContain('1. Restart web service (high risk; command recorded');
    expect(prompt).toContain('2 commands recorded for governed plan review');
    expect(prompt).not.toContain('systemctl restart nginx');
    expect(prompt).not.toContain('systemctl stop nginx');
    expect(prompt).not.toContain('systemctl status nginx');
    expect(briefing.commandSummary).toBe(
      '2 commands recorded for governed plan review; 1 rollback command recorded',
    );
    expect(briefing.safetyNote).toBe(
      'Command details stay in governed remediation context; execution requires the approval flow.',
    );
    expect(JSON.stringify(briefing)).not.toContain('systemctl');
  });

  it('builds an operator briefing from current finding facts before a Patrol record exists', () => {
    expect(
      buildPatrolAssistantFindingBriefing({
        title: 'High CPU usage',
        subject: 'web-server',
        severity: 'warning',
        findingStatus: 'active',
        loopState: 'investigating',
        timesRaised: 3,
      }),
    ).toEqual({
      sourceLabel: 'Pulse Patrol',
      title: 'Operator briefing attached',
      subject: 'High CPU usage on web-server',
      statusLabel: undefined,
      detailLines: [
        'Attention: active warning finding; raised 3 times; loop investigating',
        'Decision: Wait for Patrol to finish the investigation before approving remediation.',
      ],
      evidence: [],
      actionLabel: undefined,
      commandSummary: undefined,
      safetyNote: undefined,
    });
  });
});
