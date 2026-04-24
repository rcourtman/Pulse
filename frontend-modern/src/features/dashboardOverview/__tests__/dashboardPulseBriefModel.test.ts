import { describe, expect, it } from 'vitest';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';
import { buildDashboardPulseBrief } from '../dashboardPulseBriefModel';
import type { DashboardEstateSummary } from '../estateSummaryModel';

const estate = (overrides: Partial<DashboardEstateSummary> = {}): DashboardEstateSummary => ({
  hasCanonicalProjection: true,
  totalSystems: 2,
  activeSystems: 2,
  healthySystems: 2,
  degradedSystems: 0,
  offlineSystems: 0,
  unknownSystems: 0,
  ignoredSystems: 0,
  outdatedSystems: 0,
  attentionSystems: 0,
  headline: '2 systems reporting',
  detail: '2 online',
  tone: 'healthy',
  surfaces: [],
  systems: [],
  ...overrides,
});

const overview = (overrides: Partial<DashboardOverview> = {}): DashboardOverview => ({
  health: {
    totalResources: 3,
    byStatus: {},
    criticalAlerts: 0,
    warningAlerts: 0,
  },
  infrastructure: {
    total: 2,
    byStatus: {},
    byType: {},
    topCPU: [],
    topMemory: [],
  },
  workloads: {
    total: 4,
    running: 4,
    stopped: 0,
    byType: {},
  },
  storage: {
    total: 1,
    totalCapacity: 100,
    totalUsed: 40,
    warningCount: 0,
    criticalCount: 0,
  },
  alerts: {
    activeCritical: 0,
    activeWarning: 0,
    total: 0,
  },
  problemResources: [],
  ...overrides,
});

const recovery = (overrides: Partial<DashboardRecoverySummary> = {}): DashboardRecoverySummary => ({
  totalProtected: 2,
  byOutcome: { success: 2 },
  latestEventTimestamp: Date.parse('2026-04-23T12:00:00Z'),
  hasData: true,
  ...overrides,
});

describe('dashboard Pulse Brief model', () => {
  it('builds a steady operator brief from dashboard facts', () => {
    const brief = buildDashboardPulseBrief({
      estate: estate(),
      overview: overview(),
      storageCapacityPercent: 40,
      recovery: recovery(),
      pendingApprovalCount: 0,
      patrolFindingCount: 0,
    });

    expect(brief.tone).toBe('healthy');
    expect(brief.body).toContain('All 2 monitored systems are reporting cleanly');
    expect(brief.body).toContain('no pending approvals, active alerts, or Patrol findings');
    expect(brief.evidence).toContain('No active dashboard issues');
    expect(brief.assistantPrompt).toContain('Use only these dashboard facts');
    expect(brief.assistantPrompt).toContain('do not run commands or change anything');
  });

  it('prioritizes concrete problem resources before lower-level context', () => {
    const brief = buildDashboardPulseBrief({
      estate: estate({ headline: '1 system needs attention', attentionSystems: 1 }),
      overview: overview({
        alerts: { activeCritical: 1, activeWarning: 1, total: 2 },
        problemResources: [
          {
            resource: {
              id: 'vm-101',
              type: 'vm',
              name: 'vm-101',
              displayName: 'database-vm',
              status: 'offline',
            } as DashboardOverview['problemResources'][number]['resource'],
            problems: ['Offline'],
            worstValue: 200,
          },
        ],
      }),
      storageCapacityPercent: 40,
      recovery: recovery(),
      pendingApprovalCount: 0,
      patrolFindingCount: 1,
    });

    expect(brief.tone).toBe('critical');
    expect(brief.body).toContain('1 system needs attention');
    expect(brief.body).toContain('Review database-vm (Offline) first');
    expect(brief.evidence).toEqual(
      expect.arrayContaining(['1 resource issue', '2 active alerts', '1 Patrol finding']),
    );
    expect(brief.assistantContext.problemResources).toEqual([
      { id: 'vm-101', name: 'database-vm', problems: ['Offline'] },
    ]);
  });

  it('does not duplicate a problem reason that is already in the resource name', () => {
    const brief = buildDashboardPulseBrief({
      estate: estate({ headline: '1 system needs attention', attentionSystems: 1 }),
      overview: overview({
        problemResources: [
          {
            resource: {
              id: 'storage-offline',
              type: 'storage',
              name: 'storage (offline)',
              displayName: 'storage (offline)',
              status: 'offline',
            } as DashboardOverview['problemResources'][number]['resource'],
            problems: ['Offline'],
            worstValue: 200,
          },
        ],
      }),
      storageCapacityPercent: 40,
      recovery: recovery(),
      pendingApprovalCount: 0,
      patrolFindingCount: 0,
    });

    expect(brief.body).toContain('Review storage (offline) first');
    expect(brief.body).not.toContain('storage (offline) (Offline)');
  });
});
