import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@solidjs/testing-library';
import { createEffect } from 'solid-js';
import { WorkloadsSurface } from '../WorkloadsSurface';
import type { State } from '@/types/api';

const mockWebSocketState: State = {
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
  pveTagColors: {},
  pveTagStyles: {},
  lastUpdate: 0,
  resources: [],
  temperatureMonitoringEnabled: false,
};

let mockLocationSearch = '?type=pod';
let mockWorkloads: Array<Record<string, unknown>> = [];
const navigateSpy = vi.fn();
type HostFilterMock = {
  id?: string;
  label?: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
};

let lastHostFilter: HostFilterMock | undefined;
let lastDrawerGuestName: string | null = null;

const requireLastHostFilter = (): HostFilterMock => {
  if (!lastHostFilter) {
    throw new Error('Expected host filter to be available');
  }
  return lastHostFilter;
};

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({
      pathname: '/kubernetes/workloads',
      search: mockLocationSearch,
    }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    connected: () => true,
    activeAlerts: () => ({}),
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
    state: mockWebSocketState,
  }),
}));

vi.mock('@/hooks/useWorkloads', () => ({
  useWorkloads: () => ({
    workloads: () => mockWorkloads as any,
    refetch: vi.fn(),
    mutate: vi.fn(),
    loading: () => false,
    error: () => null,
  }),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    resources: () => [],
    policyPosture: () => null,
    refetch: vi.fn(),
    mutate: vi.fn(),
    loading: () => false,
    error: () => null,
  }),
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: {
    getAllMetadata: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 70, critical: 85 }),
    getBackupThresholds: () => ({ freshHours: 24, staleHours: 72 }),
  }),
}));

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: {
    focusInput: () => false,
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
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
  WorkloadsFilter: (props: { hostFilter?: HostFilterMock }) => {
    createEffect(() => {
      lastHostFilter = props.hostFilter;
    });
    return (
      <div data-testid="workloads-filter">
        {props.hostFilter ? 'host-filter-enabled' : 'host-filter-disabled'}
      </div>
    );
  },
}));

vi.mock('../GuestDrawer', () => ({
  GuestDrawer: (props: { guest: { name: string } }) => {
    lastDrawerGuestName = props.guest.name;
    return <div data-testid="guest-drawer">{props.guest.name}</div>;
  },
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
      'app-container': new Set(['name', 'status']),
      pod: new Set(['name', 'status']),
    },
    GuestRow: (props: { guest: { name: string } }) => (
      <tr data-testid={`guest-row-${props.guest.name}`}>
        <td>{props.guest.name}</td>
        <td>running</td>
      </tr>
    ),
  };
});

