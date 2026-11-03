import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, renderHook, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, onCleanup, onMount } from 'solid-js';
import { Route, Router } from '@solidjs/router';
import type { Resource } from '@/types/resource';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { syncSessionSettingsCapabilities } from '@/stores/sessionSettingsCapabilities';
import { WorkloadsSurface, type WorkloadsSurfaceProps } from '../WorkloadsSurface';
import { useWorkloadsState, type WorkloadsStateOptions } from '../useWorkloadsState';
import { WORKLOAD_TABLE_CONTAINER_TABLET_WIDTH, getGuestColumnStyle } from '../guestRowModel';
import workloadWebLinksActionSource from '../WorkloadWebLinksAction.tsx?raw';
import workloadsStateSource from '../useWorkloadsState.ts?raw';
import guestDrawerOverviewSource from '../GuestDrawerOverview.tsx?raw';
import nodeDrawerOverviewSource from '../NodeDrawerOverview.tsx?raw';
import {
  filterWorkloads,
  createWorkloadSortComparator,
  groupWorkloads,
  computeWorkloadStats,
} from '../workloadSelectors';
import { getKubernetesContextKey, workloadNodeScopeId } from '../workloadTopology';
import { getCanonicalWorkloadId, normalizeWorkloadViewModeParam } from '@/utils/workloads';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';

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
const runtimeInventorySourcesApiMocks = vi.hoisted(() => ({
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

const metricHistoryMocks = vi.hoisted(() => ({ options: [] as Array<{ series: string }> }));

vi.mock('../useWorkloadTableMetricHistory', () => ({
  useWorkloadTableMetricHistory: (options: { series: string }) => {
    metricHistoryMocks.options.push(options);
    return {
      getGuestMetricSeries: () => [],
      getNodeMetricSeries: () => [],
    };
  },
}));

vi.mock('@/api/connections', () => ({
  ConnectionsAPI: {
    list: connectionsApiMocks.list,
  },
}));

vi.mock('@/api/runtimeInventorySources', () => ({
  RuntimeInventorySourcesAPI: {
    list: runtimeInventorySourcesApiMocks.list,
  },
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: { getAllMetadata: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 70, critical: 85 }),
    getBackupThresholds: () => ({ freshHours: 24, staleHours: 72 }),
  }),
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

// Platform pages build the one workload state and pass it to the surface.
function PageOwnedWorkloadsSurface(
  props: WorkloadsStateOptions & Omit<WorkloadsSurfaceProps, 'state'>,
) {
  const state = useWorkloadsState(props);
  return (
    <WorkloadsSurface
      state={state}
      forcedPlatform={props.forcedPlatform}
      forcedViewMode={props.forcedViewMode}
      emptyStateTitle={props.emptyStateTitle}
      emptyStateDescription={props.emptyStateDescription}
      tableTitle={props.tableTitle}
    />
  );
}

describe('Workloads performance contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    // Session capability signals live at module scope; reset so a gated case
    // cannot leak its viewer identity into the next test.
    syncSessionSettingsCapabilities(null);
    mockLocationSearch = '';
    mockWorkloads = [];
    mockInfrastructureResources = [];
    setMockWorkloadsSignal = null;
    setWsConnectedSignal = null;
    workloadsRefetchMock.mockReset();
    navigateSpy.mockReset();
    connectionsApiMocks.list.mockReset();
    connectionsApiMocks.list.mockResolvedValue({ connections: [], systems: [] });
    runtimeInventorySourcesApiMocks.list.mockReset();
    runtimeInventorySourcesApiMocks.list.mockResolvedValue({ sources: [] });
    resetCreateNonSuspendingQueryCacheForTest();
    guestRowMountCount = 0;
    guestRowUnmountCount = 0;
    wsConnected = true;
    wsInitialDataReceived = true;
    wsReconnecting = false;
  });

  it('cannot carry a disposed platform metadata read into its replacement workload state', async () => {
    const read = vi.mocked(GuestMetadataAPI.getAllMetadata);
    let finish!: (metadata: Record<string, GuestMetadata>) => void;
    read.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const first = renderHook(() => useWorkloadsState({}));
    first.result.handleCustomUrlUpdate('guest', 'https://confirmed.example');
    first.cleanup();
    read.mockReturnValueOnce(new Promise(() => {}));
    const replacement = renderHook(() => useWorkloadsState({}));
    finish({ guest: { id: 'guest', customUrl: 'https://obsolete.example' } });
    await flushEffects();
    expect(first.result.guestMetadata()).toEqual({});
    expect(replacement.result.guestMetadata()).toEqual({});
    replacement.cleanup();
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
    // The Trends toggle on the Workloads surface draws guest rows only: grouped
    // node rows render no metric cells, so it must not poll the infrastructure
    // summary the hosts table owns.
    it('reads only guest metric history so Trends does not poll the infrastructure summary', () => {
      metricHistoryMocks.options.length = 0;
      const view = renderHook(() => useWorkloadsState({}));

      const requested = metricHistoryMocks.options.map((options) => options.series);
      expect(requested).toContain('guests');
      expect(requested).not.toContain('nodes');
      view.cleanup();
    });

    // useWorkloadViewportSync measures dividers and summaryTableFocus keeps a click
    // on one from clearing the open drawer, both through this attribute. A group
    // with a Proxmox host resource renders the host NodeGroupHeader row; a group
    // without one renders the plain group row. Both must carry it.
    it('tags every group divider row with its group id', async () => {
      mockLocationSearch = '?type=all';
      mockInfrastructureResources = [
        {
          id: 'agent-node-a',
          type: 'agent',
          name: 'node-a',
          platformType: 'proxmox-pve',
          status: 'online',
          proxmox: { instance: 'node-a', node: 'node-a' },
        } as Resource,
      ];
      mockWorkloads = [
        makeGuest(1, { name: 'on-node-a', node: 'node-a', instance: 'node-a' }),
        makeGuest(2, { name: 'on-node-b', node: 'node-b', instance: 'node-b' }),
      ];

      const { container } = render(() => (
        <PageOwnedWorkloadsSurface forcedGroupingMode="grouped" />
      ));

      await waitFor(() => {
        expect(container.querySelectorAll('tr[data-summary-group-id]')).toHaveLength(2);
      });
      const scopeFor = (name: string) => workloadNodeScopeId({ instance: name, node: name });
      const dividers = Array.from(container.querySelectorAll('tr[data-summary-group-id]'));
      expect(dividers.map((row) => row.getAttribute('data-summary-group-id'))).toEqual([
        scopeFor('node-a'),
        scopeFor('node-b'),
      ]);
      // Only the host header row carries the host status dot, so this proves the
      // first divider is the NodeGroupHeader path and the second the plain row.
      expect(dividers[0].textContent).toContain('node-a');
      expect(dividers[0].querySelector('[role="img"][aria-label]')).not.toBeNull();
      expect(dividers[1].textContent).toContain('node-b');
      expect(dividers[1].querySelector('[role="img"][aria-label]')).toBeNull();
    });

    it('keeps the workloads route visible when websocket connectivity degrades but REST workload data is healthy', async () => {
      mockLocationSearch = '?type=all';
      wsConnected = false;
      wsInitialDataReceived = false;
      wsReconnecting = true;
      mockWorkloads = [makeGuest(1, { name: 'route-owned-workload' })];

      render(() => <PageOwnedWorkloadsSurface />);

      await waitFor(() => {
        expect(
          document.querySelector('[data-testid="guest-row-route-owned-workload"]'),
        ).toBeInTheDocument();
      });

      expect(document.body).not.toHaveTextContent('Connection lost');
      expect(document.body).not.toHaveTextContent('Attempting to reconnect…');
    });

    it('keeps workload rows visible while runtime source-health refresh is still pending', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [makeGuest(1, { name: 'refresh-stable-workload' })];
      runtimeInventorySourcesApiMocks.list.mockImplementationOnce(
        () => new Promise(() => undefined),
      );

      render(() => <PageOwnedWorkloadsSurface />);

      await waitFor(() => {
        expect(
          document.querySelector('[data-testid="guest-row-refresh-stable-workload"]'),
        ).toBeInTheDocument();
      });

      expect(document.body).not.toHaveTextContent('Loading view...');
      expect(document.body).not.toHaveTextContent('Loading...');
    });

    it('loads viewer-safe source health without requesting the admin connections ledger', async () => {
      mockWorkloads = [makeGuest(1, { name: 'viewer-workload' })];
      runtimeInventorySourcesApiMocks.list.mockResolvedValueOnce({
        sources: [
          {
            type: 'pve',
            name: 'viewer-source',
            state: 'stale',
            surfaces: ['vms'],
          },
        ],
      });

      render(() => <PageOwnedWorkloadsSurface />);

      await waitFor(() => expect(runtimeInventorySourcesApiMocks.list).toHaveBeenCalledTimes(1));
      expect(connectionsApiMocks.list).not.toHaveBeenCalled();
      expect(workloadsStateSource).not.toContain('/api/connections');
      expect(workloadsStateSource).not.toContain('ConnectionsAPI.list');
    });

    it('surfaces blocked Proxmox inventory sources when the platform workloads table is empty', () => {
      render(() => (
        <Router>
          <Route
            path="/"
            component={() => (
              <WorkloadsSurface
                forcedPlatform="proxmox-pve"
                emptyStateTitle="No Proxmox workloads"
                emptyStateDescription="Proxmox VMs and LXCs appear here when inventory is available."
                state={
                  {
                    setClearSurfaceRootRef: vi.fn(),
                    surfaceConnected: () => true,
                    surfaceInitialDataReceived: () => true,
                    allGuests: () => [],
                    filteredGuests: () => [],
                    search: () => '',
                    viewMode: () => 'all',
                    statusMode: () => 'all',
                    hostFilterConfig: () => undefined,
                    platformFilterConfig: () => undefined,
                    namespaceFilterConfig: () => undefined,
                    containerRuntimeFilterConfig: () => undefined,
                    workloadsGuestsEmptyState: () => ({
                      title: 'No guests found',
                      description: 'No guests match your current filters',
                    }),
                    workloadsNoInventoryState: () => ({
                      title: 'No workload inventory available',
                      description:
                        'Pulse has infrastructure sources, but no VM, container, or pod inventory is available right now. Review source credentials, permissions, and collection status in Settings → Infrastructure.',
                      actionLabel: 'Review infrastructure sources',
                    }),
                    workloadInventoryIssues: () => [
                      {
                        name: 'delly',
                        type: 'pve',
                        typeLabel: 'Proxmox VE',
                        state: 'unreachable',
                        stateLabel: 'Source unreachable',
                        coverageLabel: 'VMs and containers',
                        description:
                          'Pulse has VMs and containers enabled for delly, but the Proxmox VE API is unreachable.',
                      },
                      {
                        name: 'Tower',
                        type: 'docker',
                        typeLabel: 'Docker',
                        state: 'unreachable',
                        stateLabel: 'Source unreachable',
                        coverageLabel: 'containers',
                        description:
                          'Pulse has containers enabled for Tower, but the Docker API is unreachable.',
                      },
                    ],
                  } as any
                }
              />
            )}
          />
        </Router>
      ));

      expect(screen.getByTestId('workload-inventory-source-issues')).toBeInTheDocument();
      expect(screen.getByText('No workload inventory available')).toBeInTheDocument();
      expect(screen.queryByText('No Proxmox workloads')).not.toBeInTheDocument();
      expect(screen.getByText('Source unreachable: delly')).toBeInTheDocument();
      expect(screen.queryByText('Source unreachable: Tower')).not.toBeInTheDocument();
      expect(
        screen.getByText(
          'Pulse has VMs and containers enabled for delly, but the Proxmox VE API is unreachable.',
        ),
      ).toBeInTheDocument();
      expect(document.body).not.toHaveTextContent('request was blocked before Pulse');
      expect(screen.getByRole('link', { name: 'Review infrastructure sources' })).toHaveAttribute(
        'href',
        '/settings/infrastructure',
      );
    });

    it('withholds the infrastructure call to action from sessions without infrastructureRead', () => {
      // No workloadsNoInventoryState stub here: the surface has to resolve the
      // copy through the shared helper so the capability gate is what is under
      // test, not a hand-written empty state.
      syncSessionSettingsCapabilities({
        settingsCapabilities: { infrastructureRead: false } as never,
      });

      render(() => (
        <Router>
          <Route
            path="/"
            component={() => (
              <WorkloadsSurface
                state={
                  {
                    setClearSurfaceRootRef: vi.fn(),
                    surfaceConnected: () => true,
                    surfaceInitialDataReceived: () => true,
                    allGuests: () => [],
                    filteredGuests: () => [],
                    search: () => '',
                    viewMode: () => 'all',
                    statusMode: () => 'all',
                    hostFilterConfig: () => undefined,
                    platformFilterConfig: () => undefined,
                    namespaceFilterConfig: () => undefined,
                    containerRuntimeFilterConfig: () => undefined,
                    workloadsGuestsEmptyState: () => ({
                      title: 'No guests found',
                      description: 'No guests match your current filters',
                    }),
                    workloadInventoryIssues: () => [
                      {
                        name: 'delly',
                        type: 'pve',
                        typeLabel: 'Proxmox VE',
                        state: 'unreachable',
                        stateLabel: 'Source unreachable',
                        coverageLabel: 'VMs and containers',
                        description:
                          'Pulse has VMs and containers enabled for delly, but the Proxmox VE API is unreachable.',
                      },
                    ],
                  } as any
                }
              />
            )}
          />
        </Router>
      ));

      // The banner itself is unaffected by the gate — only the destination is.
      expect(screen.getByTestId('workload-inventory-source-issues')).toBeInTheDocument();
      expect(screen.getByText('Source unreachable: delly')).toBeInTheDocument();
      expect(screen.getByText('No workload inventory available')).toBeInTheDocument();

      expect(
        screen.queryByRole('link', { name: 'Review infrastructure sources' }),
      ).not.toBeInTheDocument();
      expect(document.querySelector('a[href="/settings/infrastructure"]')).toBeNull();
      expect(document.body).not.toHaveTextContent('Settings → Infrastructure');
      expect(document.body).toHaveTextContent(
        'Contact a Pulse administrator to check source credentials, permissions, and collection status.',
      );
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

      const { getByTestId } = render(() => <PageOwnedWorkloadsSurface />);

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

      render(() => <PageOwnedWorkloadsSurface />);

      await waitFor(() => {
        expect(screen.getByText('vm-missing-agent')).toBeInTheDocument();
      });

      expect(screen.queryByText('2 running workloads have no Pulse Agent')).toBeNull();
      expect(screen.queryByRole('link', { name: 'Install agent' })).toBeNull();
      expect(document.body).not.toHaveTextContent('Add agent for AI actions');
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

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

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

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

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

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

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

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

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

    it('Profile S: keeps a medium estate virtualized', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = makeGuests(PROFILES.S);

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

      await waitFor(() => {
        const rowCount = getGuestRowCount(container);
        expect(rowCount).toBeGreaterThan(0);
        expect(rowCount).toBeLessThanOrEqual(140);
      });
    });

    it('unchanged poll-like updates do not remount table rows', async () => {
      mockLocationSearch = '?type=all';
      const guests = makeGuests(40);
      mockWorkloads = guests;

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

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

        const { container, unmount } = render(() => <PageOwnedWorkloadsSurface />);

        await waitFor(() => {
          expect(getGuestRowCount(container)).toBe(
            expectedByMode[mode] > 140 ? 140 : expectedByMode[mode],
          );
        });

        unmount();
      }
    });
  });

  describe('Workload derivation contracts', () => {
    it.each([false, true])(
      'keeps Proxmox nodes whose instance and node names join to one string apart (reversed=%s)',
      (reversed) => {
        // Two installs: instance "lab-east" + node "pve1" and instance "lab" + node
        // "east-pve1" both join to "lab-east-pve1". Each guest must resolve to the
        // node of its own install, whichever order the snapshot lists them in.
        const makeNode = (
          id: string,
          instance: string,
          node: string,
          capacity: number,
          status: string,
        ) =>
          ({
            id,
            type: 'agent',
            name: node,
            platformType: 'proxmox-pve',
            status,
            proxmox: { instance, node },
            memory: { current: 1, total: capacity, used: 1 },
          }) as Resource;
        const nodeResources = [
          makeNode('agent-east-1', 'lab-east', 'pve1', 8, 'online'),
          makeNode('agent-east-2', 'lab', 'east-pve1', 64, 'offline'),
        ];
        mockInfrastructureResources = reversed ? [...nodeResources].reverse() : nodeResources;
        mockWorkloads = [
          makeGuest(1, {
            id: 'raw-a',
            instance: 'lab-east',
            node: 'pve1',
            type: 'vm',
            workloadType: 'vm',
          }),
          makeGuest(2, {
            id: 'raw-b',
            instance: 'lab',
            node: 'east-pve1',
            type: 'vm',
            workloadType: 'vm',
          }),
        ];
        function SnapshotNodes() {
          const state = useWorkloadsState({});
          return (
            <pre data-testid="snapshot-node-parents">
              {JSON.stringify({
                count: state.infrastructureNodes().length,
                parents: state.allGuests().map((guest) => {
                  const parent = state.guestParentNodeMap()[getCanonicalWorkloadId(guest)];
                  return {
                    guest: guest.instance,
                    instance: parent?.instance,
                    node: parent?.name,
                    capacity: parent?.memory.total,
                    status: parent?.status,
                  };
                }),
              })}
            </pre>
          );
        }
        render(() => <SnapshotNodes />);
        expect(JSON.parse(screen.getByTestId('snapshot-node-parents').textContent!)).toEqual({
          count: 2,
          parents: [
            {
              guest: 'lab-east',
              instance: 'lab-east',
              node: 'pve1',
              capacity: 8,
              status: 'online',
            },
            { guest: 'lab', instance: 'lab', node: 'east-pve1', capacity: 64, status: 'offline' },
          ],
        });
      },
    );

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
        makeGuest(2, {
          id: 'raw-b',
          canonicalResourceId: 'raw-a',
          instance: 'shared',
          node: 'node-x',
          vmid: 42,
        }),
      ];
      const canonicalId = getCanonicalWorkloadId(
        makeGuest(1, { id: 'raw-a', instance: 'shared', node: 'node-x', vmid: 42 }) as any,
      );

      const { container } = render(() => <PageOwnedWorkloadsSurface />);

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getGuestRowCount(container)).toBe(1);
      });
      expect(canonicalId).toBe('raw-a');
    });

    it('keeps distinct canonical resources even when their native coordinates coincide', async () => {
      mockLocationSearch = '?type=all';
      mockWorkloads = [
        makeGuest(1, { id: 'vm-distinct-a', instance: 'shared', node: 'node-x', vmid: 42 }),
        makeGuest(2, { id: 'vm-distinct-b', instance: 'shared', node: 'node-x', vmid: 42 }),
      ];
      const { container } = render(() => <PageOwnedWorkloadsSurface />);
      await waitFor(() => expect(getGuestRowCount(container)).toBe(2));
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
        <PageOwnedWorkloadsSurface
          forcedPlatform="proxmox-pve"
          forcedViewMode="container"
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
      expect(screen.getByRole('columnheader', { name: 'ID' })).toBeInTheDocument();
      expect(screen.getByRole('columnheader', { name: 'Backup' })).toBeInTheDocument();
      expect(screen.queryByRole('columnheader', { name: 'Runtime' })).toBeNull();
      expect(screen.queryByRole('columnheader', { name: 'Image' })).toBeNull();
      expect(screen.queryByRole('columnheader', { name: 'Context' })).toBeNull();
      expect(screen.queryByRole('columnheader', { name: 'Update' })).toBeNull();

      fireEvent.click(screen.getByTestId('guest-row-toggle-pve-lxc'));
      expect(await screen.findByTestId('guest-drawer')).toBeInTheDocument();
      expect(screen.getByTestId('drawer-nested-context')).toHaveTextContent('Docker 1');
    });

    // Bundle shape, not behaviour: nothing in jsdom notices a lazy chunk turning
    // eager, so these stay as source guards on the entry files.
    it('keeps the web-links dialog, physical disks and drawer history charts in lazy chunks', () => {
      expect(workloadWebLinksActionSource).toContain(
        "lazy(() => import('./WorkloadWebLinksDialog'))",
      );
      expect(workloadWebLinksActionSource).toContain('<Show when={open()}>');
      expect(workloadWebLinksActionSource).not.toMatch(
        /^import (?!type )[^;]*from '\.\/(workloadWebLinksModel|useWorkloadWebLinksState)';/m,
      );
      expect(guestDrawerOverviewSource).toContain("import('./GuestPhysicalDisks')");
      expect(guestDrawerOverviewSource).not.toMatch(
        /^import (?!type )[^;]*from '\.\/GuestPhysicalDisks';/m,
      );
      expect(guestDrawerOverviewSource).toContain("import('./GuestDrawerHistory')");
      expect(guestDrawerOverviewSource).not.toMatch(
        /^import (?!type )[^;]*from '\.\/GuestDrawerHistory';/m,
      );
      expect(nodeDrawerOverviewSource).toContain("import('./GuestDrawerHistory')");
      expect(nodeDrawerOverviewSource).not.toMatch(
        /^import (?!type )[^;]*from '\.\/GuestDrawerHistory';/m,
      );
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

describe('Workloads Backup column fit', () => {
  // The backup badge does not truncate: Running is 73px and Unknown 79px with
  // the shield, plus 8px of cell padding a side. The compact layout starts at a
  // 900px table, where the Proxmox guest columns all show.
  const badgeWidth = 79 + 16;

  it('keeps the widest backup badge whole at the narrowest compact table', () => {
    const proxmoxColumns = ['name', 'type', 'vmid', 'cpu', 'memory', 'disk', 'uptime', 'backup'];
    const percent = Number.parseFloat(
      String(getGuestColumnStyle('backup', false, 'compact', proxmoxColumns)?.width),
    );
    expect((percent / 100) * WORKLOAD_TABLE_CONTAINER_TABLET_WIDTH).toBeGreaterThanOrEqual(
      badgeWidth,
    );
  });

  it('keeps the widest backup badge whole in the wide layout', () => {
    expect(
      Number.parseFloat(String(getGuestColumnStyle('backup', false, 'wide')?.width)),
    ).toBeGreaterThanOrEqual(badgeWidth);
  });
});
