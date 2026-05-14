import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Recovery from '@/components/Recovery/Recovery';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import createNonSuspendingQuerySource from '@/hooks/createNonSuspendingQuery.ts?raw';
import useRecoveryPointsFacetsSource from '@/hooks/useRecoveryPointsFacets.ts?raw';
import useRecoveryPointsSeriesSource from '@/hooks/useRecoveryPointsSeries.ts?raw';
import useRecoveryPointsSource from '@/hooks/useRecoveryPoints.ts?raw';
import useRecoveryRollupsSource from '@/hooks/useRecoveryRollups.ts?raw';
import type {
  ProtectionRollup,
  ProtectionRollupTransport,
  RecoveryPoint,
  RecoveryPointTransport,
} from '@/types/recovery';
import { parseRecoveryDateKey } from '@/utils/recoveryDatePresentation';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { ROUTE_STATE_REPLACE_OPTIONS } from '@/utils/routeStateNavigation';
import {
  getRecoveryPointItemTypeKey,
  getRecoveryRollupItemTypeKey,
} from '@/utils/recoveryItemTypePresentation';
import {
  getRecoveryPointPlatform,
  getRecoveryRollupPlatforms,
} from '@/utils/recoveryPlatformModel';

let mockLocationSearch = '';
let mockLocationPath = '/recovery';
const navigateSpy = vi.hoisted(() => vi.fn());
const mockLocationVersion = vi.hoisted(() => ({
  read: undefined as undefined | (() => number),
  bump: undefined as undefined | (() => void),
}));

const apiFetchMock = vi.hoisted(() => vi.fn());
const wsState = vi.hoisted(() => ({ resources: [] as any[] }));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  const solid = await vi.importActual<typeof import('solid-js')>('solid-js');
  if (!mockLocationVersion.read || !mockLocationVersion.bump) {
    const [version, setVersion] = solid.createSignal(0);
    mockLocationVersion.read = version;
    mockLocationVersion.bump = () => setVersion((current) => current + 1);
  }
  return {
    ...actual,
    useLocation: () => ({
      get pathname() {
        mockLocationVersion.read?.();
        return mockLocationPath;
      },
      get search() {
        mockLocationVersion.read?.();
        return mockLocationSearch;
      },
    }),
    useNavigate: () => navigateSpy,
  };
});

const rollupsPayload: Array<ProtectionRollup | ProtectionRollupTransport> = [
  {
    rollupId: 'res:vm-123',
    itemResourceId: 'vm-123',
    display: { subjectType: 'proxmox-vm', itemType: 'vm', entityIdLabel: '123' },
    lastAttemptAt: '2026-02-14T10:00:00.000Z',
    lastSuccessAt: '2026-02-14T10:00:00.000Z',
    lastOutcome: 'success',
    platforms: ['proxmox-pve'],
  },
  {
    rollupId: 'ext:truenas-1',
    itemRef: { type: 'truenas-dataset', name: 'tank/apps', id: 'tank/apps' },
    display: { itemType: 'dataset' },
    lastAttemptAt: '2026-02-13T09:00:00.000Z',
    lastSuccessAt: null,
    lastOutcome: 'failed',
    platforms: ['truenas'],
  },
];

const pointsByRollupId: Record<string, Array<RecoveryPoint | RecoveryPointTransport>> = {
  'res:vm-123': [
    {
      id: 'p1',
      platform: 'proxmox-pve',
      kind: 'backup',
      mode: 'local',
      outcome: 'success',
      itemResourceId: 'vm-123',
      completedAt: '2026-02-14T10:00:00.000Z',
      sizeBytes: 1234,
      cluster: 'lab-cluster',
      node: 'pve-01',
      namespace: 'finance',
      repositoryRef: {
        type: 'recovery-target',
        name: 'fast-store',
      },
      display: {
        itemLabel: 'VM 123',
        itemType: 'vm',
        subjectType: 'proxmox-vm',
        clusterLabel: 'Lab Cluster',
        nodeHostLabel: 'pve-01',
        namespaceLabel: 'Finance',
        entityIdLabel: '123',
      },
    },
  ],
  'ext:truenas-1': [
    {
      id: 't1',
      platform: 'truenas',
      kind: 'snapshot',
      mode: 'snapshot',
      outcome: 'failed',
      completedAt: '2026-02-13T09:00:00.000Z',
      sizeBytes: 0,
      display: { itemLabel: 'tank/apps', itemType: 'dataset', subjectType: 'truenas-dataset' },
    },
  ],
};

let facetsPayload: any;
type RecoveryPointsResponse = {
  data: any[];
  meta: { page: number; limit: number; total: number; totalPages: number };
};

const requestParam = (url: URL, key: string) => String(url.searchParams.get(key) || '').trim();

const matchesTextQuery = (haystackParts: unknown[], query: string): boolean => {
  if (!query) return true;
  return haystackParts
    .flatMap((part) => {
      if (part && typeof part === 'object') return Object.values(part as Record<string, unknown>);
      return [part];
    })
    .filter((part) => part !== null && part !== undefined)
    .join(' ')
    .toLowerCase()
    .includes(query.toLowerCase());
};

const filterRollupsForRequest = (url: URL) => {
  const platform = requestParam(url, 'platform');
  const itemType = requestParam(url, 'itemType');
  const query = requestParam(url, 'q');

  return rollupsPayload.filter((rollup) => {
    if (
      platform &&
      !getRecoveryRollupPlatforms(rollup).some((entry) => String(entry || '').trim() === platform)
    ) {
      return false;
    }
    if (itemType && getRecoveryRollupItemTypeKey(rollup) !== itemType) return false;
    return matchesTextQuery(
      [
        rollup.rollupId,
        rollup.itemResourceId,
        rollup.itemRef,
        rollup.subjectRef,
        rollup.display,
        getRecoveryRollupPlatforms(rollup).join(' '),
        rollup.lastOutcome,
      ],
      query,
    );
  });
};

