import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import type { InvestigationRecord, RemediationPlan } from '@/api/ai';
import type { PatrolRunRecord } from '@/api/patrol';

import {
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantFindingBriefing,
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantFindingHandoffActions,
  buildPatrolAssistantProposedFixBriefingInput,
  buildPatrolRunAssistantHandoff,
  buildPatrolInvestigationRecordPresentation,
  buildPatrolRemediationPlanAssistantBriefing,
  buildPatrolRemediationPlanAssistantModelContext,
  patrolAssistantFindingHandoffRequiresApprovalMode,
} from '../patrolInvestigationContextModel';

describe('patrolInvestigationContextModel', () => {
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

    expect(handoff).not.toHaveProperty('prompt');
    expect(JSON.stringify(handoff.context)).not.toContain('tool_choice');
    expect(JSON.stringify(handoff.context)).not.toContain('No endpoints found');
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
      runType: 'Targeted check',
      runStatus: 'error',
      runtimeFailure: true,
    });
    expect(handoff.context.handoffContext).toBeUndefined();
    expect(handoff.context.briefing).toMatchObject({
      sourceLabel: 'Pulse Patrol',
      title: 'Patrol run attached',
      actionLabel: 'Review Patrol runtime failure',
    });
    expect(JSON.stringify(handoff)).not.toContain('provider trace');
    expect(JSON.stringify(handoff)).not.toContain('tool_choice');
    expect(JSON.stringify(handoff)).not.toContain('No endpoints found');
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

  it('normalizes safe action artifact briefing metadata without command text', () => {
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
        actionState: 'pending',
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

  it('keeps a typed pending action approval-bound without a legacy approval id', () => {
    const [action] = buildPatrolAssistantFindingHandoffActions({
      id: 'finding-typed-action',
      title: 'Unhealthy workload',
      severity: 'warning',
      status: 'active',
      resourceId: 'docker:container:web',
      pendingApproval: {
        id: '',
        status: 'pending_approval',
        riskLevel: 'governed',
        requestedAt: '2026-07-10T18:00:00Z',
        actionId: 'action-typed-1',
        actionApprovalPolicy: 'admin',
        actionRequestedBy: 'pulse_patrol',
      },
    });

    expect(action).toMatchObject({
      findingId: 'finding-typed-action',
      actionId: 'action-typed-1',
      actionApprovalPolicy: 'admin',
      actionRequiresApproval: true,
    });
    expect(action.approvalId).toBeUndefined();
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

    expect(handoff).not.toHaveProperty('prompt');
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

  it('qualifies the finding resource with its record node and bounds long context lines', () => {
    const description = 'Disk latency spiked during the backup window. '.repeat(20).trim();
    const handoff = buildPatrolAssistantFindingHandoff({
      id: 'finding-node',
      title: 'Disk latency',
      subject: 'web-server',
      description,
      investigationRecord: {
        id: 'record-node',
        finding_id: 'finding-node',
        subject: {
          resource_id: 'vm-100',
          resource_name: 'web-server',
          resource_type: 'vm',
          node: 'pve1',
        },
        status: 'completed',
      } as unknown as InvestigationRecord,
    });

    const lines = handoff.context.handoffContext?.split('\n') ?? [];
    expect(lines).toContain('Resource: web-server (vm vm-100 node pve1)');
    const descriptionLine = lines.find((line) => line.startsWith('Description: '));
    expect(description.length).toBeGreaterThan(500);
    expect(descriptionLine?.endsWith('...')).toBe(true);
    expect(descriptionLine?.slice('Description: '.length).length).toBeLessThanOrEqual(500);
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
      title: 'Patrol finding attached',
      subject: 'High CPU usage on web-server',
      statusLabel: 'Completed · Fix Queued · High confidence',
      detailLines: [
        'Backup job saturated CPU.',
        'Existing action artifact: Restart the workload service · medium risk · 1 command recorded for approval context · destructive action artifact',
      ],
      evidence: ['CPU stayed above 95% for 10 minutes', 'Verified: CPU returned below 50%'],
      actionLabel: undefined,
      commandSummary: '1 command recorded for approval context',
      safetyNote:
        'Command details stay in approval context. Destructive actions require governed approval.',
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

    expect(modelContext).toContain('[Patrol Finding Action Context]');
    expect(modelContext).toContain(
      'Pulse is attaching observed finding context and any existing governed action artifact',
    );
    expect(modelContext).toContain(
      'The selected language model should decide whether remediation is appropriate',
    );
    expect(modelContext).toContain('Existing Action Artifact: Restore web service');
    expect(modelContext).toContain(
      '2 commands recorded for governed plan review · 1 rollback command recorded',
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
      '2 commands recorded for governed plan review · 1 rollback command recorded',
    );
    expect(briefing.safetyNote).toBe(
      'Assistant should decide remediation from evidence. Command execution requires governed approval.',
    );
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
        investigationOutcome: 'fix_rejected',
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
        actionReferenceCount: 0,
      },
    });
    expect(handoff).not.toHaveProperty('prompt');
    expect(handoff.context.briefing).toMatchObject({
      title: 'Patrol finding attached',
      subject: 'Provider connection issue on Patrol runtime',
    });
    expect(handoff.context.briefing?.actionLabel).toBeUndefined();
    expect(handoff.context.briefing?.actionHref).toBeUndefined();
    expect(handoff.context.handoffMetadata).toEqual({
      kind: 'patrol_finding',
    });
    expect(handoff.context.handoffActions).toBeUndefined();
    expect(handoff.context.handoffContext).not.toContain('Patrol Next Step');
    expect(handoff.context.handoffContext).toContain(
      'Model Boundary: This Patrol finding handoff is model-only context',
    );
  });

  it('builds a finding briefing from current finding facts before a Patrol record exists', () => {
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
      title: 'Patrol finding attached',
      subject: 'High CPU usage on web-server',
      statusLabel: undefined,
      detailLines: [],
      evidence: [],
      actionLabel: undefined,
      commandSummary: undefined,
      safetyNote: undefined,
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
      title: 'Patrol finding attached',
      subject: 'CPU saturation on node-1',
      statusLabel: 'Pending approval · High risk',
      detailLines: [],
      evidence: [],
      actionLabel: undefined,
      commandSummary: undefined,
      safetyNote: 'Execution requires the governed approval flow.',
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
      title: 'Patrol finding attached',
      subject: 'CPU saturation on node-1',
      statusLabel: 'Fix Queued',
      detailLines: [],
      evidence: [],
      actionLabel: undefined,
      commandSummary: undefined,
      safetyNote: undefined,
    });
  });

  it('builds queued-fix recovery briefing from safe action artifact metadata', () => {
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
      title: 'Patrol finding attached',
      subject: 'CPU saturation on node-1',
      statusLabel: 'Fix Queued',
      detailLines: [
        'Existing action artifact: Restart workload service · target node-1 · high risk · 1 command recorded for approval context · destructive action artifact · rationale service is wedged',
      ],
      evidence: [],
      actionLabel: undefined,
      commandSummary: '1 command recorded for approval context',
      safetyNote:
        'Command details stay in approval context. Destructive actions require governed approval.',
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

  it('keeps trust counters out of default Patrol chrome', () => {
    // Trust counters remain backend/state evidence. The default Patrol surface
    // should not reintroduce a second current-work/history strip to explain them.
    const surfaceSource = readFileSync(
      resolve(__dirname, '..', 'PatrolIntelligenceSurface.tsx'),
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
    expect(surfaceSource).not.toContain('PatrolIntelligenceSummary');
    expect(surfaceSource).not.toContain('compactAssessmentSummary');
    expect(surfaceSource).not.toContain('state.patrolStatus()?.trust?.regressed_at_least_once');
    expect(headerSource).not.toContain('aria-label="Patrol trust summary header"');
    expect(workspaceSource).not.toContain('aria-label="Patrol trust summary"');
  });

  it('names the operator job on the canonical Patrol surface when control is available', async () => {
    // The Patrol page header is the most visible piece of operator-facing
    // copy on the canonical Patrol surface. The IA framing must keep the
    // product boundary clear without turning the page header into a
    // mini spec for the full operations loop.
    const { getPatrolPageHeaderMeta, PATROL_PAGE_DESCRIPTION, PATROL_PAGE_TITLE_TOOLTIP } =
      await import('@/utils/patrolPagePresentation');
    expect(PATROL_PAGE_DESCRIPTION).toBe(
      'See what needs a decision, choose the next step, and keep a verified record.',
    );
    expect(PATROL_PAGE_DESCRIPTION).toContain('verified record');
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toMatchObject({
      title: 'Patrol',
      description: PATROL_PAGE_DESCRIPTION,
      titleTooltip: PATROL_PAGE_DESCRIPTION,
    });
    expect(getPatrolPageHeaderMeta({ autonomyLocked: true })).toMatchObject({
      title: 'Patrol',
      description: PATROL_PAGE_DESCRIPTION,
      titleTooltip: PATROL_PAGE_DESCRIPTION,
    });
  });
});
