import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@solidjs/testing-library';
import { createSignal, onCleanup, onMount } from 'solid-js';
import type { Resource } from '@/types/resource';
import { Dashboard } from '../Dashboard';
import dashboardSource from '../Dashboard.tsx?raw';
import dashboardStateCardsSource from '../DashboardStateCards.tsx?raw';
import dashboardStatsStripSource from '../DashboardStatsStrip.tsx?raw';
import dashboardFilterSource from '../DashboardFilter.tsx?raw';
import dashboardWorkloadTableSource from '../DashboardWorkloadTable.tsx?raw';
import workloadPanelSource from '../WorkloadPanel.tsx?raw';
import workloadTableHeaderSource from '../WorkloadTableHeader.tsx?raw';
import dashboardFilterModelSource from '../dashboardFilterModel.ts?raw';
import dashboardControlsStateSource from '../useDashboardControlsState.ts?raw';
import dashboardGuestMetadataStateSource from '../useDashboardGuestMetadataState.ts?raw';
import dashboardSelectionModelSource from '../dashboardSelectionModel.ts?raw';
import dashboardSelectionStateSource from '../useDashboardSelectionState.ts?raw';
import dashboardWorkloadDerivedStateSource from '../useDashboardWorkloadDerivedState.ts?raw';
import dashboardWorkloadFilterOptionsSource from '../useDashboardWorkloadFilterOptions.ts?raw';
import dashboardWorkloadFilterConfigModelSource from '../dashboardWorkloadFilterConfigModel.ts?raw';
import dashboardWorkloadRouteModelSource from '../dashboardWorkloadRouteModel.ts?raw';
import dashboardWorkloadRouteStateModelSource from '../dashboardWorkloadRouteStateModel.ts?raw';
import dashboardWorkloadUrlSyncModelSource from '../dashboardWorkloadUrlSyncModel.ts?raw';
import dashboardWorkloadViewportSyncSource from '../useDashboardWorkloadViewportSync.ts?raw';
import dashboardWorkloadRouteStateSource from '../useDashboardWorkloadRouteState.ts?raw';
import dashboardWorkloadUrlSyncSource from '../useDashboardWorkloadUrlSync.ts?raw';
import dashboardStateSource from '../useDashboardState.ts?raw';
import dashboardFilterStateSource from '../useDashboardFilterState.ts?raw';
import groupedTableWindowingSource from '../useGroupedTableWindowing.ts?raw';
import thresholdSliderSource from '../ThresholdSlider.tsx?raw';
import thresholdSliderModelSource from '../thresholdSliderModel.ts?raw';
import thresholdSliderStateSource from '../useThresholdSliderState.ts?raw';
import enhancedCpuBarSource from '../EnhancedCPUBar.tsx?raw';
import enhancedCpuBarModelSource from '../enhancedCpuBarModel.ts?raw';
import enhancedCpuBarStateSource from '../useEnhancedCPUBarState.ts?raw';
import stackedDiskBarSource from '../StackedDiskBar.tsx?raw';
import stackedDiskBarModelSource from '../stackedDiskBarModel.ts?raw';
import stackedDiskBarStateSource from '../useStackedDiskBarState.ts?raw';
import stackedMemoryBarSource from '../StackedMemoryBar.tsx?raw';
import stackedMemoryBarModelSource from '../stackedMemoryBarModel.ts?raw';
import stackedMemoryBarStateSource from '../useStackedMemoryBarState.ts?raw';
import diskListSource from '../DiskList.tsx?raw';
import diskListModelSource from '../diskListModel.ts?raw';
import diskListStateSource from '../useDiskListState.ts?raw';
import metricBarSource from '../MetricBar.tsx?raw';
import metricBarModelSource from '../metricBarModel.ts?raw';
import metricBarStateSource from '../useMetricBarState.ts?raw';
import guestDrawerSource from '../GuestDrawer.tsx?raw';
import guestDrawerOverviewSource from '../GuestDrawerOverview.tsx?raw';
import guestDrawerModelSource from '../guestDrawerModel.ts?raw';
import guestRowSource from '../GuestRow.tsx?raw';
import guestRowCellsSource from '../GuestRowCells.tsx?raw';
import guestDrawerStateSource from '../useGuestDrawerState.ts?raw';
import guestRowModelSource from '../guestRowModel.tsx?raw';
import guestRowStateSource from '../useGuestRowState.ts?raw';
import workloadTopologySource from '../workloadTopology.ts?raw';
import {
  filterWorkloads,
  createWorkloadSortComparator,
  groupWorkloads,
  computeWorkloadStats,
} from '../workloadSelectors';
import { getKubernetesContextKey } from '../workloadTopology';
import { filterResources } from '@/components/Infrastructure/infrastructureSelectors';
import { getCanonicalWorkloadId, normalizeWorkloadViewModeParam } from '@/utils/workloads';

// Stub ResizeObserver for jsdom
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

let mockLocationSearch = '';
let mockWorkloads: Array<Record<string, unknown>> = [];
let setMockWorkloadsSignal: ((next: Array<Record<string, unknown>>) => void) | null = null;
let guestRowMountCount = 0;
let guestRowUnmountCount = 0;
let wsConnected = true;
let wsInitialDataReceived = true;
let wsReconnecting = false;

