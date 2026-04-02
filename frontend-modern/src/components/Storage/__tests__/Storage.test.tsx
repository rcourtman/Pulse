import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI } from '@/api/charts';
import StorageSummary from '@/components/Storage/StorageSummary';
import storageSummarySource from '@/components/Storage/StorageSummary.tsx?raw';
import type { Alert } from '@/types/api';
import type { Resource, ResourceType } from '@/types/resource';
import Storage from '@/components/Storage/Storage';
import { ROUTE_STATE_REPLACE_OPTIONS } from '@/utils/routeStateNavigation';

// Stub ResizeObserver for jsdom (used by HistoryChart in pool detail panels)
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

// Stub HTMLCanvasElement.getContext for jsdom (used by HistoryChart canvas rendering)
if (typeof HTMLCanvasElement.prototype.getContext === 'function') {
  const fakeGradient = { addColorStop: () => {} };
  const fakeContext = {
    setTransform: () => {},
    clearRect: () => {},
    beginPath: () => {},
    moveTo: () => {},
    lineTo: () => {},
    stroke: () => {},
    fill: () => {},
    fillText: () => {},
    measureText: () => ({ width: 0 }),
    createLinearGradient: () => fakeGradient,
    setLineDash: () => {},
    save: () => {},
    restore: () => {},
    arc: () => {},
    closePath: () => {},
  };
  HTMLCanvasElement.prototype.getContext = function () {
    return fakeContext as unknown as ReturnType<typeof HTMLCanvasElement.prototype.getContext>;
  } as typeof HTMLCanvasElement.prototype.getContext;
}

let mockLocationSearch = '';
let mockLocationPath = '/storage';
const navigateSpy = vi.fn();

let wsConnected = true;
let wsInitialDataReceived = true;
let wsReconnecting = false;
let wsActiveAlerts: Record<string, Alert> = {};
let wsState: any = {
  resources: [] as Resource[],
  storage: [],
  pbs: [],
};
const reconnectSpy = vi.fn();

let hookResources: Resource[] = [];
let hookLoading = false;
let hookError: unknown = undefined;
let alertsActivationState: 'active' | 'pending_review' | 'snoozed' | null = 'active';
const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
  const url =
    typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
  if (url.includes('/api/license/entitlements')) {
    return new Response(JSON.stringify({ entitlements: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }
  if (url.includes('/api/metrics-store/history')) {
    return new Response(JSON.stringify({ points: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }
  return new Response(JSON.stringify({}), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
});

let nodeResources: Resource[] = [
  {
    id: 'node-1',
    type: 'agent',
    name: 'pve1',
    displayName: 'pve1',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    uptime: 1000,
    lastSeen: Date.now(),
    platformData: { proxmox: { instance: 'cluster-main' } },
  },
  {
    id: 'node-2',
    type: 'agent',
    name: 'pve2',
    displayName: 'pve2',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    uptime: 1000,
    lastSeen: Date.now(),
    platformData: { proxmox: { instance: 'cluster-main' } },
  },
];

type StorageResourceOptions = {
  platformId?: string;
  platformType?: string;
  status?: string;
  total?: number;
  used?: number;
  free?: number;
  current?: number;
  storageType?: string;
  content?: string;
  shared?: boolean;
  parentId?: string;
  parentName?: string;
  includePlatformNode?: boolean;
  platformData?: Record<string, unknown>;
  storage?: Resource['storage'];
  pbs?: Resource['pbs'];
  incidentCategory?: Resource['incidentCategory'];
  incidentSeverity?: Resource['incidentSeverity'];
  incidentPriority?: number;
  incidentLabel?: Resource['incidentLabel'];
  incidentSummary?: Resource['incidentSummary'];
  incidentImpactSummary?: Resource['incidentImpactSummary'];
  incidentAction?: Resource['incidentAction'];
  metricsTarget?: Resource['metricsTarget'];
};

const buildStorageResource = (
  id: string,
  name: string,
  node: string,
  options: StorageResourceOptions = {},
): Resource => ({
  id,
  type: 'storage',
  name,
  displayName: name,
  platformId: options.platformId || 'cluster-main',
  platformType: (options.platformType || 'proxmox-pve') as Resource['platformType'],
  sourceType: 'api',
  parentId: options.parentId,
  parentName: options.parentName,
  status: (options.status || 'online') as Resource['status'],
  incidentCategory: options.incidentCategory,
  incidentSeverity: options.incidentSeverity,
  incidentPriority: options.incidentPriority,
  incidentLabel: options.incidentLabel,
  incidentSummary: options.incidentSummary,
  incidentImpactSummary: options.incidentImpactSummary,
  incidentAction: options.incidentAction,
  metricsTarget: options.metricsTarget,
  storage: options.storage,
  pbs: options.pbs,
  disk: {
    current: options.current ?? 50,
    total: options.total ?? 1_000,
    used: options.used ?? 500,
    free: options.free ?? 500,
  },
  lastSeen: Date.now(),
  platformData: {
    ...(options.includePlatformNode === false ? {} : { node }),
    type: options.storageType || 'lvm',
    content: options.content || 'images,rootdir',
    shared: options.shared ?? false,
    ...(options.platformData || {}),
  },
});

const buildPhysicalDiskResource = (
  id: string,
  parentId: string | null,
  nodeName: string,
  instance = 'cluster-main',
): Resource => ({
  id,
  type: 'physical_disk',
  name: `/dev/${id}`,
  displayName: `/dev/${id}`,
  platformId: instance,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  parentId: parentId ?? undefined,
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: nodeName },
  canonicalIdentity: { hostname: nodeName },
  platformData: {
    ...(parentId ? { proxmox: { nodeName, instance } } : {}),
    physicalDisk: {
      devPath: `/dev/${id}`,
      model: 'Test Disk',
      health: 'PASSED',
    },
  },
});

const buildAlert = (
  id: string,
  resourceId: string,
  level: Alert['level'],
  acknowledged = false,
): Alert => ({
  id,
  type: 'storage-capacity',
  level,
  resourceId,
  resourceName: resourceId,
  node: 'pve1',
  instance: 'cluster-main',
  message: 'alert',
  value: 95,
  threshold: 90,
  startTime: '2026-02-08T00:00:00Z',
  acknowledged,
});

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockLocationPath, search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: wsState,
    activeAlerts: wsActiveAlerts,
    connected: () => wsConnected,
    initialDataReceived: () => wsInitialDataReceived,
    reconnecting: () => wsReconnecting,
    reconnect: reconnectSpy,
  }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => alertsActivationState,
  }),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useStorageRecoveryResources: () => ({
    resources: () => hookResources,
    loading: () => hookLoading,
    error: () => hookError,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
  useUnifiedResources: () => ({
    resources: () => nodeResources,
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => nodeResources,
    infra: () => nodeResources,
    workloads: () => [],
    byType: (type: ResourceType) => nodeResources.filter((r) => r.type === type),
    byPlatform: () => [],
    filtered: () => [],
    get: (id: string) => nodeResources.find((r) => r.id === id),
    children: () => [],
    statusCounts: () => ({}),
    topByCpu: () => [],
    topByMemory: () => [],
    hasResources: () => nodeResources.length > 0,
  }),
}));

