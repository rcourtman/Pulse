import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal, type JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { PatrolIntelligenceWorkspace } from '../PatrolIntelligenceWorkspace';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

const fixture = vi.hoisted(() => {
  const finding = (
    id: string,
    title: string,
    resource: { resourceId: string; resourceName: string },
    extra: Record<string, unknown> = {},
  ) => ({
    id,
    title,
    ...resource,
    source: 'ai-patrol',
    resourceType: 'app-container',
    category: 'reliability',
    severity: 'warning',
    description: `${title}.`,
    detectedAt: '2026-10-24T08:00:00Z',
    lastSeenAt: '2026-10-24T09:00:00Z',
    status: 'active',
    ...extra,
  });
  const web = { resourceId: 'app-container:host-1:web', resourceName: 'afs-web' };
  const findings = [
    finding('web-disk', 'Web volume is filling', web, { timesRaised: 3 }),
    finding('web-restart', 'Web container keeps restarting', web, {
      investigationOutcome: 'fix_queued',
    }),
    finding(
      'db-backup',
      'Database backup is stale',
      { resourceId: 'vm-db-01', resourceName: 'db-01' },
      { investigationOutcome: 'fix_queued' },
    ),
    finding(
      'cache-evictions',
      'Cache evicts hot keys',
      { resourceId: 'vm-cache-01', resourceName: 'cache-01' },
      { investigationOutcome: 'fix_failed' },
    ),
  ];
  return {
    finding,
    web,
    findings,
    // Tests that change the findings swap in a signal accessor.
    read: () => findings,
    attentionIds: ['web-disk', 'web-restart', 'db-backup', 'cache-evictions'],
    approvalIds: ['web-restart', 'db-backup'],
  };
});

vi.mock('@solidjs/router', () => ({
  A: (props: { href: string; children?: JSX.Element }) => <a href={props.href}>{props.children}</a>,
  useLocation: () => ({ hash: '' }),
}));

vi.mock('@/components/patrol', async () => {
  const { ApprovalBanner } = await vi.importActual<
    typeof import('@/components/patrol/ApprovalBanner')
  >('@/components/patrol/ApprovalBanner');
  return {
    ApprovalBanner,
    ApprovalSection: () => null,
    InvestigationSection: () => null,
    RunHistoryPanel: () => null,
  };
});

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ get: () => undefined }),
}));

vi.mock('../usePatrolIntelligenceState', () => ({
  resolvePatrolBlockedActionCause: (blocked?: string, readiness?: string) =>
    blocked || readiness || undefined,
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get findings() {
      return [];
    },
    get patrolFindings() {
      return fixture.read();
    },
    findingsLoading: false,
    patrolFindingsLoading: false,
    findingsError: null,
    patrolFindingsError: null,
    get findingsNeedingAttention() {
      return [];
    },
    get patrolFindingsNeedingAttention() {
      return fixture.read().filter((finding) => fixture.attentionIds.includes(finding.id));
    },
    get findingsWithPendingApprovals() {
      return [];
    },
    get patrolFindingsWithPendingApprovals() {
      return fixture.read().filter((finding) => fixture.approvalIds.includes(finding.id));
    },
    get patrolPendingApprovals() {
      return fixture.approvalIds.map((targetId) => ({
        id: `approval-${targetId}`,
        status: 'pending',
        toolId: 'investigation_fix',
        targetId,
        context: `Apply the fix for ${targetId}`,
        riskLevel: 'medium',
        expiresAt: '2099-01-01T00:00:00Z',
      }));
    },
    findingsSignal: () => [],
    patrolFindingsSignal: () => fixture.read(),
    loadFindings: vi.fn(),
    loadPatrolFindings: vi.fn(),
  },
}));

// Like the Patrol state hook, the queue holds only active findings.
const activeFindings = () => fixture.read().filter((finding) => finding.status === 'active');

// The latest check failed and its evidence is stale, so the unscoped queue
// shows both Patrol check groups.
const makeState = () =>
  ({
    activeTab: () => 'findings',
    autoFixLocked: () => false,
    autonomyLevel: () => 'monitor',
    blockedCause: () => '',
    blockedReason: () => '',
    displayRunHistory: () => [],
    findingsFilterOverride: () => 'active',
    findingsTabBadgeCount: () => activeFindings().length,
    findingsTabBadgeFindings: () => activeFindings(),
    handleAutonomyChange: vi.fn(),
    handleRefreshPatrol: vi.fn(),
    handleRunPatrol: vi.fn(),
    historicalRegressionCount: () => 0,
    intelligenceSummary: () => null,
    isUpdatingAutonomy: () => false,
    patrolPendingApprovalCount: () => fixture.approvalIds.length,
    patrolReadiness: () => null,
    patrolRunHistory: {
      value: () => [
        {
          completed_at: '2026-10-20T08:00:00Z',
          error_count: 2,
          resources_checked: 12,
          status: 'error',
        },
      ],
      loading: () => false,
    },
    patrolStatus: () => ({
      error_count: 2,
      next_patrol_at: '2026-10-20T14:00:00Z',
      running: false,
    }),
    patrolStream: {},
    runtimeState: () => 'active',
    selectedRun: () => null,
    selectedRunFindingIds: () => undefined,
    selectedRunHasFindingsSnapshot: () => undefined,
    selectedRunScopeResourceIds: () => [],
    setActiveTab: vi.fn(),
    setFindingsFilterOverride: vi.fn(),
    setSelectedRun: vi.fn(),
    shouldShowPatrolSetupOnly: () => false,
    triggerPatrolDisabledReason: () => '',
  }) as unknown as PatrolIntelligenceState;

