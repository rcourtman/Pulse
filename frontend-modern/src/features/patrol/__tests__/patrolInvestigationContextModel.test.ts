import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import type { RemediationPlan } from '@/api/ai';
import type { PatrolRunRecord } from '@/api/patrol';

import {
  buildPatrolAssessmentAssistantHandoff,
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantFindingHandoffActions,
  buildPatrolAssistantFindingPrompt,
  buildPatrolAssistantProposedFixBriefingInput,
  buildPatrolConfigurationFailureHandoff,
  buildPatrolInvestigationContextSummary,
  buildPatrolRunAssistantHandoff,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantModelContext,
  buildPatrolRemediationPlanAssistantPrompt,
  patrolAssistantFindingHandoffRequiresApprovalMode,
  selectPatrolSupportingRecentChanges,
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

  it('normalizes same-state recent changes before Patrol uses them as Assistant context', () => {
    const changes = selectPatrolSupportingRecentChanges([
      {
        id: 'change-1',
        observedAt: '2026-05-06T12:08:00Z',
        resourceId: 'app-container-1',
        kind: 'state_transition',
        from: 'online',
        to: 'online',
        sourceType: 'pulse_diff',
        sourceAdapter: 'docker_adapter',
        confidence: 'high',
        reason: 'resource state changed',
        metadata: {
          changedFields: ['docker.command'],
        },
      },
    ]);

    expect(changes).toHaveLength(1);
    expect(changes[0]).toMatchObject({
      from: undefined,
      to: undefined,
      reason: 'Docker command changed while online',
    });
  });

  it('keeps same-state recent changes out of Patrol Assistant no-op wording', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Patrol runtime issue',
        description: 'Coverage is incomplete.',
      },
      investigationContext: {
        recentChangeCount: 1,
        correlationCount: 0,
        governedResourceCount: 0,
        hasContext: true,
        summaryText: '1 recent change',
      },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'change-1',
            observedAt: '2026-05-06T12:08:00Z',
            resourceId: 'app-container-1',
            kind: 'state_transition',
            from: 'online',
            to: 'online',
            sourceType: 'pulse_diff',
            sourceAdapter: 'docker_adapter',
            confidence: 'high',
            reason: 'resource state changed',
            metadata: {
              changedFields: ['docker.command'],
            },
          },
        ],
      },
      activeFindings: [],
    });

    expect(handoff.context.handoffContext).toContain('Docker command changed while online');
    expect(handoff.context.handoffContext).not.toContain('online to online');
  });

  it('includes bounded related-resource context in assessment handoff evidence', () => {
    const longRelatedResource =
      'storage-pool-with-a-very-long-description-that-keeps-going-beyond-the-handoff-limit-for-operators';
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Issues detected',
      },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'change-related',
            observedAt: '2026-05-06T12:08:00Z',
            resourceId: 'vm-100',
            kind: 'metric_anomaly',
            sourceType: 'heuristic',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'high',
            reason: 'CPU pressure increased after storage activity',
            relatedResources: [
              'backup-job',
              'cache-node',
              longRelatedResource,
              'db-primary',
              'db-replica',
            ],
          },
        ],
      },
      activeFindings: [],
    });

    const relatedEvidence = handoff.context.briefing?.evidence?.find((line) =>
      line.includes('related resources'),
    );

    expect(relatedEvidence).toContain('related resources backup-job');
    expect(relatedEvidence).toContain('and 1 more');
    expect(relatedEvidence).not.toContain(longRelatedResource);
    expect(relatedEvidence).not.toContain('db-replica');
    expect(handoff.context.handoffContext).toContain('related resources backup-job');
    expect(handoff.context.handoffContext).toContain('and 1 more');
    expect(handoff.context.handoffContext).not.toContain(longRelatedResource);
    expect(handoff.context.handoffContext).not.toContain('db-replica');
  });

  it('builds a model-only Assistant handoff for the current Patrol assessment', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Issues detected',
        description:
          'Patrol surfaced one active critical finding and recent coverage is incomplete.',
        eyebrow: 'Current assessment',
      },
      overallHealth: { grade: 'B', score: 84 },
      scoreChipLabel: 'Health',
      metricState: {
        primaryLabel: 'Infrastructure findings',
        primaryValue: 1,
        secondaryLabel: 'Runtime issues',
        secondaryValue: 1,
        fixedLabel: 'Fixed',
        fixedValue: 2,
      },
      verification: {
        title: 'Full patrol verified recently',
        description: 'Latest full run completed with findings.',
        lastFullRunAt: '2026-05-06T12:00:00Z',
        activityMixLabel: '1 full patrol · 2 scoped runs',
      },
      recency: {
        label: 'Last patrol',
        timestamp: '2026-05-06T12:10:00Z',
      },
      latestRun: {
        kindLabel: 'Full patrol',
        status: { label: 'issues found' },
        timestamp: '2026-05-06T12:10:00Z',
        coverageSummary: '12 resources checked',
        findingsSnapshotAvailable: true,
      },
      investigationContext: {
        recentChangeCount: 2,
        correlationCount: 2,
        governedResourceCount: 4,
        hasContext: true,
        summaryText: '2 recent changes · 2 correlations · 4 policy-covered resources',
      },
      supportingEvidence: {
        recentChanges: [
          {
            id: 'change-1',
            observedAt: '2026-05-06T12:08:00Z',
            occurredAt: '2026-05-06T12:07:30Z',
            resourceId: 'vm-100',
            kind: 'metric_anomaly',
            sourceType: 'heuristic',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'high',
            actor: 'Pulse Patrol',
            relatedResources: ['backup-job'],
            reason: 'CPU spike after backup job',
          },
          {
            id: 'change-2',
            observedAt: '2026-05-06T12:07:00Z',
            resourceId: 'vm-100',
            kind: 'command_executed',
            sourceType: 'agent_action',
            sourceAdapter: 'agent:ops-helper',
            confidence: 'medium',
            reason: 'systemctl restart workload.service',
            metadata: {
              command: 'systemctl restart workload.service',
            },
          },
        ],
        correlations: [
          {
            source_id: 'backup-job',
            source_name: 'Nightly backup job',
            source_type: 'job',
            target_id: 'vm-100',
            target_name: 'web-server',
            target_type: 'vm',
            event_pattern: 'backup_started -> cpu_spike',
            occurrences: 4,
            avg_delay: 120000000000,
            confidence: 0.92,
            last_seen: '2026-05-06T12:08:00Z',
            description: 'CPU pressure usually follows this backup job.',
          },
        ],
      },
      activeFindings: [
        {
          id: 'finding-1',
          title: 'High CPU usage',
          description: 'CPU stayed above 95% during backup.',
          severity: 'critical',
          status: 'active',
          resourceId: 'vm-100',
          resourceName: 'web-server',
          resourceType: 'vm',
          detectedAt: '2026-05-06T12:00:00Z',
          lastSeenAt: '2026-05-06T12:10:00Z',
          investigationStatus: 'completed',
          investigationOutcome: 'fix_queued',
          loopState: 'awaiting_approval',
          timesRaised: 3,
          regressionCount: 1,
          lastRegressionAt: '2026-05-06T12:06:00Z',
          investigationRecord: {
            id: 'record-1',
            finding_id: 'finding-1',
            subject: {
              resource_id: 'vm-100',
              resource_name: 'web-server',
              resource_type: 'vm',
              node: 'pve-1',
            },
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
              destructive: false,
            },
            verification: ['CPU returned below 50%'],
            rollback: [],
            tools_used: ['ssh.exec'],
            started_at: '2026-05-06T12:00:00Z',
          },
        },
        {
          id: 'finding-2',
          title: 'Patrol provider warning',
          severity: 'warning',
          status: 'active',
          resourceId: 'vm-100',
          resourceName: 'web-server',
          resourceType: 'vm',
        },
      ],
    });

    expect(handoff.prompt).toContain('Discuss the current Pulse Patrol assessment');
    expect(handoff.prompt).toContain('Start by reviewing 1 governed action reference');
    expect(handoff.prompt).toContain('Do not infer, repeat, or execute raw command text');
    expect(handoff.context.autonomousMode).toBe(false);
    expect(handoff.context.handoffContext).toContain('[Patrol Assessment Context]');
    expect(handoff.context.handoffContext).toContain('Source: Pulse Patrol current assessment');
    expect(handoff.context.handoffContext).toContain('Health: Health B 84/100');
    expect(handoff.context.handoffContext).toContain(
      'Supporting Context: 2 recent changes · 2 correlations · 4 policy-covered resources',
    );
    expect(handoff.context.handoffContext).toContain(
      'Recent Change 1: Metric anomaly: CPU spike after backup job',
    );
    expect(handoff.context.handoffContext).toContain('related resources backup-job');
    expect(handoff.context.briefing?.evidence).toEqual(
      expect.arrayContaining([expect.stringContaining('related resources backup-job')]),
    );
    expect(handoff.context.handoffContext).toContain(
      'Recent Change 2: Command executed: execution event recorded',
    );
    expect(handoff.context.handoffContext).toContain('Correlation 1: Nightly backup job');
    expect(handoff.context.handoffContext).toContain('Finding 1: High CPU usage');
    expect(handoff.context.handoffContext).toContain('1 command recorded for approval context');
    expect(handoff.context.handoffResources).toEqual([
      { id: 'vm-100', name: 'web-server', type: 'vm', node: 'pve-1' },
      { id: 'backup-job', name: 'Nightly backup job', type: 'job', node: undefined },
    ]);
    expect(handoff.context.handoffMetadata).toEqual({
      kind: 'patrol_assessment',
    });
    expect(handoff.context.briefing).toMatchObject({
      sourceLabel: 'Pulse Patrol',
      title: 'Patrol assessment attached',
      subject: 'Issues detected',
      actionLabel: '1 governed action reference attached',
      safetyNote:
        'Review action posture in the governed flow; raw command payloads stay out of Assistant.',
      suggestedPrompts: [
        'Prioritize findings and safest next step',
        'Explain recent changes and correlations',
        'Summarize governed remediation risks',
      ],
    });
    expect(JSON.stringify(handoff)).not.toContain('systemctl restart workload.service');
  });

  it('frames coverage-incomplete assessment handoffs as a verification gap', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Coverage incomplete',
        description:
          'Patrol coverage is incomplete: recent activity was limited to scoped runs, so overall infrastructure health is not fully verified.',
        eyebrow: 'Patrol assessment',
      },
      overallHealth: {
        grade: 'C',
        score: 65,
        prediction: 'Patrol coverage is incomplete.',
        factors: [{ category: 'coverage' }],
      },
      scoreChipLabel: 'Assessment',
      metricState: {
        primaryLabel: 'Active findings',
        primaryValue: 0,
        secondaryLabel: 'Warnings',
        secondaryValue: 0,
        fixedLabel: 'Fixed',
        fixedValue: 0,
      },
      verification: {
        title: 'Recently verified',
        description: 'The most recent full patrol completed successfully.',
        lastFullRunAt: '2026-05-04T21:38:51Z',
        activityMixLabel: '8 full, 3 alert-triggered',
      },
      latestRun: {
        kindLabel: 'Scoped run',
        status: { label: 'issues found' },
        timestamp: '2026-05-07T21:39:18Z',
        coverageSummary: 'Checked 1 of 2 scoped resources',
        findingsSnapshotAvailable: true,
      },
      investigationContext: {
        recentChangeCount: 100,
        correlationCount: 29,
        governedResourceCount: 116,
        hasContext: true,
        summaryText: '100 recent changes · 29 correlations · 116 policy-covered resources',
      },
      recommendedNextStep: {
        title: 'Verify full coverage',
        description:
          'Run a full Patrol sweep before treating this assessment as an all-clear; recent evidence is incomplete or limited to targeted activity.',
        actionLabel: 'Run Patrol',
        actionKind: 'run_patrol',
      },
      activeFindings: [],
    });

    expect(handoff.prompt).toContain('why Patrol coverage is incomplete');
    expect(handoff.prompt).toContain('what the latest scoped activity did and did not prove');
    expect(handoff.prompt).toContain(
      'Patrol\'s visible recommended next step is "Verify full coverage"',
    );
    expect(handoff.prompt).toContain('available Patrol-owned action: Run Patrol');
    expect(handoff.context.briefing).toMatchObject({
      actionLabel: 'Recommended: Run Patrol',
      safetyNote:
        'Assistant can explain the gap; full Patrol runs, diagnostics, and remediation remain operator-controlled.',
      suggestedPrompts: [
        'Explain why coverage is incomplete',
        'Explain scoped activity and full-run gap',
        'Identify early warning signals before full verification',
      ],
    });
    expect(handoff.context.handoffContext).toContain('Assessment: Coverage incomplete');
    expect(handoff.context.handoffContext).toContain('Recommended Next Step: Verify full coverage');
    expect(handoff.context.handoffContext).toContain(
      'Recommended Next Step Action: Run Patrol (run_patrol)',
    );
    expect(handoff.context.handoffContext).toContain(
      'Supporting Context: 100 recent changes · 29 correlations · 116 policy-covered resources',
    );
    expect(handoff.context).toMatchObject({
      targetType: 'patrol-assessment',
      targetId: 'pulse-patrol-assessment',
      autonomousMode: false,
    });
    expect(handoff.context.context).toMatchObject({
      recommendedNextStepTitle: 'Verify full coverage',
      recommendedNextStepActionKind: 'run_patrol',
    });
    expect(handoff.context.handoffMetadata).toMatchObject({
      kind: 'patrol_assessment',
      recommendedNextStep: 'Verify full coverage',
      recommendedNextStepDetail:
        'Run a full Patrol sweep before treating this assessment as an all-clear; recent evidence is incomplete or limited to targeted activity.',
      recommendedNextStepAction: 'Run Patrol',
      recommendedNextStepActionKind: 'run_patrol',
    });
  });

  it('prioritizes active findings over secondary coverage caveats in assessment handoffs', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Issues detected',
        description:
          'Patrol surfaced one active warning finding in your infrastructure. Review the active findings for more detail.',
        eyebrow: 'Patrol assessment',
      },
      overallHealth: {
        grade: 'B',
        score: 85,
        prediction: 'Coverage is incomplete for secondary activity.',
        factors: [{ category: 'coverage' }],
      },
      verification: {
        title: 'Recently verified',
        description: 'The most recent full patrol completed successfully and checked 58 resources.',
        lastFullRunAt: '2026-05-12T21:22:35Z',
        activityMixLabel: '8 full, 3 alert-cleared',
      },
      latestRun: {
        kindLabel: 'Full patrol',
        status: { label: 'issues found' },
        timestamp: '2026-05-12T21:22:35Z',
        coverageSummary: 'Checked 58 resources',
        findingsSnapshotAvailable: true,
      },
      investigationContext: {
        recentChangeCount: 3,
        correlationCount: 70,
        governedResourceCount: 55,
        hasContext: true,
        summaryText: '3 recent changes · 70 correlations · 55 policy-covered resources',
      },
      recommendedNextStep: {
        title: 'Review active findings',
        description:
          'Use the findings workspace to prioritize current risk, recent changes, and governed remediation.',
        actionLabel: 'Review findings',
        actionKind: 'review_findings',
      },
      activeFindings: [
        {
          id: 'finding-backup',
          title: 'Backup failed',
          severity: 'warning',
          status: 'active',
          resourceId: 'backup-delly',
          resourceName: 'delly',
          resourceType: 'backup',
        },
      ],
    });

    expect(handoff.prompt).toContain('Start by prioritizing 1 active finding');
    expect(handoff.prompt).not.toContain('why Patrol coverage is incomplete');
    expect(handoff.prompt).not.toContain('what the latest scoped activity did and did not prove');
    expect(handoff.context.briefing).toMatchObject({
      actionLabel: 'Recommended: Review findings',
      safetyNote:
        'Assistant can explain the Patrol recommendation; Patrol runs, settings changes, diagnostics, and remediation remain operator-controlled.',
      suggestedPrompts: [
        'Prioritize findings and safest next step',
        'Explain recent changes and correlations',
        'List evidence to verify before action',
      ],
    });
  });

  it('links route-owned Patrol assessment recommendations in Assistant briefing', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Patrol runtime issue',
        description: 'Patrol coverage is incomplete.',
      },
      overallHealth: {
        grade: 'C',
        score: 60,
        factors: [{ category: 'coverage' }],
      },
      recommendedNextStep: {
        title: 'Restore Patrol visibility',
        description: 'Fix the Patrol runtime issue before treating the assessment as current.',
        actionLabel: 'Open Patrol provider settings',
        actionKind: 'open_provider_settings',
      },
      activeFindings: [],
    });

    expect(handoff.context.briefing).toMatchObject({
      actionLabel: 'Recommended: Open Patrol provider settings',
      actionHref: '/settings/system-ai',
      suggestedPrompts: [
        'Explain why Patrol visibility is blocked',
        'What should I check in provider settings?',
        'What should I verify after restoring Patrol?',
      ],
    });
    expect(handoff.context.briefing?.detailLines).toEqual(
      expect.arrayContaining([
        'Recommended next step: Restore Patrol visibility',
        'Reason: Fix the Patrol runtime issue before treating the assessment as current.',
        'Available action: Open Patrol provider settings',
      ]),
    );
    expect(handoff.context.handoffContext).toContain(
      'Recommended Next Step Action: Open Patrol provider settings (open_provider_settings)',
    );
    expect(handoff.context.context).toMatchObject({
      recommendedNextStepActionKind: 'open_provider_settings',
    });
    expect(handoff.context.handoffMetadata).toMatchObject({
      kind: 'patrol_assessment',
      recommendedNextStep: 'Restore Patrol visibility',
      recommendedNextStepDetail:
        'Fix the Patrol runtime issue before treating the assessment as current.',
      recommendedNextStepAction: 'Open Patrol provider settings',
      recommendedNextStepActionKind: 'open_provider_settings',
      recommendedNextStepActionHref: '/settings/system-ai',
    });
  });

  it('marks unavailable recommended Patrol actions in assessment handoffs', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Coverage incomplete',
        description: 'Patrol coverage is incomplete.',
      },
      overallHealth: {
        grade: 'C',
        score: 65,
        factors: [{ category: 'coverage' }],
      },
      recommendedNextStep: {
        title: 'Verify full coverage',
        description: 'Run a full Patrol sweep before treating this assessment as an all-clear.',
        actionLabel: 'Run Patrol',
        actionKind: 'run_patrol',
        actionDisabledReason: 'Patrol is already running',
      },
    });

    expect(handoff.prompt).toContain(
      'Patrol-owned action "Run Patrol" is currently unavailable: Patrol is already running',
    );
    expect(handoff.context.handoffContext).toContain(
      'Recommended Next Step Action Status: unavailable - Patrol is already running',
    );
    expect(handoff.context.context).toMatchObject({
      recommendedNextStepActionKind: 'run_patrol',
      recommendedNextStepActionDisabledReason: 'Patrol is already running',
    });
    expect(handoff.context.briefing).toMatchObject({
      actionLabel: 'Recommended: Run Patrol',
      safetyNote:
        'Assistant can explain the gap; full Patrol runs, diagnostics, and remediation remain operator-controlled. Run Patrol is currently unavailable: Patrol is already running.',
    });
    expect(handoff.context.briefing?.detailLines).toEqual(
      expect.arrayContaining([
        'Recommended next step: Verify full coverage',
        'Reason: Run a full Patrol sweep before treating this assessment as an all-clear.',
        'Action unavailable: Run Patrol - Patrol is already running',
      ]),
    );
  });

  it('withholds unsafe recommendation text from assessment handoffs', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Coverage incomplete',
        description: 'Patrol coverage is incomplete.',
      },
      overallHealth: {
        grade: 'C',
        score: 65,
        factors: [{ category: 'coverage' }],
      },
      recommendedNextStep: {
        title: 'Run sudo systemctl restart workload.service',
        description: 'Use token abc123 before running curl against the host.',
        actionLabel: 'sudo restart',
        actionKind: 'run_patrol',
      },
    });

    expect(handoff.context.handoffContext).toContain('Recommended Next Step: Run Patrol');
    expect(handoff.context.handoffContext).toContain(
      'Recommended Next Step Detail: sensitive or command detail withheld',
    );
    expect(handoff.context.handoffContext).toContain(
      'Recommended Next Step Action: Run Patrol (run_patrol)',
    );
    expect(JSON.stringify(handoff)).not.toContain('systemctl');
    expect(JSON.stringify(handoff)).not.toContain('abc123');
    expect(JSON.stringify(handoff)).not.toContain('curl');
  });

  it('carries live governed approval posture into assessment finding handoffs', () => {
    const handoff = buildPatrolAssessmentAssistantHandoff({
      assessment: {
        title: 'Issues detected',
      },
      activeFindings: [
        {
          id: 'finding-1',
          title: 'High CPU usage',
          severity: 'critical',
          status: 'active',
          resourceId: 'vm-100',
          resourceName: 'web-server',
          resourceType: 'vm',
          pendingApproval: {
            id: 'approval-1',
            status: 'pending',
            riskLevel: 'high',
            requestedAt: '2026-05-06T12:00:00Z',
            expiresAt: '2026-05-06T12:10:00Z',
            targetName: 'web-server',
            actionId: 'action-1',
            actionApprovalPolicy: 'admin',
            actionPlanExpiresAt: '2026-05-06T12:10:00Z',
            actionPlanMessage: 'Restart after the backup window clears.',
            actionPreflight: 'Restart workload service',
            actionDryRunSummary: 'No provider-supported dry run is available for this action.',
            actionRequestedBy: 'pulse_patrol',
          },
          proposedFix: {
            description: 'Restart the workload service',
            riskLevel: 'high',
            targetHost: 'web-server',
            commandCount: 1,
            destructive: true,
          },
        },
      ],
    });

    expect(handoff.context.autonomousMode).toBe(false);
    expect(handoff.prompt).toContain('Start by reviewing 1 pending governed approval');
    expect(handoff.prompt).toContain('approval policy, dry-run posture');
    expect(handoff.context.handoffContext).toContain('Finding 1: High CPU usage');
    expect(handoff.context.handoffContext).toContain('approval approval-1');
    expect(handoff.context.handoffContext).toContain('live approval pending');
    expect(handoff.context.handoffContext).toContain('high risk');
    expect(handoff.context.handoffContext).toContain('approval target web-server');
    expect(handoff.context.handoffContext).toContain('expires 2026-05-06T12:10:00Z');
    expect(handoff.context.handoffContext).toContain('requested by pulse_patrol');
    expect(handoff.context.handoffContext).toContain('1 command recorded for approval context');
    expect(handoff.context.handoffContext).toContain('destructive proposed fix');
    expect(handoff.context.handoffActions).toHaveLength(1);
    expect(handoff.context.handoffActions?.[0]).toMatchObject({
      findingId: 'finding-1',
      approvalId: 'approval-1',
      approvalStatus: 'pending',
      approvalRequestedAt: '2026-05-06T12:00:00Z',
      approvalExpiresAt: '2026-05-06T12:10:00Z',
      actionId: 'action-1',
      actionRequestedBy: 'pulse_patrol',
      actionApprovalPolicy: 'admin',
      actionRequiresApproval: true,
      actionPlanExpiresAt: '2026-05-06T12:10:00Z',
      actionPlanMessage: 'Restart after the backup window clears.',
      actionPreflight: 'Restart workload service',
      actionDryRunSummary: 'No provider-supported dry run is available for this action.',
      description: 'Restart the workload service',
      riskLevel: 'high',
      destructive: true,
      targetHost: 'web-server',
      targetResourceId: 'vm-100',
      targetResourceName: 'web-server',
      targetResourceType: 'vm',
    });
    expect(handoff.context.context).toMatchObject({
      pendingApprovalCount: 1,
    });
    expect(handoff.context.briefing).toMatchObject({
      actionLabel: '1 pending governed approval attached',
      safetyNote:
        'Review approvals in the governed flow; approval policy is attached; dry-run posture is attached; destructive actions remain approval-bound; raw command payloads stay out of Assistant.',
    });
    expect(handoff.context.briefing?.suggestedPrompts).toContain(
      'Review pending approvals and safest next step',
    );
    expect(JSON.stringify(handoff)).not.toContain('systemctl restart workload.service');
  });

  it('builds a model-only Assistant handoff for a Patrol run runtime failure', () => {
    const run: PatrolRunRecord = {
      id: 'run-runtime-error',
      started_at: '2026-05-07T12:00:00Z',
      completed_at: '2026-05-07T12:00:03Z',
      duration_ms: 3000,
      type: 'scoped',
      trigger_reason: 'alert_fired',
      scope_resource_ids: ['vm-100'],
      effective_scope_resource_ids: ['vm-100'],
      scope_resource_types: ['vm'],
      resources_checked: 1,
      nodes_checked: 0,
      guests_checked: 1,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      truenas_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 0,
      existing_findings: 0,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: 'Runtime failure prevented analysis.',
      finding_ids: [],
      error_count: 1,
      error_summary: 'Selected model does not support Patrol tools',
      error_detail:
        "agentic patrol failed: API error (404): No endpoints found that support the provided 'tool_choice' value.",
      status: 'error',
      triage_flags: 0,
      triage_skipped_llm: false,
      tool_call_count: 1,
      ai_analysis: '<｜DSML｜trace>provider trace</｜DSML｜trace>Visible runtime summary.',
    };

    const handoff = buildPatrolRunAssistantHandoff(run);

    expect(handoff.prompt).toContain('Discuss this Pulse Patrol run');
    expect(handoff.prompt).toContain('Start by explaining the Patrol runtime failure');
    expect(handoff.prompt).toContain('Provider rejected Patrol tool calls');
    expect(handoff.prompt).not.toContain('tool_choice');
    expect(handoff.prompt).not.toContain('No endpoints found');
    expect(handoff.context.autonomousMode).toBe(false);
    expect(handoff.context).toMatchObject({
      targetType: 'patrol-run',
      targetId: 'run-runtime-error',
      context: {
        source: 'pulse-patrol-run',
        runId: 'run-runtime-error',
        effectiveStatus: 'error',
        errorCount: 1,
        resourcesChecked: 1,
        findingSnapshotCount: 0,
        handoffResourceCount: 1,
      },
    });
    expect(handoff.context.handoffResources).toBeUndefined();
    expect(handoff.context.handoffMetadata).toEqual({
      kind: 'patrol_run',
      runId: 'run-runtime-error',
      runType: 'Scoped run',
      runStatus: 'error',
      runtimeFailure: true,
    });
    expect(handoff.context.handoffContext).toBeUndefined();
    expect(handoff.context.briefing).toMatchObject({
      sourceLabel: 'Pulse Patrol',
      title: 'Patrol run attached',
      actionLabel: 'Review Patrol runtime failure',
      suggestedPrompts: [
        'Explain why this Patrol run failed',
        'List provider or model checks',
        'What should I retry after fixing it?',
      ],
    });
    expect(JSON.stringify(handoff)).not.toContain('provider trace');
    expect(JSON.stringify(handoff)).not.toContain('tool_choice');
    expect(JSON.stringify(handoff)).not.toContain('No endpoints found');
  });

  it('builds a model-only Assistant handoff for a Patrol configuration failure', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({
      message: 'The selected Patrol model is a reasoning-only model family.',
      code: 'patrol_readiness_not_ready',
      status: 400,
      details: {
        cause: 'model_unsupported_tools',
        provider: 'ollama',
        command: 'systemctl restart pulse.service',
      },
      autonomyLevel: 'approval',
      fullModeUnlocked: false,
      investigationBudget: 10,
      investigationTimeoutSec: 120,
      readiness: {
        status: 'not_ready',
        cause: 'model_unsupported_tools',
        summary: 'The selected Patrol model is a reasoning-only model family.',
        provider: 'ollama',
        model: 'ollama:deepseek-r1:7b',
      },
      runtimeState: 'active',
    });

    expect(handoff.prompt).toContain('Discuss this Pulse Patrol configuration failure');
    expect(handoff.prompt).toContain('Do not infer, repeat, or execute raw command text');
    expect(handoff.context.autonomousMode).toBe(false);
    expect(handoff.context.handoffMetadata).toMatchObject({
      kind: 'patrol_configuration_failure',
      runtimeFailure: true,
    });
    expect(handoff.context.handoffContext).toContain('[Patrol Configuration Failure Context]');
    expect(handoff.context.handoffContext).toContain('Server Code: patrol_readiness_not_ready');
    expect(handoff.context.handoffContext).toContain('Provider: ollama');
    expect(handoff.context.handoffContext).toContain('Model: ollama:deepseek-r1:7b');
    expect(handoff.context.handoffContext).toContain(
      'Command: sensitive or command detail withheld',
    );
    expect(handoff.context.briefing).toMatchObject({
      sourceLabel: 'Pulse Patrol',
      title: 'Patrol configuration failure attached',
      actionLabel: 'Review Patrol configuration failure',
      suggestedPrompts: [
        'Explain why Patrol configuration failed',
        'List provider or model checks',
        'What should I change before retrying?',
      ],
    });
    expect(JSON.stringify(handoff)).not.toContain('systemctl restart pulse.service');
  });

  it('labels saved Patrol configuration readiness issues separately from save failures', () => {
    const handoff = buildPatrolConfigurationFailureHandoff({
      saved: true,
      message: 'Patrol was saved, but the selected model cannot run Patrol tools yet.',
      code: 'patrol_readiness_not_ready',
      readiness: {
        status: 'not_ready',
        cause: 'model_unsupported_tools',
        summary: 'The selected Patrol model cannot run tools.',
        provider: 'ollama',
        model: 'ollama:deepseek-r1:7b',
      },
    });

    expect(handoff.prompt).toContain('Discuss this Pulse Patrol configuration issue');
    expect(handoff.context.briefing).toMatchObject({
      title: 'Patrol configuration issue attached',
      actionLabel: 'Review Patrol configuration issue',
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
      rollback: [],
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
    expect(presentation.rollbackSummaries).toEqual([]);
    expect(presentation.impact).toBe('');
  });

  it('keeps previous-fix operational memory at the finding shell, not in record presentation', () => {
    // Operational memory (previousResolvedFixSummary) lives on the finding
    // shell and is rendered by FindingsPanel as a distinct row. The
    // investigation-record presentation must not absorb it into impact,
    // verification, or rollback — those represent the CURRENT investigation,
    // not history. This test pins the boundary so future refactors do not
    // collapse the per-record schema with the per-finding memory shell.
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'rec-prev-fix-isolation',
      finding_id: 'f-prev-fix-isolation',
      subject: { resource_id: 'vm-9' },
      trigger: { detected_at: '2026-05-08T12:00:00Z' },
      status: 'completed',
      evidence: [],
      verification: [],
      rollback: [],
      tools_used: [],
      started_at: '2026-05-08T12:00:00Z',
    });
    // The presentation shape exposes confidence, conclusion, impact,
    // recommendedAction, etc., but no previousResolvedFixSummary field —
    // that lives on UnifiedFinding/Finding shells, not InvestigationRecord.
    const opaque = presentation as unknown as Record<string, unknown>;
    expect(opaque.previousResolvedFixSummary).toBeUndefined();
    expect(opaque.previous_resolved_fix_summary).toBeUndefined();
  });

  it('does not let the patrol context model synthesize impact from trust counters', () => {
    // Trust counters (FindingsTrustSummary on the patrol-status response) are
    // an operator-page concern, not a per-finding context concern. The
    // investigation-context model must not derive impact, recommendation, or
    // any other per-finding text from trust counts; the source authoring rule
    // (see ai-runtime contract) forbids synthesis from severity, category,
    // OR aggregate counts. This test pins that boundary.
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'rec-trust-isolation',
      finding_id: 'f-trust-isolation',
      subject: { resource_id: 'vm-9' },
      trigger: { detected_at: '2026-05-08T12:00:00Z' },
      status: 'completed',
      // Intentionally empty impact/rollback so the test reflects what the
      // model produces when only trust signals are available externally.
      evidence: [],
      verification: [],
      rollback: [],
      tools_used: [],
      started_at: '2026-05-08T12:00:00Z',
    });
    expect(presentation.impact).toBeFalsy();
    expect(presentation.rollbackSummaries).toEqual([]);
  });

  it('seeds an explain-intent prompt with explanation framing rather than discussion framing', () => {
    const discussPrompt = buildPatrolAssistantFindingPrompt({
      title: 'Backup job failing',
      subject: 'vm-101',
      description: 'Datastore quota exhausted',
    });
    const explainPrompt = buildPatrolAssistantFindingPrompt({
      title: 'Backup job failing',
      subject: 'vm-101',
      description: 'Datastore quota exhausted',
      intent: 'explain',
    });

    expect(discussPrompt).toContain("I'd like to discuss");
    expect(explainPrompt).toContain('Explain this Patrol finding');
    expect(explainPrompt.toLowerCase()).toContain('walk me through what we know');
    // Both prompts include the title and subject so the seed reads naturally.
    expect(explainPrompt).toContain('Backup job failing');
    expect(explainPrompt).toContain('vm-101');
  });

  it('seeds a verify_fix-intent prompt that asks the LLM to confirm the fix actually worked', () => {
    // Verify fix is the post-remediation check. After a fix has run,
    // the operator asks "did that actually clear the underlying
    // condition" — and the LLM should check via Pulse tools rather
    // than trust the fix command's exit code. Verification is
    // read-only; no state-changing tool calls.
    const prompt = buildPatrolAssistantFindingPrompt({
      title: 'Backup job failing',
      subject: 'vm-101',
      description: 'Datastore quota exhausted',
      intent: 'verify_fix',
    });

    expect(prompt).toContain('Verify the fix applied to this Patrol finding');
    expect(prompt).toContain('Backup job failing');
    expect(prompt).toContain('vm-101');
    // The verification dimensions: condition cleared, evidence,
    // confidence, residual risk.
    expect(prompt.toLowerCase()).toContain('condition');
    expect(prompt.toLowerCase()).toContain('cleared');
    expect(prompt.toLowerCase()).toContain('evidence');
    expect(prompt.toLowerCase()).toContain('confident');
    expect(prompt.toLowerCase()).toMatch(/residual|monitor/);
    // Read-only safety: no state-changing commands during verification.
    expect(prompt.toLowerCase()).toContain('read-only');
    expect(prompt).not.toContain("I'd like to discuss");
    expect(prompt).not.toContain('Investigate this Patrol finding now');
  });

  it('seeds a why-intent prompt that focuses on cause, not current state', () => {
    // Why-did-this-happen is the diagnostic counterpart to Explain. Where
    // Explain says "tell me what we know" and Investigate says "go find
    // out what's true now," Why focuses the LLM on cause — recent
    // changes, correlations, prior incidents, regression patterns. Not
    // "is it bad right now" but "what made it become bad."
    const prompt = buildPatrolAssistantFindingPrompt({
      title: 'Backup job failing',
      subject: 'vm-101',
      description: 'Datastore quota exhausted',
      intent: 'why',
    });

    expect(prompt).toContain('Why did this Patrol finding happen');
    expect(prompt).toContain('Backup job failing');
    expect(prompt).toContain('vm-101');
    expect(prompt.toLowerCase()).toContain('cause, not on current state');
    // The diagnostic signals the prompt directs the LLM toward.
    expect(prompt.toLowerCase()).toMatch(/recent changes|correlations|prior incidents|regression/);
    // Synthesis: what caused it, what evidence supports the cause, what
    // would have to be true for it to recur.
    expect(prompt.toLowerCase()).toContain('most likely caused');
    expect(prompt.toLowerCase()).toContain('evidence');
    expect(prompt.toLowerCase()).toContain('recur');
    // Safety: cause investigation may need tools but must not run
    // anything that changes state without operator approval.
    expect(prompt.toLowerCase()).toContain('operator approval');
    expect(prompt).not.toContain("I'd like to discuss");
    expect(prompt).not.toContain('Investigate this Patrol finding now');
  });

  it('seeds an investigate-intent prompt that lets the model decide whether tools are needed', () => {
    // Investigate is the action counterpart to Explain. Where Explain says
    // "tell me what we know," Investigate says "go find out what's true
    // right now" — the prompt should provide the available context without
    // forcing a specific tool path before the model has reasoned about it.
    const prompt = buildPatrolAssistantFindingPrompt({
      title: 'Backup job failing',
      subject: 'vm-101',
      description: 'Datastore quota exhausted',
      intent: 'investigate',
    });

    expect(prompt).toContain('Investigate this Patrol finding now');
    expect(prompt).toContain('Backup job failing');
    expect(prompt).toContain('vm-101');
    // Model-owned tool choice — the differentiator vs Explain.
    expect(prompt.toLowerCase()).toContain('decide whether the available pulse tools are needed');
    expect(prompt.toLowerCase()).toContain('fresh evidence');
    // Synthesis instruction — root cause + confidence + safe next step.
    expect(prompt.toLowerCase()).toContain('root cause');
    expect(prompt.toLowerCase()).toContain('confidence');
    expect(prompt.toLowerCase()).toContain('safe next step');
    // Safety: any command-running must route through governed approval,
    // not the LLM's own judgment.
    expect(prompt.toLowerCase()).toContain('governed approval');
    // Must not be the open-ended Discuss prompt.
    expect(prompt).not.toContain("I'd like to discuss");
  });

  it('surfaces investigation impact and rollback when the backend record carries them', () => {
    const presentation = buildPatrolInvestigationRecordPresentation({
      id: 'record-2',
      finding_id: 'finding-2',
      subject: { resource_id: 'vm-101' },
      trigger: { title: 'Backup job failing', detected_at: '2026-05-06T12:00:00Z' },
      status: 'completed',
      outcome: 'fix_queued',
      confidence: 'medium',
      conclusion: 'Datastore quota exhausted.',
      impact: 'Nightly backups will be skipped; recovery window grows by one day per skip.',
      recommended_action: 'Free 200GB on the datastore before the next backup window.',
      evidence: [{ kind: 'metrics', summary: 'datastore 99% full' }],
      verification: ['Backup job exits 0 on next run'],
      rollback: ['Restore prior retention policy', 'Re-pin previous datastore mount'],
      tools_used: [],
      started_at: '2026-05-06T12:00:00Z',
    });

    expect(presentation.impact).toBe(
      'Nightly backups will be skipped; recovery window grows by one day per skip.',
    );
    expect(presentation.rollbackSummaries).toEqual([
      'Restore prior retention policy',
      'Re-pin previous datastore mount',
    ]);
  });

  it('normalizes safe proposed-fix briefing metadata without command text', () => {
    const briefing = buildPatrolAssistantProposedFixBriefingInput({
      description: 'Restart the workload service',
      commands: ['systemctl restart workload.service'],
      risk_level: 'high',
      target_host: 'node-1',
      rationale: 'Service stayed wedged after IO pressure.',
      destructive: true,
    });

    expect(briefing).toEqual({
      description: 'Restart the workload service',
      riskLevel: 'high',
      targetHost: 'node-1',
      rationale: 'Service stayed wedged after IO pressure.',
      commandCount: 1,
      destructive: true,
    });
    expect(JSON.stringify(briefing)).not.toContain('systemctl restart workload.service');
  });

  it('carries approval requester identity into safe Patrol handoff metadata', () => {
    const briefing = buildPatrolAssistantApprovalBriefingInput({
      id: 'approval-1',
      toolId: 'investigation_fix',
      command: 'systemctl restart nginx',
      targetType: 'investigation',
      targetId: 'finding-1',
      targetName: 'node-1',
      context: 'Restart nginx after Patrol investigation',
      requestedBy: 'pulse_patrol',
      riskLevel: 'high',
      status: 'pending',
      requestedAt: '2026-05-06T12:00:00Z',
      expiresAt: '2026-05-06T12:10:00Z',
      plan: {
        actionId: 'action-1',
        approvalPolicy: 'admin',
      },
    });

    expect(briefing).toMatchObject({
      id: 'approval-1',
      actionId: 'action-1',
      actionApprovalPolicy: 'admin',
      actionRequestedBy: 'pulse_patrol',
    });
    expect(JSON.stringify(briefing)).not.toContain('systemctl restart nginx');
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
          rollback: [],
          tools_used: [],
          started_at: '2026-05-06T12:00:00Z',
        },
      }),
    ).toContain('Use that record as the main context before suggesting next actions.');
  });

  it('leads finding prompts with live governed approval context without command text', () => {
    const prompt = buildPatrolAssistantFindingPrompt({
      title: 'High CPU usage',
      subject: 'web-server',
      description: 'CPU stayed above 95%.',
      pendingApproval: {
        id: 'approval-1',
        status: 'pending',
        riskLevel: 'high',
        targetName: 'web-server',
        actionApprovalPolicy: 'operator',
        actionPreflight: 'Service restart would be attempted after health checks.',
        actionDryRunSummary: 'Dry run completed with one restart action.',
      },
      proposedFix: buildPatrolAssistantProposedFixBriefingInput({
        description: 'Restart the workload service',
        commands: ['systemctl restart workload.service'],
        riskLevel: 'high',
        targetHost: 'web-server',
      }),
    });

    expect(prompt).toContain('Start by reviewing governed approval approval-1');
    expect(prompt).toContain('approval status pending');
    expect(prompt).toContain('high risk');
    expect(prompt).toContain('approval policy attached');
    expect(prompt).toContain('dry-run posture attached');
    expect(prompt).toContain('safest next step');
    expect(prompt).not.toContain('systemctl restart workload.service');
  });

  it('leads finding prompts with proposed-fix posture without command text', () => {
    const prompt = buildPatrolAssistantFindingPrompt({
      title: 'Nginx down',
      subject: 'node-1',
      description: 'The service stopped responding.',
      investigationOutcome: 'fix_queued',
      remediationId: 'remediation-1',
      proposedFix: buildPatrolAssistantProposedFixBriefingInput({
        description: 'Restart nginx',
        commands: ['systemctl restart nginx'],
        riskLevel: 'medium',
        targetHost: 'node-1',
        destructive: true,
      }),
    });

    expect(prompt).toContain('Start by reviewing the governed action posture');
    expect(prompt).toContain('medium risk');
    expect(prompt).toContain('1 command recorded for approval context');
    expect(prompt).toContain('destructive action');
    expect(prompt).toContain('safest next step');
    expect(prompt).not.toContain('systemctl restart nginx');
  });

  it('builds finding-level Assistant handoff actions without raw command text', () => {
    const actions = buildPatrolAssistantFindingHandoffActions({
      id: 'finding-1',
      title: 'Nginx down',
      resourceId: 'agent-1',
      resourceName: 'node-1',
      resourceType: 'agent',
      pendingApproval: {
        id: 'approval-1',
        status: 'pending',
        riskLevel: 'high',
        requestedAt: '2026-05-06T12:00:00Z',
        expiresAt: '2026-05-06T12:10:00Z',
        targetName: 'node-1',
        actionId: 'restart-nginx',
        actionApprovalPolicy: 'operator',
        actionPlanExpiresAt: '2026-05-06T12:10:00Z',
        actionPlanMessage: 'Restart nginx after validating load balancer drain.',
        actionPreflight: 'Would restart nginx on node-1.',
        actionDryRunSummary: 'One service restart would be attempted.',
        actionRequestedBy: 'pulse_patrol',
      },
      proposedFix: buildPatrolAssistantProposedFixBriefingInput({
        description: 'Restart nginx',
        commands: ['systemctl restart nginx'],
        riskLevel: 'high',
        targetHost: 'node-1',
        destructive: false,
      }),
    });

    expect(actions).toEqual([
      {
        findingId: 'finding-1',
        recordId: undefined,
        approvalId: 'approval-1',
        approvalStatus: 'pending',
        approvalRequestedAt: '2026-05-06T12:00:00Z',
        approvalExpiresAt: '2026-05-06T12:10:00Z',
        actionId: 'restart-nginx',
        actionRequestedBy: 'pulse_patrol',
        actionApprovalPolicy: 'operator',
        actionRequiresApproval: true,
        actionPlanExpiresAt: '2026-05-06T12:10:00Z',
        actionPlanMessage: 'Restart nginx after validating load balancer drain.',
        actionPreflight: 'Would restart nginx on node-1.',
        actionDryRunSummary: 'One service restart would be attempted.',
        fixId: undefined,
        description: 'Restart nginx',
        riskLevel: 'high',
        destructive: false,
        targetHost: 'node-1',
        targetResourceId: 'agent-1',
        targetResourceName: 'node-1',
        targetResourceType: 'agent',
        targetNode: undefined,
      },
    ]);
    expect(JSON.stringify(actions)).not.toContain('systemctl restart nginx');
  });

  it('skips finding-level Assistant handoff actions when no governed action exists', () => {
    expect(
      buildPatrolAssistantFindingHandoffActions({
        id: 'finding-1',
        title: 'High CPU usage',
        resourceId: 'agent-1',
      }),
    ).toEqual([]);
  });

  it('builds finding-level Assistant handoff context, resources, and actions together', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'finding-1',
      title: 'Nginx down',
      subject: 'node-1',
      description: 'The service stopped responding.',
      severity: 'warning',
      findingStatus: 'active',
      investigationStatus: 'completed',
      investigationOutcome: 'fix_queued',
      loopState: 'awaiting_approval',
      timesRaised: 3,
      regressionCount: 1,
      lastRegressionAt: '2026-05-06T11:59:00Z',
      remediationId: 'remediation-1',
      resourceId: 'agent-1',
      resourceName: 'node-1',
      resourceType: 'agent',
      detectedAt: '2026-05-06T11:50:00Z',
      lastSeenAt: '2026-05-06T12:00:00Z',
      pendingApproval: {
        id: 'approval-1',
        status: 'pending',
        riskLevel: 'high',
        requestedAt: '2026-05-06T12:00:00Z',
        expiresAt: '2026-05-06T12:10:00Z',
        targetName: 'node-1',
        actionId: 'restart-nginx',
        actionApprovalPolicy: 'operator',
        actionPlanMessage: 'Restart nginx after validating load balancer drain.',
        actionPreflight: 'Would restart nginx on node-1.',
        actionDryRunSummary: 'One service restart would be attempted.',
        actionRequestedBy: 'pulse_patrol',
      },
      proposedFix: buildPatrolAssistantProposedFixBriefingInput({
        description: 'Restart nginx',
        commands: ['systemctl restart nginx'],
        riskLevel: 'high',
        targetHost: 'node-1',
      }),
    });

    expect(handoff.prompt).toContain('Start by reviewing governed approval approval-1');
    expect(handoff.context).toMatchObject({
      targetType: 'agent',
      targetId: 'agent-1',
      findingId: 'finding-1',
      autonomousMode: false,
      handoffResources: [{ id: 'agent-1', name: 'node-1', type: 'agent' }],
      context: {
        source: 'pulse-patrol-finding',
        findingId: 'finding-1',
        resourceId: 'agent-1',
        resourceName: 'node-1',
        resourceType: 'agent',
        pendingApprovalId: 'approval-1',
        actionReferenceCount: 1,
      },
    });
    expect(handoff.context.handoffContext).toContain('[Patrol Finding Context]');
    expect(handoff.context.handoffContext).toContain('Approval: approval-1');
    expect(handoff.context.handoffContext).toContain('Action Requested By: pulse_patrol');
    expect(handoff.context.handoffContext).toContain(
      'Dry-Run Posture: One service restart would be attempted.',
    );
    expect(handoff.context.handoffContext).toContain(
      'Command Boundary: Command details stay in governed approval or remediation context',
    );
    expect(handoff.context.handoffActions).toHaveLength(1);
    expect(JSON.stringify(handoff)).not.toContain('systemctl restart nginx');
  });

  it('builds a drawer briefing for Assistant handoff without exposing raw commands', () => {
    // Pin "now" so the relative-time formatting in the briefing
    // (`last regression {relative}`) is deterministic against the
    // fixed fixture timestamp two hours earlier. Restored after.
    const pinnedNow = new Date('2026-05-06T14:06:00Z');
    vi.useFakeTimers();
    vi.setSystemTime(pinnedNow);

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
        rollback: [],
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
        'Attention: active critical finding; regressed 2 times; last regression 2 hours ago; loop awaiting approval; approval approval-1; live approval pending; destructive proposed fix; fix queued for governed review',
        'Backup job saturated CPU.',
        'Approve a controlled restart after the backup completes.',
        `Decision: review live governed approval approval-1 before execution; approval pending; target web-server; expires ${approvalExpiresAt}; requested ${approvalRequestedAt}; action artifact fix-1; risk high; destructive true`,
      ],
      evidence: ['CPU stayed above 95% for 10 minutes', 'Verified: CPU returned below 50%'],
      actionLabel: 'Restart the workload service',
      commandSummary: '1 command recorded for approval context',
      safetyNote:
        'Command details stay in approval context; destructive actions require governed approval.',
      suggestedPrompts: [
        'Review approval risk and next step',
        'Explain Patrol evidence and confidence',
        'Summarize remediation without command text',
      ],
    });
    expect(JSON.stringify(briefing)).not.toContain('systemctl restart workload.service');
    vi.useRealTimers();
  });

  it('treats existing remediation artifacts as non-authoritative Assistant context', () => {
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
    const modelContext = buildPatrolRemediationPlanAssistantModelContext({
      title: 'Nginx down',
      subject: 'node-1',
      plan,
    });
    const briefing = buildPatrolRemediationPlanAssistantBriefing({
      title: 'Nginx down',
      subject: 'node-1',
      plan,
    });

    expect(prompt).toBe(
      'Review this Patrol finding and decide the safest next step: "Nginx down" on node-1.',
    );
    expect(modelContext).toContain('[Patrol Finding Action Context]');
    expect(modelContext).toContain(
      'Pulse is attaching observed finding context and any existing governed action artifact',
    );
    expect(modelContext).toContain(
      'The selected language model should decide whether remediation is appropriate',
    );
    expect(modelContext).toContain('Existing Action Artifact: Restore web service');
    expect(modelContext).toContain(
      '2 commands recorded for governed plan review; 1 rollback command recorded',
    );
    expect(modelContext).toContain('Treat this as approval state, not remediation guidance.');
    expect(modelContext).toContain('Do not assume any Patrol-authored action is correct.');
    expect(modelContext).not.toContain('1. Restart web service');
    expect(modelContext).not.toContain('2. Check service health');
    expect(modelContext).not.toContain('systemctl restart nginx');
    expect(modelContext).not.toContain('systemctl stop nginx');
    expect(modelContext).not.toContain('systemctl status nginx');
    expect(briefing.title).toBe('Patrol finding attached');
    expect(briefing.subject).toBe('Nginx down on node-1');
    expect(briefing.detailLines).toEqual([
      'Existing action artifact: Restore web service',
      'Restart the service and verify health.',
    ]);
    expect(briefing.evidence).toBeUndefined();
    expect(briefing.actionLabel).toBeUndefined();
    expect(briefing.commandSummary).toBe(
      '2 commands recorded for governed plan review; 1 rollback command recorded',
    );
    expect(briefing.safetyNote).toBe(
      'Assistant should decide remediation from evidence; command execution requires governed approval.',
    );
    expect(briefing.suggestedPrompts).toBeUndefined();
    expect(JSON.stringify(briefing)).not.toContain('systemctl');
  });

  it('forces approval-required Assistant mode for governed finding handoffs', () => {
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        pendingApproval: { id: 'approval-1', status: 'pending' },
      }),
    ).toBe(true);
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        remediationId: 'plan-1',
      }),
    ).toBe(true);
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationOutcome: 'fix_queued',
      }),
    ).toBe(true);
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationRecord: {
          id: 'record-1',
          finding_id: 'finding-1',
          subject: { resource_id: 'agent-1' },
          trigger: { detected_at: '2026-05-06T12:00:00Z' },
          status: 'completed',
          evidence: [],
          proposed_fix: {
            id: 'fix-1',
            description: 'Restart service',
            commands: ['systemctl restart nginx'],
            destructive: false,
          },
          verification: [],
          rollback: [],
          tools_used: [],
          started_at: '2026-05-06T12:00:00Z',
        },
      }),
    ).toBe(true);
    expect(
      patrolAssistantFindingHandoffRequiresApprovalMode({
        investigationOutcome: 'needs_attention',
      }),
    ).toBe(false);
  });

  it('keeps context-only Patrol finding handoffs approval scoped', () => {
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'finding-context-only',
      title: 'Provider connection issue',
      subject: 'Patrol runtime',
      description: 'Pulse Patrol could not maintain a healthy provider connection.',
      severity: 'warning',
      findingStatus: 'active',
      loopState: 'detected',
      resourceId: 'pulse-patrol-runtime',
      resourceName: 'Patrol runtime',
      resourceType: 'service',
      nextStepAction: {
        label: 'Open Patrol provider settings',
        href: '/settings/system-ai',
      },
    });

    expect(handoff.context).toMatchObject({
      targetType: 'service',
      targetId: 'pulse-patrol-runtime',
      findingId: 'finding-context-only',
      autonomousMode: false,
      context: {
        source: 'pulse-patrol-finding',
        findingId: 'finding-context-only',
        resourceId: 'pulse-patrol-runtime',
        resourceName: 'Patrol runtime',
        resourceType: 'service',
        nextStepActionLabel: 'Open Patrol provider settings',
        nextStepActionHref: '/settings/system-ai',
        actionReferenceCount: 0,
      },
    });
    expect(handoff.prompt).toContain(
      'Patrol\'s visible next step is "Open Patrol provider settings"',
    );
    expect(handoff.context.briefing).toMatchObject({
      actionLabel: 'Open Patrol provider settings',
      actionHref: '/settings/system-ai',
      suggestedPrompts: [
        'Review Patrol next step',
        'Explain current Patrol loop state',
        'Check prerequisites before next step',
      ],
    });
    expect(handoff.context.handoffMetadata).toMatchObject({
      kind: 'patrol_finding',
      recommendedNextStep: 'Open Patrol provider settings',
      recommendedNextStepAction: 'Open Patrol provider settings',
      recommendedNextStepActionHref: '/settings/system-ai',
    });
    expect(handoff.context.handoffActions).toBeUndefined();
    expect(handoff.context.handoffContext).toContain(
      'Patrol Next Step: Open Patrol provider settings',
    );
    expect(handoff.context.handoffContext).toContain('Patrol Next Step Route: /settings/system-ai');
    expect(handoff.context.handoffContext).toContain(
      'Operator Boundary: This Patrol finding handoff is model-only context',
    );
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
      suggestedPrompts: [
        'Explain current finding status',
        'Explain current Patrol loop state',
        'Explain recurrence and what changed',
      ],
    });
  });

  it('builds a pending approval briefing before full investigation record hydration', () => {
    const briefing = buildPatrolAssistantFindingBriefing({
      title: 'CPU saturation',
      subject: 'node-1',
      findingStatus: 'active',
      loopState: 'fix_queued',
      pendingApproval: {
        id: 'approval-1',
        status: 'pending',
        riskLevel: 'high',
        requestedAt: '2026-05-06T12:00:00Z',
        expiresAt: '2026-05-06T12:10:00Z',
        targetName: 'node-1',
      },
    });

    expect(briefing).toEqual({
      sourceLabel: 'Pulse Patrol',
      title: 'Operator briefing attached',
      subject: 'CPU saturation on node-1',
      statusLabel: 'Pending approval · High risk',
      detailLines: [
        'Attention: active finding; loop fix queued; live approval pending',
        'Decision: Review live governed approval approval-1 before execution. Status: pending. Target: node-1. Risk: high. Expires: 2026-05-06T12:10:00Z. Requested: 2026-05-06T12:00:00Z.',
      ],
      evidence: [],
      actionLabel: 'Approval approval-1',
      commandSummary: undefined,
      safetyNote: 'Execution requires the governed approval flow.',
      suggestedPrompts: [
        'Review approval risk and next step',
        'Explain current finding status',
        'List approval prerequisites before action',
      ],
    });
  });

  it('builds a queued-fix recovery briefing when live approval details are unavailable', () => {
    expect(
      buildPatrolAssistantFindingBriefing({
        title: 'CPU saturation',
        subject: 'node-1',
        findingStatus: 'active',
        investigationOutcome: 'fix_queued',
        loopState: 'fix_queued',
      }),
    ).toEqual({
      sourceLabel: 'Pulse Patrol',
      title: 'Operator briefing attached',
      subject: 'CPU saturation on node-1',
      statusLabel: 'Fix Queued',
      detailLines: [
        'Attention: active finding; loop fix queued; fix queued for governed review',
        'Decision: Recover or regenerate the governed approval before execution; do not execute from chat context.',
      ],
      evidence: [],
      actionLabel: undefined,
      commandSummary: undefined,
      safetyNote: undefined,
      suggestedPrompts: [
        'Review approval risk and next step',
        'Explain current finding status',
        'List approval prerequisites before action',
      ],
    });
  });

  it('builds queued-fix recovery briefing from safe proposed-fix metadata', () => {
    expect(
      buildPatrolAssistantFindingBriefing({
        title: 'CPU saturation',
        subject: 'node-1',
        findingStatus: 'active',
        investigationOutcome: 'fix_queued',
        loopState: 'fix_queued',
        proposedFix: {
          description: 'Restart workload service',
          riskLevel: 'high',
          targetHost: 'node-1',
          rationale: 'service is wedged',
          commandCount: 1,
          destructive: true,
        },
      }),
    ).toEqual({
      sourceLabel: 'Pulse Patrol',
      title: 'Operator briefing attached',
      subject: 'CPU saturation on node-1',
      statusLabel: 'Fix Queued',
      detailLines: [
        'Attention: active finding; loop fix queued; fix queued for governed review',
        'Proposed fix: Restart workload service; target node-1; high risk; 1 command recorded for approval context; destructive proposed fix; rationale service is wedged',
        'Decision: Recover or regenerate the governed approval before execution; do not execute from chat context.',
      ],
      evidence: [],
      actionLabel: 'Restart workload service',
      commandSummary: '1 command recorded for approval context',
      safetyNote:
        'Command details stay in approval context; destructive actions require governed approval.',
      suggestedPrompts: [
        'Review approval risk and next step',
        'Explain current finding status',
        'Summarize remediation without command text',
      ],
    });
  });
});

