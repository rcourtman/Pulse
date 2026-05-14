import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { aiChatStore } from '@/stores/aiChat';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import type { PatrolRunRecord } from '@/api/patrol';
import { PatrolIntelligenceSummary } from '../PatrolIntelligenceSummary';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

describe('PatrolIntelligenceSummary', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('opens Assistant with model-only context for the current Patrol assessment', async () => {
    const openWithPrompt = vi.spyOn(aiChatStore, 'openWithPrompt').mockImplementation(() => {});
    const loadPendingApprovals = vi
      .spyOn(aiIntelligenceStore, 'loadPendingApprovals')
      .mockResolvedValue(undefined);
    vi.spyOn(aiIntelligenceStore, 'patrolPendingApprovals', 'get').mockReturnValue([
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart workload.service',
        targetType: 'finding',
        targetId: 'finding-1',
        targetName: 'web-server',
        context: 'Restart the workload service after backup pressure clears.',
        riskLevel: 'high',
        status: 'pending',
        requestedAt: '2026-05-06T12:00:00Z',
        expiresAt: '2026-05-06T12:10:00Z',
        plan: {
          actionId: 'action-1',
          requiresApproval: true,
          approvalPolicy: 'admin',
          message: 'Restart after the backup window clears.',
          expiresAt: '2026-05-06T12:10:00Z',
        },
        preflight: {
          intendedChange: 'Restart workload service',
          dryRunAvailable: false,
          dryRunSummary: 'No provider-supported dry run is available for this action.',
        },
      },
    ]);

    const patrolState = {
      ...createPatrolState(),
      summaryDetailsExpanded: () => true,
    };
    render(() => <PatrolIntelligenceSummary state={patrolState} />);

    expect(screen.getByTestId('patrol-recommended-next-step').textContent).toContain(
      'Review the pending Patrol approval',
    );
    expect(screen.getByTestId('patrol-summary-details').textContent).toContain(
      'risk, dry-run posture, and expiry',
    );

    fireEvent.click(screen.getByTestId('patrol-recommended-next-step-action'));

    expect(patrolState.setSelectedRun).toHaveBeenCalledWith(null);
    expect(patrolState.setActiveTab).toHaveBeenCalledWith('findings');
    expect(patrolState.setFindingsFilterOverride).toHaveBeenCalledWith('approvals');

    fireEvent.click(screen.getByTestId('patrol-assessment-assistant-button'));

    await waitFor(() => expect(openWithPrompt).toHaveBeenCalledTimes(1));
    expect(loadPendingApprovals).toHaveBeenCalledTimes(1);
    const [prompt, context] = openWithPrompt.mock.calls[0] as [string, Record<string, unknown>];
    expect(prompt).toContain('Discuss the current Pulse Patrol assessment');
    expect(prompt).toContain('Start by reviewing 1 pending governed approval');
    expect(prompt).toContain('approval policy, dry-run posture');
    expect(prompt).toContain('Do not infer, repeat, or execute raw command text');
    expect(context.autonomousMode).toBe(false);
    expect(context.handoffContext).toContain('[Patrol Assessment Context]');
    expect(context.handoffContext).toContain('Source: Pulse Patrol current assessment');
    expect(context.handoffContext).toContain('Supporting Context: 2 recent changes');
    expect(context.handoffContext).toContain(
      'Recommended Next Step: Review the pending Patrol approval',
    );
    expect(context.handoffContext).toContain(
      'Recommended Next Step Action: Review approvals (review_approvals)',
    );
    expect(context.handoffContext).toContain('Recent Change 1: Metric anomaly');
    expect(context.handoffContext).toContain('Correlation 1: Nightly backup job');
    expect(context.handoffContext).toContain('Finding 1: High CPU usage');
    expect(context.handoffContext).toContain('approval approval-1');
    expect(context.handoffContext).toContain('live approval pending');
    expect(context.handoffContext).toContain('high risk');
    expect(context.handoffContext).toContain('approval target web-server');
    expect(context.handoffContext).toContain('expires 2026-05-06T12:10:00Z');
    expect(context.context).toMatchObject({
      pendingApprovalCount: 1,
      recommendedNextStepTitle: 'Review the pending Patrol approval',
      recommendedNextStepActionKind: 'review_approvals',
    });
    expect(context.handoffResources).toEqual([
      { id: 'vm-100', name: 'web-server', type: 'vm', node: 'pve-1' },
      { id: 'backup-job', name: 'Nightly backup job', type: 'job', node: undefined },
    ]);
    expect(context.handoffActions).toHaveLength(1);
    expect((context.handoffActions as Array<Record<string, unknown>>)[0]).toMatchObject({
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
    });
    expect(context.briefing).toMatchObject({
      actionLabel: '1 pending governed approval attached',
      safetyNote:
        'Review approvals in the governed flow; approval policy is attached; dry-run posture is attached; raw command payloads stay out of Assistant.',
      suggestedPrompts: [
        'Prioritize findings and safest next step',
        'Explain recent changes and correlations',
        'Review pending approvals and safest next step',
      ],
    });
    expect((context.briefing as { detailLines?: string[] }).detailLines).toEqual(
      expect.arrayContaining([
        expect.stringContaining('Recommended next step: Review the pending Patrol approval'),
      ]),
    );
    expect(JSON.stringify(context)).not.toContain('systemctl restart workload.service');
  });

  it('passes current run-action availability into assessment Assistant handoff', async () => {
    const openWithPrompt = vi.spyOn(aiChatStore, 'openWithPrompt').mockImplementation(() => {});
    vi.spyOn(aiIntelligenceStore, 'loadPendingApprovals').mockResolvedValue(undefined);
    vi.spyOn(aiIntelligenceStore, 'patrolPendingApprovals', 'get').mockReturnValue([]);

    const patrolState = {
      ...createPatrolState(),
      activePatrolFindings: () => [],
      canTriggerPatrol: () => false,
      correlationTotal: () => 0,
      correlations: () => [],
      intelligenceSummary: () => ({
        timestamp: '2026-05-06T12:15:00Z',
        overall_health: {
          grade: 'C',
          score: 65,
          trend: 'stable',
          factors: [
            {
              category: 'coverage',
              name: 'Coverage',
              impact: -10,
              description: 'Patrol coverage is incomplete.',
            },
          ],
          prediction: 'Patrol coverage is incomplete.',
        },
        findings_count: {
          critical: 0,
          warning: 0,
          watch: 0,
          info: 0,
          total: 0,
        },
        predictions_count: 0,
        recent_changes_count: 0,
        recent_changes: [],
        policy_posture: {
          total_resources: 4,
          sensitivity_counts: {},
          routing_counts: {},
        },
        learning: {
          resources_with_knowledge: 0,
          total_notes: 0,
          resources_with_baselines: 0,
          patterns_detected: 0,
          correlations_learned: 0,
          incidents_tracked: 0,
        },
      }),
      manualRunRequested: () => false,
      patrolRunHistory: makePatrolRunHistory([
        makePatrolRunRecord({
          id: 'run-scoped-1',
          started_at: '2026-05-06T12:00:00Z',
          completed_at: '2026-05-06T12:01:00Z',
          type: 'scoped',
          status: 'error',
          error_count: 1,
        }),
      ]),
      patrolStream: {
        phase: () => 'Running',
        currentTool: () => '',
        tokens: () => 0,
        resynced: () => false,
        resyncReason: () => '',
        bufferStartSeq: () => 0,
        bufferEndSeq: () => 0,
        outputTruncated: () => false,
        reconnectCount: () => 0,
        isStreaming: () => true,
        errorMessage: () => '',
      },
      runtimeState: () => 'running',
      summaryDetailsExpanded: () => true,
      summaryStats: () => ({
        criticalFindings: 0,
        warningFindings: 0,
        totalActive: 0,
        fixedCount: 0,
        hasAnyPatrolFindings: false,
      }),
      triggerPatrolDisabledReason: () => 'Patrol is already running',
    } satisfies PatrolIntelligenceState;

    render(() => <PatrolIntelligenceSummary state={patrolState} />);

    fireEvent.click(screen.getByTestId('patrol-assessment-assistant-button'));

    await waitFor(() => expect(openWithPrompt).toHaveBeenCalledTimes(1));
    const [prompt, context] = openWithPrompt.mock.calls[0] as [string, Record<string, unknown>];

    expect(prompt).toContain(
      'Patrol-owned action "Run Patrol" is currently unavailable: Patrol is already running',
    );
    expect(context.handoffContext).toContain(
      'Recommended Next Step Action Status: unavailable - Patrol is already running',
    );
    expect(context.context).toMatchObject({
      recommendedNextStepActionKind: 'run_patrol',
      recommendedNextStepActionDisabledReason: 'Patrol is already running',
    });
    expect(context.briefing).toMatchObject({
      actionLabel: 'Recommended: Run Patrol',
      safetyNote: expect.stringContaining(
        'Run Patrol is currently unavailable: Patrol is already running.',
      ),
    });
  });
});