const pushMockWorkloads = (next: Array<Record<string, unknown>>) => {
  mockWorkloads = next;
  setMockWorkloadsSignal?.(next);
};

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/workloads', search: mockLocationSearch }),
    useNavigate: () => vi.fn(),
  };
});

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    connected: () => wsConnected,
    activeAlerts: () => ({}),
    initialDataReceived: () => wsInitialDataReceived,
    reconnecting: () => wsReconnecting,
    reconnect: vi.fn(),
    state: {
      connectedInfrastructure: [],
      metrics: [],
      performance: {
        apiCallDuration: {},
        lastPollDuration: 0,
        pollingStartTime: '',
        totalApiCalls: 0,
        failedApiCalls: 0,
        cacheHits: 0,
        cacheMisses: 0,
      },
      connectionHealth: {},
      stats: {
        startTime: '',
        uptime: 0,
        pollingCycles: 0,
        webSocketClients: 0,
        version: 'dev',
      },
      activeAlerts: [],
      recentlyResolved: [],
      lastUpdate: 0,
      resources: [],
      temperatureMonitoringEnabled: false,
    },
  }),
}));

vi.mock('@/hooks/useWorkloads', () => ({
  useWorkloads: () => {
    const [workloads, setWorkloads] = createSignal(mockWorkloads as any);
    setMockWorkloadsSignal = (next) => setWorkloads(next as any);
    return {
      workloads,
      refetch: vi.fn(),
      mutate: vi.fn(),
      loading: () => false,
      error: () => null,
    };
  },
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: { getAllMetadata: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ activationState: () => 'active' }),
}));

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: { focusInput: () => false },
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/url', () => ({
  isKioskMode: () => false,
  subscribeToKioskMode: () => () => undefined,
}));

vi.mock('@/components/Workloads/WorkloadsSummary', () => ({
  WorkloadsSummary: () => <div data-testid="workloads-summary">summary</div>,
}));

vi.mock('@/components/shared/InfrastructureSelector', () => ({
  InfrastructureSelector: () => (
    <div data-testid="infrastructure-selector">infrastructure-selector</div>
  ),
}));

vi.mock('../DashboardFilter', () => ({
  DashboardFilter: () => <div data-testid="dashboard-filter">filter</div>,
}));

vi.mock('../GuestDrawer', () => ({
  GuestDrawer: () => <div data-testid="guest-drawer">drawer</div>,
}));

vi.mock('../GuestRow', () => {
  const columns = [
    { id: 'name', label: 'Name', toggleable: false, sortKey: 'name' },
    { id: 'status', label: 'Status', toggleable: false, sortKey: 'status' },
  ];
  return {
    GUEST_COLUMNS: columns,
    VIEW_MODE_COLUMNS: {
      all: new Set(['name', 'status']),
      vm: new Set(['name', 'status']),
      'system-container': new Set(['name', 'status']),
      docker: new Set(['name', 'status']),
      k8s: new Set(['name', 'status']),
    },
    GuestRow: (props: { guest: { name: string } }) => {
      onMount(() => {
        guestRowMountCount += 1;
      });
      onCleanup(() => {
        guestRowUnmountCount += 1;
      });
      return (
        <tr data-testid={`guest-row-${props.guest.name}`}>
          <td>{props.guest.name}</td>
          <td>running</td>
        </tr>
      );
    },
  };
});

const makeGuest = (i: number, overrides?: Record<string, unknown>) => ({
  id: `guest-${i}`,
  vmid: 100 + i,
  name: `workload-${i}`,
  node: `node-${i % 5}`,
  instance: `cluster-${i % 3}`,
  status: i % 7 === 0 ? 'stopped' : 'running',
  type: i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'docker' : 'vm',
  cpu: (i % 100) / 100,
  cpus: 2,
  memory: { total: 4 * 1024, used: ((i % 80) / 100) * 4 * 1024, free: 0, usage: (i % 80) / 100 },
  disk: { total: 100 * 1024, used: ((i % 60) / 100) * 100 * 1024, free: 0, usage: (i % 60) / 100 },
  networkIn: i * 100,
  networkOut: i * 50,
  diskRead: i * 10,
  diskWrite: i * 5,
  uptime: i * 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: new Date().toISOString(),
  workloadType: i % 4 === 0 ? 'system-container' : i % 3 === 0 ? 'docker' : 'vm',
  ...overrides,
});

const makeResource = (overrides?: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'vm',
  name: 'secret-host-1',
  displayName: 'secret-host-1',
  platformId: 'platform-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.now(),
  policy: {
    sensitivity: 'restricted',
    routing: { scope: 'local-only', redact: ['hostname'] },
  },
  aiSafeSummary: 'Production Host',
  ...overrides,
});

const PROFILES = {
  S: 400,
  M: 1500,
  L: 5000,
} as const;

const makeGuests = (count: number, overrides?: (i: number) => Record<string, unknown>) =>
  Array.from({ length: count }, (_, i) => makeGuest(i, overrides?.(i)));

const getTypeDistribution = (guests: Array<Record<string, unknown>>) =>
  guests.reduce<{
    vm: number;
    lxc: number;
    docker: number;
  }>(
    (acc, guest) => {
      const type = guest.type;
      if (type === 'vm') acc.vm += 1;
      if (type === 'lxc') acc.lxc += 1;
      if (type === 'docker') acc.docker += 1;
      return acc;
    },
    { vm: 0, lxc: 0, docker: 0 },
  );

const expectedTypeDistribution = (count: number) => {
  const lxc = Math.floor((count - 1) / 4) + 1;
  const multiplesOf3 = Math.floor((count - 1) / 3) + 1;
  const multiplesOf12 = Math.floor((count - 1) / 12) + 1;
  const docker = multiplesOf3 - multiplesOf12;
  const vm = count - lxc - docker;
  return { vm, lxc, docker };
};