vi.mock('@/components/Storage/DiskList', () => ({
  DiskList: (props: { nodes: Resource[]; selectedNode: string | null; searchTerm: string }) => (
    <div data-testid="disk-list">
      disk-view:{props.selectedNode || 'all'}:{props.searchTerm || ''}
    </div>
  ),
}));

describe('Storage', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock);
    fetchMock.mockClear();
    mockLocationPath = '/storage';
    mockLocationSearch = '';
    navigateSpy.mockReset();

    wsConnected = true;
    wsInitialDataReceived = true;
    wsReconnecting = false;
    wsActiveAlerts = {};
    alertsActivationState = 'active';
    reconnectSpy.mockReset();
    wsState = {
      resources: [],
      storage: [],
      pbs: [],
    };
    nodeResources = [
      {
        id: 'node-1',
        type: 'agent',
        name: 'pve1',
        displayName: 'pve1',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        platformData: { proxmox: { instance: 'cluster-main' } },
      },
      {
        id: 'node-2',
        type: 'agent',
        name: 'pve2',
        displayName: 'pve2',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        platformData: { proxmox: { instance: 'cluster-main' } },
      },
    ];
    hookResources = [];
    hookLoading = false;
    hookError = undefined;
  });

  afterEach(() => {
    cleanup();
  });

  it('renders pools from v2 resources and filters by selected node', () => {
    hookResources = [
      buildStorageResource('storage-1', 'Local-LVM-PVE1', 'pve1', {
        parentId: 'node-1',
        parentName: 'pve1',
        includePlatformNode: false,
      }),
      buildStorageResource('storage-2', 'Local-LVM-PVE2', 'pve2', {
        parentId: 'node-2',
        parentName: 'pve2',
        includePlatformNode: false,
      }),
    ];

    render(() => <Storage />);

    expect(screen.getByText('Local-LVM-PVE1')).toBeInTheDocument();
    expect(screen.getByText('Local-LVM-PVE1')).toBeInTheDocument();
    expect(screen.getByText('Local-LVM-PVE2')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Node'), {
      target: { value: 'node-1' },
    });

    expect(screen.getByText('Local-LVM-PVE1')).toBeInTheDocument();
    expect(screen.queryByText('Local-LVM-PVE2')).not.toBeInTheDocument();
  });

  it('routes hovered pool rows into the shared summary highlight contract and keeps the summary sticky', async () => {
    const storageSummarySpy = vi.spyOn(ChartsAPI, 'getStorageSummaryCharts').mockResolvedValue({
      pools: {
        'pool:alpha': {
          name: 'Alpha-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 45 },
            { timestamp: Date.now(), value: 47 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 450 },
            { timestamp: Date.now(), value: 470 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 550 },
            { timestamp: Date.now(), value: 530 },
          ],
        },
        'pool:beta': {
          name: 'Beta-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 68 },
            { timestamp: Date.now(), value: 70 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 680 },
            { timestamp: Date.now(), value: 700 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 320 },
            { timestamp: Date.now(), value: 300 },
          ],
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: Date.now() - 60_000,
      },
    });

    hookResources = [
      buildStorageResource('storage-display-alpha', 'Alpha-Store', 'pve1', {
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:alpha',
        },
      }),
      buildStorageResource('storage-display-beta', 'Beta-Store', 'pve2', {
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:beta',
        },
      }),
    ];

    render(() => <Storage />);

    const summary = await screen.findByTestId('storage-summary');
    const stickyWrapper = summary.closest('[data-sticky-summary="true"]');
    expect(stickyWrapper).toHaveAttribute('data-sticky-summary-desktop-only', 'false');
    expect(stickyWrapper?.className).toContain('sticky');
    expect(stickyWrapper?.className).toContain('top-0');

    const alphaRow = screen.getByText('Alpha-Store').closest('tr')!;
    expect(alphaRow).toHaveAttribute('data-summary-series-id', 'pool:alpha');
    expect(alphaRow).toHaveAttribute('data-summary-row-active', 'false');

    fireEvent.pointerEnter(alphaRow, { pointerType: 'mouse' });

    await waitFor(() => {
      expect(alphaRow).toHaveAttribute('data-summary-row-active', 'true');
      expect(alphaRow.className).not.toContain('bg-sky-50');
      expect(alphaRow.className).not.toContain('ring-sky-400/25');
      expect(
        summary.querySelectorAll(
          '[data-highlight-series-active="true"][data-highlight-series-id="pool:alpha"]',
        ).length,
      ).toBe(3);
      expect(
        summary.querySelectorAll(
          '[data-highlight-series-active="true"][data-highlight-series-id="pool:alpha"][data-active-series-display="isolate"][data-rendered-series-count="1"]',
        ).length,
      ).toBe(3);
      expect(summary.querySelectorAll('[data-summary-card-state="inactive"]').length).toBe(1);
    });

    fireEvent.pointerLeave(alphaRow, { pointerType: 'mouse' });

    await waitFor(() => {
      expect(alphaRow).toHaveAttribute('data-summary-row-active', 'false');
      expect(summary.querySelectorAll('[data-highlight-series-active="true"]').length).toBe(0);
      expect(summary.querySelectorAll('[data-summary-card-state="inactive"]').length).toBe(0);
    });

    const poolUsageChart = screen
      .getByText('Pool Usage')
      .closest('[data-summary-card-state]')
      ?.querySelector('svg');
    expect(poolUsageChart).not.toBeNull();
    if (!poolUsageChart) {
      storageSummarySpy.mockRestore();
      return;
    }

    (
      poolUsageChart as unknown as {
        getBoundingClientRect: () => DOMRect;
      }
    ).getBoundingClientRect = () =>
      ({
        left: 0,
        top: 0,
        width: 200,
        height: 50,
        right: 200,
        bottom: 50,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as unknown as DOMRect;

    fireEvent.mouseMove(poolUsageChart, { clientX: 199, clientY: 26 });

    await waitFor(() => {
      expect(
        summary.querySelectorAll(
          '[data-highlight-series-active="true"][data-highlight-series-id="pool:alpha"]',
        ).length,
      ).toBe(3);
      expect(summary.querySelectorAll('[data-summary-card-state="inactive"]').length).toBe(1);
    });

    fireEvent.mouseLeave(poolUsageChart);

    await waitFor(() => {
      expect(summary.querySelectorAll('[data-highlight-series-active="true"]').length).toBe(0);
      expect(summary.querySelectorAll('[data-summary-card-state="inactive"]').length).toBe(0);
    });

    storageSummarySpy.mockRestore();
  });

  it('renders compact table columns and supports sort/group controls', async () => {
    hookResources = [
      buildStorageResource('storage-1', 'Alpha-Store', 'pve1', {
        current: 20,
        used: 200,
        free: 800,
        total: 1_000,
        storageType: 'dir',
        status: 'online',
      }),
      buildStorageResource('storage-2', 'Beta-Store', 'pve1', {
        current: 80,
        used: 800,
        free: 200,
        total: 1_000,
        storageType: 'zfspool',
        status: 'degraded',
        incidentSeverity: 'warning',
        incidentPriority: 60,
        incidentLabel: 'Protection Reduced',
        incidentSummary: 'Pool redundancy is reduced.',
        incidentAction: 'Replace affected member disk',
        storage: {
          platform: 'proxmox',
          topology: 'pool',
          protection: 'raidz',
          protectionReduced: true,
          protectionSummary: 'Protection Reduced',
          consumerImpactSummary: 'Affects 2 dependent resources.',
        },
      }),
    ];

    render(() => <Storage />);

    expect(screen.getByRole('columnheader', { name: 'Storage' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Source' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Protection' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Type' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Host' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Usage' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Primary Issue' })).toBeInTheDocument();
    expect(screen.getAllByText('PVE').length).toBeGreaterThan(0);
    expect(screen.getAllByText('pve1').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Protection Reduced').length).toBeGreaterThan(0);
    expect(
      screen
        .getAllByText('Protection Reduced')
        .some((element) => element.getAttribute('title') === 'Pool redundancy is reduced.'),
    ).toBe(true);

    fireEvent.change(screen.getByLabelText('Sort By'), {
      target: { value: 'usage' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Sort Direction' }));

    await waitFor(() => {
      const orderedRowIds = Array.from(document.querySelectorAll('tr[data-row-id]')).map((row) =>
        row.getAttribute('data-row-id'),
      );
      expect(orderedRowIds.slice(0, 2)).toEqual(['storage-1', 'storage-2']);
    });

    fireEvent.click(screen.getByRole('button', { name: 'By Status' }));

    // Group headers now show just the key name (without prefix)
    expect(screen.getAllByText('degraded').length).toBeGreaterThan(0);
    expect(screen.getAllByText('online').length).toBeGreaterThan(0);
  });

  it('round-trips managed storage query params through canonical URL state', async () => {
    hookResources = [
      buildStorageResource('storage-1', 'Ceph-Store', 'pve1', { storageType: 'cephfs' }),
    ];
    nodeResources = [...nodeResources, buildPhysicalDiskResource('sdb', 'node-2', 'pve2')];
    mockLocationSearch =
      '?tab=disks&q=ceph&group=status&source=proxmox-pve&status=warning&node=node-2&sort=usage&order=desc&from=proxmox-overview';

    render(() => <Storage />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [initialPath, initialOptions] = navigateSpy.mock.calls.at(-1) as [
      string,
      { replace?: boolean },
    ];
    const initialParams = new URLSearchParams(initialPath.split('?')[1] || '');
    expect(initialParams.get('tab')).toBe('disks');
    expect(initialParams.get('q')).toBe('ceph');
    expect(initialParams.get('group')).toBe('status');
    expect(initialParams.get('source')).toBe('proxmox-pve');
    expect(initialParams.get('sort')).toBe('usage');
    expect(initialParams.get('order')).toBeNull();
    expect(initialParams.get('status')).toBe('warning');
    expect(initialParams.get('node')).toBe('node-2');
    expect(initialParams.get('from')).toBe('proxmox-overview');
    expect(initialParams.get('search')).toBeNull();
    expect(initialOptions?.replace).toBe(true);
    expect(screen.getByRole('tab', { name: 'Physical Disks' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect((screen.getByLabelText('Node') as HTMLSelectElement).value).toBe('node-2');
    expect((screen.getByLabelText('Sort By') as HTMLSelectElement).value).toBe('usage');
    expect((screen.getByLabelText('Source') as HTMLSelectElement).value).toBe('proxmox-pve');
    expect((screen.getByLabelText('Status') as HTMLSelectElement).value).toBe('warning');

    // Grouping controls are only shown on the Pools view.
    fireEvent.click(screen.getByRole('tab', { name: 'Pools' }));
    expect(screen.getByRole('button', { name: 'By Status' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );

    fireEvent.click(screen.getByRole('button', { name: 'By Type' }));

    await waitFor(() => {
      const [nextPath] = navigateSpy.mock.calls.at(-1) as [string];
      const nextParams = new URLSearchParams(nextPath.split('?')[1] || '');
      expect(nextParams.get('group')).toBe('type');
    });

    fireEvent.change(screen.getByLabelText('Status'), {
      target: { value: 'available' },
    });

    await waitFor(() => {
      const [nextPath] = navigateSpy.mock.calls.at(-1) as [string];
      const nextParams = new URLSearchParams(nextPath.split('?')[1] || '');
      expect(nextParams.get('status')).toBe('available');
    });
  });

  it('restores view and filters from URL params', () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    nodeResources = [...nodeResources, buildPhysicalDiskResource('sdb', 'node-2', 'pve2')];
    mockLocationSearch = '?tab=disks&node=node-2&q=ceph&source=proxmox-pve';

    render(() => <Storage />);

    expect(screen.getByTestId('disk-list')).toHaveTextContent('disk-view:node-2:ceph');
    expect(screen.getByRole('tab', { name: 'Physical Disks' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect((screen.getByLabelText('Node') as HTMLSelectElement).value).toBe('node-2');
    expect((screen.getByLabelText('Source') as HTMLSelectElement).value).toBe('proxmox-pve');
  });

  it('trims whitespace-padded storage URL params back to canonical state', async () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    nodeResources = [...nodeResources, buildPhysicalDiskResource('sdb', 'node-2', 'pve2')];
    mockLocationSearch =
      '?tab=%20disks%20&node=%20node-2%20&q=%20ceph%20&source=%20proxmox-pve%20&group=%20status%20&sort=%20usage%20&order=%20asc%20&status=%20available%20';

    render(() => <Storage />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [nextPath, nextOptions] = navigateSpy.mock.calls.at(-1) as [
      string,
      { replace?: boolean; scroll?: boolean },
    ];
    const nextParams = new URLSearchParams(nextPath.split('?')[1] || '');
    expect(nextParams.get('tab')).toBe('disks');
    expect(nextParams.get('node')).toBe('node-2');
    expect(nextParams.get('q')).toBe('ceph');
    expect(nextParams.get('source')).toBe('proxmox-pve');
    expect(nextParams.get('group')).toBe('status');
    expect(nextParams.get('sort')).toBe('usage');
    expect(nextParams.get('order')).toBe('asc');
    expect(nextParams.get('status')).toBe('available');
    expect(nextOptions?.replace).toBe(true);
    expect(nextOptions?.scroll).toBe(false);

    expect(screen.getByRole('tab', { name: 'Physical Disks' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect((screen.getByLabelText('Node') as HTMLSelectElement).value).toBe('node-2');
    expect((screen.getByLabelText('Source') as HTMLSelectElement).value).toBe('proxmox-pve');
    expect((screen.getByLabelText('Status') as HTMLSelectElement).value).toBe('available');
    expect((screen.getByLabelText('Sort By') as HTMLSelectElement).value).toBe('usage');
  });

  it('canonicalizes source aliases in storage URL params back to owned option values', async () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    mockLocationSearch = '?source=%20PVE%20';

    render(() => <Storage />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/storage?source=proxmox-pve',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    expect((screen.getByLabelText('Source') as HTMLSelectElement).value).toBe('proxmox-pve');
  });

  it('collapses explicit all node sentinels in storage URL params back to canonical unset state', async () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    mockLocationSearch = '?node=%20ALL%20';

    render(() => <Storage />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/storage', ROUTE_STATE_REPLACE_OPTIONS);
    });

    expect((screen.getByLabelText('Node') as HTMLSelectElement).value).toBe('all');
  });

  it('pins storage group scope into the route without conflating it with expansion', async () => {
    hookResources = [
      buildStorageResource('storage-1', 'Node-Store', 'pve1'),
      buildStorageResource('storage-2', 'Edge-Store', 'pve2'),
    ];
    mockLocationSearch = '?group=node';
    navigateSpy.mockImplementation((nextPath: string) => {
      mockLocationSearch = nextPath.includes('?') ? nextPath.slice(nextPath.indexOf('?')) : '';
    });

    render(() => <Storage />);

    await waitFor(() => {
      expect(
        document.querySelector('tr[data-summary-group-id="storage:node:pve1"]'),
      ).toBeTruthy();
    });

    let groupRow = document.querySelector(
      'tr[data-summary-group-id="storage:node:pve1"]',
    ) as HTMLTableRowElement | null;
    expect(groupRow).not.toBeNull();
    if (!groupRow) {
      return;
    }

    fireEvent.click(groupRow);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/storage?group=node&summaryGroup=storage%3Anode%3Apve1',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
    });

    expect(screen.getByTestId('storage-summary-scope')).toHaveTextContent('Pinned');
    expect(screen.getByRole('button', { name: 'Reset pinned scope' })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Unpin summary scope for pve1' }),
    ).not.toBeInTheDocument();
  });

  it('uses shared preview and pinned group-member emphasis for storage pool rows', async () => {
    hookResources = [
      buildStorageResource('storage-1', 'Node-Store', 'pve1'),
      buildStorageResource('storage-2', 'Edge-Store', 'pve2'),
    ];
    mockLocationSearch = '?group=node';
    navigateSpy.mockImplementation((nextPath: string) => {
      mockLocationSearch = nextPath.includes('?') ? nextPath.slice(nextPath.indexOf('?')) : '';
    });

    render(() => <Storage />);

    await waitFor(() => {
      expect(
        document.querySelector('tr[data-summary-group-id="storage:node:pve1"]'),
      ).toBeTruthy();
    });

    const groupRow = document.querySelector(
      'tr[data-summary-group-id="storage:node:pve1"]',
    ) as HTMLTableRowElement | null;
    expect(groupRow).not.toBeNull();
    if (!groupRow) {
      return;
    }

    fireEvent.pointerEnter(groupRow, { pointerType: 'mouse' });

    await waitFor(() => {
      expect(
        document.querySelectorAll('tr[data-summary-group-member-active="preview"]'),
      ).toHaveLength(1);
    });

    fireEvent.pointerLeave(groupRow, { pointerType: 'mouse' });

    await waitFor(() => {
      expect(
        document.querySelectorAll('tr[data-summary-group-member-active="preview"]'),
      ).toHaveLength(0);
    });

    fireEvent.click(groupRow);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/storage?group=node&summaryGroup=storage%3Anode%3Apve1',
        ROUTE_STATE_REPLACE_OPTIONS,
      );
      expect(
        document.querySelectorAll('tr[data-summary-group-member-active="pinned"]'),
      ).toHaveLength(1);
    });
  });

  it('shows ceph summary card and pool expand chevron', async () => {
    hookResources = [
      buildStorageResource('storage-ceph', 'Ceph-Pool-1', 'pve1', {
        storageType: 'cephfs',
        platformId: 'cluster-main',
        current: 35,
        used: 350,
        free: 650,
        total: 1_000,
      }),
    ];

    nodeResources = [
      ...nodeResources,
      {
        id: 'ceph-1',
        type: 'ceph',
        name: 'Primary Ceph',
        displayName: 'Primary Ceph',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        disk: { current: 35, total: 1_000, used: 350, free: 650 },
        lastSeen: Date.now(),
        platformData: {
          proxmox: { instance: 'cluster-main' },
          ceph: {
            healthStatus: 'HEALTH_WARN',
            healthMessage: '1 OSD nearfull',
            numMons: 3,
            numMgrs: 2,
            numOsds: 6,
            numOsdsUp: 6,
            numOsdsIn: 6,
            numPGs: 256,
            pools: [
              {
                name: 'rbd',
                storedBytes: 350,
                availableBytes: 650,
                objects: 10_000,
                percentUsed: 35,
              },
            ],
            services: [],
          },
        },
      } as Resource,
    ];

    render(() => <Storage />);

    expect(screen.getByText('Ceph Summary')).toBeInTheDocument();
    expect(screen.getByText('Primary Ceph')).toBeInTheDocument();

    // All pools now have a toggle details button (not just Ceph)
    const toggleBtn = screen.getByRole('button', { name: 'Expand Ceph-Pool-1' });
    expect(toggleBtn).toBeInTheDocument();
    fireEvent.click(toggleBtn);
  });

  it('uses canonical storage metrics target ids for expanded pool history charts', async () => {
    const metricsHistorySpy = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue({
      resourceType: 'storage',
      resourceId: 'pool:tank',
      metric: 'usage',
      range: '7d',
      start: Date.now() - 7 * 24 * 60 * 60 * 1000,
      end: Date.now(),
      points: [],
      source: 'store',
    });

    hookResources = [
      buildStorageResource('storage-truenas-display', 'tank', 'truenas01', {
        platformId: 'truenas-1',
        platformType: 'truenas',
        storageType: 'zfs-pool',
        parentName: 'truenas01',
        includePlatformNode: false,
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:tank',
        },
        storage: {
          type: 'zfs-pool',
          platform: 'truenas',
          topology: 'pool',
          isZfs: true,
        },
      }),
    ];

    render(() => <Storage />);

    fireEvent.click(screen.getByRole('button', { name: 'Expand tank' }));

    await waitFor(() => {
      expect(metricsHistorySpy).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceType: 'storage',
          resourceId: 'pool:tank',
          metric: 'usage',
        }),
      );
    });

    metricsHistorySpy.mockRestore();
  });

  it('applies alert highlight styling for rows with active unacknowledged alerts', async () => {
    hookResources = [buildStorageResource('storage-alerted', 'Alerted-Store', 'pve1')];
    wsActiveAlerts = {
      'alert-1': buildAlert('alert-1', 'storage-alerted', 'critical'),
    };

    render(() => <Storage />);

    await waitFor(() => {
      const row = screen.getByText('Alerted-Store').closest('tr');
      expect(row).toHaveAttribute('data-alert-state', 'unacknowledged');
      expect(row).toHaveAttribute('data-alert-severity', 'critical');
      expect(row?.getAttribute('style')).toContain('#ef4444');
    });
  });

  it('marks acknowledged-only alert rows with parity state attributes', async () => {
    hookResources = [buildStorageResource('storage-acked', 'Acknowledged-Store', 'pve1')];
    wsActiveAlerts = {
      'alert-1': buildAlert('alert-1', 'storage-acked', 'warning', true),
    };

    render(() => <Storage />);

    await waitFor(() => {
      const row = screen.getByText('Acknowledged-Store').closest('tr');
      expect(row).toHaveAttribute('data-alert-state', 'acknowledged');
      expect(row).toHaveAttribute('data-alert-severity', 'none');
      expect(row?.getAttribute('style')).toContain('156, 163, 175');
    });
  });

  it('supports resource deep-links by highlighting rows', async () => {
    hookResources = [
      buildStorageResource('storage-ceph-link', 'Ceph-Link-Store', 'pve1', {
        storageType: 'cephfs',
      }),
      buildStorageResource('storage-other', 'Other-Store', 'pve2'),
    ];
    mockLocationSearch = '?resource=storage-ceph-link';

    render(() => <Storage />);

    await waitFor(() => {
      const row = document.querySelector('tr[data-row-id="storage-ceph-link"]');
      expect(row).toHaveAttribute('data-resource-highlighted', 'true');
      expect(document.querySelector('tr[data-row-id="storage-other"]')).toHaveAttribute(
        'data-resource-highlighted',
        'false',
      );
    });
  });

  it('keeps storage summary page-scoped when a focused pool is expanded', async () => {
    const storageSummarySpy = vi.spyOn(ChartsAPI, 'getStorageSummaryCharts').mockResolvedValue({
      pools: {
        'pool:alpha': {
          name: 'Alpha-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 45 },
            { timestamp: Date.now(), value: 47 },
          ],
          used: [
          ],
          avail: [
          ],
        },
        'pool:beta': {
          name: 'Beta-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 58 },
            { timestamp: Date.now(), value: 60 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 680 },
            { timestamp: Date.now(), value: 700 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 320 },
            { timestamp: Date.now(), value: 300 },
          ],
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: Date.now() - 60_000,
      },
    });

    hookResources = [
      buildStorageResource('storage-display-alpha', 'Alpha-Store', 'pve1', {
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:alpha',
        },
      }),
      buildStorageResource('storage-display-beta', 'Beta-Store', 'pve2', {
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:beta',
        },
      }),
    ];

    render(() => <Storage />);

    const summary = await screen.findByTestId('storage-summary');

    fireEvent.click(screen.getByRole('button', { name: 'Expand Alpha-Store' }));

    await waitFor(() => {
      expect(
        summary.querySelectorAll(
          '[data-highlight-series-active="true"][data-highlight-series-id="pool:alpha"]',
        ).length,
      ).toBe(1);
      expect(summary.querySelectorAll('[data-summary-card-state="inactive"]').length).toBe(3);
      expect(
        screen
          .getByText('Used Capacity')
          .closest('[data-summary-card-state]')
          ?.textContent?.includes('No history yet'),
      ).toBe(false);
      expect(
        screen
          .getByText('Available Space')
          .closest('[data-summary-card-state]')
          ?.textContent?.includes('No history yet'),
      ).toBe(false);
      expect(
        screen
          .getByText('Pool Usage')
          .closest('[data-summary-card-state]')
          ?.getAttribute('data-summary-card-state'),
      ).toBe('active');
    });

    storageSummarySpy.mockRestore();
  });

  it('shows compact synchronized readouts on sibling storage cards without duplicating the source card tooltip', async () => {
    const storageSummarySpy = vi.spyOn(ChartsAPI, 'getStorageSummaryCharts').mockResolvedValue({
      pools: {
        'pool:alpha': {
          name: 'Alpha-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 45 },
            { timestamp: Date.now(), value: 47 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 450 },
            { timestamp: Date.now(), value: 470 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 550 },
            { timestamp: Date.now(), value: 530 },
          ],
        },
        'pool:beta': {
          name: 'Beta-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 58 },
            { timestamp: Date.now(), value: 60 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 680 },
            { timestamp: Date.now(), value: 700 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 320 },
            { timestamp: Date.now(), value: 300 },
          ],
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: Date.now() - 60_000,
      },
    });

    render(() => (
      <StorageSummary
        poolCount={2}
        diskCount={0}
        timeRange="1h"
        chartHoverSync={{
          sourceKey: 'pool-usage',
          seriesId: 'pool:beta',
          timestamp: Date.now(),
        }}
      />
    ));

    await screen.findByTestId('storage-summary');

    await waitFor(() => {
      const readouts = document.querySelectorAll('[data-summary-sync-readout="true"]');
      expect(readouts).toHaveLength(2);
      for (const readout of readouts) {
        expect(readout.getAttribute('data-summary-sync-empty')).toBe('false');
        expect(readout.getAttribute('data-summary-sync-timestamp')).not.toBe('');
      }
    });

    storageSummarySpy.mockRestore();
  });

  it('offers a deliberate jump affordance when the focused storage row is off-screen', async () => {
    const storageSummarySpy = vi.spyOn(ChartsAPI, 'getStorageSummaryCharts').mockResolvedValue({
      pools: {
        'pool:alpha': {
          name: 'Alpha-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 45 },
            { timestamp: Date.now(), value: 47 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 450 },
            { timestamp: Date.now(), value: 470 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 550 },
            { timestamp: Date.now(), value: 530 },
          ],
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: Date.now() - 60_000,
      },
    });

    hookResources = [
      buildStorageResource('storage-display-alpha', 'Alpha-Store', 'pve1', {
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:alpha',
        },
      }),
    ];

    render(() => <Storage />);

    await screen.findByTestId('storage-summary');
    const alphaRow = screen.getByText('Alpha-Store').closest('tr') as HTMLTableRowElement;
    expect(alphaRow).toBeTruthy();

    alphaRow.getBoundingClientRect = vi.fn(() => ({
      top: window.innerHeight + 120,
      bottom: window.innerHeight + 168,
      left: 0,
      right: 480,
      width: 480,
      height: 48,
      x: 0,
      y: window.innerHeight + 120,
      toJSON: () => ({}),
    })) as unknown as typeof alphaRow.getBoundingClientRect;
    alphaRow.scrollIntoView = vi.fn();

    fireEvent.click(screen.getByRole('button', { name: 'Expand Alpha-Store' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Jump to row' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Jump to row' }));

    expect(alphaRow.scrollIntoView).toHaveBeenCalledWith({
      behavior: 'smooth',
      block: 'center',
    });

    storageSummarySpy.mockRestore();
  });

  it('reveals opened pool detail without hard-centering the focused row', async () => {
    const storageSummarySpy = vi.spyOn(ChartsAPI, 'getStorageSummaryCharts').mockResolvedValue({
      pools: {
        'pool:alpha': {
          name: 'Alpha-Store',
          usage: [
            { timestamp: Date.now() - 60_000, value: 45 },
            { timestamp: Date.now(), value: 47 },
          ],
          used: [
            { timestamp: Date.now() - 60_000, value: 450 },
            { timestamp: Date.now(), value: 470 },
          ],
          avail: [
            { timestamp: Date.now() - 60_000, value: 550 },
            { timestamp: Date.now(), value: 530 },
          ],
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: Date.now() - 60_000,
      },
    });

    const scrollToSpy = vi.fn();
    Object.defineProperty(window, 'innerHeight', {
      configurable: true,
      value: 800,
    });
    Object.defineProperty(window, 'scrollY', {
      configurable: true,
      value: 180,
    });
    Object.defineProperty(window, 'scrollTo', {
      configurable: true,
      value: scrollToSpy,
    });

    const rowRect = {
      top: 680,
      bottom: 728,
      left: 0,
      right: 480,
      width: 480,
      height: 48,
      x: 0,
      y: 680,
      toJSON: () => ({}),
    } satisfies DOMRect;
    const detailRect = {
      top: 732,
      bottom: 1012,
      left: 0,
      right: 480,
      width: 480,
      height: 280,
      x: 0,
      y: 732,
      toJSON: () => ({}),
    } satisfies DOMRect;
    const rectSpy = vi
      .spyOn(HTMLElement.prototype, 'getBoundingClientRect')
      .mockImplementation(function mockRect(this: HTMLElement) {
        if (this.dataset.summarySeriesId === 'pool:alpha') {
          return rowRect;
        }
        if (this.dataset.inlineDetailFor === 'pool:alpha') {
          return detailRect;
        }
        return {
          top: 0,
          bottom: 32,
          left: 0,
          right: 32,
          width: 32,
          height: 32,
          x: 0,
          y: 0,
          toJSON: () => ({}),
        } satisfies DOMRect;
      });

    hookResources = [
      buildStorageResource('storage-display-alpha', 'Alpha-Store', 'pve1', {
        metricsTarget: {
          resourceType: 'storage',
          resourceId: 'pool:alpha',
        },
      }),
    ];

    render(() => <Storage />);

    await screen.findByTestId('storage-summary');
    fireEvent.click(screen.getByRole('button', { name: 'Expand Alpha-Store' }));

    await waitFor(() => {
      expect(document.querySelector('[data-inline-detail-for="pool:alpha"]')).toBeTruthy();
      expect(scrollToSpy).toHaveBeenCalledWith({ top: 636, behavior: 'smooth' });
    });

    rectSpy.mockRestore();
    storageSummarySpy.mockRestore();
  });

  it('restores canonical TrueNAS source filters from the storage route', async () => {
    nodeResources = [
      {
        id: 'truenas-main',
        type: 'agent',
        name: 'TrueNAS Main',
        displayName: 'TrueNAS Main',
        platformId: 'truenas-main',
        platformType: 'truenas',
        sourceType: 'hybrid',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        canonicalIdentity: { hostname: 'truenas-main' },
        platformData: { sources: ['agent', 'truenas'] },
      } as Resource,
      {
        id: 'pve-main',
        type: 'agent',
        name: 'PVE Main',
        displayName: 'PVE Main',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'hybrid',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        canonicalIdentity: { hostname: 'pve-main' },
        platformData: { proxmox: { instance: 'cluster-main' }, sources: ['agent', 'proxmox-pve'] },
      } as Resource,
    ];
    hookResources = [
      buildStorageResource('storage-truenas-display', 'tank', 'truenas-main', {
        platformId: 'truenas-1',
        platformType: 'truenas',
        parentId: 'truenas-main',
        parentName: 'truenas-main',
        includePlatformNode: false,
        storageType: 'zfs-pool',
        storage: {
          platform: 'truenas',
          type: 'zfs-pool',
          topology: 'pool',
          isZfs: true,
        },
      }),
      buildStorageResource('storage-pve-main', 'local-zfs', 'pve-main', {
        parentId: 'pve-main',
        parentName: 'pve-main',
        includePlatformNode: false,
        storage: {
          platform: 'proxmox-pve',
          type: 'zfspool',
          topology: 'pool',
          isZfs: true,
        },
      }),
    ];
    mockLocationSearch = '?source=truenas&node=truenas-main';

    render(() => <Storage />);

    await waitFor(() => {
      expect(screen.getByLabelText('Source')).toHaveValue('truenas');
    });

    expect(screen.getByLabelText('Node')).toHaveValue('truenas-main');
    expect(
      Array.from(screen.getByLabelText('Source').querySelectorAll('option')).map((option) => ({
        value: option.value,
        label: option.textContent,
      })),
    ).toEqual([
      { value: 'all', label: 'All Sources' },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'truenas', label: 'TrueNAS' },
    ]);
    expect(screen.getByText('tank')).toBeInTheDocument();
    expect(screen.queryByText('local-zfs')).not.toBeInTheDocument();
  });

  it('canonicalizes whitespace-padded resource deep-links without dropping other params', async () => {
    hookResources = [
      buildStorageResource('storage-ceph-link', 'Ceph-Link-Store', 'pve1', {
        storageType: 'cephfs',
      }),
      buildStorageResource('storage-other', 'Other-Store', 'pve2'),
    ];
    mockLocationSearch = '?resource=%20storage-ceph-link%20&from=proxmox-overview';

    render(() => <Storage />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [nextPath, nextOptions] = navigateSpy.mock.calls.at(-1) as [
      string,
      { replace?: boolean },
    ];
    const nextParams = new URLSearchParams(nextPath.split('?')[1] || '');
    expect(nextParams.get('resource')).toBe('storage-ceph-link');
    expect(nextParams.get('from')).toBe('proxmox-overview');
    expect(nextOptions?.replace).toBe(true);

    await waitFor(() => {
      const row = document.querySelector('tr[data-row-id="storage-ceph-link"]');
      expect(row).toHaveAttribute('data-resource-highlighted', 'true');
    });
  });

  it('shows zfs health indicators when unified storage resources carry pool errors', async () => {
    hookResources = [
      buildStorageResource('storage-zfs', 'local-zfs', 'pve1', {
        platformId: 'cluster-main',
        storageType: 'zfspool',
        current: 60,
        used: 600,
        free: 400,
        total: 1_000,
        platformData: {
          zfsPool: {
            name: 'rpool',
            state: 'DEGRADED',
            status: 'Degraded',
            scan: 'none',
            readErrors: 1,
            writeErrors: 0,
            checksumErrors: 2,
            devices: [
              {
                name: 'sda',
                type: 'disk',
                state: 'DEGRADED',
                readErrors: 1,
                writeErrors: 0,
                checksumErrors: 2,
              },
            ],
          },
        },
      }),
    ];

    render(() => <Storage />);

    await waitFor(() => {
      expect(screen.getAllByText('DEGRADED').length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText('DEGRADED')[0]).toHaveAttribute(
      'title',
      '1 read, 2 checksum errors',
    );
  });

  it('keeps the storage content card in loading mode while the route is still fetching its first dataset', () => {
    hookLoading = true;
    wsConnected = false;
    wsInitialDataReceived = false;

    render(() => <Storage />);

    expect(screen.getByText('Loading storage resources...')).toBeInTheDocument();
    expect(
      screen.queryByText('Waiting for storage data from connected platforms.'),
    ).not.toBeInTheDocument();
  });

  it('keeps storage content visible when the websocket stream is reconnecting but REST data is healthy', () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    wsConnected = false;
    wsInitialDataReceived = false;
    wsReconnecting = true;

    render(() => <Storage />);

    expect(screen.getByText('Node-Store')).toBeInTheDocument();
    expect(screen.queryByText('Reconnecting to backend data stream…')).not.toBeInTheDocument();
  });

  it('suppresses stale-data disconnect banners when storage resources are still available', () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    wsConnected = false;
    wsInitialDataReceived = true;
    wsReconnecting = false;

    render(() => <Storage />);

    expect(screen.getByText('Node-Store')).toBeInTheDocument();
    expect(
      screen.queryByText('Storage data stream disconnected. Data may be stale.'),
    ).not.toBeInTheDocument();
  });

  it('shows warning when v2 fetch reports an error', () => {
    hookError = new Error('network');

    render(() => <Storage />);

    expect(
      screen.getByText('Unable to refresh storage resources. Showing latest available data.'),
    ).toBeInTheDocument();
  });

  it('switches to physical disks view', () => {
    render(() => <Storage />);

    fireEvent.click(screen.getByRole('tab', { name: 'Physical Disks' }));

    expect(screen.getByTestId('disk-list')).toBeInTheDocument();
  });

  it('limits physical disk node filter options to hosts with disk resources', () => {
    nodeResources = [
      {
        id: 'node-1',
        type: 'agent',
        name: 'pve1',
        displayName: 'pve1',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        platformData: { proxmox: { instance: 'cluster-main' } },
      },
      {
        id: 'agent-standalone',
        type: 'agent',
        name: 'mini',
        displayName: 'mini',
        platformId: 'agent-mini',
        platformType: 'agent',
        sourceType: 'agent',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        platformData: { agent: { agentId: 'agent-mini', hostname: 'mini' } },
      },
      buildPhysicalDiskResource('sda', null, 'pve1'),
    ];

    render(() => <Storage />);

    fireEvent.click(screen.getAllByRole('tab', { name: 'Physical Disks' })[0]!);

    const options = screen
      .getAllByRole('option')
      .map((option) => option.textContent)
      .filter(Boolean);
    expect(options).toContain('All Disk Hosts');
    expect(options).toContain('pve1');
    expect(options).not.toContain('mini');
  });

  it('resets stale disk node selections that do not map to a physical disk parent', async () => {
    mockLocationSearch = '?tab=disks&node=agent-standalone';
    nodeResources = [
      {
        id: 'node-1',
        type: 'agent',
        name: 'pve1',
        displayName: 'pve1',
        platformId: 'cluster-main',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        platformData: { proxmox: { instance: 'cluster-main' } },
      },
      {
        id: 'agent-standalone',
        type: 'agent',
        name: 'mini',
        displayName: 'mini',
        platformId: 'agent-mini',
        platformType: 'agent',
        sourceType: 'agent',
        status: 'online',
        uptime: 1000,
        lastSeen: Date.now(),
        platformData: { agent: { agentId: 'agent-mini', hostname: 'mini' } },
      },
      buildPhysicalDiskResource('sda', 'node-1', 'pve1'),
    ];

    render(() => <Storage />);

    await waitFor(() => {
      expect(screen.getByTestId('disk-list')).toHaveTextContent('disk-view:all:');
    });
    expect(screen.getAllByLabelText('Node')[0]).toHaveValue('all');
  });

  it('renders the storage view as canonical subtabs', () => {
    render(() => <Storage />);

    expect(screen.getAllByRole('tablist', { name: 'Storage view' })[0]).toBeInTheDocument();
    expect(screen.getAllByRole('tab', { name: 'Pools' })[0]).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getAllByRole('tab', { name: 'Physical Disks' })[0]).toHaveAttribute(
      'aria-selected',
      'false',
    );
  });

  it('shows loading placeholder when pool resources are loading', () => {
    hookLoading = true;

    render(() => <Storage />);

    expect(screen.getByText('Loading storage resources...')).toBeInTheDocument();
  });

  it('GA contract: Storage served at /storage is the only canonical path', async () => {
    hookResources = [
      buildStorageResource('storage-ga', 'GA-Store', 'pve1', { status: 'degraded' }),
    ];
    mockLocationPath = '/storage';
    mockLocationSearch = '?source=proxmox-pve&status=warning';

    render(() => <Storage />);

    await waitFor(() => {
      expect(screen.getByText('GA-Store')).toBeInTheDocument();
    });

    const path =
      navigateSpy.mock.calls.length > 0
        ? (navigateSpy.mock.calls.at(-1) as [string])[0]
        : `${mockLocationPath}${mockLocationSearch}`;
    expect(path.startsWith('/storage')).toBe(true);
    expect(path).not.toContain('/storage-v2');
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('source')).toBe('proxmox-pve');
    expect(params.get('status')).toBe('warning');
  });

  it('versions same-tab storage summary cache keys with the summary contract', () => {
    expect(storageSummarySource).toContain('const STORAGE_SUMMARY_IN_MEMORY_CACHE_VERSION = 1;');
    expect(storageSummarySource).toContain(
      "return `${STORAGE_SUMMARY_IN_MEMORY_CACHE_VERSION}::${normalizeOrgScope(getOrgID())}::${range}::${nodeId || '__all__'}`;",
    );
  });
});
