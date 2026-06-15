import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PatrolRunRecord } from '@/api/patrol';
import { PatrolIntelligenceSummary } from '../PatrolIntelligenceSummary';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

describe('PatrolIntelligenceSummary', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('keeps the compact assessment strip descriptive only', () => {
    const patrolState = createPatrolState();
    render(() => <PatrolIntelligenceSummary state={patrolState} />);

    expect(screen.getByText('Patrol assessment')).toBeInTheDocument();
    expect(screen.getByText('1 critical · 84/100')).toBeInTheDocument();
    expect(screen.queryByTestId('patrol-recommended-next-step')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-recommended-next-step-action')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details-toggle')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-assessment-assistant-button')).not.toBeInTheDocument();
  });

  it('surfaces recent activity mix and trigger mode in the compact assessment strip', () => {
    const patrolState = {
      ...createPatrolState(),
      patrolRunHistory: makePatrolRunHistory([
        makePatrolRunRecord({
          id: 'run-alert-scoped',
          started_at: todayAt(11, 0),
          completed_at: todayAt(11, 2),
          type: 'scoped',
          trigger_reason: 'alert_fired',
          new_findings: 1,
        }),
        makePatrolRunRecord({
          id: 'run-anomaly-scoped',
          started_at: todayAt(10, 15),
          completed_at: todayAt(10, 16),
          type: 'scoped',
          trigger_reason: 'anomaly',
        }),
        makePatrolRunRecord({
          id: 'run-full-review',
          started_at: todayAt(9, 0),
          completed_at: todayAt(9, 3),
          type: 'full',
          trigger_reason: 'scheduled',
        }),
      ]),
      patrolStatus: () => ({
        trigger_status: {
          running: false,
          pending_triggers: 4,
          current_interval_ms: 10000,
          recent_events: 12,
          is_busy_mode: true,
          alert_triggers_enabled: true,
          anomaly_triggers_enabled: false,
        },
      }),
    } as unknown as PatrolIntelligenceState;

    render(() => <PatrolIntelligenceSummary state={patrolState} />);

    expect(
      screen.getByText('Recent activity mix: 1 full, 1 alert-triggered, 1 anomaly-triggered'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Trigger mode: 4 queued · busy mode · anomalies off'),
    ).toBeInTheDocument();
  });

  it('explains runtime-blocked event triggers in the compact assessment strip', () => {
    const patrolState = {
      ...createPatrolState(),
      patrolStatus: () => ({
        trigger_status: {
          running: false,
          pending_triggers: 0,
          current_interval_ms: 300000,
          recent_events: 0,
          is_busy_mode: false,
          alert_triggers_enabled: true,
          anomaly_triggers_enabled: true,
          event_triggers_blocked: true,
          event_triggers_blocked_reason: 'background_automation_disabled',
          event_triggers_blocked_message:
            'Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
        },
      }),
    } as unknown as PatrolIntelligenceState;

    render(() => <PatrolIntelligenceSummary state={patrolState} />);

    expect(
      screen.getByText(
        'Trigger mode: Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
      ),
    ).toBeInTheDocument();
  });

  it('does not surface disabled run actions on the compact assessment strip', () => {
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

    expect(screen.queryByTestId('patrol-recommended-next-step')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-recommended-next-step-action')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details-toggle')).not.toBeInTheDocument();
  });

  it('does not leave the loading shell behind once patrol evidence has rendered', () => {
    const patrolState = {
      ...createPatrolState(),
      initialSurfaceReady: () => false,
      intelligenceSummary: () => null,
    } satisfies PatrolIntelligenceState;

    render(() => <PatrolIntelligenceSummary state={patrolState} />);

    expect(screen.queryByTestId('patrol-summary-loading')).not.toBeInTheDocument();
  });
});

function todayAt(hours: number, minutes: number): string {
  const value = new Date();
  value.setHours(hours, minutes, 0, 0);
  return value.toISOString();
}

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