const getGuestRowCount = (container: HTMLElement) =>
  container.querySelectorAll('tr[data-testid^="guest-row-"]').length;

describe('Dashboard performance contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockLocationSearch = '';
    mockWorkloads = [];
    setMockWorkloadsSignal = null;
    guestRowMountCount = 0;
    guestRowUnmountCount = 0;
    wsConnected = true;
    wsInitialDataReceived = true;
    wsReconnecting = false;
  });

  describe('Fixture profile validation', () => {
    for (const [profile, count] of Object.entries(PROFILES)) {
      it(`builds profile ${profile} with stable size and type distribution`, () => {
        const guests = makeGuests(count);
        const distribution = getTypeDistribution(guests);
        const expectedDistribution = expectedTypeDistribution(count);

        expect(guests).toHaveLength(count);
        expect(distribution).toEqual(expectedDistribution);
      });
    }
  });

  describe('Baseline structural contracts', () => {
    it('renders Profile S dashboard table and guest rows', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.S);

      const { container, getByTestId } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getByTestId('workloads-summary')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(PROFILES.S);
      });
    });

    it('keeps the workloads route visible when websocket connectivity degrades but REST workload data is healthy', async () => {
      mockLocationSearch = '?type=all';
      wsConnected = false;
      wsInitialDataReceived = false;
      wsReconnecting = true;
      mockWorkloads = [makeGuest(1, { name: 'route-owned-workload' })];

      render(() => <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />);

      await waitFor(() => {
        expect(document.querySelector('[data-testid="dashboard-filter"]')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(
          document.querySelector('[data-testid="guest-row-route-owned-workload"]'),
        ).toBeInTheDocument();
      });

      expect(document.body).not.toHaveTextContent('Connection lost');
      expect(document.body).not.toHaveTextContent('Attempting to reconnect…');
    });

    it('keeps governed resource search aligned with the preferred display label', () => {
      const resources = [makeResource()];

      const filtered = filterResources(resources, new Set(), new Set(), ['Production']);
      const rawFiltered = filterResources(resources, new Set(), new Set(), ['secret-host']);

      expect(filtered).toHaveLength(1);
      expect(rawFiltered).toHaveLength(0);
    });

    it('keeps projected pod context keys aligned with the canonical cluster label', () => {
      const guest = makeGuest(1, {
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'cluster-a',
        instance: 'cluster-b',
        node: 'worker-a',
      });

      expect(getKubernetesContextKey(guest as any)).toBe('cluster-a');
    });
  });

  describe('Workload windowing contracts', () => {
    it('Profile M: caps mounted guest rows when windowing is active', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.M);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        const rowCount = getGuestRowCount(container);
        expect(rowCount).toBeGreaterThan(0);
        expect(rowCount).toBeLessThanOrEqual(140);
      });
    });

    it('Profile L: keeps mounted guest rows capped under large load', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.L);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(
        () => {
          const rowCount = getGuestRowCount(container);
          expect(rowCount).toBeGreaterThan(0);
          expect(rowCount).toBeLessThanOrEqual(140);
        },
        { timeout: 20000 },
      );
    });

    it('Profile S: renders all guest rows without windowing', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.S);

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(PROFILES.S);
      });
    });

    it('unchanged poll-like updates do not remount table rows', async () => {
      mockLocationSearch = '?type=all';
      const guests = makeGuests(40);
      mockWorkloads = guests;

      const { container } = render(() => (
        <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(40);
      });

      const mountsAfterInitialRender = guestRowMountCount;
      const unmountsAfterInitialRender = guestRowUnmountCount;
      expect(mountsAfterInitialRender).toBe(40);

      pushMockWorkloads(guests.map((guest) => ({ ...guest })));

      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(40);
      });

      expect(guestRowMountCount).toBe(mountsAfterInitialRender);
      expect(guestRowUnmountCount).toBe(unmountsAfterInitialRender);
    });
  });

  describe('Filter mode contract', () => {
    it('applies all/vm/system-container/docker type filters deterministically for Profile S', async () => {
      const profileGuests = makeGuests(PROFILES.S);
      const expectedByMode = {
        all: PROFILES.S,
        vm: profileGuests.filter((guest) => guest.type === 'vm').length,
        'system-container': profileGuests.filter((guest) => guest.type === 'lxc').length,
        docker: profileGuests.filter((guest) => guest.type === 'docker').length,
      };

      for (const mode of ['all', 'vm', 'system-container', 'docker'] as const) {
        mockLocationSearch = `?type=${mode}`;
        mockWorkloads = profileGuests;

        const { container, unmount } = render(() => (
          <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
        ));

        await waitFor(() => {
          expect(getGuestRowCount(container)).toBe(expectedByMode[mode]);
        });

        unmount();
      }
    });
  });

  describe('Workload derivation contracts', () => {
    it('normalizes dashboard view mode aliases through the shared workload helper', () => {
      expect(normalizeWorkloadViewModeParam('all')).toBe('all');
      expect(normalizeWorkloadViewModeParam('docker')).toBe('app-container');
      expect(normalizeWorkloadViewModeParam('Kubernetes')).toBe('pod');
      expect(normalizeWorkloadViewModeParam('host')).toBeNull();
    });

    it('deduplicates workloads by canonical workload ID before rendering rows', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [
        makeGuest(1, { id: 'raw-a', instance: 'shared', node: 'node-x', vmid: 42 }),
        makeGuest(2, { id: 'raw-b', instance: 'shared', node: 'node-x', vmid: 42 }),
      ];
      const canonicalId = getCanonicalWorkloadId(
        makeGuest(1, { id: 'raw-a', instance: 'shared', node: 'node-x', vmid: 42 }) as any,
      );

      const { container } = render(() => <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />);

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(1);
      });
      expect(canonicalId).toBe('shared:node-x:42');
    });

    it('routes org scope normalization through the shared helper', () => {
      expect(dashboardSource).toContain('useDashboardState');
      expect(dashboardStateSource).toContain('useDashboardGuestMetadataState');
      expect(dashboardGuestMetadataStateSource).toContain('normalizeOrgScope(getOrgID())');
      expect(dashboardStateSource).not.toContain("const DEFAULT_ORG_SCOPE = 'default'");
      expect(dashboardStateSource).not.toContain('const normalizeOrgScope =');
    });

    it('keeps hot-path dashboard state in the shared dashboard state owner', () => {
      expect(dashboardSource).toContain('useDashboardState');
      expect(dashboardSource).toContain('DashboardStateCards');
      expect(dashboardSource).toContain('DashboardStatsStrip');
      expect(dashboardSource).toContain('DashboardWorkloadTable');
      expect(dashboardSource).not.toContain('const [search, setSearch] = createSignal(');
      expect(dashboardStateSource).toContain('useDashboardControlsState');
      expect(dashboardStateSource).toContain('useDashboardGuestMetadataState');
      expect(dashboardStateSource).toContain('useDashboardSelectionState');
      expect(dashboardStateSource).toContain('useDashboardWorkloadDerivedState');
      expect(dashboardStateSource).toContain('useDashboardWorkloadRouteState');
      expect(dashboardStateSource).toContain('createWorkloadSortComparator');
      expect(dashboardStateSource).toContain('filterWorkloads(params)');
      expect(dashboardStateSource).not.toContain('useBreakpoint');
      expect(dashboardStateSource).not.toContain('useColumnVisibility');
      expect(dashboardStateSource).not.toContain('blurFocusedTypeToSearch');
      expect(dashboardStateSource).not.toContain("from './GuestRow'");
      expect(dashboardStateSource).not.toContain('GuestMetadataAPI.getAllMetadata()');
      expect(dashboardStateSource).not.toContain('buildWorkloadsPath({');
      expect(dashboardStateSource).not.toContain('parseWorkloadsLinkSearch');
      expect(dashboardStateSource).not.toContain('const [selectedGuestId, setSelectedGuestIdRaw]');
      expect(dashboardStateSource).not.toContain('const [hoveredWorkloadId, setHoveredWorkloadId]');
      expect(dashboardStateSource).not.toContain('groupWorkloads(');
      expect(dashboardStateSource).not.toContain('computeWorkloadStats(');
      expect(dashboardStateSource).not.toContain('computeWorkloadIOEmphasis(');
      expect(dashboardStateSource).not.toContain('buildNodeByInstance(');
      expect(dashboardStateSource).not.toContain('buildGuestParentNodeMap(');
      expect(dashboardStateSource).not.toContain('useGroupedTableWindowing');
      expect(dashboardGuestMetadataStateSource).toContain('GuestMetadataAPI.getAllMetadata()');
      expect(dashboardGuestMetadataStateSource).toContain("eventBus.on('org_switched'");
      expect(dashboardGuestMetadataStateSource).toContain("window.addEventListener('pulse:metadata-changed'");
      expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadUrlSync');
      expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadFilterOptions');
      expect(dashboardWorkloadRouteStateSource).not.toContain('buildWorkloadsPath({');
      expect(dashboardWorkloadRouteStateSource).not.toContain('normalizeWorkloadViewModeParam');
      expect(dashboardWorkloadRouteStateSource).not.toContain('const [handledTypeParam, setHandledTypeParam]');
      expect(dashboardWorkloadRouteStateSource).not.toContain(
        "const [workloadsRouteActive, setWorkloadsRouteActive] = createSignal(false)",
      );
      expect(dashboardWorkloadRouteStateSource).not.toContain('const workloadNodeOptions = createMemo');
      expect(dashboardWorkloadRouteStateSource).not.toContain(
        'const containerRuntimeFilterConfig = createMemo',
      );
      expect(dashboardWorkloadRouteStateSource).toContain(
        "from './dashboardWorkloadRouteStateModel'",
      );
      expect(dashboardWorkloadRouteStateSource).toContain(
        'resolveDashboardWorkloadNodeSelection({',
      );
      expect(dashboardWorkloadRouteStateSource).toContain(
        'DASHBOARD_WORKLOAD_ROUTE_RESET_STATE',
      );
      expect(dashboardWorkloadRouteStateSource).toContain('isWorkloadsRoute,');
      expect(dashboardWorkloadFilterOptionsSource).toContain(
        "from './dashboardWorkloadFilterConfigModel'",
      );
      expect(dashboardWorkloadFilterOptionsSource).toContain(
        'buildDashboardWorkloadNodeOptions(options.allGuests())',
      );
      expect(dashboardWorkloadFilterOptionsSource).not.toContain(
        'const onContextChange = (value: string) =>',
      );
      expect(dashboardWorkloadFilterOptionsSource).toContain(
        'buildDashboardContainerRuntimeFilterConfig({',
      );
      expect(dashboardWorkloadFilterOptionsSource).toContain('buildDashboardHostFilterConfig({');
      expect(dashboardWorkloadFilterOptionsSource).toContain(
        'buildDashboardNamespaceFilterConfig({',
      );
      expect(dashboardWorkloadFilterConfigModelSource).toContain(
        'export const buildDashboardContainerRuntimeFilterConfig',
      );
      expect(dashboardWorkloadFilterConfigModelSource).toContain(
        'export const buildDashboardHostFilterConfig',
      );
      expect(dashboardWorkloadFilterConfigModelSource).toContain(
        'export const buildDashboardNamespaceFilterConfig',
      );
      expect(dashboardWorkloadRouteModelSource).toContain(
        'export const deserializeDashboardWorkloadViewMode',
      );
      expect(dashboardWorkloadRouteModelSource).toContain(
        "normalizeWorkloadViewModeParam(raw) ?? 'all'",
      );
      expect(dashboardWorkloadRouteModelSource).not.toContain(
        'export const buildDashboardContainerRuntimeFilterConfig',
      );
      expect(dashboardWorkloadRouteModelSource).not.toContain(
        'export const buildDashboardHostFilterConfig',
      );
      expect(dashboardWorkloadRouteModelSource).not.toContain(
        'export const buildDashboardNamespaceFilterConfig',
      );
      expect(dashboardWorkloadRouteStateModelSource).toContain(
        'export const DASHBOARD_WORKLOAD_ROUTE_RESET_STATE',
      );
      expect(dashboardWorkloadRouteStateModelSource).toContain(
        'export const resolveDashboardWorkloadNodeSelection',
      );
      expect(dashboardWorkloadRouteStateModelSource).toContain(
        'export const deserializeDashboardContainerRuntime',
      );
      expect(dashboardWorkloadUrlSyncSource).not.toContain('buildWorkloadsPath({');
      expect(dashboardWorkloadUrlSyncSource).not.toContain('normalizeWorkloadViewModeParam');
      expect(dashboardWorkloadUrlSyncSource).not.toContain('parseWorkloadsLinkSearch');
      expect(dashboardWorkloadUrlSyncSource).toContain(
        "from './dashboardWorkloadRouteModel'",
      );
      expect(dashboardWorkloadUrlSyncSource).toContain(
        "from './dashboardWorkloadUrlSyncModel'",
      );
      expect(dashboardWorkloadUrlSyncSource).toContain(
        'const [handledTypeParam, setHandledTypeParam]',
      );
      expect(dashboardWorkloadUrlSyncSource).toContain('parseDashboardWorkloadUrlParams');
      expect(dashboardWorkloadUrlSyncSource).toContain(
        'resolveDashboardManagedWorkloadsNavigateTarget({',
      );
      expect(dashboardWorkloadUrlSyncModelSource).toContain('parseWorkloadsLinkSearch(search)');
      expect(dashboardWorkloadUrlSyncModelSource).toContain('buildWorkloadsPath({');
      expect(dashboardWorkloadUrlSyncModelSource).toContain(
        'resolveDashboardManagedWorkloadsNavigateTarget',
      );
      expect(dashboardWorkloadUrlSyncModelSource).toContain(
        'resolveDashboardWorkloadRuntimeParam',
      );
      expect(dashboardWorkloadUrlSyncModelSource).toContain(
        'normalizeWorkloadViewModeParam(params.type)',
      );
      expect(dashboardControlsStateSource).toContain('useBreakpoint');
      expect(dashboardControlsStateSource).toContain('useColumnVisibility');
      expect(dashboardControlsStateSource).toContain('usePersistentSignal');
      expect(dashboardControlsStateSource).toContain('blurFocusedTypeToSearch');
      expect(dashboardControlsStateSource).toContain('DEFAULT_DASHBOARD_SORT_KEY');
      expect(dashboardWorkloadDerivedStateSource).toContain('groupWorkloads(');
      expect(dashboardWorkloadDerivedStateSource).toContain('computeWorkloadStats(');
      expect(dashboardWorkloadDerivedStateSource).toContain('computeWorkloadIOEmphasis(');
      expect(dashboardWorkloadDerivedStateSource).toContain("from './workloadTopology'");
      expect(dashboardWorkloadDerivedStateSource).toContain('buildNodeByInstance(');
      expect(dashboardWorkloadDerivedStateSource).toContain('buildGuestParentNodeMap(');
      expect(dashboardWorkloadDerivedStateSource).toContain('useGroupedTableWindowing');
      expect(dashboardWorkloadDerivedStateSource).toContain('useDashboardWorkloadViewportSync');
      expect(dashboardWorkloadDerivedStateSource).not.toContain('window.addEventListener');
      expect(dashboardWorkloadDerivedStateSource).not.toContain('getBoundingClientRect');
      expect(dashboardWorkloadViewportSyncSource).toContain('window.addEventListener');
      expect(dashboardWorkloadViewportSyncSource).toContain('window.removeEventListener');
      expect(dashboardWorkloadViewportSyncSource).toContain('getBoundingClientRect');
      expect(dashboardWorkloadViewportSyncSource).toContain('groupedWindowing.onScroll');
      expect(dashboardWorkloadRouteStateSource).not.toContain("from './workloadTopology'");
      expect(dashboardWorkloadRouteModelSource).toContain("from './workloadTopology'");
      expect(dashboardWorkloadRouteModelSource).toContain('workloadNodeScopeId');
      expect(dashboardWorkloadRouteModelSource).toContain('getKubernetesContextKey');
      expect(dashboardSelectionStateSource).toContain('const [selectedGuestId, setSelectedGuestIdRaw]');
      expect(dashboardSelectionStateSource).toContain('const [hoveredWorkloadId, setHoveredWorkloadId]');
      expect(dashboardSelectionStateSource).toContain('setHandledResourceId(null)');
      expect(dashboardSelectionStateSource).toContain("from './dashboardSelectionModel'");
      expect(dashboardSelectionStateSource).not.toContain('parseWorkloadsLinkSearch');
      expect(dashboardSelectionStateSource).not.toContain('getCanonicalWorkloadId');
      expect(dashboardSelectionModelSource).toContain('parseWorkloadsLinkSearch(search)');
      expect(dashboardSelectionModelSource).toContain('getCanonicalWorkloadId');
      expect(dashboardSelectionModelSource).toContain('resolveDashboardResourceSelection');
      expect(dashboardSelectionModelSource).toContain('dashboardHasHoveredWorkload');
      expect(groupedTableWindowingSource).toContain('DEFAULT_WINDOW_SIZE');
      expect(groupedTableWindowingSource).toContain('DEFAULT_ENABLE_THRESHOLD');
      expect(groupedTableWindowingSource).toContain('DEFAULT_OVERSCAN_ROWS');
      expect(groupedTableWindowingSource).toContain('getVisibleSlice');
      expect(groupedTableWindowingSource).toContain('onScroll');
      expect(groupedTableWindowingSource).toContain('revealIndex');
      expect(dashboardStateSource).not.toContain('const DEFAULT_WINDOW_SIZE =');
      expect(dashboardStateSource).not.toContain('const DEFAULT_ENABLE_THRESHOLD =');
      expect(dashboardStateSource).not.toContain('const DEFAULT_OVERSCAN_ROWS =');
      expect(dashboardSource).not.toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
      expect(workloadPanelSource).toContain(
        'createMemo(() => getCanonicalWorkloadId(guest()))',
      );
      expect(workloadPanelSource).toContain('buildWorkloadSummaryGroupScope');
      expect(workloadPanelSource).toContain('data-summary-group-id');
      expect(workloadPanelSource).toContain('setHoveredWorkloadGroupScope');
      expect(dashboardWorkloadTableSource).not.toContain(
        'createMemo(() => getCanonicalWorkloadId(guest()))',
      );
      expect(dashboardStateSource).not.toContain('const guestId = () => {');
    });

    it('keeps dashboard filter state in canonical dashboard filter owners', () => {
      expect(dashboardFilterSource).toContain('useDashboardFilterState');
      expect(dashboardFilterSource).not.toContain('const [filtersOpen, setFiltersOpen] =');
      expect(dashboardFilterSource).not.toContain('useBreakpoint');
      expect(dashboardFilterSource).not.toContain("props.setSortKey('name')");
      expect(dashboardFilterStateSource).toContain('countActiveDashboardFilters');
      expect(dashboardFilterStateSource).not.toContain('props.containerRuntimeFilter?.onChange');
      expect(dashboardFilterStateSource).toContain('useBreakpoint');
      expect(dashboardFilterStateSource).toContain('DEFAULT_DASHBOARD_SORT_KEY');
      expect(dashboardFilterModelSource).toContain('export const countActiveDashboardFilters');
      expect(dashboardFilterModelSource).toContain('export const hasActiveDashboardFilters');
      expect(dashboardFilterModelSource).toContain("DEFAULT_DASHBOARD_SORT_KEY: DashboardSortKey = 'type'");
      expect(dashboardStateSource).toContain('useDashboardWorkloadRouteState');
      expect(dashboardStateSource).toContain('filterWorkloads(params)');
      expect(dashboardStateSource).not.toContain('const containerRuntimeFilterConfig = createMemo');
      expect(dashboardStateSource).not.toContain('useGroupedTableWindowing');
      expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadUrlSync');
      expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadFilterOptions');
      expect(dashboardWorkloadRouteStateSource).toContain('containerRuntimeFilterConfig');
      expect(dashboardWorkloadRouteStateSource).toContain('hostFilterConfig');
      expect(dashboardWorkloadRouteStateSource).toContain('namespaceFilterConfig');
      expect(dashboardWorkloadDerivedStateSource).toContain('useGroupedTableWindowing');
      expect(dashboardWorkloadDerivedStateSource).toContain('useDashboardWorkloadViewportSync');
    });

    it('keeps threshold slider runtime and derivations in canonical slider owners', () => {
      expect(thresholdSliderSource).toContain('useThresholdSliderState');
      expect(thresholdSliderSource).not.toContain('const [thumbPosition, setThumbPosition] =');
      expect(thresholdSliderSource).not.toContain('const handleMouseDown = () => {');
      expect(thresholdSliderSource).not.toContain('formatTemperature(');
      expect(thresholdSliderStateSource).toContain('window.addEventListener');
      expect(thresholdSliderStateSource).toContain('document.addEventListener');
      expect(thresholdSliderStateSource).toContain('onCleanup');
      expect(thresholdSliderModelSource).toContain('export function getThresholdSliderPosition');
      expect(thresholdSliderModelSource).toContain('export function getThresholdSliderTitle');
      expect(thresholdSliderModelSource).toContain('export function getThresholdSliderLabel');
    });

    it('keeps stacked disk bar runtime and derivations in canonical owners', () => {
      expect(stackedDiskBarSource).toContain('useStackedDiskBarState');
      expect(stackedDiskBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
      expect(stackedDiskBarSource).not.toContain('const tooltipContent = createMemo(() => {');
      expect(stackedDiskBarSource).not.toContain('const SEGMENT_COLORS =');
      expect(stackedDiskBarStateSource).toContain('new ResizeObserver');
      expect(stackedDiskBarStateSource).toContain('useTooltip');
      expect(stackedDiskBarModelSource).toContain('export function buildStackedDiskBarPresentation');
      expect(stackedDiskBarModelSource).toContain('const SEGMENT_COLORS');
      expect(stackedDiskBarModelSource).toContain('tooltipTitle');
    });

    it('keeps stacked memory bar runtime and derivations in canonical owners', () => {
      expect(stackedMemoryBarSource).toContain('useStackedMemoryBarState');
      expect(stackedMemoryBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
      expect(stackedMemoryBarSource).not.toContain('const segments = createMemo(() => {');
      expect(stackedMemoryBarSource).not.toContain('const MEMORY_COLORS =');
      expect(stackedMemoryBarStateSource).toContain('new ResizeObserver');
      expect(stackedMemoryBarStateSource).toContain('useTooltip');
      expect(stackedMemoryBarModelSource).toContain(
        'export function buildStackedMemoryBarPresentation',
      );
      expect(stackedMemoryBarModelSource).toContain('const MEMORY_COLORS');
      expect(stackedMemoryBarModelSource).toContain('tooltipRows');
    });

    it('keeps metric bar runtime and derivations in canonical owners', () => {
      expect(metricBarSource).toContain('useMetricBarState');
      expect(metricBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
      expect(metricBarSource).not.toContain('const progressColorClass = createMemo(() => {');
      expect(metricBarSource).not.toContain('const showSublabel = createMemo(() => {');
      expect(metricBarStateSource).toContain('new ResizeObserver');
      expect(metricBarModelSource).toContain('export function buildMetricBarPresentation');
      expect(metricBarModelSource).toContain('estimateTextWidth');
      expect(metricBarModelSource).toContain('getMetricColorClass');
    });

    it('keeps enhanced CPU bar runtime and derivations in canonical owners', () => {
      expect(enhancedCpuBarSource).toContain('useEnhancedCPUBarState');
      expect(enhancedCpuBarSource).not.toContain('const tip = useTooltip()');
      expect(enhancedCpuBarSource).not.toContain('const barColor = createMemo(() =>');
      expect(enhancedCpuBarSource).not.toContain('const anomalyRatio = createMemo(() =>');
      expect(enhancedCpuBarStateSource).toContain('useTooltip');
      expect(enhancedCpuBarModelSource).toContain(
        'export function buildEnhancedCPUBarPresentation',
      );
      expect(enhancedCpuBarModelSource).toContain('getMetricColorClass');
      expect(enhancedCpuBarModelSource).toContain('tooltipUsageClass');
    });

    it('keeps guest row contract and hot-path state in canonical row owners', () => {
      expect(guestRowSource).toContain('useGuestRowState');
      expect(guestRowSource).toContain("from './GuestRowCells'");
      expect(guestRowSource).not.toContain('export const GUEST_COLUMNS');
      expect(guestRowSource).not.toContain('const guestId = createMemo(');
      expect(guestRowSource).not.toContain('function NetworkInfoCell(');
      expect(guestRowSource).not.toContain('function OSInfoCell(');
      expect(guestRowSource).not.toContain('function BackupStatusCell(');
      expect(guestRowModelSource).toContain('export const GUEST_COLUMNS');
      expect(guestRowModelSource).toContain('export const VIEW_MODE_COLUMNS');
      expect(guestRowStateSource).toContain('getCanonicalWorkloadId');
      expect(guestRowStateSource).toContain('buildMetricKey');
      expect(guestRowStateSource).toContain("from '@/routing/resourceLinks'");
      expect(guestRowStateSource).toContain("from './workloadTopology'");
      expect(guestRowStateSource).not.toContain("./infrastructureLink");
      expect(guestRowStateSource).toContain('getWorkloadTypeBadge');
      expect(guestRowCellsSource).toContain('export { BackupIndicator');
      expect(guestRowCellsSource).toContain('function NetworkInfoCell(');
      expect(guestRowCellsSource).toContain('function OSInfoCell(');
      expect(guestRowCellsSource).toContain('useTooltip');
    });

    it('keeps guest drawer runtime and derivations in canonical drawer owners', () => {
      expect(guestDrawerSource).toContain('useGuestDrawerState');
      expect(guestDrawerSource).toContain('GuestDrawerOverview');
      expect(guestDrawerSource).not.toContain('const guestId = () =>');
      expect(guestDrawerSource).not.toContain('const infrastructureHref = () =>');
      expect(guestDrawerSource).not.toContain('Filesystems');
      expect(guestDrawerSource).not.toContain('WebInterfaceUrlField');
      expect(guestDrawerStateSource).toContain('getCanonicalWorkloadId');
      expect(guestDrawerStateSource).toContain('buildInfrastructureHrefForWorkload');
      expect(guestDrawerStateSource).toContain("from '@/routing/resourceLinks'");
      expect(guestDrawerStateSource).toContain("from './workloadTopology'");
      expect(guestDrawerStateSource).not.toContain("./infrastructureLink");
      expect(guestDrawerStateSource).toContain('getDiscoveryResourceTypeForWorkload');
      expect(guestDrawerStateSource).toContain('getWebInterfaceTargetLabelForWorkload');
      expect(guestDrawerStateSource).toContain('guestOsSummary');
      expect(guestDrawerModelSource).toContain('export const getGuestDrawerBackupPresentation');
      expect(guestDrawerModelSource).toContain('export const normalizeGuestDrawerTags');
      expect(guestDrawerOverviewSource).toContain('WebInterfaceUrlField');
      expect(guestDrawerOverviewSource).toContain('DiskList');
      expect(guestDrawerOverviewSource).toContain('Filesystems');
      expect(workloadTopologySource).toContain('export const workloadNodeScopeId');
      expect(workloadTopologySource).toContain('export const getKubernetesContextKey');
      expect(workloadTopologySource).toContain('export const getWorkloadDockerHostId');
      expect(workloadTopologySource).toContain('export const buildNodeByInstance');
      expect(workloadTopologySource).toContain('export const buildGuestParentNodeMap');
    });

    it('keeps dashboard shell rendering in canonical section owners', () => {
      expect(dashboardSource).toContain('DashboardStateCards');
      expect(dashboardSource).toContain('DashboardStatsStrip');
      expect(dashboardSource).toContain('DashboardWorkloadTable');
      expect(dashboardSource).not.toContain('TableHeader');
      expect(dashboardSource).not.toContain('NodeGroupHeader');
      expect(dashboardWorkloadTableSource).toContain('WorkloadTableHeader');
      expect(dashboardWorkloadTableSource).toContain('WorkloadPanel');
      expect(dashboardWorkloadTableSource).not.toContain('<TableHead');
      expect(dashboardWorkloadTableSource).not.toContain('NodeGroupHeader');
      expect(dashboardWorkloadTableSource).not.toContain('GuestDrawer');
      expect(workloadTableHeaderSource).toContain('TableHead');
      expect(workloadTableHeaderSource).toContain("col.sortKey as WorkloadSortKey");
      expect(workloadTableHeaderSource).not.toContain('NodeGroupHeader');
      expect(workloadPanelSource).toContain('NodeGroupHeader');
      expect(workloadPanelSource).toContain('GuestDrawer');
      expect(workloadPanelSource).toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
      expect(dashboardSelectionStateSource).toContain('activeSummaryWorkloadGroupScope');
      expect(dashboardSelectionStateSource).toContain('focusedSummaryWorkloadGroupScope');
      expect(dashboardSelectionStateSource).toContain('setHoveredWorkloadGroupScope');
      expect(dashboardSource).toContain('hoveredGroupScope={state.hoveredSummaryWorkloadGroupScope()}');
      expect(dashboardSource).toContain('focusedGroupScope={state.focusedSummaryWorkloadGroupScope()}');
      expect(workloadPanelSource).not.toContain('TableHead');
      expect(dashboardStateCardsSource).toContain('buildInfrastructureWorkspacePath');
      expect(dashboardStateCardsSource).toContain('dashboardInfrastructureEmptyState().title');
      expect(dashboardStateCardsSource).toContain('dashboardDisconnectedState().actionLabel');
      expect(dashboardStateCardsSource).toContain("buildInfrastructureWorkspacePath('install')");
      expect(dashboardStatsStripSource).toContain('totalStats().running');
      expect(dashboardStatsStripSource).toContain('totalStats().stopped');
    });

    it('keeps disk-list runtime and derivations in canonical disk-list owners', () => {
      expect(diskListSource).toContain('useDiskListState');
      expect(diskListSource).not.toContain('const getUsagePercent =');
      expect(diskListSource).not.toContain('const getDiskStatusTooltip =');
      expect(diskListStateSource).toContain('getDashboardGuestDiskStatusMessage');
      expect(diskListModelSource).toContain('export const buildDashboardDiskPresentation');
      expect(diskListModelSource).toContain('export const getDashboardDiskUsagePercent');
    });

    it('filterWorkloads returns all guests when no filters active', () => {
      const guests = makeGuests(PROFILES.S);
      const result = filterWorkloads({
        guests: guests as any,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(result).toHaveLength(PROFILES.S);
    });

    it('keeps pod context selection aligned with the projected canonical cluster label', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'cluster-a',
          instance: 'cluster-b',
          node: 'worker-a',
        }),
        makeGuest(2, {
          id: 'pod-b',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'cluster-b',
          instance: 'cluster-a',
          node: 'worker-b',
        }),
      ];

      expect(getKubernetesContextKey(guests[0] as any)).toBe('cluster-a');

      const result = filterWorkloads({
        guests: guests as any,
        viewMode: 'pod',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: 'cluster-a',
      });

      expect(result.map((guest) => guest.id)).toEqual(['pod-a']);
    });

    it('filterWorkloads correctly filters by viewMode', () => {
      const guests = makeGuests(PROFILES.S);
      const vmCount = guests.filter((g) => g.type === 'vm').length;
      const result = filterWorkloads({
        guests: guests as any,
        viewMode: 'vm',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(result).toHaveLength(vmCount);
    });

    it('createWorkloadSortComparator preserves array length', () => {
      const guests = makeGuests(PROFILES.M);
      const comparator = createWorkloadSortComparator('cpu', 'asc');
      expect(comparator).not.toBeNull();
      const sorted = [...(guests as any)].sort(comparator!);
      expect(sorted).toHaveLength(PROFILES.M);
    });

    it('groupWorkloads produces correct total count in grouped mode', () => {
      const guests = makeGuests(PROFILES.S);
      const groups = groupWorkloads(guests as any, 'grouped', null);
      const total = Object.values(groups).reduce((sum, g) => sum + g.length, 0);
      expect(total).toBe(PROFILES.S);
    });

    it('computeWorkloadStats counts match total', () => {
      const guests = makeGuests(PROFILES.S);
      const stats = computeWorkloadStats(guests as any);
      expect(stats.total).toBe(PROFILES.S);
      expect(stats.vms + stats.containers + stats.appContainers + stats.pods).toBe(PROFILES.S);
      expect(stats.running + stats.degraded + stats.stopped).toBe(PROFILES.S);
    });
  });
});