describe('Workloads pod workloads integration', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    localStorage.clear();
    lastDrawerGuestName = null;
  });

  it('renders pod workloads in the unified workloads table and shows cluster filter in pod view', async () => {
    lastHostFilter = undefined;
    mockWorkloads = [
      {
        id: 'v2-k8s-pod-visible',
        vmid: 0,
        name: 'api-6c4d8',
        node: 'worker-1',
        instance: 'cluster-visible',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];
    mockLocationSearch = '?type=pod';
    const { getByText, getByTestId } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-6c4d8')).toBeInTheDocument();
    });

    expect(getByTestId('workloads-filter')).toHaveTextContent('host-filter-enabled');
  });

  it('excludes app-container workloads when a platform page owns them as nested context', async () => {
    mockLocationSearch = '?type=all';
    mockWorkloads = [
      {
        id: 'pve-lxc-101',
        vmid: 101,
        name: 'media-lxc',
        node: 'pve-a',
        instance: 'pve-a',
        status: 'running',
        type: 'system-container',
        workloadType: 'system-container',
        platformType: 'proxmox-pve',
        platformScopes: ['proxmox-pve'],
      },
      {
        id: 'app-container:pve-a:grafana',
        vmid: 0,
        name: 'grafana',
        node: 'media-lxc',
        instance: 'pve-a',
        status: 'running',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        platformScopes: ['proxmox-pve', 'docker'],
        containerRuntime: 'docker',
      },
    ];

    render(() => (
      <WorkloadsSurface
        vms={[]}
        containers={[]}
        nodes={[]}
        useWorkloads
        forcedPlatform="proxmox-pve"
        forcedViewMode="all"
        excludedWorkloadTypes={['app-container']}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('media-lxc')).toBeInTheDocument();
    });

    expect(screen.queryByText('grafana')).not.toBeInTheDocument();
  });

  it('uses filtered-empty copy for table-only surfaces when filters remove every workload', () => {
    render(() => (
      <WorkloadsSurface
        vms={[]}
        containers={[]}
        nodes={[]}
        useWorkloads
        forcedViewMode="vm"
        emptyStateTitle="No vSphere VMs"
        emptyStateDescription="Virtual machines appear here once the vCenter connection enumerates them."
        state={
          {
            setClearSurfaceRootRef: vi.fn(),
            kioskMode: () => false,
            surfaceConnected: () => true,
            surfaceInitialDataReceived: () => true,
            allGuests: () => [{ id: 'vm-1' }],
            filteredGuests: () => [],
            search: () => '',
            viewMode: () => 'vm',
            statusMode: () => 'stopped',
            hostFilterConfig: () => undefined,
            platformFilterConfig: () => undefined,
            namespaceFilterConfig: () => undefined,
            containerRuntimeFilterConfig: () => undefined,
            workloadsGuestsEmptyState: () => ({
              title: 'No guests found',
              description: 'No guests match your current filters',
            }),
          } as any
        }
      />
    ));

    expect(screen.getByText('No guests found')).toBeInTheDocument();
    expect(screen.getByText('No guests match your current filters')).toBeInTheDocument();
    expect(screen.queryByText('No vSphere VMs')).not.toBeInTheDocument();
  });

  it('keeps page-owned table-only empty copy when no operator filters are active', () => {
    render(() => (
      <WorkloadsSurface
        vms={[]}
        containers={[]}
        nodes={[]}
        useWorkloads
        forcedViewMode="vm"
        emptyStateTitle="No vSphere VMs"
        emptyStateDescription="Virtual machines appear here once the vCenter connection enumerates them."
        state={
          {
            setClearSurfaceRootRef: vi.fn(),
            kioskMode: () => false,
            surfaceConnected: () => true,
            surfaceInitialDataReceived: () => true,
            allGuests: () => [],
            filteredGuests: () => [],
            search: () => '',
            viewMode: () => 'vm',
            statusMode: () => 'all',
            hostFilterConfig: () => undefined,
            platformFilterConfig: () => undefined,
            namespaceFilterConfig: () => undefined,
            containerRuntimeFilterConfig: () => undefined,
            workloadsGuestsEmptyState: () => ({
              title: 'No guests found',
              description: 'No guests match your current filters',
            }),
          } as any
        }
      />
    ));

    expect(screen.getByText('No vSphere VMs')).toBeInTheDocument();
    expect(
      screen.getByText('Virtual machines appear here once the vCenter connection enumerates them.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('No guests found')).not.toBeInTheDocument();
  });

  it('does not let preselected host filtering suppress pod workloads', async () => {
    lastHostFilter = undefined;
    mockWorkloads = [
      {
        id: 'v2-k8s-pod-visible',
        vmid: 0,
        name: 'api-6c4d8',
        node: 'worker-1',
        instance: 'cluster-visible',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];
    mockLocationSearch = '?type=pod&resource=legacy:pve1:101';

    const { getByText } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-6c4d8')).toBeInTheDocument();
    });
  });

  it('renders only native v2 kubernetes workloads', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=pod';
    mockWorkloads = [
      {
        id: 'v2-k8s-pod-1',
        vmid: 0,
        name: 'api-native',
        node: 'worker-v2',
        instance: 'cluster-visible',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];

    const { getByText, queryByText } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-native')).toBeInTheDocument();
    });

    expect(queryByText('api-6c4d8')).not.toBeInTheDocument();
  });

  it('filters kubernetes workloads by selected cluster', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=pod';
    mockWorkloads = [
      {
        id: 'v2-k8s-pod-a',
        vmid: 0,
        name: 'api-a',
        node: 'worker-a',
        instance: 'cluster-a',
        contextLabel: 'cluster-a',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
      {
        id: 'v2-k8s-pod-b',
        vmid: 0,
        name: 'api-b',
        node: 'worker-b',
        instance: 'cluster-b',
        contextLabel: 'cluster-b',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];

    const { getByText, queryByText } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-a')).toBeInTheDocument();
      expect(getByText('api-b')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(lastHostFilter).toBeDefined();
      expect(lastHostFilter?.label).toBe('K8s cluster');
    });
    const hostFilter = requireLastHostFilter();
    hostFilter.onChange('cluster-a');

    await waitFor(() => {
      expect(getByText('api-a')).toBeInTheDocument();
      expect(queryByText('api-b')).not.toBeInTheDocument();
    });
  });

  it('applies kubernetes context from URL query params', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=pod&context=cluster-b';
    mockWorkloads = [
      {
        id: 'v2-k8s-pod-a',
        vmid: 0,
        name: 'api-a',
        node: 'worker-a',
        instance: 'cluster-a',
        contextLabel: 'cluster-a',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
      {
        id: 'v2-k8s-pod-b',
        vmid: 0,
        name: 'api-b',
        node: 'worker-b',
        instance: 'cluster-b',
        contextLabel: 'cluster-b',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];

    const { getByText, queryByText } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-b')).toBeInTheDocument();
      expect(queryByText('api-a')).not.toBeInTheDocument();
    });
  });

  it('applies non-kubernetes agent query params to workload filtering', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=all&agent=pve-a';
    mockWorkloads = [
      {
        id: 'v2-vm-a',
        vmid: 101,
        name: 'vm-a',
        node: 'pve-a',
        instance: 'cluster-main',
        status: 'running',
        type: 'vm',
        cpu: 0,
        cpus: 2,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'vm',
      },
      {
        id: 'v2-vm-b',
        vmid: 102,
        name: 'vm-b',
        node: 'pve-b',
        instance: 'cluster-main',
        status: 'running',
        type: 'vm',
        cpu: 0,
        cpus: 2,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'vm',
      },
    ];

    const { getByText, queryByText } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('vm-a')).toBeInTheDocument();
      expect(queryByText('vm-b')).not.toBeInTheDocument();
    });
  });

  it('opens the Kubernetes workload drawer from resource deep links in mixed workloads', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=pod&resource=v2-k8s-pod-b';
    mockWorkloads = [
      {
        id: 'v2-vm-101',
        vmid: 101,
        name: 'vm-general',
        node: 'pve-a',
        instance: 'cluster-main',
        status: 'running',
        type: 'vm',
        cpu: 0,
        cpus: 2,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'vm',
      },
      {
        id: 'v2-k8s-pod-a',
        vmid: 0,
        name: 'api-a',
        node: 'worker-a',
        instance: 'cluster-a',
        contextLabel: 'cluster-a',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
      {
        id: 'v2-k8s-pod-b',
        vmid: 0,
        name: 'api-b',
        node: 'worker-b',
        instance: 'cluster-b',
        contextLabel: 'cluster-b',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];

    const { queryByText, getByTestId } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByTestId('guest-row-api-a')).toBeInTheDocument();
      expect(getByTestId('guest-row-api-b')).toBeInTheDocument();
      expect(queryByText('vm-general')).not.toBeInTheDocument();
      expect(getByTestId('guest-drawer')).toBeInTheDocument();
      expect(getByTestId('guest-drawer')).toHaveTextContent('api-b');
      expect(lastDrawerGuestName).toBe('api-b');
    });
  });

  it('groups Kubernetes workloads by context with the same grouped table behavior as other workloads', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=pod';
    mockWorkloads = [
      {
        id: 'v2-k8s-pod-a',
        vmid: 0,
        name: 'api-a',
        node: 'worker-a',
        instance: 'cluster-a',
        contextLabel: 'cluster-a',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
      {
        id: 'v2-k8s-pod-b',
        vmid: 0,
        name: 'api-b',
        node: 'worker-b',
        instance: 'cluster-b',
        contextLabel: 'cluster-b',
        status: 'running',
        type: 'pod',
        cpu: 0,
        cpus: 0,
        memory: { total: 0, used: 0, free: 0, usage: 0 },
        disk: { total: 0, used: 0, free: 0, usage: 0 },
        networkIn: 0,
        networkOut: 0,
        diskRead: 0,
        diskWrite: 0,
        uptime: 0,
        template: false,
        lastBackup: 0,
        tags: [],
        lock: '',
        lastSeen: new Date().toISOString(),
        workloadType: 'pod',
        namespace: 'default',
      },
    ];

    const { getByText, getAllByText } = render(() => (
      <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('cluster-a')).toBeInTheDocument();
      expect(getByText('cluster-b')).toBeInTheDocument();
      expect(getAllByText('Pods').length).toBeGreaterThan(0);
      expect(getByText('api-a')).toBeInTheDocument();
      expect(getByText('api-b')).toBeInTheDocument();
    });
  });

  it('canonicalizes type=all workload query params', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=all';
    mockWorkloads = [];

    render(() => <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    expect(path).toBe('/kubernetes/workloads');
    expect(options?.replace).toBe(true);
  });

  it('normalizes non-canonical type to k8s when context is present', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=all&context=cluster-a';
    mockWorkloads = [];

    render(() => <WorkloadsSurface vms={[]} containers={[]} nodes={[]} useWorkloads />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('type')).toBe('pod');
    expect(params.get('context')).toBe('cluster-a');
    expect(options?.replace).toBe(true);
  });
});