function createPatrolState(): PatrolIntelligenceState {
  return {
    activePatrolFindings: () => [
      {
        id: 'finding-1',
        source: 'ai-patrol',
        resourceId: 'vm-100',
        resourceName: 'web-server',
        resourceType: 'vm',
        category: 'performance',
        severity: 'critical',
        title: 'High CPU usage',
        description: 'CPU stayed above 95%.',
        detectedAt: '2026-05-06T12:00:00Z',
        lastSeenAt: '2026-05-06T12:10:00Z',
        status: 'active',
        investigationStatus: 'completed',
        investigationOutcome: 'fix_queued',
        loopState: 'awaiting_approval',
        timesRaised: 3,
        regressionCount: 1,
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
    ],
    blockedReason: () => undefined,
    canTriggerPatrol: () => true,
    circuitBreakerStatus: () => undefined,
    correlationTotal: () => 2,
    correlations: () => [
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
    hasInvestigationContext: () => true,
    initialSurfaceReady: () => true,
    isTriggeringPatrol: () => false,
    intelligenceSummary: () => ({
      overall_health: {
        grade: 'B',
        score: 84,
        factors: [],
        prediction: 'Patrol surfaced one active critical finding.',
      },
      recent_changes_count: 2,
      recent_changes: [
        {
          id: 'change-1',
          observedAt: '2026-05-06T12:08:00Z',
          resourceId: 'vm-100',
          kind: 'metric_anomaly',
          sourceType: 'heuristic',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
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
      policy_posture: {
        total_resources: 4,
        sensitivity_counts: {},
        routing_counts: {},
      },
    }),
    investigationContextSummary: () =>
      '2 recent changes · 2 correlations · 4 policy-covered resources',
    handleRunPatrol: vi.fn(),
    manualRunRequested: () => false,
    patrolRunHistory: makePatrolRunHistory([
      makePatrolRunRecord({
        id: 'run-1',
        started_at: '2026-05-06T12:00:00Z',
        completed_at: '2026-05-06T12:10:00Z',
        status: 'issues_found',
        resources_checked: 12,
        new_findings: 1,
        finding_ids: ['finding-1'],
      }),
    ]),
    patrolStatus: () => ({
      last_patrol_at: '2026-05-06T12:10:00Z',
      last_activity_at: '2026-05-06T12:10:00Z',
    }),
    policyPosture: () => ({
      total_resources: 4,
      sensitivity_counts: {},
      routing_counts: {},
    }),
    recentChangeCount: () => 2,
    runtimeState: () => 'active',
    patrolStream: {
      isStreaming: () => false,
    },
    supportingRecentChanges: () => [
      {
        id: 'change-1',
        observedAt: '2026-05-06T12:08:00Z',
        resourceId: 'vm-100',
        kind: 'metric_anomaly',
        sourceType: 'heuristic',
        sourceAdapter: 'proxmox_adapter',
        confidence: 'high',
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
    setActiveTab: vi.fn(),
    setFindingsFilterOverride: vi.fn(),
    setSelectedRun: vi.fn(),
    setSummaryDetailsExpanded: vi.fn(),
    summaryDetailsExpanded: () => false,
    summaryStats: () => ({
      criticalFindings: 1,
      warningFindings: 0,
      totalActive: 1,
      fixedCount: 0,
      hasAnyPatrolFindings: true,
    }),
    triggerPatrolDisabledReason: () => '',
  } as unknown as PatrolIntelligenceState;
}

function makePatrolRunRecord(overrides: Partial<PatrolRunRecord>): PatrolRunRecord {
  return {
    id: 'run-1',
    started_at: '2026-05-06T12:00:00Z',
    completed_at: '2026-05-06T12:10:00Z',
    duration_ms: 600000,
    type: 'full',
    resources_checked: 0,
    nodes_checked: 0,
    guests_checked: 0,
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
    findings_summary: '',
    finding_ids: [],
    error_count: 0,
    status: 'healthy',
    triage_flags: 0,
    tool_call_count: 0,
    ...overrides,
  };
}

function makePatrolRunHistory(
  runs: PatrolRunRecord[],
): PatrolIntelligenceState['patrolRunHistory'] {
  return {
    error: () => null,
    loading: () => false,
    refetch: vi.fn(async () => runs),
    resolvedOnce: () => true,
    value: () => runs,
  };
}
