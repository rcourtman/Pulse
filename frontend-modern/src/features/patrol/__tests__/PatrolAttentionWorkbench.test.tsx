import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  AttentionItem,
  AttentionItemDetail,
  AttentionListResponse,
  AttentionSummary,
} from '@/api/patrolAttention';

const apiMocks = vi.hoisted(() => ({
  getList: vi.fn(),
  getDetail: vi.fn(),
  getSummary: vi.fn(),
  acknowledge: vi.fn(),
  unacknowledge: vi.fn(),
  suppress: vi.fn(),
  unsuppress: vi.fn(),
  planAction: vi.fn(),
  getAction: vi.fn(),
  dismissFinding: vi.fn(),
  reopenFinding: vi.fn(),
  loadPatrolFindings: vi.fn(),
  createRule: vi.fn(),
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    dismissFinding: (...args: unknown[]) => apiMocks.dismissFinding(...args),
    reopenFinding: (...args: unknown[]) => apiMocks.reopenFinding(...args),
    loadPatrolFindings: (...args: unknown[]) => apiMocks.loadPatrolFindings(...args),
    get patrolFindings() {
      return [];
    },
  },
}));

vi.mock('@/api/patrol', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/patrol')>();
  return {
    ...original,
    createSuppressionRuleFromFinding: (...args: unknown[]) => apiMocks.createRule(...args),
  };
});

vi.mock('@/api/patrolAttention', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/patrolAttention')>();
  return {
    ...original,
    getPatrolAttention: (...args: unknown[]) => apiMocks.getList(...args),
    getPatrolAttentionDetail: (...args: unknown[]) => apiMocks.getDetail(...args),
    getPatrolAttentionSummary: (...args: unknown[]) => apiMocks.getSummary(...args),
    acknowledgePatrolAttention: (...args: unknown[]) => apiMocks.acknowledge(...args),
    unacknowledgePatrolAttention: (...args: unknown[]) => apiMocks.unacknowledge(...args),
    suppressPatrolAttention: (...args: unknown[]) => apiMocks.suppress(...args),
    unsuppressPatrolAttention: (...args: unknown[]) => apiMocks.unsuppress(...args),
    planPatrolAttentionAction: (...args: unknown[]) => apiMocks.planAction(...args),
  };
});

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: {
    getAction: (...args: unknown[]) => apiMocks.getAction(...args),
  },
}));

vi.mock('@/features/actions/ActionReviewDialog', async () => {
  const { Show } = await import('solid-js');
  return {
    ActionReviewDialog: (props: {
      detail: { audit?: { id?: string } } | null;
      onClose: () => void;
      onChanged: (next: { audit?: { id?: string } }) => Promise<void> | void;
    }) => (
      <Show when={props.detail}>
        {(value) => (
          <div role="dialog" aria-label="Governed action review">
            <span>{value().audit?.id}</span>
            <button type="button" onClick={() => void props.onChanged(value())}>
              Complete action
            </button>
            <button type="button" onClick={props.onClose}>
              Close action review
            </button>
          </div>
        )}
      </Show>
    ),
  };
});

import {
  getDistinctPatrolImpact,
  getPatrolDecisionDisplayTitle,
  PatrolAttentionWorkbench,
  sortPatrolAttentionDecisions,
} from '../PatrolAttentionWorkbench';
import { patrolAttentionStore } from '@/stores/patrolAttention';
import { buildPatrolAttentionAssistantHandoff } from '../patrolInvestigationContextModel';
import type { UnifiedFinding } from '@/stores/aiIntelligence';

const evaluatedAt = '2026-07-19T08:00:00Z';

const summary = (overrides: Partial<AttentionSummary> = {}): AttentionSummary => ({
  activeCount: 0,
  openCount: 0,
  acknowledgedCount: 0,
  suppressedCount: 0,
  uncertainCount: 0,
  resolvedCount: 0,
  calm: true,
  coverageState: 'current',
  evaluatedAt,
  ...overrides,
});

const item = (overrides: Partial<AttentionItem> = {}): AttentionItem => ({
  id: 'record-1',
  operationalRecordId: 'record-1',
  subjectResourceId: 'pve:vm:101',
  subjectResourceName: 'Database VM',
  subjectResourceType: 'vm',
  kind: 'disk',
  title: 'Disk pressure on Database VM',
  plainLanguageSummary: 'The database disk is nearly full.',
  severity: 'critical',
  state: 'open',
  firstObservedAt: '2026-07-19T07:00:00Z',
  lastObservedAt: evaluatedAt,
  evidenceFreshness: 'fresh',
  evidenceCompleteness: 'complete',
  impact: 'Writes may fail.',
  protectionPosture: {
    subjectResourceId: 'pve:vm:101',
    state: 'attention',
    freshness: 'current',
    verification: 'unverified',
    coverage: 'complete',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: [],
    explanation: 'The latest backup has not been verified.',
    evaluatedAt,
  },
  relatedResources: [{ resourceId: 'pbs:datastore:main' }],
  recommendedNextStep: 'Free disk space or expand the volume.',
  availableActions: [],
  verificationState: 'not_available',
  ...overrides,
});

