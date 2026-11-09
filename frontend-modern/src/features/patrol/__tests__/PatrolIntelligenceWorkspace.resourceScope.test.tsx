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
    approvalsError: () => null as string | null,
    approvalsPending: () => false,
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
    get approvalsError() {
      return fixture.approvalsError();
    },
    get approvalsReadState() {
      return fixture.approvalsError()
        ? 'unavailable'
        : fixture.approvalsPending()
          ? 'pending'
          : 'ready';
    },
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
const makeState = (overrides: Record<string, unknown> = {}) =>
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
    ...overrides,
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
    fixture.approvalsError = () => null;
    fixture.approvalsPending = () => false;
  });

  it('qualifies the saved queued summary through failed reads and recovery, retaining its scoped count', () => {
    const [error, setError] = createSignal<string | null>('unavailable');
    fixture.approvalsError = error;
    render(() => (
      <PatrolIntelligenceWorkspace
        state={makeState()}
        findingResourceId={fixture.web.resourceId}
        findingResourceName="afs-web"
      />
    ));
    expect(
      screen.getByText(
        /^Patrol found 2 issues on afs-web — 1 needs approval · last known, 1 recurring\./,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Patrol found.*action recovery/)).not.toBeInTheDocument();
    expect(screen.getByText('Apply the fix for web-restart')).toBeInTheDocument();
    expect(screen.getByText('Last known')).toBeInTheDocument();
    expect(screen.getAllByRole('link', { name: 'Review in Actions' })).toHaveLength(1);
    expect(workGroupLabels()).toEqual(['1 recurring issue']);
    expect(reviewRows()).toHaveLength(2);
    setError(null);
    expect(
      screen.getByText(/^Patrol found 2 issues on afs-web — 1 needs approval, 1 recurring\./),
    ).toBeInTheDocument();
    expect(screen.queryByText('Last known')).not.toBeInTheDocument();
  });

  it('keeps the initial and refreshing empty read pending until it succeeds', () => {
    const [pending, setPending] = createSignal(true);
    fixture.approvalsPending = pending;
    const savedApprovalIds = fixture.approvalIds;
    fixture.approvalIds = [];
    try {
      render(() => (
        <PatrolIntelligenceWorkspace
          state={makeState()}
          findingResourceId={fixture.web.resourceId}
          findingResourceName="afs-web"
        />
      ));
      expect(
        screen.getByText(
          /^Patrol found 2 issues on afs-web — 1 queued · checking approval status, 1 recurring\./,
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText(/Patrol found.*action recovery/)).not.toBeInTheDocument();
      expect(reviewRows()).toHaveLength(2);
      setPending(false);
      expect(
        screen.getByText(
          /^Patrol found 2 issues on afs-web — 1 needs action recovery, 1 recurring\./,
        ),
      ).toBeInTheDocument();
      setPending(true);
      expect(screen.getByText(/1 queued · checking approval status/)).toBeInTheDocument();
    } finally {
      fixture.approvalIds = savedApprovalIds;
    }
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
    expect(workGroupLabels()).toEqual(['1 recurring issue']);
    // The banner shows the resource's own approval, not every pending approval.
    expect(screen.getByText('Action awaiting approval')).toBeInTheDocument();
    expect(screen.queryByText('1 approval waiting')).not.toBeInTheDocument();
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
      '1 failed action',
      'Latest check needs review',
      '1 recurring issue',
      'Coverage stale',
    ]);
    expect(screen.getByText('2 actions awaiting your approval')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Needs Attention \(4\)$/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Approvals \(2\)$/ })).toBeInTheDocument();
  });

  it('shows the work groups only once there is open work or a group beyond stale coverage', () => {
    // The last check was clean but is overdue, so stale coverage is the only
    // group Patrol has. Alone it is not open work.
    const staleOnly = {
      patrolPendingApprovalCount: () => 0,
      patrolRunHistory: {
        value: () => [
          {
            completed_at: '2026-10-20T08:00:00Z',
            error_count: 0,
            resources_checked: 12,
            status: 'completed',
          },
        ],
        loading: () => false,
      },
      patrolStatus: () => ({
        error_count: 0,
        next_patrol_at: '2026-10-20T14:00:00Z',
        running: false,
      }),
    };

    fixture.read = () => [];
    render(() => <PatrolIntelligenceWorkspace state={makeState(staleOnly)} />);
    expect(screen.queryByRole('list', { name: 'Patrol work groups' })).not.toBeInTheDocument();
    cleanup();

    // With one open issue the same stale coverage joins the list.
    fixture.read = () => [fixture.findings[2]];
    render(() => <PatrolIntelligenceWorkspace state={makeState(staleOnly)} />);
    expect(workGroupLabels()).toEqual(['Coverage stale']);
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
    expect(workGroupLabels()).toEqual(['2 recurring issues']);
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
    expect(screen.queryByText('1 approval waiting')).not.toBeInTheDocument();
    expect(workGroupLabels()).toEqual(['2 recurring issues']);
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
      '1 failed action',
      'Latest check needs review',
      '2 recurring issues',
      'Coverage stale',
    ]);
    expect(screen.getByText('2 actions awaiting your approval')).toBeInTheDocument();
  });
});

describe('Patrol workspace current action summary', () => {
  afterEach(() => {
    cleanup();
    fixture.read = () => fixture.findings;
    fixture.approvalsError = () => null;
    fixture.approvalsPending = () => false;
  });

  it('follows typed action transitions in the selected resource without borrowing other approvals', () => {
    const [state, setState] = createSignal('pending_approval');
    fixture.read = () => [
      fixture.finding('web-restart', 'Web container keeps restarting', fixture.web, {
        investigationOutcome: 'fix_queued',
        investigationRecord: { action: { state: state() } },
      }),
      fixture.findings[2],
    ];
    render(() => (
      <PatrolIntelligenceWorkspace
        state={makeState()}
        findingResourceId={fixture.web.resourceId}
        findingResourceName="afs-web"
      />
    ));
    for (const [next, clause] of [
      ['pending_approval', '1 needs approval'],
      ['planned', '1 ready to run'],
      ['approved', '1 approved to run'],
      ['executing', '1 in progress'],
      ['completed', '1 completed action'],
      ['failed', '1 failed fix'],
      ['rejected', '1 rejected action'],
      ['expired', '1 expired action'],
    ]) {
      setState(next);
      expect(
        screen.getByText(
          `Patrol found 1 issue on afs-web — ${clause}. Open a row to review evidence and record the outcome.`,
        ),
      ).toBeInTheDocument();
      if (next !== 'pending_approval')
        expect(screen.queryByText(/Patrol found.*action recovery/)).not.toBeInTheDocument();
      // The legacy list remains independently reviewable; its two saved
      // requests do not overwrite a typed action or escape the resource scope.
      expect(screen.getByText('Apply the fix for web-restart')).toBeInTheDocument();
      expect(screen.queryByText('Apply the fix for db-backup')).not.toBeInTheDocument();
      expect(reviewRows()).toHaveLength(1);
    }
  });
});