describe('Patrol page header IA framing', () => {
  it('surfaces presenter-owned coverage wording on the recency line in the page header', () => {
    // Third wedge of the Patrol page IA reframe. The recency line tells the
    // operator when Pulse last ran; the coverage signal tells them what it
    // covered. Pin the wiring so the recency render reads
    // resourcesCheckedLabel from getPatrolRecencyPresentation and gates on a
    // truthy <Show> so zero-coverage runs do not render a coverage phrase,
    // and failed or scoped runs do not get hardcoded "verified" wording.
    const headerSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
      'utf-8',
    );
    expect(headerSource).toContain('recency().resourcesCheckedLabel');
    expect(headerSource).toContain('Show when={recency().resourcesCheckedLabel}');
    expect(headerSource).not.toContain('verified {recency().resourcesChecked}');
  });

  it('keeps trust-at-a-glance state inside the primary assessment readout', () => {
    // The default Patrol scan path has one status owner: the assessment strip.
    // Regression trust counters may feed that compact readout, but the header
    // and workspace tabs should not repeat another active/regressed strip.
    const summarySource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceSummary.tsx'),
      'utf-8',
    );
    const headerSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
      'utf-8',
    );
    const workspaceSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceWorkspace.tsx'),
      'utf-8',
    );
    expect(summarySource).toContain('compactAssessmentSummary');
    expect(summarySource).toContain('state.patrolStatus()?.trust?.regressed_at_least_once');
    expect(headerSource).not.toContain('aria-label="Patrol trust summary header"');
    expect(workspaceSource).not.toContain('aria-label="Patrol trust summary"');
  });

  it('names the proactive trust loop on the canonical Patrol surface', async () => {
    // The Patrol page header is the most visible piece of operator-facing
    // copy on the canonical Patrol surface. The IA framing must keep the
    // product boundary clear: Pulse probes and assembles evidence, the
    // configured model reasons over it, and action stays approval-bound.
    const { getPatrolPageHeaderMeta, PATROL_PAGE_DESCRIPTION, PATROL_PAGE_TITLE_TOOLTIP } =
      await import('@/utils/patrolPagePresentation');
    expect(PATROL_PAGE_DESCRIPTION).toContain('Pulse checks your infrastructure');
    expect(PATROL_PAGE_DESCRIPTION).toContain('configured model');
    expect(PATROL_PAGE_DESCRIPTION).toContain('right evidence');
    expect(PATROL_PAGE_DESCRIPTION).toContain('approval policy');
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toMatchObject({
      title: 'Patrol',
      description: PATROL_PAGE_DESCRIPTION,
      titleTooltip: PATROL_PAGE_DESCRIPTION,
    });
  });
});
