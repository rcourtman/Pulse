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
}));

vi.mock('@/api/patrolAttention', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/patrolAttention')>();
  return {
    ...original,
    getPatrolAttention: (...args: unknown[]) => apiMocks.getList(...args),
    getPatrolAttentionDetail: (...args: unknown[]) => apiMocks.getDetail(...args),
    getPatrolAttentionSummary: (...args: unknown[]) => apiMocks.getSummary(...args),
  };
});

import { PatrolAttentionWorkbench } from '../PatrolAttentionWorkbench';
import { patrolAttentionStore } from '@/stores/patrolAttention';

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
  });

  afterEach(() => {
    cleanup();
    patrolAttentionStore.clear();
  });

  const renderWorkbench = () =>
    render(() => (
      <Router>
        <Route path="/patrol" component={PatrolAttentionWorkbench} />
      </Router>
    ));

  it('renders a plain trustworthy calm state without a proof strip', async () => {
    const calm = summary();
    apiMocks.getList.mockResolvedValue(listResponse([], calm));
    renderWorkbench();

    expect(await screen.findByRole('heading', { name: 'Nothing needs your attention' }))
      .toBeInTheDocument();
    expect(screen.getByText(/current operational lifecycle evaluation/i)).toBeInTheDocument();
    expect(screen.queryByText(/trust score/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/auto-resolved/i)).not.toBeInTheDocument();
  });

  it('opens deepest typed evidence, protection, and timeline detail from one queue item', async () => {
    const active = item();
    apiMocks.getList.mockResolvedValue(
      listResponse([active], summary({ activeCount: 1, openCount: 1, calm: false })),
    );
    apiMocks.getDetail.mockResolvedValue(detail(active));
    renderWorkbench();

    const openButton = await screen.findByRole('button', {
      name: 'Open Disk pressure on Database VM',
    });
    fireEvent.click(openButton);

    const detailRegion = await screen.findByRole('complementary', {
      name: 'Disk pressure on Database VM',
    });
    expect(within(detailRegion).getByText(/Impact: Writes may fail\./)).toBeInTheDocument();
    expect(within(detailRegion).getByText(/latest backup has not been verified/i))
      .toBeInTheDocument();
    expect(within(detailRegion).getByText('Proxmox VE')).toBeInTheDocument();
    expect(within(detailRegion).getByText('Observing to Open')).toBeInTheDocument();
    expect(window.location.search).toBe('?attention=record-1');
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
        name: 'Open Disk pressure on Database VM',
      }),
    );

    const detailRegion = await screen.findByRole('complementary', {
      name: 'Disk pressure on Database VM',
    });
    expect(
      within(detailRegion).getByText('Showing the latest 3 of 5 observations.'),
    ).toBeInTheDocument();
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
    expect(screen.queryByRole('heading', { name: 'Nothing needs your attention' }))
      .not.toBeInTheDocument();
  });
});