const filterPointsForRequest = (url: URL) => {
  const rollupId = requestParam(url, 'rollupId');
  const platform = requestParam(url, 'platform');
  const itemType = requestParam(url, 'itemType');
  const query = requestParam(url, 'q');
  const cluster = requestParam(url, 'cluster');
  const node = requestParam(url, 'node');
  const namespace = requestParam(url, 'namespace');
  const mode = requestParam(url, 'mode');
  const outcome = requestParam(url, 'outcome');
  const data = rollupId ? pointsByRollupId[rollupId] || [] : Object.values(pointsByRollupId).flat();

  return data.filter((point) => {
    if (platform && getRecoveryPointPlatform(point) !== platform) return false;
    if (itemType && getRecoveryPointItemTypeKey(point) !== itemType) return false;
    if (mode && String(point.mode || '').trim() !== mode) return false;
    if (outcome && String(point.outcome || '').trim() !== outcome) return false;
    if (cluster && String(point.display?.clusterLabel || point.cluster || '').trim() !== cluster) {
      return false;
    }
    if (
      node &&
      String(
        point.display?.nodeHostLabel || point.display?.nodeAgentLabel || point.node || '',
      ).trim() !== node
    ) {
      return false;
    }
    if (
      namespace &&
      String(point.display?.namespaceLabel || point.namespace || '').trim() !== namespace
    ) {
      return false;
    }
    return matchesTextQuery(
      [
        point.id,
        point.itemResourceId,
        point.itemRef,
        point.subjectRef,
        point.display,
        point.platform,
        (point as any).provider,
        point.kind,
        point.mode,
        point.outcome,
        point.cluster,
        point.node,
        point.namespace,
        point.repositoryRef,
      ],
      query,
    );
  });
};

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
  apiFetchJSON: apiFetchMock,
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: wsState,
  }),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useStorageRecoveryResources: () => ({
    resources: () => [{ id: 'vm-123', name: 'VM 123' }],
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

describe('Recovery', () => {
  beforeEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
    localStorage.clear();
    navigateSpy.mockReset();
    navigateSpy.mockImplementation((path: string) => {
      const next = new URL(String(path || '/recovery'), 'http://localhost');
      mockLocationPath = next.pathname;
      mockLocationSearch = next.search;
      mockLocationVersion.bump?.();
    });
    apiFetchMock.mockClear();
    mockLocationSearch = '';
    mockLocationPath = '/recovery';
    wsState.resources = [
      {
        id: 'pbs-resource-1',
        type: 'pbs',
        name: 'pbs-main',
        displayName: 'pbs-main',
        platformId: 'pbs-main',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        status: 'online',
        lastSeen: Date.parse('2026-03-10T10:00:00Z'),
        platformData: {
          pbs: {
            instanceId: 'pbs-main',
            datastores: [
              {
                name: 'fast-store',
                used: 500,
                total: 1000,
                usage: 50,
                status: 'ok',
                deduplicationFactor: 2.25,
              },
            ],
          },
        },
      },
    ];

    facetsPayload = {
      clusters: [],
      nodesAgents: [],
      namespaces: [],
      itemTypes: ['dataset', 'vm'],
      hasSize: true,
      hasVerification: false,
      hasEntityId: false,
    };

    apiFetchMock.mockImplementation(async (url: string) => {
      const u = new URL(url, 'http://localhost');
      if (u.pathname === '/api/recovery/rollups') {
        const data = filterRollupsForRequest(u);
        return {
          data,
          meta: { page: 1, limit: 500, total: data.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/points') {
        const data = filterPointsForRequest(u);
        return {
          data,
          meta: { page: 1, limit: 500, total: data.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/facets') {
        return {
          data: facetsPayload,
        };
      }
      if (u.pathname === '/api/recovery/series') {
        const from = u.searchParams.get('from');
        const to = u.searchParams.get('to');
        if (from && to) {
          const fromDate = new Date(from);
          const toDate = new Date(to);
          const daySpan = Math.round(
            (toDate.getTime() - fromDate.getTime()) / (24 * 60 * 60 * 1000),
          );
          if (daySpan >= 300) {
            return {
              data: Array.from({ length: 365 }, (_, index) => {
                const pointDate = new Date(fromDate);
                pointDate.setDate(pointDate.getDate() + index);
                const isoDay = pointDate.toISOString().slice(0, 10);
                const recent = index > 320 ? 1 : 0;
                return {
                  day: isoDay,
                  total: recent,
                  snapshot: 0,
                  local: 0,
                  remote: recent,
                };
              }),
            };
          }
        }
        return {
          data: [
            { day: '2026-02-13', total: 1, snapshot: 1, local: 0, remote: 0 },
            { day: '2026-02-14', total: 1, snapshot: 0, local: 1, remote: 0 },
          ],
        };
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });
  });

  afterEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
    cleanup();
  });

  it('keeps recovery polling on the shared retained-value query primitive', () => {
    expect(createNonSuspendingQuerySource).toContain('export function createNonSuspendingQuery');
    expect(createNonSuspendingQuerySource).toContain('retaining the last fulfilled value');
    expect(useRecoveryRollupsSource).toContain("from '@/hooks/createNonSuspendingQuery'");
    expect(useRecoveryPointsSource).toContain("from '@/hooks/createNonSuspendingQuery'");
    expect(useRecoveryPointsFacetsSource).toContain("from '@/hooks/createNonSuspendingQuery'");
    expect(useRecoveryPointsSeriesSource).toContain("from '@/hooks/createNonSuspendingQuery'");
    expect(useRecoveryRollupsSource).not.toContain(
      "from '@/features/recovery/createNonSuspendingQuery'",
    );
    expect(useRecoveryPointsSource).not.toContain(
      "from '@/features/recovery/createNonSuspendingQuery'",
    );
    expect(useRecoveryPointsFacetsSource).not.toContain(
      "from '@/features/recovery/createNonSuspendingQuery'",
    );
    expect(useRecoveryPointsSeriesSource).not.toContain(
      "from '@/features/recovery/createNonSuspendingQuery'",
    );
  });

  it('renders protected rollups and resolves unified resource names', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));

    expect(await screen.findByText('VM 123')).toBeInTheDocument();
    expect(screen.getByText('tank/apps')).toBeInTheDocument();
  });

  it('shows one primary recovery table at a time with unified recovery labels', async () => {
    render(() => <Recovery />);

    expect(screen.getByTestId('recovery-page').className).toContain('gap-2');
    expect(screen.getByTestId('recovery-page').className).not.toContain('gap-3');
    const recoveryPage = screen.getByTestId('recovery-page');
    expect(screen.queryByTestId('recovery-summary')).not.toBeInTheDocument();
    expect(screen.queryByText('Posture')).not.toBeInTheDocument();
    expect(screen.queryByText('Freshness')).not.toBeInTheDocument();
    expect(screen.queryByText('Focused drill-in')).not.toBeInTheDocument();
    expect(screen.queryByRole('tablist', { name: /recovery data view/i })).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Protection coverage' })).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getAllByRole('table')).toHaveLength(1);
    });
    const historyTable = screen.getAllByRole('table')[0];
    const historyControls = screen.getByRole('group', { name: /recovery events controls/i });
    const activityBars = screen.getByTestId('recovery-activity-bars');
    expect(recoveryPage).toContainElement(historyControls);
    expect(recoveryPage).toContainElement(historyTable);
    const activityHeading = screen.getByText('Recovery Activity');
    expect(activityBars.parentElement?.className).toContain('h-[136px] sm:h-[150px]');
    expect(
      screen.queryByText('Daily recovery points across the selected history window.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/^Range$/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\/ day/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Lowest active day')).not.toBeInTheDocument();
    expect(screen.queryByText('Below normal')).not.toBeInTheDocument();
    expect(recoveryPage).toContainElement(activityHeading);
    expect(
      activityHeading.compareDocumentPosition(historyControls) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).not.toBe(0);
    expect(
      historyControls.compareDocumentPosition(historyTable) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).not.toBe(0);
    expect(activityHeading.closest('.rounded-md')).not.toBeNull();
    expect(historyControls.closest('.rounded-md')).not.toBeNull();
    expect(historyTable.closest('.rounded-md')).not.toBeNull();
    expect(activityHeading.closest('.rounded-md')).not.toBe(historyControls.closest('.rounded-md'));
    expect(historyControls.closest('.rounded-md')).not.toBe(historyTable.closest('.rounded-md'));
    const historySearch = within(historyControls).getByPlaceholderText(
      'Search recovery history or enter a date...',
    );
    expect(historySearch).toBeInTheDocument();
    expect(historySearch.closest('div.relative')?.className).toContain('w-full');
    expect(screen.getByText(/Showing 1 - 2 of 2 recovery points/i)).toBeInTheDocument();
    expect(within(historyControls).queryByText(/day group/i)).not.toBeInTheDocument();
    expect(within(historyControls).getByRole('button', { name: 'Filter' })).toBeInTheDocument();
    const rangeControls = screen.getByRole('group', { name: 'Recovery activity range' });
    expect(within(rangeControls).getByRole('button', { name: '7d' })).toBeInTheDocument();
    expect(within(rangeControls).getByRole('button', { name: '30d' })).toBeInTheDocument();
    expect(within(rangeControls).getByRole('button', { name: '90d' })).toBeInTheDocument();
    expect(within(rangeControls).getByRole('button', { name: '365d' })).toBeInTheDocument();
    expect(within(historyControls).queryByRole('button', { name: '7d' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '7d' })).toHaveLength(1);
    expect(within(historyTable).getByText('Item Type')).toBeInTheDocument();
    expect(within(historyTable).getByText('Item')).toBeInTheDocument();
    expect(within(historyTable).getByText('Platform')).toBeInTheDocument();
    expect(within(historyTable).queryByText('ITEM TYPE')).not.toBeInTheDocument();
    expect(within(historyTable).queryByText('PLATFORM')).not.toBeInTheDocument();
    expect(within(historyTable).getByText('Target')).toBeInTheDocument();
    expect(within(historyTable).queryByText('Details')).not.toBeInTheDocument();
    const historyRow = within(historyTable).getByLabelText('Healthy').closest('tr');
    expect(historyRow).not.toBeNull();
    expect(within(historyRow!).getByLabelText('Healthy')).toBeInTheDocument();
    expect(within(historyRow!).getByText('VMID 123')).toBeInTheDocument();
    expect(within(historyRow!).getByText('VMID 123').parentElement?.className).toContain(
      'items-baseline',
    );
    expect(within(historyRow!).getByText('VM').className).toContain('bg-');
    expect(within(historyRow!).getByText('VM').className).toContain('rounded');
    expect(within(historyRow!).getByText('VM').className).toContain('px-1 py-0.5');
    expect(within(historyRow!).getByText('Local Copy').className).not.toContain('rounded');
    expect(within(historyRow!).getByText('Success').className).not.toContain('rounded');
    expect(activityHeading).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Protection coverage' }));

    await screen.findByText('VM 123');
    const inventoryControls = screen.getByRole('group', { name: /protection coverage controls/i });
    const protectedSearch = within(inventoryControls).getByPlaceholderText(
      'Search protection coverage...',
    );
    expect(protectedSearch).toBeInTheDocument();
    expect(protectedSearch.closest('div.relative')?.className).toContain('w-full');
    expect(within(inventoryControls).queryByText(/^\d+ stale$/i)).not.toBeInTheDocument();
    expect(within(inventoryControls).queryByText(/^\d+ never succeeded$/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Protected inventory')).not.toBeInTheDocument();
    expect(screen.queryByText('Needs Attention')).not.toBeInTheDocument();
    expect(screen.queryByText(/^2 protected resources$/i)).not.toBeInTheDocument();
    expect(screen.getByText('Showing 2 coverage items')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Prev' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getAllByRole('table')).toHaveLength(1);
    });
    const inventoryTable = screen.getAllByRole('table')[0];
    const inventoryBody = inventoryTable.querySelector('tbody');
    expect(recoveryPage).toContainElement(inventoryControls);
    expect(recoveryPage).toContainElement(inventoryTable);
    expect(
      inventoryControls.compareDocumentPosition(inventoryTable) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).not.toBe(0);
    expect(inventoryControls.closest('.rounded-md')).not.toBeNull();
    expect(inventoryTable.closest('.rounded-md')).not.toBeNull();
    expect(inventoryControls.closest('.rounded-md')).not.toBe(
      inventoryTable.closest('.rounded-md'),
    );
    expect(within(inventoryTable).getByText('Item')).toBeInTheDocument();
    expect(within(inventoryTable).getByText('Item Type')).toBeInTheDocument();
    expect(within(inventoryTable).getByText('Platform')).toBeInTheDocument();
    expect(within(inventoryTable).getByText('Status')).toBeInTheDocument();
    expect(within(inventoryTable).queryByText('ITEM TYPE')).not.toBeInTheDocument();
    expect(within(inventoryTable).queryByText('PLATFORM')).not.toBeInTheDocument();
    expect(inventoryBody?.className).toContain('divide-y');
    expect(inventoryBody?.className).toContain('divide-border');
    const inventoryRows = inventoryBody ? Array.from(inventoryBody.querySelectorAll('tr')) : [];
    expect(inventoryRows.length).toBeGreaterThan(1);
    expect(within(inventoryRows[0]!).getByText('tank/apps')).toBeInTheDocument();
    expect(within(inventoryRows[0]!).getByText('Never succeeded')).toBeInTheDocument();
    expect(within(inventoryTable).getAllByText('VM').length).toBeGreaterThan(0);
    const vmRow = screen.getByText('VM 123').closest('tr');
    expect(vmRow).not.toBeNull();
    expect(within(vmRow!).getByLabelText('Stale')).toBeInTheDocument();
    expect(within(vmRow!).getAllByText('PVE').length).toBeGreaterThan(0);
    expect(within(vmRow!).getAllByText('VM').length).toBeGreaterThan(0);
    expect(within(vmRow!).getByText('VMID 123')).toBeInTheDocument();
    expect(within(vmRow!).getByText('VMID 123').parentElement?.className).toContain(
      'items-baseline',
    );
    expect(within(vmRow!).getAllByText('VM')[0].className).toContain('bg-');
    expect(within(vmRow!).getAllByText('VM')[0].className).toContain('rounded');
    expect(within(vmRow!).getAllByText('VM')[0].className).toContain('px-1 py-0.5');
    expect(within(vmRow!).getByText('Stale').className).not.toContain('rounded');
    expect(screen.queryByText('Backups By Date')).not.toBeInTheDocument();
    expect(screen.queryByText('Recovery Activity')).not.toBeInTheDocument();

    fireEvent.click(await screen.findByText('VM 123'));

    await waitFor(() => {
      expect(screen.getAllByRole('table')).toHaveLength(1);
    });
    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);
    expect(screen.queryByText('Protected inventory')).not.toBeInTheDocument();
    const focusedHistoryTable = screen.getAllByRole('table')[0];
    const focusedHistoryControls = screen.getByRole('group', { name: /recovery events controls/i });
    expect(within(focusedHistoryTable).getAllByText(/^1 event$/i)).toHaveLength(1);
    expect(within(focusedHistoryControls).getByText('VM 123')).toBeInTheDocument();
    expect(within(focusedHistoryTable).getByText('VMID 123')).toBeInTheDocument();
  });

  it('persists the secondary protection coverage view in the route when explicitly opened', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?view=inventory',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });
  });

  it('offers a reciprocal page-header return from protection coverage to recovery events', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?view=inventory',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });
    mockLocationSearch = '?view=inventory';
    mockLocationVersion.bump?.();

    expect(
      await screen.findByRole('button', { name: 'Back to recovery events' }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('group', { name: /recovery events controls/i }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Back to recovery events' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
    });
    mockLocationSearch = '';
    mockLocationVersion.bump?.();

    expect(
      await screen.findByRole('group', { name: /recovery events controls/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('group', { name: /protection coverage controls/i }),
    ).not.toBeInTheDocument();
  });

  it('restores the explicit recovery workspace view from the route', async () => {
    mockLocationSearch = '?view=events';

    render(() => <Recovery />);

    expect(
      await screen.findByRole('group', { name: /recovery events controls/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('group', { name: /protection coverage controls/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Protected inventory')).not.toBeInTheDocument();
  });

  it('derives the recovery events workspace from focused route state when no explicit view is set', async () => {
    mockLocationSearch = '?rollupId=res%3Avm-123';

    render(() => <Recovery />);

    expect(
      await screen.findByRole('group', { name: /recovery events controls/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('group', { name: /protection coverage controls/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Protected inventory')).not.toBeInTheDocument();
  });

  it('renders canonical rollup and history item labels when linked resources are unavailable', async () => {
    rollupsPayload.push({
      rollupId: 'res:vm-404',
      itemResourceId: 'vm-404',
      display: { itemLabel: 'Archive VM' },
      lastAttemptAt: '2026-02-12T08:00:00.000Z',
      lastSuccessAt: '2026-02-12T08:00:00.000Z',
      lastOutcome: 'success',
      platforms: ['proxmox-pve'],
    });
    pointsByRollupId['res:vm-404'] = [
      {
        id: 'p404',
        platform: 'proxmox-pve',
        kind: 'backup',
        mode: 'local',
        outcome: 'success',
        completedAt: '2026-02-12T08:00:00.000Z',
        display: { itemLabel: 'Archive VM' },
      },
    ];

    try {
      render(() => <Recovery />);

      fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
      const item = await screen.findByText('Archive VM');
      fireEvent.click(item);

      await waitFor(() => {
        expect(navigateSpy).toHaveBeenCalledWith(
          '/recovery?rollupId=res%3Avm-404',
          ROUTE_STATE_REPLACE_OPTIONS,
        );
      });

      const tables = await screen.findAllByRole('table');
      const historyTable = tables[tables.length - 1];
      expect(within(historyTable).getByText('Archive VM')).toBeInTheDocument();
      expect(within(historyTable).queryByText('vm-404')).not.toBeInTheDocument();
    } finally {
      rollupsPayload.pop();
      delete pointsByRollupId['res:vm-404'];
    }
  });

  it('does not render decorative recovery summary cards on the event-first surface', async () => {
    render(() => <Recovery />);

    expect(screen.queryByTestId('recovery-summary')).not.toBeInTheDocument();
    expect(screen.queryByText(/^Coverage$/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\d+ protected resources/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/^attention$/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Primary Item')).not.toBeInTheDocument();
    expect(screen.queryByText(/^types$/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Primary Platform')).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Protection coverage' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Recovery activity range' })).toBeInTheDocument();
  });

  it('normalizes legacy provider-shaped recovery payloads before rendering', async () => {
    rollupsPayload.push({
      rollupId: 'ext:legacy-provider-rollup',
      subjectRef: { type: 'truenas-dataset', name: 'tank/legacy', id: 'tank/legacy' },
      lastAttemptAt: '2026-02-12T08:00:00.000Z',
      lastSuccessAt: '2026-02-12T08:00:00.000Z',
      lastOutcome: 'success',
      providers: ['truenas'],
    });
    pointsByRollupId['ext:legacy-provider-rollup'] = [
      {
        id: 'legacy-point',
        provider: 'truenas',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        completedAt: '2026-02-12T08:00:00.000Z',
        display: { itemType: 'dataset', subjectType: 'truenas-dataset' },
      },
    ];

    try {
      render(() => <Recovery />);

      fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
      expect(await screen.findByText('tank/legacy')).toBeInTheDocument();
      expect(screen.getAllByText('TrueNAS').length).toBeGreaterThan(0);

      fireEvent.click(screen.getByText('tank/legacy'));
      // The legacy `providers: ['truenas']` payload should normalize into the
      // canonical "TrueNAS" platform label on whichever surface renders it
      // (events table badge, "+ Filter > Platform" menu, or active chip).
      const eventsControls = await screen.findByRole('group', {
        name: /recovery events controls/i,
      });
      fireEvent.click(within(eventsControls).getByRole('button', { name: 'Filter' }));
      fireEvent.click(await screen.findByRole('menuitem', { name: 'Platform' }));
      const platformMenu = await screen.findByRole('menu');
      expect(within(platformMenu).getByRole('button', { name: 'TrueNAS' })).toBeInTheDocument();
    } finally {
      rollupsPayload.pop();
      delete pointsByRollupId['ext:legacy-provider-rollup'];
    }
  });

  it('surfaces stale protected inventory health even when the latest outcome was successful', async () => {
    rollupsPayload.push({
      rollupId: 'res:vm-stale',
      itemResourceId: 'vm-stale',
      display: { itemLabel: 'Stale VM', itemType: 'vm', entityIdLabel: '999' },
      lastAttemptAt: '2026-02-01T10:00:00.000Z',
      lastSuccessAt: '2026-02-01T10:00:00.000Z',
      lastOutcome: 'success',
      platforms: ['proxmox-pve'],
    });

    try {
      render(() => <Recovery />);

      fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
      const staleRow = (await screen.findByText('Stale VM')).closest('tr');
      expect(staleRow).not.toBeNull();
      expect(within(staleRow!).getByLabelText('Stale')).toBeInTheDocument();
      expect(within(staleRow!).getByText('Stale')).toBeInTheDocument();
      expect(within(staleRow!).queryByText('Success')).not.toBeInTheDocument();
    } finally {
      rollupsPayload.pop();
    }
  });

  it('derives canonical item types from shared fallback fields when itemType is absent', async () => {
    const originalRollupDisplay = rollupsPayload[0].display;
    const originalPointDisplay = pointsByRollupId['res:vm-123'][0].display;
    rollupsPayload[0].display = { subjectType: 'proxmox-vm' };
    pointsByRollupId['res:vm-123'][0].display = { subjectType: 'proxmox-vm' };

    try {
      render(() => <Recovery />);

      fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
      expect(await screen.findByText('VM 123')).toBeInTheDocument();
      const inventoryTable = (await screen.findAllByRole('table'))[0];
      expect(within(inventoryTable).getAllByText('VM').length).toBeGreaterThan(0);

      fireEvent.click(screen.getByText('VM 123'));

      const tables = await screen.findAllByRole('table');
      const historyTable = tables[tables.length - 1];
      expect(within(historyTable).getByText('VM')).toBeInTheDocument();
    } finally {
      rollupsPayload[0].display = originalRollupDisplay;
      pointsByRollupId['res:vm-123'][0].display = originalPointDisplay;
    }
  });

  it('labels Kubernetes cluster recovery subjects without generic cluster wording', async () => {
    rollupsPayload.push({
      rollupId: 'res:k8s-production-1',
      itemResourceId: 'k8s-production-1',
      display: {
        itemLabel: 'Healthy Production',
        itemType: 'cluster',
      },
      lastAttemptAt: '2026-02-15T11:00:00.000Z',
      lastSuccessAt: '2026-02-15T11:00:00.000Z',
      lastOutcome: 'success',
      platforms: ['kubernetes'],
    });
    pointsByRollupId['res:k8s-production-1'] = [];

    try {
      render(() => <Recovery />);

      fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
      expect(await screen.findByText('Healthy Production')).toBeInTheDocument();
      const inventoryTable = (await screen.findAllByRole('table'))[0];
      expect(within(inventoryTable).getAllByText('K8s Cluster').length).toBeGreaterThan(0);
      expect(
        within(inventoryTable).queryByText('Cluster', { exact: true }),
      ).not.toBeInTheDocument();
      // Open the inventory FilterBar's "+ Filter > Item Type" submenu and
      // assert the canonical K8s Cluster wording without the generic alias.
      fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
      fireEvent.click(await screen.findByRole('menuitem', { name: 'Item Type' }));
      const itemTypeMenu = await screen.findByRole('menu');
      expect(within(itemTypeMenu).getByRole('button', { name: 'K8s Cluster' })).toBeInTheDocument();
      expect(
        within(itemTypeMenu).queryByRole('button', { name: 'Cluster' }),
      ).not.toBeInTheDocument();
    } finally {
      rollupsPayload.pop();
      delete pointsByRollupId['res:k8s-production-1'];
    }
  });

  it('renders degraded recovery records when item refs and details metadata are omitted', async () => {
    rollupsPayload.push({
      rollupId: 'res:pvc-1',
      itemResourceId: 'pvc-1',
      display: {
        itemLabel: 'default/data',
        itemType: 'pvc',
      },
      lastAttemptAt: '2026-02-15T11:00:00.000Z',
      lastSuccessAt: '2026-02-15T11:00:00.000Z',
      lastOutcome: 'success',
      platforms: ['kubernetes'],
    });
    pointsByRollupId['res:pvc-1'] = [
      {
        id: 'pvc-point-1',
        platform: 'kubernetes',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        completedAt: '2026-02-15T11:00:00.000Z',
        display: {
          itemLabel: 'default/data',
          itemType: 'pvc',
          namespaceLabel: 'default',
          detailsSummary: 'Immutable copy',
        },
      },
    ];

    try {
      render(() => <Recovery />);

      await screen.findByText('default/data');
      const tables = await screen.findAllByRole('table');
      const historyTable = tables[tables.length - 1];
      expect(within(historyTable).getByText('default/data')).toBeInTheDocument();
      expect(within(historyTable).getByText('PVC')).toBeInTheDocument();
      expect(within(historyTable).getByText('K8s')).toBeInTheDocument();
      expect(within(historyTable).queryByText('Immutable copy')).not.toBeInTheDocument();

      fireEvent.click(within(historyTable).getByText('11:00'));

      const detailsPanel = await screen.findByText('Recovery Point Details');
      const detailsCell = detailsPanel.closest('td');
      expect(detailsCell).not.toBeNull();
      expect(
        within(detailsCell as HTMLTableCellElement).getByText('Item Type'),
      ).toBeInTheDocument();
      expect(within(detailsCell as HTMLTableCellElement).getByText('PVC')).toBeInTheDocument();
      expect(
        within(detailsCell as HTMLTableCellElement).getByText('Namespace / Group'),
      ).toBeInTheDocument();
      expect(within(detailsCell as HTMLTableCellElement).getByText('default')).toBeInTheDocument();
      expect(
        within(detailsCell as HTMLTableCellElement).queryByText('Item Ref'),
      ).not.toBeInTheDocument();
      expect(
        within(detailsCell as HTMLTableCellElement).queryByText('Target Ref'),
      ).not.toBeInTheDocument();
    } finally {
      rollupsPayload.pop();
      delete pointsByRollupId['res:pvc-1'];
    }
  });

  it('keeps recovery history width aligned with canonical column specs', async () => {
    mockLocationSearch = '?rollupId=res%3Avm-123';

    render(() => <Recovery />);

    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);
    const tables = await screen.findAllByRole('table');
    const historyTable = tables.find((table) => within(table).queryByText('Local Copy'));
    expect(historyTable).toBeDefined();
    expect(historyTable).toHaveStyle({ 'min-width': '980px', 'table-layout': 'fixed' });
  });

  it('keeps the protection coverage table CSP-safe when coverage is opened', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    const protectedInventoryTable = await screen.findByRole('table');
    expect(protectedInventoryTable).not.toHaveAttribute('style');
    expect(protectedInventoryTable.className).toContain('table-fixed');
    expect(protectedInventoryTable.className).toContain('min-w-[640px]');
  });

  it('keeps canonical item and platform columns visible when legacy hidden-column ids exist', async () => {
    mockLocationSearch = '?rollupId=res%3Avm-123';
    facetsPayload.clusters = ['lab-cluster'];
    localStorage.setItem(
      STORAGE_KEYS.RECOVERY_HIDDEN_COLUMNS,
      JSON.stringify(['subject', 'source', 'cluster']),
    );

    render(() => <Recovery />);

    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);

    const tables = await screen.findAllByRole('table');
    const historyTable = tables[tables.length - 1];
    expect(within(historyTable).getByText('Item')).toBeInTheDocument();
    expect(within(historyTable).getByText('Platform')).toBeInTheDocument();
    expect(within(historyTable).queryByText('Subject')).not.toBeInTheDocument();
    expect(within(historyTable).queryByText('Source')).not.toBeInTheDocument();
    expect(within(historyTable).queryByText('Cluster / Site')).not.toBeInTheDocument();
  });

  it('focuses history when a rollup is clicked', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    const item = await screen.findByText('VM 123');
    fireEvent.click(item);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?rollupId=res%3Avm-123',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    const controls = await screen.findByRole('group', { name: /recovery events controls/i });
    const itemFilterTrigger = within(controls).getByTestId('recovery-history-item-filter-trigger');
    expect(itemFilterTrigger).toHaveTextContent('VM 123');
    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);
    const tables = await screen.findAllByRole('table');
    const table = tables[tables.length - 1];
    expect(within(table).getAllByText('Local Copy').length).toBeGreaterThan(0);
    expect(within(table).getAllByText('Success').length).toBeGreaterThan(0);

    fireEvent.click(within(table).getByText('10:00'));

    expect(await screen.findByText('Recovery Point Details')).toBeInTheDocument();
    const detailsPanel = screen.getByText('Recovery Point Details').closest('td');
    expect(detailsPanel).not.toBeNull();
    expect(
      within(detailsPanel as HTMLTableCellElement).queryByText('Restore action path'),
    ).not.toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText('Restore readiness'),
    ).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText('Available candidate'),
    ).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText(
        'Succeeded; verification is not recorded.',
      ),
    ).toBeInTheDocument();
    expect(within(detailsPanel as HTMLTableCellElement).getByText('Item Type')).toBeInTheDocument();
    expect(within(detailsPanel as HTMLTableCellElement).getByText('VM')).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText('Cluster / Site'),
    ).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText('Host / Agent'),
    ).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText('Namespace / Group'),
    ).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).queryByText('Target Ref'),
    ).not.toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getByText('Lab Cluster'),
    ).toBeInTheDocument();
    expect(
      within(detailsPanel as HTMLTableCellElement).getAllByText('pve-01').length,
    ).toBeGreaterThan(0);
    expect(within(detailsPanel as HTMLTableCellElement).getByText('Finance')).toBeInTheDocument();
  });

  it('uses the current history result set for cross-day protection chain context', async () => {
    const originalPoints = pointsByRollupId['res:vm-123'];
    pointsByRollupId['res:vm-123'] = [
      {
        ...originalPoints[0],
        id: 'p1-local',
        itemResourceId: 'vm-123',
        completedAt: '2026-02-14T10:00:00.000Z',
      },
      {
        id: 'p1-remote',
        platform: 'proxmox-pbs',
        itemResourceId: 'vm-123',
        kind: 'backup',
        mode: 'remote',
        outcome: 'success',
        verified: true,
        completedAt: '2026-02-13T09:30:00.000Z',
        repositoryRef: {
          type: 'pbs-datastore',
          namespace: 'pbs-main',
          name: 'fast-store',
        },
        display: {
          itemType: 'vm',
          subjectType: 'proxmox-vm',
          entityIdLabel: '123',
        },
      },
    ];

    try {
      mockLocationSearch = '?rollupId=res%3Avm-123';
      render(() => <Recovery />);

      await screen.findByText(/Showing 1 - 2 of 2 recovery points/i);
      fireEvent.click(screen.getByText('10:00'));

      const detailsPanel = await screen.findByText('Recovery Point Details');
      const detailsCell = detailsPanel.closest('td');
      expect(detailsCell).not.toBeNull();
      expect(
        within(detailsCell as HTMLTableCellElement).getByText('Protection chain'),
      ).toBeInTheDocument();
      expect(
        within(detailsCell as HTMLTableCellElement).getByText('Remote copy'),
      ).toBeInTheDocument();
      expect(
        within(detailsCell as HTMLTableCellElement).getByText(/fast-store · PBS/i),
      ).toBeInTheDocument();
    } finally {
      pointsByRollupId['res:vm-123'] = originalPoints;
    }
  });

  it('keeps recovery event context visible when mobile columns collapse', async () => {
    const originalWidth = window.innerWidth;
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      writable: true,
      value: 500,
    });

    try {
      mockLocationSearch = '?rollupId=res%3Avm-123';
      render(() => <Recovery />);

      await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);

      const tables = await screen.findAllByRole('table');
      const historyTable = tables[tables.length - 1];
      const historyRow = within(historyTable).getByText('VMID 123').closest('tr');
      expect(historyRow).not.toBeNull();
      expect(within(historyRow!).getByText('Local Copy')).toBeInTheDocument();
      expect(within(historyRow!).getByText('PVE')).toBeInTheDocument();
      expect(within(historyRow!).getByText('fast-store')).toBeInTheDocument();
      expect(within(historyRow!).getByText('Verification unknown')).toBeInTheDocument();
      expect(within(historyTable).queryByText('Method')).not.toBeInTheDocument();
      expect(within(historyTable).queryByText('Target')).not.toBeInTheDocument();
    } finally {
      Object.defineProperty(window, 'innerWidth', {
        configurable: true,
        writable: true,
        value: originalWidth,
      });
    }
  });

  it('keeps optional history placement columns on the neutral recovery vocabulary', async () => {
    facetsPayload.clusters = ['lab-cluster'];
    facetsPayload.nodesAgents = ['pve-01'];
    facetsPayload.namespaces = ['finance'];
    mockLocationSearch = '?rollupId=res%3Avm-123';

    render(() => <Recovery />);

    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);

    fireEvent.click(screen.getByRole('button', { name: /columns/i }));

    expect(await screen.findByText('Cluster / Site')).toBeInTheDocument();
    expect(screen.getByText('Host / Agent')).toBeInTheDocument();
    expect(screen.getByText('Namespace / Group')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Cluster / Site'));

    const tables = await screen.findAllByRole('table');
    const table = tables[tables.length - 1];
    expect(within(table).getByText('Cluster / Site')).toBeInTheDocument();
    expect(within(table).getByText('Lab Cluster')).toBeInTheDocument();
  });

  it('filters protected rollups by platform', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    expect(await screen.findByText('VM 123')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Platform' }));
    const platformMenu = await screen.findByRole('menu');
    fireEvent.click(within(platformMenu).getByRole('button', { name: 'TrueNAS' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?view=inventory&platform=truenas',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
    });
    expect(screen.getByText('tank/apps')).toBeInTheDocument();

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('platform=truenas'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('platform=truenas'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('platform=truenas'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('platform=truenas'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('keeps route-owned recovery platform and node filters visible while options hydrate', async () => {
    mockLocationSearch = '?view=events&platform=truenas&node=tower';
    apiFetchMock.mockImplementation(() => new Promise(() => {}));

    render(() => <Recovery />);

    // FilterBar renders active filters as chips. Route-owned platform and
    // node values become removable chips in the events bar.
    expect(await screen.findByRole('button', { name: /^Platform: TrueNAS/ })).toBeInTheDocument();
    expect(
      await screen.findByRole('button', { name: /^Host \/ Agent: tower/ }),
    ).toBeInTheDocument();
  });

  it('uses the shared reset action for protection coverage filters', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    expect(await screen.findByText('VM 123')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Clear all' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Platform' }));
    const platformMenu = await screen.findByRole('menu');
    fireEvent.click(within(platformMenu).getByRole('button', { name: 'TrueNAS' }));

    const clearAllButton = await screen.findByRole('button', { name: 'Clear all' });
    fireEvent.click(clearAllButton);

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /^Platform: TrueNAS/ })).not.toBeInTheDocument();
      expect(screen.getByText('VM 123')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Clear all' })).not.toBeInTheDocument();
    });
  });

  it('treats the focused rollup as part of the recovery events filter surface', async () => {
    mockLocationSearch = '?rollupId=res%3Avm-123';

    render(() => <Recovery />);

    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);

    const controls = await screen.findByRole('group', { name: /recovery events controls/i });
    expect(within(controls).getByTestId('recovery-history-item-filter-trigger')).toHaveTextContent(
      'VM 123',
    );

    fireEvent.click(within(controls).getByRole('button', { name: 'Clear all' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
      expect(screen.getByTestId('recovery-history-item-filter-trigger')).toHaveTextContent(
        'Any item',
      );
      expect(screen.queryByRole('button', { name: 'Clear all' })).not.toBeInTheDocument();
    });
  });

  it('treats chart day selection as a shared recovery events filter', async () => {
    render(() => <Recovery />);

    const bars = await screen.findByTestId('recovery-activity-bars');
    fireEvent.click(within(bars).getByRole('button', { name: /Feb 14: 1 recovery point/ }));

    const controls = await screen.findByRole('group', { name: /recovery events controls/i });
    expect(
      await within(controls).findByRole('button', {
        name: /^Day: Feb 14, 2026 - 1 recovery point/,
      }),
    ).toBeInTheDocument();
    expect(within(controls).getByRole('button', { name: 'Remove Day filter' })).toBeInTheDocument();
    expect(screen.getByText(/Showing 1 - 1 of 1 recovery points/i)).toBeInTheDocument();

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?day=2026-02-14',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    fireEvent.click(within(controls).getByRole('button', { name: 'Clear all' }));

    await waitFor(() => {
      expect(
        within(controls).queryByRole('button', {
          name: /^Day: Feb 14, 2026 - 1 recovery point/,
        }),
      ).not.toBeInTheDocument();
      expect(within(bars).queryByRole('button', { pressed: true })).not.toBeInTheDocument();
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
    });
  });

  it('keeps recovery event utility controls grouped on the shared toolbar rail', async () => {
    mockLocationSearch = '?view=events';

    render(() => <Recovery />);

    const controls = await screen.findByRole('group', { name: /recovery events controls/i });
    // Both the "+ Filter" trigger and the Columns picker live inside the
    // events controls group; the item filter trigger is also grouped here.
    expect(within(controls).getByRole('button', { name: 'Filter' })).toBeInTheDocument();
    expect(within(controls).getByRole('button', { name: /columns/i })).toBeInTheDocument();
    expect(
      within(controls).getByTestId('recovery-history-item-filter-trigger'),
    ).toBeInTheDocument();
    // Recovery events opted into shared saved views — the bookmark menu
    // sits next to "+ Filter" and persists named filter combos under the
    // canonical localStorage key.
    expect(within(controls).getByRole('button', { name: 'Saved views' })).toBeInTheDocument();
  });

  it('lets operators create and clear the item filter from the recovery events controls', async () => {
    render(() => <Recovery />);

    const controls = await screen.findByRole('group', { name: /recovery events controls/i });
    const itemFilterTrigger = within(controls).getByTestId('recovery-history-item-filter-trigger');
    expect(itemFilterTrigger).toHaveTextContent('Any item');

    fireEvent.click(itemFilterTrigger);
    expect(await screen.findByTestId('recovery-history-item-filter-panel')).toBeInTheDocument();

    fireEvent.click(await screen.findByTestId('recovery-history-item-filter-option-ext:truenas-1'));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?rollupId=ext%3Atruenas-1',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    expect(screen.getByTestId('recovery-history-item-filter-trigger')).toHaveTextContent(
      'tank/apps',
    );

    fireEvent.click(screen.getByTestId('recovery-history-item-filter-trigger'));
    fireEvent.click(await screen.findByTestId('recovery-history-item-filter-clear'));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
      expect(screen.getByTestId('recovery-history-item-filter-trigger')).toHaveTextContent(
        'Any item',
      );
    });
  });

  it('keeps recovery filter surfaces on canonical platform vocabulary', async () => {
    render(() => <Recovery />);

    // Events FilterBar surfaces "Platform" via the "+ Filter" menu rather
    // than as a labelled select, so assert the menu vocabulary instead.
    fireEvent.click(await screen.findByRole('button', { name: 'Filter' }));
    expect(await screen.findByRole('menuitem', { name: 'Platform' })).toBeInTheDocument();
    expect(screen.queryByText('All Providers')).not.toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Filter' }));
    expect(await screen.findByRole('menuitem', { name: 'Platform' })).toBeInTheDocument();
    expect(screen.queryByText('All Providers')).not.toBeInTheDocument();
  });

  it('filters recovery transport by canonical item type', async () => {
    render(() => <Recovery />);

    expect(await screen.findByText('VM 123')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Item Type' }));
    const itemTypeMenu = await screen.findByRole('menu');
    fireEvent.click(within(itemTypeMenu).getByRole('button', { name: 'Dataset' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?itemType=dataset',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
    });
    expect(screen.getByText('tank/apps')).toBeInTheDocument();

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('itemType=dataset'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('itemType=dataset'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('itemType=dataset'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('itemType=dataset'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('uses one canonical free-text query across protection coverage and history transport', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    const protectedSearch = await screen.findByPlaceholderText('Search protection coverage...');
    fireEvent.input(protectedSearch, { target: { value: 'tank' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?view=inventory&q=tank',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    await waitFor(() => {
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
      expect(screen.getByText('tank/apps')).toBeInTheDocument();
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('q=tank'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('q=tank'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('q=tank'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('q=tank'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('promotes clear day text in recovery search to the shared Day filter', async () => {
    render(() => <Recovery />);

    const historyControls = await screen.findByRole('group', {
      name: /recovery events controls/i,
    });
    const historySearch = within(historyControls).getByPlaceholderText(
      'Search recovery history or enter a date...',
    );

    fireEvent.input(historySearch, { target: { value: 'Feb 14' } });

    await waitFor(() => {
      expect(
        within(historyControls).getByText('Feb 14, 2026 - 1 recovery point'),
      ).toBeInTheDocument();
      expect(historySearch).toHaveValue('');
      expect(navigateSpy).toHaveBeenCalledWith(
        expect.stringContaining('day=2026-02-14'),
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    const selectedStart = parseRecoveryDateKey('2026-02-14');
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
      expect(latestPointsUrl).not.toContain('q=Feb');
    });
  });

  it('uses canonical protection state filtering for protected inventory', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Filter' }));
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Protection state' }));
    const stateMenu = await screen.findByRole('menu');
    fireEvent.click(within(stateMenu).getByRole('button', { name: 'Never succeeded' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?view=inventory&state=never-succeeded',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    await waitFor(() => {
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
      expect(screen.getByText('tank/apps')).toBeInTheDocument();
    });

    const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
    expect(urls.some((url) => url.includes('outcome=failed'))).toBe(false);
  });

  it('bounds the protected inventory surface with pagination for larger estates', async () => {
    const largeRollups = Array.from({ length: 26 }, (_, index) => ({
      rollupId: `estate:item-${index + 1}`,
      itemResourceId: `estate-item-${index + 1}`,
      display: {
        itemLabel: `estate-item-${index + 1}`,
        itemType: index % 2 === 0 ? 'container' : 'vm',
      },
      lastAttemptAt: '2026-02-14T10:00:00.000Z',
      lastSuccessAt: '2026-02-14T10:00:00.000Z',
      lastOutcome: 'success',
      platforms: ['proxmox-pbs'],
    }));

    apiFetchMock.mockImplementation(async (url: string) => {
      const u = new URL(url, 'http://localhost');
      if (u.pathname === '/api/recovery/rollups') {
        return {
          data: largeRollups,
          meta: { page: 1, limit: 500, total: largeRollups.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/points') {
        return {
          data: [],
          meta: { page: 1, limit: 500, total: 0, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/facets') {
        return {
          data: {
            clusters: [],
            nodesAgents: [],
            namespaces: [],
            itemTypes: ['container', 'vm'],
            hasSize: true,
            hasVerification: false,
            hasEntityId: false,
          },
        };
      }
      if (u.pathname === '/api/recovery/series') {
        return {
          data: [
            { day: '2026-02-13', total: 1, snapshot: 1, local: 0, remote: 0 },
            { day: '2026-02-14', total: 1, snapshot: 0, local: 1, remote: 0 },
          ],
        };
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: 'Protection coverage' }));
    expect(await screen.findByText('Showing 26 coverage items')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Prev' })).not.toBeInTheDocument();
  });

  it('canonicalizes legacy stale flags into protected inventory state', async () => {
    mockLocationSearch = '?stale=%20TRUE%20';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?view=inventory&state=stale',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    // FilterBar surfaces the active value as a chip rather than a labelled
    // select. Assert the canonical chip text instead.
    expect(
      await screen.findByRole('button', { name: /^Protection state: Stale$/ }),
    ).toBeInTheDocument();
  });

  it('drops protected-only stale state when the recovery events view owns the route', async () => {
    mockLocationSearch = '?view=events&stale=1&range=90&status=failed';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?range=90&status=failed',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    const controls = await screen.findByRole('group', { name: /recovery events controls/i });
    fireEvent.click(within(controls).getByRole('button', { name: 'Clear all' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
    });
  });

  it('normalizes legacy provider aliases into canonical platform route state', async () => {
    mockLocationSearch = '?provider=proxmox';
    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?platform=proxmox-pve',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });
  });

  it('collapses unknown recovery platform values back to canonical unset state', async () => {
    mockLocationSearch = '?provider=%20custom-provider%20';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const filteredUrls = urls.filter(
        (url) =>
          url.includes('/api/recovery/rollups') ||
          url.includes('/api/recovery/points') ||
          url.includes('/api/recovery/facets') ||
          url.includes('/api/recovery/series'),
      );
      expect(
        filteredUrls.some(
          (url) =>
            url.includes('platform=custom-provider') || url.includes('provider=custom-provider'),
        ),
      ).toBe(false);
    });
  });

  it('adds cluster to the URL and API queries when the cluster filter changes', async () => {
    facetsPayload.clusters = ['dev-cluster', 'prod-cluster'];

    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: /^filter$/i }));

    fireEvent.click(await screen.findByRole('menuitem', { name: 'Cluster / Site' }));
    const clusterMenu = await screen.findByRole('menu');
    fireEvent.click(within(clusterMenu).getByRole('button', { name: 'dev-cluster' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?cluster=dev-cluster',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('cluster=dev-cluster'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('cluster=dev-cluster'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('cluster=dev-cluster'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('cluster=dev-cluster'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('collapses explicit all recovery filters back to canonical route state', async () => {
    mockLocationSearch =
      '?provider=%20ALL%20&scope=%20ALL%20&mode=%20all%20&status=%20ALL%20&verification=%20All%20&cluster=%20ALL%20&node=%20all%20&namespace=%20All%20';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const filteredUrls = urls.filter(
        (url) =>
          url.includes('/api/recovery/rollups') ||
          url.includes('/api/recovery/points') ||
          url.includes('/api/recovery/facets') ||
          url.includes('/api/recovery/series'),
      );
      expect(
        filteredUrls.some(
          (url) =>
            url.includes('platform=all') ||
            url.includes('platform=ALL') ||
            url.includes('provider=all') ||
            url.includes('provider=ALL') ||
            url.includes('scope=all') ||
            url.includes('scope=ALL') ||
            url.includes('mode=all') ||
            url.includes('mode=ALL') ||
            url.includes('status=all') ||
            url.includes('status=ALL') ||
            url.includes('verification=all') ||
            url.includes('verification=ALL') ||
            url.includes('cluster=all') ||
            url.includes('cluster=ALL') ||
            url.includes('node=all') ||
            url.includes('node=ALL') ||
            url.includes('namespace=all') ||
            url.includes('namespace=All'),
        ),
      ).toBe(false);
    });
  });

  it('keeps facets aligned with node and namespace history filters', async () => {
    facetsPayload.nodesAgents = ['node-agent-1', 'node-agent-2'];
    facetsPayload.namespaces = ['tenant-a', 'tenant-b'];

    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: /^filter$/i }));

    fireEvent.click(await screen.findByRole('menuitem', { name: 'Host / Agent' }));
    const hostMenu = await screen.findByRole('menu');
    fireEvent.click(within(hostMenu).getByRole('button', { name: 'node-agent-1' }));

    fireEvent.click(await screen.findByRole('button', { name: /^filter$/i }));
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Namespace / Group' }));
    const namespaceMenu = await screen.findByRole('menu');
    fireEvent.click(within(namespaceMenu).getByRole('button', { name: 'tenant-a' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?namespace=tenant-a&node=node-agent-1',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) =>
          url.includes('/api/recovery/rollups') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      const hasPoints = urls.some(
        (url) =>
          url.includes('/api/recovery/points') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      const hasSeries = urls.some(
        (url) =>
          url.includes('/api/recovery/series') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      const hasFacets = urls.some(
        (url) =>
          url.includes('/api/recovery/facets') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('keeps the history card mounted while filter refetches are in flight', async () => {
    mockLocationSearch = '?rollupId=res%3Avm-123';
    facetsPayload.clusters = ['dev-cluster'];

    let delayedPointsReady = false;
    let resolveDelayedPoints!: (value: RecoveryPointsResponse) => void;

    apiFetchMock.mockImplementation(async (url: string) => {
      const u = new URL(url, 'http://localhost');
      if (u.pathname === '/api/recovery/rollups') {
        return {
          data: rollupsPayload,
          meta: { page: 1, limit: 500, total: rollupsPayload.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/points') {
        const rid = u.searchParams.get('rollupId') || '';
        const data = pointsByRollupId[rid] || [];
        if (u.searchParams.get('cluster') === 'dev-cluster') {
          return await new Promise<RecoveryPointsResponse>((resolve) => {
            delayedPointsReady = true;
            resolveDelayedPoints = resolve as (value: RecoveryPointsResponse) => void;
          });
        }
        return {
          data,
          meta: { page: 1, limit: 500, total: data.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/facets') {
        return {
          data: facetsPayload,
        };
      }
      if (u.pathname === '/api/recovery/series') {
        return {
          data: [
            { day: '2026-02-13', total: 1, snapshot: 1, local: 0, remote: 0 },
            { day: '2026-02-14', total: 1, snapshot: 0, local: 1, remote: 0 },
          ],
        };
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    render(() => <Recovery />);

    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);

    fireEvent.click(screen.getByRole('button', { name: /^filter$/i }));
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Cluster / Site' }));
    const clusterMenu = await screen.findByRole('menu');
    fireEvent.click(within(clusterMenu).getByRole('button', { name: 'dev-cluster' }));

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      expect(
        urls.some(
          (url) => url.includes('/api/recovery/points') && url.includes('cluster=dev-cluster'),
        ),
      ).toBe(true);
    });

    expect(screen.getByText(/Showing 1 - 1 of 1 recovery points/i)).toBeInTheDocument();

    if (!delayedPointsReady) {
      throw new Error('Expected delayed recovery points request to be pending');
    }
    resolveDelayedPoints({
      data: pointsByRollupId['res:vm-123'],
      meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
    });

    await waitFor(() => {
      expect(screen.getByText(/Showing 1 - 1 of 1 recovery points/i)).toBeInTheDocument();
    });
  });

  it('narrows recovery point history to the selected timeline day', async () => {
    render(() => <Recovery />);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      expect(pointUrls.length).toBeGreaterThan(0);
    });

    const initialPointUrls = apiFetchMock.mock.calls
      .map((call) => String(call[0] || ''))
      .filter((url) => url.includes('/api/recovery/points'));
    const initialPointsUrl = initialPointUrls[initialPointUrls.length - 1];

    const timelineButtons = await screen.findAllByRole('button', { name: /recovery point/i });
    fireEvent.click(timelineButtons[0]);

    const selectedDay = '2026-02-13';
    const selectedStart = parseRecoveryDateKey(selectedDay);
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).not.toBe(initialPointsUrl);
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });

    await waitFor(() => {
      const facetUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/facets'));
      const latestFacetsUrl = facetUrls[facetUrls.length - 1];
      expect(latestFacetsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestFacetsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        expect.stringContaining('day=2026-02-13'),
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });
  });

  it('restores selected timeline day from the recovery URL', async () => {
    mockLocationSearch = '?day=2026-02-13';

    render(() => <Recovery />);

    expect(
      await screen.findByRole('group', { name: /recovery events controls/i }),
    ).toBeInTheDocument();

    const selectedStart = parseRecoveryDateKey('2026-02-13');
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });
  });

  it('canonicalizes whitespace-padded timeline day and range from the recovery URL', async () => {
    mockLocationSearch = '?day=%202026-02-13%20&range=%207%20';

    render(() => <Recovery />);

    expect(
      await screen.findByRole('group', { name: /recovery events controls/i }),
    ).toBeInTheDocument();

    const selectedStart = parseRecoveryDateKey('2026-02-13');
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });

    await waitFor(() => {
      const rollupUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/rollups'));
      const latestRollupsUrl = rollupUrls[rollupUrls.length - 1];
      expect(latestRollupsUrl).toContain('from=');
      expect(latestRollupsUrl).toContain('to=');
    });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?range=7&day=2026-02-13',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });
  });

  it('persists the selected timeline range in the recovery URL', async () => {
    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: '7d' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?range=7', ROUTE_STATE_REPLACE_OPTIONS);
    });
  });

  it('restores the selected timeline range from the recovery URL', async () => {
    mockLocationSearch = '?range=7';

    render(() => <Recovery />);

    await screen.findByRole('group', { name: /recovery events controls/i });

    const end = new Date();
    end.setHours(23, 59, 59, 999);
    const start = new Date(end);
    start.setDate(start.getDate() - 6);
    start.setHours(0, 0, 0, 0);

    await waitFor(() => {
      const rollupUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/rollups'));
      const seriesUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/series'));
      const latestRollupsUrl = rollupUrls[rollupUrls.length - 1];
      const latestSeriesUrl = seriesUrls[seriesUrls.length - 1];
      expect(latestRollupsUrl).toContain(`from=${encodeURIComponent(start.toISOString())}`);
      expect(latestRollupsUrl).toContain(`to=${encodeURIComponent(end.toISOString())}`);
      expect(latestSeriesUrl).toContain(`from=${encodeURIComponent(start.toISOString())}`);
      expect(latestSeriesUrl).toContain(`to=${encodeURIComponent(end.toISOString())}`);
    });
  });

  it('renders sparse 365d timeline ticks instead of one label slot per day', async () => {
    mockLocationSearch = '?range=365';

    const { container } = render(() => <Recovery />);

    expect(
      await screen.findByRole('group', { name: /recovery events controls/i }),
    ).toBeInTheDocument();

    const labels = await waitFor(() => {
      const labelsRow = container
        .querySelector('[data-testid="recovery-activity-bars"]')
        ?.parentElement?.querySelector('.pointer-events-none.absolute.inset-x-0.bottom-0.h-4');
      const renderedLabels = Array.from(labelsRow?.querySelectorAll('span') || []);
      expect(renderedLabels.length).toBeGreaterThan(0);
      return renderedLabels;
    });

    expect(labels.length).toBeLessThan(50);
    expect(labels[0]?.textContent).toBeTruthy();
    expect(labels.at(-1)?.textContent).toBeTruthy();
  });

  it('clears the shared recovery search query on Escape', async () => {
    mockLocationSearch = '?q=gdfdgd';
    render(() => <Recovery />);

    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', ROUTE_STATE_REPLACE_OPTIONS);
    });
  });
});