const mirroredFinding = (overrides: Partial<UnifiedFinding> = {}): UnifiedFinding => ({
  id: 'finding-1',
  source: 'ai-patrol',
  resourceId: 'pve:vm:101',
  resourceName: 'Database VM',
  resourceType: 'vm',
  category: 'capacity',
  severity: 'warning',
  title: 'Disk on Database VM is 95% full',
  description: 'The root volume has been filling for a week.',
  detectedAt: '2026-07-19T07:00:00Z',
  status: 'active',
  mirrorsAlertId: 'alert-record-1',
  mirrorsAlertType: 'disk',
  ...overrides,
});

const listResponse = (
  items: AttentionItem[],
  responseSummary: AttentionSummary,
): AttentionListResponse => ({
  data: items,
  summary: responseSummary,
  meta: { page: 1, limit: 50, total: items.length, totalPages: items.length ? 1 : 0 },
});

const detail = (value: AttentionItem): AttentionItemDetail => ({
  item: value,
  operationalRecord: {
    id: value.operationalRecordId,
    canonicalSpecId: 'disk-pressure',
    subjectResourceId: value.subjectResourceId,
    state: value.state,
    severity: value.severity,
    firstObservedAt: value.firstObservedAt,
    lastObservedAt: value.lastObservedAt,
    stateChangedAt: value.lastObservedAt,
    evidenceIds: ['evidence-1'],
    causeKey: 'disk-pressure:pve:vm:101',
    relatedResourceIds: value.relatedResources.map((resource) => resource.resourceId),
    impactSummary: value.impact,
    recommendedNextStep: value.recommendedNextStep,
  },
  timeline: [
    {
      id: 'transition-1',
      operationalRecordId: value.operationalRecordId,
      from: 'observing',
      to: 'open',
      at: value.firstObservedAt,
      cause: 'detector_decision',
      causeKey: 'disk-pressure:pve:vm:101',
      evidenceIds: ['evidence-1'],
    },
  ],
  evidence: [
    {
      id: 'evidence-1',
      source: { provider: 'pve', collector: 'resource-monitor' },
      subject: { resourceId: value.subjectResourceId },
      observedAt: value.lastObservedAt,
      ingestedAt: value.lastObservedAt,
      completeness: 'complete',
      confidence: 'confirmed',
      permissions: 'sufficient',
    },
  ],
});

