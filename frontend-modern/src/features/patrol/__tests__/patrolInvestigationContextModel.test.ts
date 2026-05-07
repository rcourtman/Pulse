import { describe, expect, it } from 'vitest';
import type { RemediationPlan } from '@/api/ai';
import type { PatrolRunRecord } from '@/api/patrol';

import {
  buildPatrolAssessmentAssistantHandoff,
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantFindingHandoffActions,
  buildPatrolAssistantFindingPrompt,
  buildPatrolAssistantProposedFixBriefingInput,
  buildPatrolInvestigationContextSummary,
  buildPatrolRunAssistantHandoff,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantPrompt,
  patrolAssistantFindingHandoffRequiresApprovalMode,
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
    expect(handoff.context.handoffResources).toEqual([{ id: 'vm-100', type: 'vm' }]);
    expect(handoff.context.handoffMetadata).toEqual({
      kind: 'patrol_run',
      runId: 'run-runtime-error',
      runType: 'Scoped run',
      runStatus: 'error',
      runtimeFailure: true,
    });
    expect(handoff.context.handoffContext).toContain('[Patrol Run Context]');
    expect(handoff.context.handoffContext).toContain('Source: Pulse Patrol run history');
    expect(handoff.context.handoffContext).toContain('Run Type: Scoped run');
    expect(handoff.context.handoffContext).toContain('Runtime Failure: Selected model');
    expect(handoff.context.handoffContext).toContain('tool_choice');
    expect(handoff.context.handoffContext).toContain('Patrol Analysis: Visible runtime summary.');
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

    expect(prompt).toContain('Start by reviewing the governed proposed fix');
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
      suggestedPrompts: [
        'Review approval risk and next step',
        'Explain Patrol evidence and confidence',
        'Summarize remediation without command text',
      ],
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
    expect(briefing.suggestedPrompts).toEqual([
      'Review plan risk and prerequisites',
      'Explain commands without command text',
      'Check rollback and verification steps',
    ]);
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
