import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@solidjs/testing-library';
import { createEffect } from 'solid-js';
import { Dashboard } from '../Dashboard';

const mockWebSocketState = {
  kubernetesClusters: [],
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
      pathname: '/workloads',
      search: mockLocationSearch,
    }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/App', () => ({
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

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: {
    getAllMetadata: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => 'active',
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

vi.mock('@/components/shared/UnifiedNodeSelector', () => ({
  UnifiedNodeSelector: () => <div data-testid="node-selector">node-selector</div>,
}));

vi.mock('../DashboardFilter', () => ({
  DashboardFilter: (props: { hostFilter?: HostFilterMock }) => {
    createEffect(() => {
      lastHostFilter = props.hostFilter;
    });
    return (
      <div data-testid="dashboard-filter">
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

describe('Dashboard pod workloads integration', () => {
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-6c4d8')).toBeInTheDocument();
    });

    expect(getByTestId('dashboard-filter')).toHaveTextContent('host-filter-enabled');
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
    ));

    await waitFor(() => {
      expect(getByText('api-a')).toBeInTheDocument();
      expect(getByText('api-b')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(lastHostFilter).toBeDefined();
      expect(lastHostFilter?.label).toBe('Cluster');
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
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
      <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />
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

    render(() => <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    expect(path).toBe('/workloads');
    expect(options?.replace).toBe(true);
  });

  it('normalizes non-canonical type to k8s when context is present', async () => {
    lastHostFilter = undefined;
    mockLocationSearch = '?type=all&context=cluster-a';
    mockWorkloads = [];

    render(() => <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />);

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