describe('PatrolAttentionWorkbench', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/patrol');
    patrolAttentionStore.clear();
    apiMocks.getList.mockReset();
    apiMocks.getDetail.mockReset();
    apiMocks.getSummary.mockReset();
    apiMocks.acknowledge.mockReset();
    apiMocks.unacknowledge.mockReset();
    apiMocks.suppress.mockReset();
    apiMocks.unsuppress.mockReset();
    apiMocks.planAction.mockReset();
    apiMocks.getAction.mockReset();
    apiMocks.dismissFinding.mockReset();
    apiMocks.reopenFinding.mockReset();
    apiMocks.loadPatrolFindings.mockReset();
    apiMocks.createRule.mockReset();
  });

  afterEach(() => {
    cleanup();
    patrolAttentionStore.clear();
  });

  const renderWorkbench = () =>
    render(() => (
      <Router>
        <Route path="/patrol" component={() => <PatrolAttentionWorkbench />} />
      </Router>
    ));

  it('renders a plain trustworthy calm state without a proof strip', async () => {
    const calm = summary({ evaluatedAt: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString() });
    apiMocks.getList.mockResolvedValue(listResponse([], calm));
    renderWorkbench();

    expect(
      await screen.findByRole('heading', { name: 'Nothing needs you right now' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/current operational evaluation has no active items/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/trust score/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/auto-resolved/i)).not.toBeInTheDocument();

    expect(screen.getByText(/Checked 1d ago/i)).toBeInTheDocument();
    apiMocks.getList.mockResolvedValue(
      listResponse(
        [],
        summary({ evaluatedAt: new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString() }),
      ),
    );
    await patrolAttentionStore.load('active');

    await waitFor(() => expect(screen.getByText(/Checked 10d ago/i)).toBeInTheDocument());
  });

  it('keeps lifecycle filters out of the primary decision surface', async () => {
    const attentionSummary = summary({
      activeCount: 1,
      openCount: 1,
      acknowledgedCount: 2,
      suppressedCount: 3,
      uncertainCount: 4,
      resolvedCount: 5,
      calm: false,
    });
    apiMocks.getList.mockResolvedValue(listResponse([item()], attentionSummary));
    renderWorkbench();

    expect(
      await screen.findByRole('heading', { name: '1 decision needs you' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start review' })).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Open Database VM · Disk pressure' }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Today's Patrol briefing")).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Decision inbox' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Review this decision' })).not.toBeInTheDocument();
    expect(screen.queryByText('Open', { exact: true })).not.toBeInTheDocument();
    expect(screen.queryByText('Review', { exact: true })).not.toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Attention state' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Attention state' })).not.toBeInTheDocument();
    expect(apiMocks.getList).toHaveBeenCalledWith('active');
  });

  it('surfaces pending governed approvals beside current Patrol decisions', async () => {
    const attentionSummary = summary({ activeCount: 1, openCount: 1, calm: false });
    apiMocks.getList.mockResolvedValue(listResponse([item()], attentionSummary));

    render(() => (
      <Router>
        <Route
          path="/patrol"
          component={() => <PatrolAttentionWorkbench pendingActionCount={2} />}
        />
      </Router>
    ));

    expect(await screen.findByRole('link', { name: 'Review 2 approvals' })).toHaveAttribute(
      'href',
      '/actions',
    );
    expect(screen.getByRole('button', { name: 'Refresh Patrol attention' })).toBeInTheDocument();
  });

  it('does not interrupt assisted mode for safe eligible work', async () => {
    const safeWork = item({
      availableActions: [
        {
          targetResourceId: 'pve:vm:101',
          capability: 'cleanup_disk',
          kind: 'cleanup',
          label: 'Clean safe temporary files',
          mode: 'execute',
          risk: 'low',
          approval: 'not-required',
          eligibility: 'eligible',
          reasons: [],
          evidenceIds: ['evidence-1'],
          expectedPostcondition: 'Disk use returns below the warning threshold.',
          verificationPolicy: 'disk-pressure',
          requiresApproval: false,
        },
      ],
      verificationState: 'pending',
    });
    apiMocks.getList.mockResolvedValue(
      listResponse([safeWork], summary({ activeCount: 1, openCount: 1, calm: false })),
    );

    render(() => (
      <Router>
        <Route
          path="/patrol"
          component={() => (
            <PatrolAttentionWorkbench autonomyLevel="assisted" autonomyLocked={false} />
          )}
        />
      </Router>
    ));

    expect(
      await screen.findByRole('heading', { name: 'Nothing needs you right now' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /1 other current issue is continuing without a decision under Safe auto-fix/i,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Open Database VM/ })).not.toBeInTheDocument();
  });

  it('keeps a long decision queue compact until the operator asks for all of it', async () => {
    const decisions = Array.from({ length: 7 }, (_, index) =>
      item({ id: `record-${index + 1}`, title: `Decision ${index + 1}` }),
    );
    apiMocks.getList.mockResolvedValue(
      listResponse(decisions, summary({ activeCount: 7, openCount: 7, calm: false })),
    );
    renderWorkbench();

    const showAll = await screen.findByRole('button', { name: 'Show all 7 decisions' });
    expect(screen.getAllByRole('button', { name: /Open Decision/ })).toHaveLength(5);

    fireEvent.click(showAll);
    expect(screen.getAllByRole('button', { name: /Open Decision/ })).toHaveLength(7);
    expect(screen.getByRole('button', { name: 'Show fewer decisions' })).toBeInTheDocument();
  });

  it('orders the operator queue by severity, actionable decisions, and freshest evidence', () => {
    const action = {
      targetResourceId: 'pve:vm:101',
      capability: 'expand_disk',
      kind: 'storage',
      label: 'Expand disk',
      mode: 'execute' as const,
      risk: 'medium',
      approval: 'required' as const,
      eligibility: 'eligible' as const,
      reasons: [],
      evidenceIds: ['evidence-1'],
      expectedPostcondition: 'Disk pressure clears.',
      verificationPolicy: 'disk-pressure',
      requiresApproval: true,
    };
    const decisions = [
      item({ id: 'warning', severity: 'warning', lastObservedAt: '2026-07-19T09:00:00Z' }),
      item({ id: 'critical-old', lastObservedAt: '2026-07-19T07:30:00Z' }),
      item({
        id: 'critical-action',
        lastObservedAt: '2026-07-19T07:00:00Z',
        availableActions: [action],
      }),
      item({ id: 'critical-new', lastObservedAt: '2026-07-19T08:30:00Z' }),
    ].map((attentionItem) => ({ item: attentionItem, reason: 'Review this issue.' }));

    expect(sortPatrolAttentionDecisions(decisions).map((decision) => decision.item.id)).toEqual([
      'critical-action',
      'critical-new',
      'critical-old',
      'warning',
    ]);
  });

  it('leads generated decision titles with the resource and removes repeated impact copy', () => {
    const active = item({
      plainLanguageSummary: 'The database disk is nearly full.',
      impact: 'The database disk is nearly full.',
    });

    expect(getPatrolDecisionDisplayTitle(active)).toBe('Database VM · Disk pressure');
    expect(getDistinctPatrolImpact(active)).toBe('');
    expect(getDistinctPatrolImpact(item())).toBe('Writes may fail.');
  });

  it('opens deepest typed evidence, protection, and timeline detail from one queue item', async () => {
    const active = item();
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(active));
    renderWorkbench();

    const openButton = await screen.findByRole('button', {
      name: 'Open Database VM · Disk pressure',
    });
    fireEvent.click(openButton);

    const detailRegion = await screen.findByRole('complementary', {
      name: 'Database VM · Disk pressure',
    });
    expect(within(detailRegion).getByText('Writes may fail.')).toBeInTheDocument();
    expect(within(detailRegion).getByText('Evidence and history')).toBeInTheDocument();
    expect(
      within(detailRegion).getByRole('button', { name: 'Copy resource identifier' }),
    ).toBeInTheDocument();
    expect(within(detailRegion).queryByText('pve:vm:101')).not.toBeInTheDocument();
    expect(
      within(detailRegion).getByText(/latest backup has not been verified/i),
    ).toBeInTheDocument();
    expect(within(detailRegion).getByText('Proxmox VE')).toBeInTheDocument();
    expect(within(detailRegion).getByText('Observing to Open')).toBeInTheDocument();
    expect(
      within(detailRegion).getByRole('button', { name: 'Back to attention list' }),
    ).toHaveTextContent('Back to list');
    expect(
      within(detailRegion).getByRole('button', { name: 'Close attention detail' }),
    ).toHaveClass('h-8', 'w-8');
    expect(within(detailRegion).getByRole('button', { name: 'Mark reviewed' })).toHaveClass(
      'min-h-11',
      'sm:min-h-0',
    );
    expect(within(detailRegion).getByRole('link', { name: 'Open resource' })).toHaveClass(
      'min-h-11',
      'sm:min-h-0',
    );
    expect(window.location.search).toBe('?attention=record-1');
  });

  it('uses plain scan-row language while keeping unknown trust metadata in detail', async () => {
    const unknownTrust = item({
      state: 'unknown',
      evidenceFreshness: 'unknown',
      evidenceCompleteness: 'complete',
      protectionPosture: {
        ...item().protectionPosture!,
        state: 'unknown',
        explanation: 'Pulse has not received enough protection evidence to classify this resource.',
        providerStates: [
          {
            provider: 'pbs',
            source: 'recovery-points',
            scope: 'primary',
            jobState: 'unknown',
            historyCompleteness: 'complete',
            permissions: 'sufficient',
            evidenceIds: [],
            verificationExpected: true,
          },
        ],
      },
    });
    apiMocks.getList.mockResolvedValue(
      listResponse([unknownTrust], summary({ activeCount: 1, uncertainCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(unknownTrust));
    renderWorkbench();

    const row = await screen.findByRole('button', { name: 'Open Database VM · Disk pressure' });
    expect(within(row).queryByText(/Evidence/)).not.toBeInTheDocument();
    expect(within(row).queryByText(/timing/i)).not.toBeInTheDocument();
    expect(within(row).queryByText('Unknown / Complete')).not.toBeInTheDocument();
    expect(within(row).queryByText(/Protection/)).not.toBeInTheDocument();

    fireEvent.click(row);

    const detailRegion = await screen.findByRole('complementary', {
      name: 'Database VM · Disk pressure',
    });
    expect(within(detailRegion).getByText('Evidence recorded')).toBeInTheDocument();
    expect(within(detailRegion).getByText('Protection status unavailable')).toBeInTheDocument();
    expect(within(detailRegion).getByText('Job status unavailable')).toBeInTheDocument();
    expect(within(detailRegion).getByText('History: Complete')).toBeInTheDocument();
    expect(
      within(detailRegion).getByText(/not received enough protection evidence/i),
    ).toBeInTheDocument();
  });

  it('keeps repeated evidence observations available without overwhelming the default detail', async () => {
    const active = item();
    const expandedDetail = detail(active);
    expandedDetail.evidence = Array.from({ length: 5 }, (_, index) => ({
      ...expandedDetail.evidence[0],
      id: `evidence-${index + 1}`,
      observedAt: new Date(Date.parse(active.lastObservedAt) - index * 60_000).toISOString(),
    }));
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(expandedDetail);
    renderWorkbench();

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );

    const detailRegion = await screen.findByRole('complementary', {
      name: 'Database VM · Disk pressure',
    });
    expect(within(detailRegion).getByText('5 observations')).toBeInTheDocument();
    expect(
      within(detailRegion).queryByText('Showing the latest 3 of 5 observations.'),
    ).not.toBeInTheDocument();
    expect(within(detailRegion).getByText('Show 2 older observations')).toBeInTheDocument();
  });

  it('does not turn partial coverage and an empty queue into a healthy claim', async () => {
    apiMocks.getList.mockResolvedValue(
      listResponse([], summary({ coverageState: 'partial', calm: true })),
    );
    renderWorkbench();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'No items in this view' })).toBeInTheDocument();
    });
    expect(screen.getByText(/protection context is incomplete/i)).toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: 'Nothing needs you right now' }),
    ).not.toBeInTheDocument();
  });

  it('marks an item reviewed and advances through the decision queue', async () => {
    const active = item();
    const next = item({ id: 'record-2', title: 'Database replication is delayed' });
    const reviewed = item({ state: 'acknowledged' });
    let isReviewed = false;
    apiMocks.getList.mockImplementation((filter: string) => {
      if (!isReviewed) {
        return Promise.resolve(
          listResponse([active, next], summary({ activeCount: 2, openCount: 2, calm: false })),
        );
      }
      const responseSummary = summary({
        activeCount: 1,
        openCount: 1,
        acknowledgedCount: 1,
        calm: false,
      });
      return Promise.resolve(
        filter === 'all'
          ? listResponse([reviewed, next], responseSummary)
          : listResponse([next], responseSummary),
      );
    });
    apiMocks.getDetail.mockImplementation((itemId: string) =>
      Promise.resolve(detail(itemId === active.id ? (isReviewed ? reviewed : active) : next)),
    );
    apiMocks.acknowledge.mockImplementation(() => {
      isReviewed = true;
      return Promise.resolve({ success: true });
    });
    apiMocks.unacknowledge.mockImplementation(() => {
      isReviewed = false;
      return Promise.resolve({ success: true });
    });
    renderWorkbench();

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );
    expect(await screen.findByText('Decision 1 of 2')).toBeInTheDocument();
    expect(screen.getByTitle('Previous decision')).toBeDisabled();
    expect(screen.getByTitle('Next decision')).toBeEnabled();

    fireEvent.click(screen.getByTitle('Next decision'));
    expect(
      await screen.findByRole('complementary', { name: 'Database replication is delayed' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Decision 2 of 2')).toBeInTheDocument();
    expect(screen.getByTitle('Previous decision')).toBeEnabled();
    expect(screen.getByTitle('Next decision')).toBeDisabled();

    fireEvent.click(screen.getByTitle('Previous decision'));
    expect(
      await screen.findByRole('complementary', { name: 'Database VM · Disk pressure' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Decision 1 of 2')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Mark reviewed' }));

    await waitFor(() => expect(apiMocks.acknowledge).toHaveBeenCalledWith('record-1'));
    expect(await screen.findByRole('status')).toHaveTextContent(
      'Marked reviewed. 1 decision remains.',
    );
    expect(
      await screen.findByRole('complementary', { name: 'Database replication is delayed' }),
    ).toBeInTheDocument();
    expect(window.location.search).toBe('?attention=record-2');

    fireEvent.click(screen.getByRole('button', { name: 'Review handled issues' }));
    expect(
      await screen.findByRole('heading', { name: '1 reviewed or suppressed issue' }),
    ).toBeInTheDocument();
    expect(apiMocks.getList).toHaveBeenLastCalledWith('all');
    expect(screen.getByText('Reviewed', { exact: true })).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );
    fireEvent.click(await screen.findByRole('button', { name: 'Return to decision inbox' }));

    await waitFor(() => expect(apiMocks.unacknowledge).toHaveBeenCalledWith('record-1'));
    expect(await screen.findByRole('status')).toHaveTextContent(
      'Returned to decision inbox. No other handled issues remain.',
    );
    expect(
      screen.getByRole('heading', { name: 'No reviewed or suppressed issues' }),
    ).toBeInTheDocument();
  });

  it('requires an explicit reason and bounded duration before temporary suppression', async () => {
    const active = item();
    const suppressed = item({ state: 'suppressed' });
    const suppressedDetail = detail(suppressed);
    suppressedDetail.operationalRecord.suppression = {
      at: evaluatedAt,
      by: 'operator',
      reason: 'Planned storage maintenance',
      expiresAt: '2026-07-20T08:00:00Z',
    };
    apiMocks.getList
      .mockResolvedValueOnce(
        listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
      )
      .mockResolvedValue(
        listResponse([], summary({ activeCount: 0, suppressedCount: 1, calm: true })),
      );
    apiMocks.getDetail.mockResolvedValueOnce(detail(active)).mockResolvedValue(suppressedDetail);
    apiMocks.suppress.mockResolvedValue({ success: true });
    renderWorkbench();

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );
    fireEvent.click(await screen.findByRole('button', { name: 'Suppress temporarily' }));

    const submit = screen.getByRole('button', { name: 'Suppress temporarily' });
    expect(submit).toBeDisabled();
    expect(submit).toHaveClass('min-h-11', 'sm:min-h-0');
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveClass('min-h-11', 'sm:min-h-0');
    expect(
      screen.getByRole('combobox', {
        name: 'Return it to active attention after',
      }),
    ).toHaveClass('min-h-11', 'sm:min-h-9');
    fireEvent.input(
      screen.getByRole('textbox', {
        name: 'Why is this safe to hide from active attention?',
      }),
      { target: { value: 'Planned storage maintenance' } },
    );
    fireEvent.change(
      screen.getByRole('combobox', {
        name: 'Return it to active attention after',
      }),
      { target: { value: String(60 * 60 * 1000) } },
    );
    fireEvent.click(submit);

    await waitFor(() => {
      expect(apiMocks.suppress).toHaveBeenCalledWith(
        'record-1',
        'Planned storage maintenance',
        expect.any(String),
      );
    });
    expect(await screen.findByRole('status')).toHaveTextContent(
      'Suppressed temporarily. Your decision inbox is clear.',
    );
    expect(screen.getByRole('button', { name: 'Review handled issues' })).toBeInTheDocument();
    expect(
      await screen.findByRole('heading', { name: 'Nothing needs you right now' }),
    ).toBeInTheDocument();
  });

  it('offers every lasting Patrol decision on the finding that mirrors the alert', async () => {
    const active = item();
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(active));
    apiMocks.dismissFinding.mockResolvedValue(true);
    render(() => (
      <Router>
        <Route
          path="/patrol"
          component={() => <PatrolAttentionWorkbench findings={() => [mirroredFinding()]} />}
        />
      </Router>
    ));

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );

    expect(await screen.findByText('Lasting decisions')).toBeInTheDocument();
    expect(screen.getByText(/Patrol also raised/i)).toHaveTextContent(
      'Disk on Database VM is 95% full',
    );
    const decisions = within(screen.getByRole('list', { name: 'Lasting decisions' }));
    for (const label of [
      'Remember as expected',
      'Dismiss: Not an issue',
      'Dismiss: Later',
      'Create rule',
    ]) {
      expect(decisions.getByRole('button', { name: label })).toBeInTheDocument();
    }
    expect(decisions.getByText(/stops raising it unless it gets worse/i)).toHaveTextContent(
      /Lasts until you reopen the finding/i,
    );
    expect(decisions.getByText(/Hides this for 7 days/i)).toBeInTheDocument();
    expect(
      decisions.getByText(/Permanent rule for this resource and category/i),
    ).toBeInTheDocument();
    // The temporary controls say so in one line instead of hiding it behind a toggle.
    expect(screen.getByText(/Both of these are short-term/i)).toBeInTheDocument();
    expect(screen.queryByText('More ways to manage this issue')).not.toBeInTheDocument();

    fireEvent.click(decisions.getByRole('button', { name: 'Remember as expected' }));
    const form = screen.getByRole('form', { name: 'Confirm Remember as expected' });
    fireEvent.input(within(form).getByLabelText(/Note for Patrol/i), {
      target: { value: 'This VM is a cold standby.' },
    });
    fireEvent.click(within(form).getByRole('button', { name: 'Confirm: Remember as expected' }));

    await waitFor(() =>
      expect(apiMocks.dismissFinding).toHaveBeenCalledWith(
        'finding-1',
        'expected_behavior',
        'This VM is a cold standby.',
      ),
    );
    expect(await screen.findByRole('status')).toHaveTextContent(/Remember as expected saved/i);
  });

  it('requires a written reason before creating a permanent rule from the detail', async () => {
    const active = item();
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(active));
    apiMocks.createRule.mockResolvedValue({ success: true, message: 'ok', rule: { id: 'r1' } });
    apiMocks.loadPatrolFindings.mockResolvedValue(undefined);
    render(() => (
      <Router>
        <Route
          path="/patrol"
          component={() => <PatrolAttentionWorkbench findings={() => [mirroredFinding()]} />}
        />
      </Router>
    ));
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );
    const decisions = within(await screen.findByRole('list', { name: 'Lasting decisions' }));
    fireEvent.click(decisions.getByRole('button', { name: 'Create rule' }));

    const form = screen.getByRole('form', { name: 'Confirm Create rule' });
    const confirm = within(form).getByRole('button', { name: 'Confirm: Create rule' });
    expect(confirm).toBeDisabled();
    fireEvent.input(within(form).getByLabelText(/Why this rule/i), {
      target: { value: 'Archive volume is meant to run near full.' },
    });
    expect(confirm).not.toBeDisabled();
    fireEvent.click(confirm);

    await waitFor(() =>
      expect(apiMocks.createRule).toHaveBeenCalledWith({
        resourceId: 'pve:vm:101',
        resourceName: 'Database VM',
        category: 'capacity',
        description: 'Archive volume is meant to run near full.',
      }),
    );
    expect(apiMocks.loadPatrolFindings).toHaveBeenCalled();
    expect(await screen.findByRole('status')).toHaveTextContent(/Rule created/i);
  });

  it('shows the remembered decision and a reopen path instead of re-offering it', async () => {
    const active = item();
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(active));
    apiMocks.reopenFinding.mockResolvedValue(true);
    render(() => (
      <Router>
        <Route
          path="/patrol"
          component={() => (
            <PatrolAttentionWorkbench
              findings={() => [
                mirroredFinding({
                  status: 'dismissed',
                  dismissedReason: 'expected_behavior',
                  userNote: 'Cold standby.',
                }),
              ]}
            />
          )}
        />
      </Router>
    ));
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );

    expect(await screen.findByText(/Remembered as expected/i)).toBeInTheDocument();
    expect(screen.getByText(/Note: Cold standby/i)).toBeInTheDocument();
    expect(screen.queryByRole('list', { name: 'Lasting decisions' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reopen finding' }));
    await waitFor(() => expect(apiMocks.reopenFinding).toHaveBeenCalledWith('finding-1'));
  });

  it('explains that an alert with no Patrol finding has nothing for Patrol to remember', async () => {
    const active = item();
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(active));
    const onOpenFindings = vi.fn();
    render(() => (
      <Router>
        <Route
          path="/patrol"
          component={() => (
            <PatrolAttentionWorkbench findings={() => []} onOpenFindings={onOpenFindings} />
          )}
        />
      </Router>
    ));
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open Database VM · Disk pressure',
      }),
    );

    expect(await screen.findByText(/nothing for Patrol to remember here/i)).toHaveTextContent(
      /turn it off for Database VM under alert thresholds/i,
    );
    expect(screen.queryByRole('list', { name: 'Lasting decisions' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Adjust alert thresholds' })).toHaveAttribute(
      'href',
      '/alerts/thresholds',
    );
    fireEvent.click(
      screen.getByRole('button', { name: 'Browse Patrol findings for this resource' }),
    );
    expect(onOpenFindings).toHaveBeenCalledWith(active);
  });

  it('collapses a flapping item into one label and a summarised timeline', async () => {
    const flapping = item({
      id: 'record-flap',
      operationalRecordId: 'record-flap',
      kind: 'docker-container-state',
      title: 'Docker container state on shlink-web-clientX',
      subjectResourceName: 'shlink-web-clientX',
      severity: 'warning',
      flapping: {
        transitionCount: 11,
        windowHours: 24,
        firstTransitionAt: '2026-07-18T09:00:00Z',
        lastTransitionAt: '2026-07-19T07:30:00Z',
      },
    });
    const flappingDetail = detail(flapping);
    flappingDetail.timeline = Array.from({ length: 11 }, (_, index) => ({
      id: `flap-${index}`,
      operationalRecordId: 'record-flap',
      from: index % 2 === 0 ? 'open' : 'resolved',
      to: index % 2 === 0 ? 'resolved' : 'open',
      at: `2026-07-18T${String(9 + index).padStart(2, '0')}:00:00Z`,
      cause: index % 2 === 0 ? 'recovery_evidence' : 'detector_decision',
      causeKey: 'runtime-state',
      evidenceIds: [],
    }));
    apiMocks.getList.mockResolvedValue(
      listResponse([flapping], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(flappingDetail);
    renderWorkbench();

    const row = await screen.findByRole('button', {
      name: 'Open shlink-web-clientX · Docker container state',
    });
    expect(within(row).getByText('Flapping · 11 changes in 24h')).toBeInTheDocument();
    fireEvent.click(row);

    expect(
      await screen.findByText(/switched between open and resolved 11 times in the last day/i),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByText('Evidence and history'));
    expect(screen.getByText(/11 open\/resolved changes in the last 24 hours/i)).toBeInTheDocument();
    expect(screen.getByText('Show all 11 transitions')).toBeInTheDocument();
  });

  it('opens the canonical governed action review from an eligible attention item', async () => {
    const actionOffer = {
      targetResourceId: 'docker:host-1/container-1',
      capability: 'restart',
      kind: 'container_restart',
      label: 'Restart this container',
      mode: 'plan' as const,
      risk: 'low' as const,
      approval: 'required' as const,
      eligibility: 'eligible' as const,
      reasons: ['fresh_confirmed_unhealthy_container'],
      evidenceIds: ['evidence-1'],
      expectedPostcondition: 'The same container is observed running after the restart.',
      verificationPolicy: 'Pulse requires a fresh container readback.',
      requiresApproval: true,
    };
    const active = item({
      subjectResourceId: 'docker:host-1/container-1',
      subjectResourceName: 'API container',
      subjectResourceType: 'app-container',
      kind: 'docker-container-health',
      title: 'API container is unhealthy',
      availableActions: [actionOffer],
    });
    const completed = item({
      ...active,
      availableActions: [{ ...actionOffer, actionId: 'act-attention-restart' }],
      verificationState: 'succeeded',
    });
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail
      .mockResolvedValueOnce(detail(active))
      .mockResolvedValueOnce(detail(completed));
    apiMocks.planAction.mockResolvedValue({ actionId: 'act-attention-restart' });
    apiMocks.getAction.mockResolvedValue({ audit: { id: 'act-attention-restart' } });
    renderWorkbench();

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Open API container is unhealthy',
      }),
    );
    const trigger = await screen.findByRole('button', { name: 'Review and approve' });
    expect(
      screen.getByText(/explicit review and approval before Pulse sends anything/i),
    ).toBeInTheDocument();
    fireEvent.click(trigger);

    await waitFor(() => {
      expect(apiMocks.planAction).toHaveBeenCalledWith('record-1', 'restart');
      expect(apiMocks.getAction).toHaveBeenCalledWith('act-attention-restart');
    });
    expect(
      await screen.findByRole('dialog', { name: 'Governed action review' }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Complete action' }));
    const currentTrigger = await screen.findByRole('button', { name: 'Review action' });
    expect(currentTrigger).not.toBe(trigger);
    expect(screen.getByText(/recorded the action result below/i)).toBeInTheDocument();
    expect(screen.getByText(/action postcondition was confirmed/i)).toBeInTheDocument();
    expect(screen.queryByText(/container is healthy/i)).not.toBeInTheDocument();
    expect(
      screen.queryByText(/explicit review and approval before Pulse sends anything/i),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Close action review' }));
    await waitFor(() => expect(currentTrigger).toHaveFocus());
  });
});

