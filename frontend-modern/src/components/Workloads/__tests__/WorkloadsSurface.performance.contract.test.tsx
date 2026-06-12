import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, onCleanup, onMount } from 'solid-js';
import type { Resource } from '@/types/resource';
import { WorkloadsSurface } from '../WorkloadsSurface';
import workloadsSource from '../WorkloadsSurface.tsx?raw';
import workloadsFilterSource from '../WorkloadsFilter.tsx?raw';
import workloadsWorkloadTableSource from '../WorkloadsTable.tsx?raw';
import metricDisplayModeSegmentedControlSource from '../MetricDisplayModeSegmentedControl.tsx?raw';
import workloadPanelSource from '../WorkloadPanel.tsx?raw';
import workloadTableHeaderSource from '../WorkloadTableHeader.tsx?raw';
import workloadsFilterModelSource from '../workloadsFilterModel.ts?raw';
import workloadsControlsStateSource from '../useWorkloadsControlsState.ts?raw';
import workloadsGuestMetadataStateSource from '../useWorkloadGuestMetadataState.ts?raw';
import workloadSelectionModelSource from '../workloadSelectionModel.ts?raw';
import workloadsSelectionStateSource from '../useWorkloadSelectionState.ts?raw';
import workloadsWorkloadDerivedStateSource from '../useWorkloadsDerivedState.ts?raw';
import workloadsWorkloadFilterOptionsSource from '../useWorkloadFilterOptions.ts?raw';
import workloadFilterConfigModelSource from '../workloadFilterConfigModel.ts?raw';
import workloadRouteModelSource from '../workloadRouteModel.ts?raw';
import workloadRouteStateModelSource from '../workloadRouteStateModel.ts?raw';
import workloadUrlSyncModelSource from '../workloadUrlSyncModel.ts?raw';
import workloadsWorkloadViewportSyncSource from '../useWorkloadViewportSync.ts?raw';
import workloadsWorkloadRouteStateSource from '../useWorkloadRouteState.ts?raw';
import workloadsWorkloadUrlSyncSource from '../useWorkloadUrlSync.ts?raw';
import workloadsStateSource from '../useWorkloadsState.ts?raw';
import workloadTableMetricHistoryStateSource from '../useWorkloadTableMetricHistory.ts?raw';
import workloadInventorySourceIssuesSource from '../workloadInventorySourceIssues.ts?raw';
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
import guestDrawerHistorySource from '../GuestDrawerHistory.tsx?raw';
import guestDrawerOverviewSource from '../GuestDrawerOverview.tsx?raw';
import guestDrawerModelSource from '../guestDrawerModel.ts?raw';
import nodeDrawerSource from '../NodeDrawer.tsx?raw';
import nodeDrawerOverviewSource from '../NodeDrawerOverview.tsx?raw';
import nodeDrawerModelSource from '../nodeDrawerModel.ts?raw';
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
let mockInfrastructureResources: Resource[] = [];
let setMockWorkloadsSignal: ((next: Array<Record<string, unknown>>) => void) | null = null;
const workloadsRefetchMock = vi.fn();
const navigateSpy = vi.fn();
let guestRowMountCount = 0;
let guestRowUnmountCount = 0;
let wsConnected = true;
let wsInitialDataReceived = true;
let wsReconnecting = false;
let setWsConnectedSignal: ((next: boolean) => void) | null = null;
const connectionsApiMocks = vi.hoisted(() => ({
  list: vi.fn(),
}));

const pushMockWorkloads = (next: Array<Record<string, unknown>>) => {
  mockWorkloads = next;
  setMockWorkloadsSignal?.(next);
};

const pushWsConnected = (next: boolean) => {
  wsConnected = next;
  setWsConnectedSignal?.(next);
};

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/proxmox/overview', search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => {
    const [connected, setConnected] = createSignal(wsConnected);
    setWsConnectedSignal = setConnected;
    return {
      connected,
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
    };
  },
}));

vi.mock('@/hooks/useWorkloads', () => ({
  useWorkloads: () => {
    const [workloads, setWorkloads] = createSignal(mockWorkloads as any);
    setMockWorkloadsSignal = (next) => setWorkloads(next as any);
    return {
      workloads,
      refetch: (...args: unknown[]) => workloadsRefetchMock(...args),
      mutate: vi.fn(),
      loading: () => false,
      error: () => null,
    };
  },
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options?: { query?: string }) => {
    void (options?.query ?? '');
    const [resources] = createSignal(mockInfrastructureResources);
    return {
      resources,
      policyPosture: () => null,
      refetch: vi.fn(),
      mutate: vi.fn(),
      loading: () => false,
      error: () => null,
    };
  },
}));

vi.mock('@/api/connections', () => ({
  ConnectionsAPI: {
    list: connectionsApiMocks.list,
  },
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: { getAllMetadata: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => 'active',
    getMetricThresholds: () => ({ warning: 70, critical: 85 }),
    getBackupThresholds: () => ({ freshHours: 24, staleHours: 72 }),
  }),
}));

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: { focusInput: () => false },
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
  logError: vi.fn(),
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

vi.mock('../WorkloadsFilter', () => ({
  WorkloadsFilter: () => <div data-testid="workloads-filter">filter</div>,
}));

vi.mock('../GuestDrawer', () => ({
  GuestDrawer: (props: { nestedWorkloadContext?: { label: string; count: number } }) => (
    <div data-testid="guest-drawer">
      drawer
      <span data-testid="drawer-nested-context">
        {props.nestedWorkloadContext
          ? `${props.nestedWorkloadContext.label} ${props.nestedWorkloadContext.count}`
          : ''}
      </span>
    </div>
  ),
}));