const workGroupLabels = () =>
  within(screen.getByRole('list', { name: 'Patrol work groups' }))
    .getAllByRole('listitem')
    .map((item) => item.querySelector('span')?.textContent?.trim());
const reviewRows = () =>
  screen
    .getAllByRole('button', { name: /^Review issue for / })
    .map((button) => button.getAttribute('aria-label'));

describe('PatrolIntelligenceWorkspace resource scope', () => {
  afterEach(() => {
    cleanup();
    fixture.read = () => fixture.findings;
  });

  it('describes the scoped resource with the counts its rows show', () => {
    render(() => (
      <PatrolIntelligenceWorkspace
        state={makeState()}
        findingResourceId="app-container:host-1:web"
        findingResourceName="afs-web"
      />
    ));

    expect(
      screen.getByText(
        'Patrol found 2 issues on afs-web — 1 needs approval, 1 recurring. Open a row to review evidence and record the outcome.',
      ),
    ).toBeInTheDocument();
    expect(workGroupLabels()).toEqual(['1 approval waiting', '1 recurring issue']);
    // The banner shows the resource's own approval, not every pending approval.
    expect(screen.getByText('Action awaiting approval')).toBeInTheDocument();
    expect(screen.getByText('Apply the fix for web-restart')).toBeInTheDocument();
    expect(screen.queryByText(/actions awaiting your approval/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Needs Attention \(2\)$/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Approvals \(1\)$/ })).toBeInTheDocument();
    expect(reviewRows()).toEqual([
      'Review issue for Web container keeps restarting',
      'Review issue for Web volume is filling',
    ]);
  });

  it('keeps the whole-queue summary without a resource scope', () => {
    render(() => <PatrolIntelligenceWorkspace state={makeState()} />);

    expect(
      screen.getByText(
        'Patrol found 4 issues on 3 affected resources — 2 need approval, 1 failed fix, 1 recurring. Open a row to review evidence and record the outcome.',
      ),
    ).toBeInTheDocument();
    expect(workGroupLabels()).toEqual([
      '2 approvals waiting',
      '1 failed action',
      'Latest check needs review',
      '1 recurring issue',
      'Coverage stale',
    ]);
    expect(screen.getByText('2 actions awaiting your approval')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Needs Attention \(4\)$/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Approvals \(2\)$/ })).toBeInTheDocument();
  });

  it('follows a findings refresh and the end of the scope', () => {
    const [findings, setFindings] = createSignal(fixture.findings);
    fixture.read = findings;
    const [scoped, setScoped] = createSignal(true);
    render(() => (
      <PatrolIntelligenceWorkspace
        state={makeState()}
        findingResourceId={scoped() ? fixture.web.resourceId : undefined}
        findingResourceName={scoped() ? fixture.web.resourceName : undefined}
      />
    ));
    expect(
      screen.getByText(/^Patrol found 2 issues on afs-web — 1 needs approval, 1 recurring\./),
    ).toBeInTheDocument();

    // A refresh that raises another finding on the resource moves every scoped count.
    setFindings([
      ...fixture.findings,
      fixture.finding('web-cert', 'Web certificate renewal keeps slipping', fixture.web, {
        timesRaised: 2,
      }),
    ]);
    expect(
      screen.getByText(/^Patrol found 3 issues on afs-web — 1 needs approval, 2 recurring\./),
    ).toBeInTheDocument();
    expect(workGroupLabels()).toEqual(['1 approval waiting', '2 recurring issues']);
    expect(reviewRows()).toHaveLength(3);

    // web-restart drops out of the loaded findings while its approval is still
    // pending: nothing ties that approval to the resource, so the banner and the
    // approvals group go together.
    const withCert = findings();
    setFindings(withCert.filter((finding) => finding.id !== 'web-restart'));
    expect(screen.queryByText('Action awaiting approval')).not.toBeInTheDocument();
    expect(workGroupLabels()).toEqual(['2 recurring issues']);

    // It returns resolved with the approval still pending: both come back, while
    // the open work stays at the two active findings.
    setFindings([...findings(), { ...fixture.findings[1], status: 'resolved' }]);
    expect(screen.getByText('Action awaiting approval')).toBeInTheDocument();
    expect(workGroupLabels()).toEqual(['1 approval waiting', '2 recurring issues']);
    expect(
      screen.getByText(/^Patrol found 2 issues on afs-web — 2 recurring\./),
    ).toBeInTheDocument();
    expect(reviewRows()).toHaveLength(2);

    // Show all finding options ends the scope: the whole queue again.
    setScoped(false);
    expect(
      screen.getByText(
        /^Patrol found 4 issues on 3 affected resources — 1 needs approval, 1 failed fix, 2 recurring\./,
      ),
    ).toBeInTheDocument();
    expect(workGroupLabels()).toEqual([
      '2 approvals waiting',
      '1 failed action',
      'Latest check needs review',
      '2 recurring issues',
      'Coverage stale',
    ]);
    expect(screen.getByText('2 actions awaiting your approval')).toBeInTheDocument();
  });
});
