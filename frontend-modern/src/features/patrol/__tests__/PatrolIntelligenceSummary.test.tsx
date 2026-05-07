import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { aiChatStore } from '@/stores/aiChat';
import { PatrolIntelligenceSummary } from '../PatrolIntelligenceSummary';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

describe('PatrolIntelligenceSummary', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('opens Assistant with model-only context for the current Patrol assessment', () => {
    const openWithPrompt = vi.spyOn(aiChatStore, 'openWithPrompt').mockImplementation(() => {});

    render(() => <PatrolIntelligenceSummary state={createPatrolState()} />);

    fireEvent.click(screen.getByTestId('patrol-assessment-assistant-button'));

    expect(openWithPrompt).toHaveBeenCalledTimes(1);
    const [prompt, context] = openWithPrompt.mock.calls[0] as [string, Record<string, unknown>];
    expect(prompt).toContain('Discuss the current Pulse Patrol assessment');
    expect(context.autonomousMode).toBe(false);
    expect(context.handoffContext).toContain('[Patrol Assessment Context]');
    expect(context.handoffContext).toContain('Source: Pulse Patrol current assessment');
    expect(context.handoffContext).toContain('Supporting Context: 1 recent change');
    expect(context.handoffContext).toContain('Finding 1: High CPU usage');
    expect(context.handoffResources).toEqual([
      { id: 'vm-100', name: 'web-server', type: 'vm', node: 'pve-1' },
    ]);
    expect(JSON.stringify(context)).not.toContain('systemctl restart workload.service');
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
    circuitBreakerStatus: () => undefined,
    correlationTotal: () => 2,
    hasInvestigationContext: () => true,
    initialSurfaceReady: () => true,
    intelligenceSummary: () => ({
      overall_health: {
        grade: 'B',
        score: 84,
        factors: [],
        prediction: 'Patrol surfaced one active critical finding.',
      },
      recent_changes_count: 1,
      policy_posture: {
        total_resources: 4,
        sensitivity_counts: {},
        routing_counts: {},
      },
    }),
    investigationContextSummary: () =>
      '1 recent change · 2 correlations · 4 policy-covered resources',
    patrolRunHistory: {
      value: () => [
        {
          id: 'run-1',
          started_at: '2026-05-06T12:00:00Z',
          completed_at: '2026-05-06T12:10:00Z',
          type: 'full',
          status: 'issues_found',
          resources_checked: 12,
          new_findings: 1,
          error_count: 0,
          finding_ids: ['finding-1'],
        },
      ],
    },
    patrolStatus: () => ({
      last_patrol_at: '2026-05-06T12:10:00Z',
      last_activity_at: '2026-05-06T12:10:00Z',
    }),
    policyPosture: () => ({
      total_resources: 4,
      sensitivity_counts: {},
      routing_counts: {},
    }),
    recentChangeCount: () => 1,
    runtimeState: () => 'active',
    summaryStats: () => ({
      criticalFindings: 1,
      warningFindings: 0,
      totalActive: 1,
      fixedCount: 0,
      hasAnyPatrolFindings: true,
    }),
  } as unknown as PatrolIntelligenceState;
}