vi.mock('../NodeDrawer', () => ({
  NodeDrawer: () => <div data-testid="node-drawer">node drawer</div>,
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
    GuestRow: (props: {
      guest: { id?: string; name: string };
      isExpanded?: boolean;
      nestedWorkloadContext?: { label: string; count: number };
      onClick?: () => void;
    }) => {
      onMount(() => {
        guestRowMountCount += 1;
      });
      onCleanup(() => {
        guestRowUnmountCount += 1;
      });
      return (
        <tr data-guest-id={props.guest.id} data-testid={`guest-row-${props.guest.name}`}>
          <td>
            <button
              type="button"
              data-testid={`guest-row-toggle-${props.guest.name}`}
              aria-expanded={props.isExpanded === true ? 'true' : 'false'}
              onClick={(event) => {
                event.stopPropagation();
                props.onClick?.();
              }}
            >
              Toggle {props.guest.name}
            </button>
            <span>{props.guest.name}</span>
            <span data-testid={`guest-row-nested-cue-${props.guest.name}`}>
              {props.nestedWorkloadContext
                ? `${props.nestedWorkloadContext.label} ${props.nestedWorkloadContext.count}`
                : ''}
            </span>
          </td>
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

const flushEffects = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('Workloads platform-page embed contract', () => {
  it('exposes metric-display-mode + history-range override hooks so platform pages can share the toggle across multiple tables', async () => {
    const stateSource = (await import('../useWorkloadsState.ts?raw')).default;
    expect(stateSource).toContain('metricDisplayMode?: Accessor<WorkloadsMetricDisplayMode>;');
    expect(stateSource).toContain(
      'onMetricDisplayModeChange?: (value: WorkloadsMetricDisplayMode) => void;',
    );
    expect(stateSource).toContain(
      'metricHistoryRange?: Accessor<WorkloadTableMetricHistoryRange>;',
    );
    expect(stateSource).toContain(
      'onMetricHistoryRangeChange?: (value: WorkloadTableMetricHistoryRange) => void;',
    );
    expect(stateSource).toContain('metricDisplayMode: props.metricDisplayMode,');
    expect(stateSource).toContain('onMetricDisplayModeChange: props.onMetricDisplayModeChange,');

    const controlsSource = (await import('../useWorkloadsControlsState.ts?raw')).default;
    // The controls layer must short-circuit to the page-provided accessor +
    // change handler when supplied; the internal persistent signal stays as
    // the fallback so standalone usage keeps working.
    expect(controlsSource).toContain('options.metricDisplayMode ?? internalMetricDisplayMode');
    expect(controlsSource).toContain('options.onMetricDisplayModeChange');
    expect(controlsSource).toContain('options.metricHistoryRange ?? internalMetricHistoryRange');

    const proxmoxSource = (await import('../../../features/proxmox/ProxmoxPageSurface.tsx?raw'))
      .default;
    expect(proxmoxSource).toContain('STORAGE_KEYS.WORKLOADS_METRIC_DISPLAY_MODE');
    expect(proxmoxSource).toContain('STORAGE_KEYS.WORKLOADS_METRIC_HISTORY_RANGE');
    expect(proxmoxSource).toContain('metricDisplayMode={metricDisplayMode}');
    expect(proxmoxSource).toContain(
      'setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}',
    );
    expect(proxmoxSource).toContain('onMetricDisplayModeChange: props.setMetricDisplayMode,');
    expect(proxmoxSource).toContain('metricHistoryRange={metricHistoryRange}');
    expect(proxmoxSource).toContain(
      'setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}',
    );
    expect(proxmoxSource).toContain('onMetricHistoryRangeChange: props.setMetricHistoryRange,');

    const nodesTableSource = (await import('../../../features/proxmox/ProxmoxNodesTable.tsx?raw'))
      .default;
    expect(nodesTableSource).toContain('useWorkloadTableMetricHistory');
    expect(nodesTableSource).toContain('MetricMiniSparkline');
    expect(nodesTableSource).toContain('isSparklineMode');
  });

  it('exposes compactGroupHeaders so platform pages can strip duplicate host stats from group rows', async () => {
    const stateSource = (await import('../useWorkloadsState.ts?raw')).default;
    expect(stateSource).toContain('compactGroupHeaders?: boolean;');
    expect(stateSource).toContain('compactGroupHeaders: () => props.compactGroupHeaders === true,');

    const tableSource = (await import('../WorkloadsTable.tsx?raw')).default;
    expect(tableSource).toContain(`| 'compactGroupHeaders'`);
    expect(tableSource).toContain('compactGroupHeaders={props.compactGroupHeaders}');

    const panelSource = (await import('../WorkloadPanel.tsx?raw')).default;
    expect(panelSource).toContain(`| 'compactGroupHeaders'`);
    // When the flag is set the panel falls back to NodeGroupHeader's
    // colspan layout: no per-column metric cells, no inline node facts.
    expect(panelSource).toContain(
      'props.compactGroupHeaders() ? undefined : props.workloadTableVisibleColumns()',
    );
    expect(panelSource).toContain(
      'props.compactGroupHeaders() ? undefined : renderGroupNodeColumnCell',
    );
    expect(panelSource).toContain('!props.compactGroupHeaders() &&');

    const proxmoxSource = (await import('../../../features/proxmox/ProxmoxPageSurface.tsx?raw'))
      .default;
    expect(proxmoxSource).toContain('compactGroupHeaders');
  });

  it('lets platform pages move grouped host drawers to a dedicated host table owner', async () => {
    const stateSource = (await import('../useWorkloadsState.ts?raw')).default;
    expect(stateSource).toContain("groupNodeDrawerMode?: 'inline' | 'disabled';");
    expect(stateSource).toContain(
      "groupNodeDrawerMode: () => props.groupNodeDrawerMode ?? 'inline',",
    );

    const tableSource = (await import('../WorkloadsTable.tsx?raw')).default;
    expect(tableSource).toContain(`| 'groupNodeDrawerMode'`);
    expect(tableSource).toContain('groupNodeDrawerMode={props.groupNodeDrawerMode}');

    const panelSource = (await import('../WorkloadPanel.tsx?raw')).default;
    expect(panelSource).toContain("props.groupNodeDrawerMode() === 'inline'");
    expect(panelSource).toContain(
      'onClick: canOpenNodeDrawer() ? handleGroupFocusToggle : undefined',
    );

    const proxmoxPageSource = (await import('../../../features/proxmox/ProxmoxPageSurface.tsx?raw'))
      .default;
    const proxmoxNodesSource = (await import('../../../features/proxmox/ProxmoxNodesTable.tsx?raw'))
      .default;
    expect(proxmoxPageSource).toContain('groupNodeDrawerMode="disabled"');
    expect(proxmoxNodesSource).toContain('NodeDrawer');
    expect(proxmoxNodesSource).toContain('data-inline-node-detail-for={node.id}');
  });
});

describe('Workloads performance contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockLocationSearch = '';
    mockWorkloads = [];
    mockInfrastructureResources = [];
    setMockWorkloadsSignal = null;
    setWsConnectedSignal = null;
    workloadsRefetchMock.mockReset();
    navigateSpy.mockReset();
    connectionsApiMocks.list.mockReset();
    connectionsApiMocks.list.mockResolvedValue({ connections: [], systems: [] });
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
    it('keeps the workloads route visible when websocket connectivity degrades but REST workload data is healthy', async () => {
      mockLocationSearch = '?type=all';
      wsConnected = false;
      wsInitialDataReceived = false;
      wsReconnecting = true;
      mockWorkloads = [makeGuest(1, { name: 'route-owned-workload' })];

      render(() => <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />);

      await waitFor(() => {
        expect(document.querySelector('[data-testid="workloads-filter"]')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(
          document.querySelector('[data-testid="guest-row-route-owned-workload"]'),
        ).toBeInTheDocument();
      });

      expect(document.body).not.toHaveTextContent('Connection lost');
      expect(document.body).not.toHaveTextContent('Attempting to reconnect…');
    });

    it('keeps workload rows visible while connection inventory refresh is still pending', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [makeGuest(1, { name: 'refresh-stable-workload' })];
      connectionsApiMocks.list.mockImplementationOnce(() => new Promise(() => undefined));

      render(() => <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />);

      await waitFor(() => {
        expect(
          document.querySelector('[data-testid="guest-row-refresh-stable-workload"]'),
        ).toBeInTheDocument();
      });

      expect(document.body).not.toHaveTextContent('Loading view...');
      expect(document.body).not.toHaveTextContent('Loading...');
    });

    it('opens workload row drawers inline without route navigation', async () => {
      mockLocationSearch = '?type=all&agent=docker-main';
      const routeSearchBeforeOpen = mockLocationSearch;
      mockWorkloads = [
        makeGuest(1, {
          id: 'app-container:docker-main:drawer-regression',
          instance: 'docker-main',
          name: 'drawer-regression',
          node: 'docker-main',
          type: 'app-container',
          workloadType: 'app-container',
        }),
      ];

      const { getByTestId } = render(() => (
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(getByTestId('guest-row-drawer-regression')).toBeInTheDocument();
      });

      fireEvent.click(getByTestId('guest-row-toggle-drawer-regression'));

      await waitFor(() => {
        expect(getByTestId('guest-drawer')).toBeInTheDocument();
      });
      expect(getByTestId('guest-row-toggle-drawer-regression')).toHaveAttribute(
        'aria-expanded',
        'true',
      );
      expect(navigateSpy).not.toHaveBeenCalled();
      expect(mockLocationSearch).toBe(routeSearchBeforeOpen);
    });

    it('keeps in-guest agent install prompts out of the monitoring overview', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [
        makeGuest(1, {
          name: 'vm-missing-agent',
          type: 'qemu',
          workloadType: 'vm',
          status: 'running',
          agentVersion: undefined,
        }),
        makeGuest(2, {
          name: 'lxc-missing-agent',
          type: 'lxc',
          workloadType: 'system-container',
          status: 'running',
          agentVersion: '',
        }),
        makeGuest(3, {
          name: 'app-container',
          type: 'docker-container',
          workloadType: 'app-container',
          status: 'running',
          agentVersion: undefined,
        }),
        makeGuest(4, {
          name: 'stopped-vm',
          type: 'qemu',
          workloadType: 'vm',
          status: 'stopped',
          agentVersion: undefined,
        }),
        makeGuest(5, {
          name: 'agented-vm',
          type: 'qemu',
          workloadType: 'vm',
          status: 'running',
          agentVersion: 'v6.0.0',
        }),
      ];

      render(() => <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />);

      await waitFor(() => {
        expect(screen.getByText('vm-missing-agent')).toBeInTheDocument();
      });

      expect(screen.queryByText('2 running workloads have no Pulse Agent')).toBeNull();
      expect(screen.queryByRole('link', { name: 'Install agent' })).toBeNull();
      expect(document.body).not.toHaveTextContent('Add agent for AI actions');
    });

    it('searches policy-redacted resources by their raw display name in operator-local UI', () => {
      // owning route search; redaction is a transmission-boundary policy
      // (docs/PRIVACY.md), so the haystack must use the raw infra name.
      const resources = [makeResource()];

      const filtered = filterResources(resources, new Set(), new Set(), ['secret-host']);
      const rawFiltered = filterResources(resources, new Set(), new Set(), ['Production']);

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

    it('does not treat the first websocket connection as a workload reconnect', async () => {
      mockLocationSearch = '?type=all';
      wsConnected = false;
      mockWorkloads = [makeGuest(1, { name: 'first-connect-workload' })];

      const { container } = render(() => (
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      expect(workloadsRefetchMock).not.toHaveBeenCalled();

      pushWsConnected(true);
      await flushEffects();

      expect(workloadsRefetchMock).not.toHaveBeenCalled();
    });

    it('refetches workloads only after a real websocket reconnect', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [makeGuest(1, { name: 'reconnect-workload' })];

      const { container } = render(() => (
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      workloadsRefetchMock.mockClear();

      pushWsConnected(false);
      await flushEffects();
      pushWsConnected(true);

      await waitFor(() => {
        expect(workloadsRefetchMock).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('Workload windowing contracts', () => {
    it('Profile M: caps mounted guest rows when windowing is active', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.M);

      const { container } = render(() => (
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
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
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
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
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
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
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
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
    it('applies all/vm/container/system-container/docker type filters deterministically for Profile S', async () => {
      const profileGuests = makeGuests(PROFILES.S);
      const expectedByMode = {
        all: PROFILES.S,
        vm: profileGuests.filter((guest) => guest.type === 'vm').length,
        container: profileGuests.filter((guest) => guest.type === 'lxc' || guest.type === 'docker')
          .length,
        'system-container': profileGuests.filter((guest) => guest.type === 'lxc').length,
        docker: profileGuests.filter((guest) => guest.type === 'docker').length,
      };

      for (const mode of ['all', 'vm', 'container', 'system-container', 'docker'] as const) {
        mockLocationSearch = `?type=${mode}`;
        mockWorkloads = profileGuests;

        const { container, unmount } = render(() => (
          <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
        ));

        await waitFor(() => {
          expect(getGuestRowCount(container)).toBe(expectedByMode[mode]);
        });

        unmount();
      }
    });
  });

  describe('Workload derivation contracts', () => {
    it('normalizes workloads view mode aliases through the shared workload helper', () => {
      expect(normalizeWorkloadViewModeParam('all')).toBe('all');
      expect(normalizeWorkloadViewModeParam('container')).toBe('container');
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

      const { container } = render(() => (
        <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(1);
      });
      expect(canonicalId).toBe('shared:node-x:42');
    });

    it('applies page-owned workload type exclusions before derived row state', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [
        makeGuest(1, {
          name: 'pve-lxc',
          type: 'lxc',
          workloadType: 'system-container',
          platformType: 'proxmox-pve',
          platformScopes: ['proxmox-pve'],
        }),
        makeGuest(2, {
          name: 'nested-docker-container',
          type: 'app-container',
          workloadType: 'app-container',
          platformType: 'docker',
          platformScopes: ['proxmox-pve', 'docker'],
          containerRuntime: 'docker',
          dockerHostId: 'proxmox-lxc-docker:cluster-1:node-1:101',
        }),
      ];

      const { container } = render(() => (
        <WorkloadsSurface
          vms={[]}
          containers={[]}
          nodes={[]}
          useWorkloads
          forcedPlatform="proxmox-pve"
          forcedViewMode="all"
          excludedWorkloadTypes={['app-container']}
          showNestedExcludedWorkloads
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('[data-testid="guest-row-pve-lxc"]')).toBeInTheDocument();
      });

      expect(getGuestRowCount(container)).toBe(1);
      expect(
        container.querySelector('[data-testid="guest-row-nested-docker-container"]'),
      ).toBeNull();
      expect(screen.queryByTestId('nested-workload-context-row')).toBeNull();
      expect(screen.getByTestId('guest-row-nested-cue-pve-lxc')).toHaveTextContent('Docker 1');

      fireEvent.click(screen.getByTestId('guest-row-toggle-pve-lxc'));
      expect(await screen.findByTestId('guest-drawer')).toBeInTheDocument();
      expect(screen.getByTestId('drawer-nested-context')).toHaveTextContent('Docker 1');
    });

    it('routes org scope normalization through the shared helper', () => {
      expect(workloadsSource).toContain('useWorkloadsState');
      expect(workloadsStateSource).toContain('useWorkloadGuestMetadataState');
      expect(workloadsGuestMetadataStateSource).toContain('normalizeOrgScope(getOrgID())');
      expect(workloadsStateSource).not.toContain("const DEFAULT_ORG_SCOPE = 'default'");
      expect(workloadsStateSource).not.toContain('const normalizeOrgScope =');
    });

    it('keeps hot-path workloads state in the shared workloads state owner', () => {
      expect(workloadsSource).toContain('useWorkloadsState');
      expect(workloadsSource).toContain('WorkloadsTable');
      expect(workloadsSource).not.toContain('const [search, setSearch] = createSignal(');
      expect(workloadsStateSource).toContain('useWorkloadsControlsState');
      expect(workloadsStateSource).toContain('useWorkloadGuestMetadataState');
      expect(workloadsStateSource).toContain('useWorkloadSelectionState');
      expect(workloadsStateSource).toContain('useWorkloadsDerivedState');
      expect(workloadsStateSource).toContain('useWorkloadRouteState');
      expect(workloadsStateSource).toContain('buildWorkloadInventorySourceIssues');
      expect(workloadsStateSource).toContain('createNonSuspendingQuery<ConnectionsListResponse');
      expect(workloadsStateSource).toContain('connectionsSnapshot.refetch({ background: true })');
      expect(workloadsStateSource).not.toContain('createResource<ConnectionsListResponse');
      expect(workloadInventorySourceIssuesSource).toContain('WORKLOAD_CAPABLE_TYPES');
      expect(workloadInventorySourceIssuesSource).toContain('formatConnectionErrorMessage');
      expect(workloadsStateSource).toContain('createWorkloadSortComparator');
      expect(workloadsStateSource).toContain('filterWorkloads(params)');
      expect(workloadsStateSource).not.toContain('useBreakpoint');
      expect(workloadsStateSource).not.toContain('useColumnVisibility');
      expect(workloadsStateSource).not.toContain('blurFocusedTypeToSearch');
      expect(workloadsStateSource).not.toContain("from './GuestRow'");
      expect(workloadsStateSource).not.toContain('GuestMetadataAPI.getAllMetadata()');
      expect(workloadsStateSource).not.toContain('buildWorkloadsPath({');
      expect(workloadsStateSource).not.toContain('parseWorkloadsLinkSearch');
      expect(workloadsStateSource).not.toContain('const [selectedGuestId, setSelectedGuestIdRaw]');
      expect(workloadsStateSource).not.toContain('const [hoveredWorkloadId, setHoveredWorkloadId]');
      expect(workloadsStateSource).not.toContain('groupWorkloads(');
      expect(workloadsStateSource).not.toContain('computeWorkloadStats(');
      expect(workloadsStateSource).not.toContain('computeWorkloadIOEmphasis(');
      expect(workloadsStateSource).not.toContain('buildNodeByInstance(');
      expect(workloadsStateSource).not.toContain('buildGuestParentNodeMap(');
      expect(workloadsStateSource).not.toContain('useGroupedTableWindowing');
      expect(workloadsGuestMetadataStateSource).toContain('GuestMetadataAPI.getAllMetadata()');
      expect(workloadsGuestMetadataStateSource).toContain("eventBus.on('org_switched'");
      expect(workloadsGuestMetadataStateSource).toContain(
        "window.addEventListener('pulse:metadata-changed'",
      );
      expect(workloadsWorkloadRouteStateSource).toContain('useWorkloadUrlSync');
      expect(workloadsWorkloadRouteStateSource).toContain('useWorkloadFilterOptions');
      expect(workloadsWorkloadRouteStateSource).not.toContain('buildWorkloadsPath({');
      expect(workloadsWorkloadRouteStateSource).not.toContain('normalizeWorkloadViewModeParam');
      expect(workloadsWorkloadRouteStateSource).not.toContain(
        'const [handledTypeParam, setHandledTypeParam]',
      );
      expect(workloadsWorkloadRouteStateSource).not.toContain(
        'const [workloadsRouteActive, setWorkloadsRouteActive] = createSignal(false)',
      );
      expect(workloadsWorkloadRouteStateSource).not.toContain(
        'const workloadNodeOptions = createMemo',
      );
      expect(workloadsWorkloadRouteStateSource).not.toContain(
        'const containerRuntimeFilterConfig = createMemo',
      );
      expect(workloadsWorkloadRouteStateSource).toContain("from './workloadRouteStateModel'");
      expect(workloadsWorkloadRouteStateSource).toContain(
        'resolveWorkloadsWorkloadNodeSelection({',
      );
      expect(workloadsWorkloadRouteStateSource).toContain('WORKLOADS_WORKLOAD_ROUTE_RESET_STATE');
      expect(workloadsWorkloadRouteStateSource).toContain('isWorkloadsRoute,');
      expect(workloadsWorkloadFilterOptionsSource).toContain("from './workloadFilterConfigModel'");
      expect(workloadsWorkloadFilterOptionsSource).toContain(
        'buildWorkloadNodeOptions(platformScopedGuests())',
      );
      expect(workloadsWorkloadFilterOptionsSource).not.toContain(
        'const onContextChange = (value: string) =>',
      );
      expect(workloadsWorkloadFilterOptionsSource).toContain(
        'buildWorkloadsContainerRuntimeFilterConfig({',
      );
      expect(workloadsWorkloadFilterOptionsSource).toContain('buildWorkloadsHostFilterConfig({');
      expect(workloadsWorkloadFilterOptionsSource).toContain(
        'buildWorkloadsNamespaceFilterConfig({',
      );
      expect(workloadFilterConfigModelSource).toContain(
        'export const buildWorkloadsContainerRuntimeFilterConfig',
      );
      expect(workloadFilterConfigModelSource).toContain(
        'export const buildWorkloadsHostFilterConfig',
      );
      expect(workloadFilterConfigModelSource).toContain(
        "WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL = 'K8s cluster'",
      );
      expect(workloadFilterConfigModelSource).toContain("getAllFilterOptionLabel('K8s clusters')");
      expect(workloadFilterConfigModelSource).toContain('WORKLOAD_TYPE_OPTIONS');
      expect(workloadFilterConfigModelSource).toContain(
        'export const buildWorkloadsNamespaceFilterConfig',
      );
      expect(workloadRouteModelSource).toContain('export const deserializeWorkloadViewMode');
      expect(workloadRouteModelSource).toContain("normalizeWorkloadViewModeParam(raw) ?? 'all'");
      expect(workloadRouteModelSource).not.toContain(
        'export const buildWorkloadsContainerRuntimeFilterConfig',
      );
      expect(workloadRouteModelSource).not.toContain('export const buildWorkloadsHostFilterConfig');
      expect(workloadRouteModelSource).not.toContain(
        'export const buildWorkloadsNamespaceFilterConfig',
      );
      expect(workloadRouteStateModelSource).toContain(
        'export const WORKLOADS_WORKLOAD_ROUTE_RESET_STATE',
      );
      expect(workloadRouteStateModelSource).toContain(
        'export const resolveWorkloadsWorkloadNodeSelection',
      );
      expect(workloadRouteStateModelSource).toContain(
        'export const deserializeWorkloadsContainerRuntime',
      );
      expect(workloadsWorkloadUrlSyncSource).not.toContain('buildWorkloadsPath({');
      expect(workloadsWorkloadUrlSyncSource).not.toContain('normalizeWorkloadViewModeParam');
      expect(workloadsWorkloadUrlSyncSource).not.toContain('parseWorkloadsLinkSearch');
      expect(workloadsWorkloadUrlSyncSource).toContain("from './workloadRouteModel'");
      expect(workloadsWorkloadUrlSyncSource).toContain("from './workloadUrlSyncModel'");
      expect(workloadsWorkloadUrlSyncSource).toContain(
        'const [handledTypeParam, setHandledTypeParam]',
      );
      expect(workloadsWorkloadUrlSyncSource).toContain(
        'if (normalized === handledPlatformParam()) return;',
      );
      expect(workloadsWorkloadUrlSyncSource).toContain(
        'const currentSelected = untrack(() => options.selectedPlatform());',
      );
      expect(workloadsWorkloadUrlSyncSource).toContain('parseWorkloadsWorkloadUrlParams');
      expect(workloadsWorkloadUrlSyncSource).toContain(
        'resolveWorkloadsManagedWorkloadsNavigateTarget({',
      );
      expect(workloadUrlSyncModelSource).toContain('parseWorkloadsLinkSearch(search)');
      expect(workloadUrlSyncModelSource).toContain('buildWorkloadsRouteSearch({');
      expect(workloadUrlSyncModelSource).toContain(
        'resolveWorkloadsManagedWorkloadsNavigateTarget',
      );
      expect(workloadUrlSyncModelSource).toContain('resolveWorkloadsWorkloadRuntimeParam');
      expect(workloadUrlSyncModelSource).toContain('normalizeWorkloadViewModeParam(params.type)');
      expect(workloadsControlsStateSource).toContain('useBreakpoint');
      expect(workloadsControlsStateSource).toContain('useColumnVisibility');
      expect(workloadsControlsStateSource).toContain('usePersistentSignal');
      expect(workloadsControlsStateSource).toContain('WORKLOADS_METRIC_HISTORY_RANGE');
      expect(workloadsControlsStateSource).toContain('blurFocusedTypeToSearch');
      expect(workloadsControlsStateSource).toContain('DEFAULT_WORKLOADS_SORT_KEY');
      expect(workloadsWorkloadDerivedStateSource).toContain('groupWorkloads(');
      expect(workloadsWorkloadDerivedStateSource).toContain('computeWorkloadStats(');
      expect(workloadsWorkloadDerivedStateSource).toContain('computeWorkloadIOEmphasis(');
      expect(workloadsWorkloadDerivedStateSource).toContain("from './workloadTopology'");
      expect(workloadsWorkloadDerivedStateSource).toContain('buildNodeByInstance(');
      expect(workloadsWorkloadDerivedStateSource).toContain('buildGuestParentNodeMap(');
      expect(workloadsWorkloadDerivedStateSource).toContain('useGroupedTableWindowing');
      expect(workloadsWorkloadDerivedStateSource).toContain('useWorkloadViewportSync');
      expect(workloadsWorkloadDerivedStateSource).not.toContain('window.addEventListener');
      expect(workloadsWorkloadDerivedStateSource).not.toContain('getBoundingClientRect');
      expect(workloadsWorkloadViewportSyncSource).toContain('window.addEventListener');
      expect(workloadsWorkloadViewportSyncSource).toContain('window.removeEventListener');
      expect(workloadsWorkloadViewportSyncSource).toContain('getBoundingClientRect');
      expect(workloadsWorkloadViewportSyncSource).toContain('groupedWindowing.onScroll');
      expect(workloadsWorkloadRouteStateSource).not.toContain("from './workloadTopology'");
      expect(workloadRouteModelSource).toContain("from './workloadTopology'");
      expect(workloadRouteModelSource).toContain('workloadHostScopeId');
      expect(workloadRouteModelSource).toContain('getKubernetesContextKey');
      expect(workloadsSelectionStateSource).toContain(
        'const [selectedGuestId, setSelectedGuestIdRaw]',
      );
      expect(workloadsSelectionStateSource).toContain(
        'const [hoveredWorkloadId, setHoveredWorkloadId]',
      );
      expect(workloadsSelectionStateSource).toContain('setHandledResourceId(null)');
      expect(workloadsSelectionStateSource).toContain("from './workloadSelectionModel'");
      expect(workloadsSelectionStateSource).not.toContain('useNavigate');
      expect(workloadsSelectionStateSource).not.toContain('createRouteStateNavigateScheduler');
      expect(workloadsSelectionStateSource).not.toContain(
        'resolveWorkloadsSelectionNavigateTarget',
      );
      expect(workloadsSelectionStateSource).not.toContain('parseWorkloadsLinkSearch');
      expect(workloadsSelectionStateSource).not.toContain('getCanonicalWorkloadId');
      expect(workloadSelectionModelSource).toContain('parseWorkloadsLinkSearch(search)');
      expect(workloadSelectionModelSource).toContain('getCanonicalWorkloadId');
      expect(workloadSelectionModelSource).toContain('resolveWorkloadResourceSelection');
      expect(workloadSelectionModelSource).not.toContain('resolveWorkloadsSelectionNavigateTarget');
      expect(workloadSelectionModelSource).toContain('workloadsHasHoveredWorkload');
      expect(groupedTableWindowingSource).toContain('DEFAULT_WINDOW_SIZE');
      expect(groupedTableWindowingSource).toContain('DEFAULT_ENABLE_THRESHOLD');
      expect(groupedTableWindowingSource).toContain('DEFAULT_OVERSCAN_ROWS');
      expect(groupedTableWindowingSource).toContain('getVisibleSlice');
      expect(groupedTableWindowingSource).toContain('onScroll');
      expect(groupedTableWindowingSource).toContain('revealIndex');
      expect(workloadsStateSource).not.toContain('const DEFAULT_WINDOW_SIZE =');
      expect(workloadsStateSource).not.toContain('const DEFAULT_ENABLE_THRESHOLD =');
      expect(workloadsStateSource).not.toContain('const DEFAULT_OVERSCAN_ROWS =');
      expect(workloadsSource).not.toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
      expect(workloadPanelSource).toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
      expect(workloadPanelSource).toContain('buildWorkloadSummaryGroupScope');
      expect(workloadPanelSource).toContain('data-summary-group-id');
      expect(workloadPanelSource).toContain('setHoveredWorkloadGroupScope');
      expect(workloadPanelSource).toContain('getGroupedTableRowClass');
      expect(workloadPanelSource).toContain('getInteractiveGroupedTableRowClass');
      expect(workloadPanelSource).toContain('getGroupedTableRowCellClass');
      expect(workloadPanelSource).toContain('? getInteractiveGroupedTableRowClass()');
      expect(workloadPanelSource).toContain(': getGroupedTableRowClass()');
      expect(workloadPanelSource).toContain(
        'onClick={canOpenNodeDrawer() ? handleGroupFocusToggle : undefined}',
      );
      expect(workloadPanelSource).toContain(
        '{...(canOpenNodeDrawer() ? groupRowInteraction : {})}',
      );
      expect(workloadPanelSource).not.toContain('cursor-pointer bg-surface-alt');
      expect(workloadsWorkloadTableSource).not.toContain(
        'createMemo(() => getCanonicalWorkloadId(guest()))',
      );
      expect(workloadsStateSource).not.toContain('const guestId = () => {');
    });

    it('keeps workloads filter state in canonical workloads filter owners', () => {
      // WorkloadsFilter migrated to FilterBar — the filter component now
      // composes the chip catalog directly and reads useBreakpoint locally,
      // so the legacy useWorkloadsFilterState hook is no longer in the
      // render path. Defaults/derivations stay in workloadsFilterModel.
      expect(workloadsFilterSource).toContain('FilterBar');
      expect(workloadsFilterSource).toContain('useBreakpoint');
      expect(workloadsFilterSource).toContain('hasActiveWorkloadsFilters');
      expect(workloadsFilterSource).toContain('DEFAULT_WORKLOADS_SORT_KEY');
      expect(workloadsSource).not.toContain('SummaryScopeBar');
      expect(workloadsSource).not.toContain('searchTrailing={pinnedScopeFallback()}');
      expect(workloadsSource).not.toContain('mobileTrailing={pinnedScopeFallback()}');
      expect(workloadsSource).toContain('setClearSurfaceRootRef');
      expect(workloadsSource).toContain('setTableRootRef={state.setTableRootRef}');
      expect(workloadsSource).toContain('data-testid="workloads-interaction-surface"');
      expect(workloadsSource).toContain('data-summary-clear-ignore');
      expect(workloadsSource).toContain('onClearPinnedSelection={state.clearPinnedSummaryScope}');
      expect(workloadsWorkloadTableSource).toContain('data-summary-clear-surface');
      expect(workloadsWorkloadTableSource).toContain('data-testid="workloads-table-surface"');
      expect(workloadsWorkloadTableSource).toContain('TableCard');
      expect(workloadsFilterSource).toContain('MetricDisplayModeSegmentedControl');
      expect(workloadsFilterSource).toContain('metricHistoryRange');
      expect(metricDisplayModeSegmentedControlSource).toContain(
        'WORKLOAD_TABLE_HISTORY_RANGES.map',
      );
      expect(metricDisplayModeSegmentedControlSource).toContain('aria-label="Sparkline range"');
      expect(workloadsFilterSource).toContain('pinnedSelectionActive');
      expect(workloadsFilterSource).toContain('Clear selection');
      expect(workloadsFilterSource).not.toContain('const [filtersOpen, setFiltersOpen] =');
      expect(workloadsFilterSource).not.toContain("props.setSortKey('name')");
      expect(workloadsFilterSource).toContain('searchTrailing={props.searchTrailing}');
      // useWorkloadsFilterState hook retired in the cleanup commit; the
      // canonical defaults / activity helpers now live in workloadsFilterModel.
      expect(workloadsFilterModelSource).toContain('export const countActiveWorkloadsFilters');
      expect(workloadsFilterModelSource).toContain('export const hasActiveWorkloadsFilters');
      expect(workloadsFilterModelSource).toContain(
        "DEFAULT_WORKLOADS_SORT_KEY: WorkloadsSortKey = 'type'",
      );
      expect(workloadsFilterModelSource).toContain('searchTrailing?: JSX.Element;');
      expect(workloadsFilterModelSource).toContain('utilityActions?: JSX.Element;');
      expect(workloadsFilterModelSource).toContain('mobileTrailing?: JSX.Element;');
      expect(workloadsStateSource).toContain('useWorkloadRouteState');
      expect(workloadsStateSource).toContain('range: workloadMetricHistoryRange');
      expect(workloadTableMetricHistoryStateSource).toContain(
        'fetchWorkloadsSummaryAndCache(parsed.range',
      );
      expect(workloadTableMetricHistoryStateSource).toContain(
        'fetchInfrastructureSummaryAndCache(parsed.range',
      );
      expect(workloadsStateSource).toContain('filterWorkloads(params)');
      expect(workloadsStateSource).not.toContain('const containerRuntimeFilterConfig = createMemo');
      expect(workloadsStateSource).not.toContain('useGroupedTableWindowing');
      expect(workloadsWorkloadRouteStateSource).toContain('useWorkloadUrlSync');
      expect(workloadsWorkloadRouteStateSource).toContain('useWorkloadFilterOptions');
      expect(workloadsWorkloadRouteStateSource).toContain('containerRuntimeFilterConfig');
      expect(workloadsWorkloadRouteStateSource).toContain('hostFilterConfig');
      expect(workloadsWorkloadRouteStateSource).toContain('namespaceFilterConfig');
      expect(workloadsWorkloadDerivedStateSource).toContain('useGroupedTableWindowing');
      expect(workloadsWorkloadDerivedStateSource).toContain('useWorkloadViewportSync');
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
      expect(stackedDiskBarSource).toContain('metric-fill-geometry');
      expect(stackedDiskBarSource).toContain('metric-fill-divider');
      expect(stackedDiskBarSource).toContain('AnimatedNumber');
      expect(stackedDiskBarModelSource).toContain('displayPercentValue');
      expect(stackedDiskBarSource).not.toContain('style={{');
      expect(stackedDiskBarSource).not.toContain('style={');
      expect(stackedDiskBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
      expect(stackedDiskBarSource).not.toContain('const tooltipContent = createMemo(() => {');
      expect(stackedDiskBarSource).not.toContain('const SEGMENT_COLORS =');
      expect(stackedDiskBarStateSource).toContain('new ResizeObserver');
      expect(stackedDiskBarStateSource).toContain('useTooltip');
      expect(stackedDiskBarModelSource).toContain(
        'export function buildStackedDiskBarPresentation',
      );
      expect(stackedDiskBarModelSource).toContain('const SEGMENT_COLORS');
      expect(stackedDiskBarModelSource).toContain('tooltipTitle');
    });

    it('keeps stacked memory bar runtime and derivations in canonical owners', () => {
      expect(stackedMemoryBarSource).toContain('useStackedMemoryBarState');
      expect(stackedMemoryBarSource).toContain('metric-fill-geometry');
      expect(stackedMemoryBarSource).toContain('metric-fill-divider');
      expect(stackedMemoryBarSource).toContain('AnimatedNumber');
      expect(stackedMemoryBarModelSource).toContain('displayPercentValue');
      expect(stackedMemoryBarSource).not.toContain('style={{');
      expect(stackedMemoryBarSource).not.toContain('style={');
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
      expect(metricBarSource).toContain('AnimatedNumber');
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
      expect(enhancedCpuBarSource).toContain('metric-fill-geometry');
      expect(enhancedCpuBarSource).toContain('AnimatedNumber');
      expect(enhancedCpuBarSource).not.toContain('style={{');
      expect(enhancedCpuBarSource).not.toContain('style={');
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
      expect(guestRowSource).not.toContain('style={{');
      expect(guestRowSource).toContain('style={getGuestColumnStyle(');
      expect(guestRowSource).toContain('props.workloadTableLayoutMode');
      expect(guestRowSource).toContain('props.visibleColumnIds');
      expect(guestRowSource).toContain('WebInterfaceNameLink');
      expect(guestRowSource).not.toContain('target="_blank"');
      expect(guestRowSource).not.toContain('data-workload-col="link"');
      expect(guestRowSource).not.toContain('Open related infrastructure');
      expect(guestRowSource).not.toContain('export const GUEST_COLUMNS');
      expect(guestRowSource).not.toContain('const guestId = createMemo(');
      expect(guestRowSource).not.toContain('function NetworkInfoCell(');
      expect(guestRowSource).not.toContain('function OSInfoCell(');
      expect(guestRowSource).not.toContain('function BackupStatusCell(');
      expect(guestRowSource).toContain('thresholds={cpuThresholds()}');
      expect(guestRowSource).toContain('thresholds={memoryThresholds()}');
      expect(guestRowSource).toContain('thresholds={diskThresholds()}');
      expect(guestRowStateSource).toContain('getWorkloadAlertThresholdScope');
      expect(guestRowStateSource).toContain('getMetricThresholds');
      expect(guestRowModelSource).toContain('export const GUEST_COLUMNS');
      expect(guestRowModelSource).toContain('export const VIEW_MODE_COLUMNS');
      expect(guestRowStateSource).toContain('getCanonicalWorkloadId');
      expect(guestRowStateSource).toContain('buildMetricKey');
      // The legacy /infrastructure cross-jump was retired; guest row state no
      // longer needs anything from resourceLinks.
      expect(guestRowStateSource).not.toContain("from '@/routing/resourceLinks'");
      expect(guestRowStateSource).toContain("from './workloadTopology'");
      expect(guestRowStateSource).not.toContain('./infrastructureLink');
      expect(guestRowModelSource).not.toContain("id: 'link'");
      expect(guestRowModelSource).not.toContain("'link'");
      expect(guestRowStateSource).not.toContain('rowStyle');
      expect(guestRowStateSource).not.toContain('box-shadow');
      expect(guestRowStateSource).toContain('getWorkloadTypeBadge');
      expect(guestRowCellsSource).toContain('export { BackupIndicator');
      expect(guestRowCellsSource).toContain('function NetworkInfoCell(');
      expect(guestRowCellsSource).toContain('function OSInfoCell(');
      expect(guestRowCellsSource).toContain('useTooltip');
    });

    it('keeps guest drawer runtime and derivations in canonical drawer owners', () => {
      expect(guestDrawerSource).toContain('useGuestDrawerState');
      expect(guestDrawerSource).toContain('GuestDrawerOverview');
      expect(guestDrawerSource).toContain('GuestDrawerHistoryRangeSelect');
      expect(guestDrawerSource).not.toContain('Open related infrastructure');
      expect(guestDrawerHistorySource).toContain('data-testid="guest-history-hover-time"');
      expect(guestDrawerHistorySource).toContain('data-testid="guest-history-range-control"');
      expect(guestDrawerHistorySource).toContain('onPointerMove={handleHoverMove}');
      expect(guestDrawerHistorySource).toContain('fallbackMetrics');
      expect(guestDrawerHistorySource).toContain('props.groups ?? GUEST_DRAWER_HISTORY_GROUPS');
      expect(guestDrawerHistorySource).toContain('flex min-h-[154px] flex-col');
      expect(guestDrawerHistorySource).toContain('relative min-h-24 flex-1');
      expect(guestDrawerHistorySource).not.toContain('relative h-24');
      expect(nodeDrawerSource).toContain('NodeDrawerOverview');
      expect(nodeDrawerSource).toContain('GuestDrawerHistoryRangeSelect');
      expect(nodeDrawerSource).toContain('NODE_DRAWER_HISTORY_GROUPS');
      expect(nodeDrawerOverviewSource).toContain('Hardware');
      expect(nodeDrawerOverviewSource).toContain('Telemetry');
      expect(nodeDrawerOverviewSource).toContain('Thermals');
      expect(nodeDrawerModelSource).toContain("id: 'thermals'");
      expect(nodeDrawerModelSource).toContain("metric: 'temperature'");
      expect(workloadPanelSource).toContain('NodeDrawer');
      expect(workloadPanelSource).toContain('data-inline-node-detail-for');
      expect(workloadPanelSource).toContain('const selectedGuestId = props.selectedGuestId()');
      expect(workloadPanelSource).toContain('props.setSelectedGuestId(null)');
      expect(workloadPanelSource).toContain('selectedGuestId === null');
      expect(guestDrawerSource).not.toContain('const guestId = () =>');
      expect(guestDrawerSource).not.toContain('const infrastructureHref = () =>');
      expect(guestDrawerSource).not.toContain('Filesystems');
      expect(guestDrawerSource).not.toContain('WebInterfaceUrlField');
      expect(guestDrawerStateSource).toContain('getCanonicalWorkloadId');
      expect(guestDrawerStateSource).toContain('historyRange');
      expect(guestDrawerStateSource).not.toContain('buildInfrastructureHrefForWorkload');
      expect(guestDrawerStateSource).not.toContain("from '@/routing/resourceLinks'");
      expect(guestDrawerStateSource).toContain("from '@/hooks/createNonSuspendingQuery'");
      expect(guestDrawerStateSource).toMatch(
        /createNonSuspendingQuery<\s*ResourceDiscovery \| null/,
      );
      expect(guestDrawerStateSource).not.toContain('createResource');
      expect(guestDrawerStateSource).toContain("from './workloadTopology'");
      expect(guestDrawerStateSource).not.toContain('./infrastructureLink');
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

    it('keeps workloads shell rendering in canonical section owners', () => {
      expect(workloadsSource).toContain('WorkloadsTable');
      expect(workloadsSource).not.toContain('TableHeader');
      expect(workloadsSource).not.toContain('NodeGroupHeader');
      expect(workloadsWorkloadTableSource).toContain('WorkloadTableHeader');
      expect(workloadsWorkloadTableSource).toContain('WorkloadPanel');
      expect(workloadsWorkloadTableSource).not.toContain('style={{');
      expect(workloadsWorkloadTableSource).toContain('style={getGuestColumnWidthStyle(');
      expect(workloadsWorkloadTableSource).toContain('props.workloadTableLayoutMode()');
      expect(workloadsWorkloadTableSource).toContain('props.workloadTableVisibleColumnIds()');
      expect(workloadsWorkloadTableSource).toContain('<colgroup>');
      expect(workloadsWorkloadTableSource).not.toContain('<TableHead');
      expect(workloadsWorkloadTableSource).not.toContain('NodeGroupHeader');
      expect(workloadsWorkloadTableSource).not.toContain('GuestDrawer');
      expect(workloadTableHeaderSource).toContain('TableHead');
      expect(workloadTableHeaderSource).toContain('col.sortKey as WorkloadSortKey');
      expect(workloadTableHeaderSource).toContain('style={getGuestColumnStyle(');
      expect(workloadTableHeaderSource).toContain('props.workloadTableLayoutMode()');
      expect(workloadTableHeaderSource).toContain('props.workloadTableVisibleColumnIds()');
      expect(workloadTableHeaderSource).toContain('aria-hidden="true"');
      expect(workloadTableHeaderSource).toContain('<span class="sr-only">{col.label}</span>');
      expect(workloadTableHeaderSource).not.toContain('style={{');
      expect(workloadTableHeaderSource).not.toContain('NodeGroupHeader');
      expect(workloadPanelSource).toContain('NodeGroupHeader');
      expect(workloadPanelSource).toContain('GuestDrawer');
      expect(workloadPanelSource).toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
      expect(workloadPanelSource).toContain('createSummaryInteractiveRowPreviewHandlers');
      expect(workloadPanelSource).toContain('resolveSummaryGroupMemberInteractionState');
      expect(workloadPanelSource).toContain('getInteractiveGroupedTableRowClass');
      expect(workloadPanelSource).toContain('getGroupedTableRowCellClass');
      expect(workloadPanelSource).not.toContain('style={{');
      expect(workloadPanelSource).not.toContain('style={');
      expect(workloadPanelSource).not.toContain('kind="scope"');
      expect(workloadPanelSource).not.toContain('leadingAction={');
      expect(workloadsSelectionStateSource).toContain('activeSummaryWorkloadGroupScope');
      expect(workloadsSelectionStateSource).toContain('focusedSummaryWorkloadGroupScope');
      expect(workloadsSelectionStateSource).toContain('setHoveredWorkloadGroupScope');
      expect(workloadsWorkloadTableSource).toContain('focusedSummaryWorkloadGroupScope');
      expect(workloadsWorkloadTableSource).toContain('hoveredSummaryWorkloadGroupScope');
      expect(workloadPanelSource).not.toContain('TableHead');
    });

    it('keeps disk-list runtime and derivations in canonical disk-list owners', () => {
      expect(diskListSource).toContain('useDiskListState');
      expect(diskListSource).not.toContain('const getUsagePercent =');
      expect(diskListSource).not.toContain('const getDiskStatusTooltip =');
      expect(diskListStateSource).toContain('getWorkloadGuestDiskStatusMessage');
      expect(diskListModelSource).toContain('export const buildWorkloadsDiskPresentation');
      expect(diskListModelSource).toContain('export const getWorkloadsDiskUsagePercent');
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