describe('attention explanation context', () => {
  it('preserves canonical finding and investigation identity when the issue has one linked finding', () => {
    const selected = detail(item());
    const handoff = buildPatrolAttentionAssistantHandoff(selected, [mirroredFinding()]);
    expect(handoff.context.findingId).toBe('finding-1');
    expect(handoff.context.handoffMetadata?.kind).toBe('patrol_finding');
    expect(handoff.context.briefing?.title).toBe('Disk pressure on Database VM');
    expect(handoff.context.handoffContext).toContain(
      'The root volume has been filling for a week.',
    );
    expect(handoff.context.handoffContext).toContain('Operational Record: record-1');
    expect(handoff.context.handoffContext).toContain('Evidence evidence-1:');
    expect(handoff.context.autonomousMode).toBe(false);
  });

  it('keeps unlinked attention evidence and existing governed action references without inventing a finding', () => {
    const selected = detail(
      item({
        availableActions: [
          {
            actionId: 'action-1',
            targetResourceId: 'pve:vm:101',
            capability: 'reboot',
            kind: 'resource',
            label: 'Reboot',
            mode: 'plan',
            risk: 'medium',
            approval: 'required',
            eligibility: 'eligible',
            reasons: [],
            evidenceIds: ['evidence-1'],
            expectedPostcondition: 'running',
            verificationPolicy: 'independent',
            requiresApproval: true,
          },
        ],
      }),
    );
    const handoff = buildPatrolAttentionAssistantHandoff(selected);
    expect(handoff.context.findingId).toBeUndefined();
    expect(handoff.context.handoffResources).toEqual([
      { id: 'pve:vm:101', name: 'Database VM', type: 'vm' },
    ]);
    expect(handoff.context.handoffActions).toEqual([
      {
        actionId: 'action-1',
        targetResourceId: 'pve:vm:101',
        actionCapability: 'reboot',
        actionRequiresApproval: true,
      },
    ]);
    expect(handoff.context.handoffContext).toContain('Evidence: fresh/complete');
    expect(handoff.context.handoffContext).toContain('observed');
  });

  it('does not bind the request to an arbitrary finding when several findings explain an issue', () => {
    const handoff = buildPatrolAttentionAssistantHandoff(detail(item()), [
      mirroredFinding(),
      mirroredFinding({ id: 'finding-2' }),
    ]);
    expect(handoff.context.findingId).toBeUndefined();
    expect(handoff.context.context?.operationalRecordId).toBe('record-1');
  });
});
