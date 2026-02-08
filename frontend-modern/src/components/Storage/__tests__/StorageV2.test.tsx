import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import StorageV2 from '@/components/Storage/StorageV2';

let mockLocationSearch = '';
let mockLocationPath = '/storage';
const navigateSpy = vi.fn();

let wsConnected = true;
let wsInitialDataReceived = true;
let wsReconnecting = false;
let wsActiveAlerts: Record<string, Alert> = {};
let wsState: any = {
  resources: [] as Resource[],
  nodes: [
    { id: 'node-1', name: 'pve1', instance: 'cluster-main' },
    { id: 'node-2', name: 'pve2', instance: 'cluster-main' },
  ],
  storage: [],
  cephClusters: [],
  pbs: [],
  physicalDisks: [],
};
const reconnectSpy = vi.fn();

let hookResources: Resource[] = [];
let hookLoading = false;
let hookError: unknown = undefined;
let alertsActivationState: 'active' | 'pending_review' | 'snoozed' | null = 'active';

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
  platformData?: Record<string, unknown>;
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
  status: (options.status || 'online') as Resource['status'],
  disk: {
    current: options.current ?? 50,
    total: options.total ?? 1_000,
    used: options.used ?? 500,
    free: options.free ?? 500,
  },
  lastSeen: Date.now(),
  platformData: {
    node,
    type: options.storageType || 'lvm',
    content: options.content || 'images,rootdir',
    shared: options.shared ?? false,
    ...(options.platformData || {}),
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
  useStorageBackupsResources: () => ({
    resources: () => hookResources,
    loading: () => hookLoading,
    error: () => hookError,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

vi.mock('@/components/Storage/DiskList', () => ({
  DiskList: (props: { selectedNode: string | null; searchTerm: string }) => (
    <div data-testid="disk-list">
      disk-view:{props.selectedNode || 'all'}:{props.searchTerm || ''}
    </div>
  ),
}));

describe('StorageV2', () => {
  beforeEach(() => {
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
      nodes: [
        { id: 'node-1', name: 'pve1', instance: 'cluster-main' },
        { id: 'node-2', name: 'pve2', instance: 'cluster-main' },
      ],
      storage: [],
      cephClusters: [],
      pbs: [],
      physicalDisks: [],
    };
    hookResources = [];
    hookLoading = false;
    hookError = undefined;
  });

  afterEach(() => {
    cleanup();
  });

  it('renders pools from v2 resources and filters by selected node', () => {
    hookResources = [
      buildStorageResource('storage-1', 'Local-LVM-PVE1', 'pve1'),
      buildStorageResource('storage-2', 'Local-LVM-PVE2', 'pve2'),
    ];

    render(() => <StorageV2 />);

    expect(screen.getByRole('heading', { name: 'Storage' })).toBeInTheDocument();
    expect(screen.getByText('Storage capacity and health across connected platforms.')).toBeInTheDocument();
    expect(screen.getByText('Local-LVM-PVE1')).toBeInTheDocument();
    expect(screen.getByText('Local-LVM-PVE2')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Node'), {
      target: { value: 'node-1' },
    });

    expect(screen.getByText('Local-LVM-PVE1')).toBeInTheDocument();
    expect(screen.queryByText('Local-LVM-PVE2')).not.toBeInTheDocument();
  });

  it('renders parity table columns and supports sort/group controls', async () => {
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
      }),
    ];

    render(() => <StorageV2 />);

    expect(screen.getByRole('columnheader', { name: 'Node' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Type' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Content' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Shared' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Used' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Free' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Total' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Usage' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Sort By'), {
      target: { value: 'usage' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Sort Direction' }));

    await waitFor(() => {
      const alpha = screen.getByText('Alpha-Store');
      const beta = screen.getByText('Beta-Store');
      expect(beta.compareDocumentPosition(alpha) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    fireEvent.change(screen.getByLabelText('Group By'), {
      target: { value: 'status' },
    });

    expect(screen.getByText('Status: degraded')).toBeInTheDocument();
    expect(screen.getByText('Status: online')).toBeInTheDocument();
  });

  it('round-trips managed storage query params through canonical URL state', async () => {
    hookResources = [buildStorageResource('storage-1', 'Ceph-Store', 'pve1', { storageType: 'cephfs' })];
    mockLocationSearch =
      '?tab=disks&search=ceph&group=status&source=proxmox-pve&status=warning&node=node-2&sort=usage&order=desc&from=proxmox-overview';

    render(() => <StorageV2 />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [initialPath, initialOptions] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const initialParams = new URLSearchParams(initialPath.split('?')[1] || '');
    expect(initialParams.get('tab')).toBe('disks');
    expect(initialParams.get('q')).toBe('ceph');
    expect(initialParams.get('group')).toBe('status');
    expect(initialParams.get('source')).toBe('proxmox-pve');
    expect(initialParams.get('sort')).toBe('usage');
    expect(initialParams.get('order')).toBe('desc');
    expect(initialParams.get('status')).toBe('warning');
    expect(initialParams.get('node')).toBe('node-2');
    expect(initialParams.get('from')).toBe('proxmox-overview');
    expect(initialParams.get('search')).toBeNull();
    expect(initialOptions?.replace).toBe(true);
    expect((screen.getByLabelText('View') as HTMLSelectElement).value).toBe('disks');
    expect((screen.getByLabelText('Node') as HTMLSelectElement).value).toBe('node-2');
    expect((screen.getByLabelText('Source') as HTMLSelectElement).value).toBe('proxmox-pve');
    expect((screen.getByLabelText('Health') as HTMLSelectElement).value).toBe('warning');
    expect((screen.getByLabelText('Group By') as HTMLSelectElement).value).toBe('status');
    expect((screen.getByLabelText('Sort By') as HTMLSelectElement).value).toBe('usage');

    fireEvent.change(screen.getByLabelText('Group By'), {
      target: { value: 'type' },
    });

    await waitFor(() => {
      const [nextPath] = navigateSpy.mock.calls.at(-1) as [string];
      const nextParams = new URLSearchParams(nextPath.split('?')[1] || '');
      expect(nextParams.get('group')).toBe('type');
    });
  });

  it('restores view and filters from URL params', () => {
    hookResources = [buildStorageResource('storage-1', 'Node-Store', 'pve1')];
    mockLocationSearch = '?tab=disks&node=node-2&q=ceph&source=proxmox-pve';

    render(() => <StorageV2 />);

    expect(screen.getByTestId('disk-list')).toHaveTextContent('disk-view:node-2:ceph');
    expect((screen.getByLabelText('View') as HTMLSelectElement).value).toBe('disks');
    expect((screen.getByLabelText('Node') as HTMLSelectElement).value).toBe('node-2');
    expect((screen.getByLabelText('Source') as HTMLSelectElement).value).toBe('proxmox-pve');
  });

  it('shows ceph summary and expandable ceph drawer details', async () => {
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

    wsState = {
      ...wsState,
      cephClusters: [
        {
          id: 'ceph-1',
          instance: 'cluster-main',
          name: 'Primary Ceph',
          health: 'HEALTH_WARN',
          healthMessage: '1 OSD nearfull',
          totalBytes: 1_000,
          usedBytes: 350,
          availableBytes: 650,
          usagePercent: 35,
          numMons: 3,
          numMgrs: 2,
          numOsds: 6,
          numOsdsUp: 6,
          numOsdsIn: 6,
          numPGs: 256,
          pools: [
            {
              id: 1,
              name: 'rbd',
              storedBytes: 350,
              availableBytes: 650,
              objects: 10_000,
              percentUsed: 35,
            },
          ],
          services: [],
          lastUpdated: Date.now(),
        },
      ],
    };

    render(() => <StorageV2 />);

    expect(screen.getByText('Ceph Summary')).toBeInTheDocument();
    expect(screen.getByText('Primary Ceph')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Toggle Ceph details for Ceph-Pool-1' }));

    await waitFor(() => {
      expect(screen.getAllByText('Ceph Cluster').length).toBeGreaterThan(0);
    });
    expect(screen.getByText(/rbd:/i)).toBeInTheDocument();
  });

  it('applies alert highlight styling for rows with active unacknowledged alerts', async () => {
    hookResources = [buildStorageResource('storage-alerted', 'Alerted-Store', 'pve1')];
    wsActiveAlerts = {
      'alert-1': buildAlert('alert-1', 'storage-alerted', 'critical'),
    };

    render(() => <StorageV2 />);

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

    render(() => <StorageV2 />);

    await waitFor(() => {
      const row = screen.getByText('Acknowledged-Store').closest('tr');
      expect(row).toHaveAttribute('data-alert-state', 'acknowledged');
      expect(row).toHaveAttribute('data-alert-severity', 'none');
      expect(row?.getAttribute('style')).toContain('156, 163, 175');
    });
  });

  it('supports resource deep-links by highlighting and auto-expanding ceph rows', async () => {
    hookResources = [
      buildStorageResource('storage-ceph-link', 'Ceph-Link-Store', 'pve1', {
        storageType: 'cephfs',
      }),
      buildStorageResource('storage-other', 'Other-Store', 'pve2'),
    ];
    mockLocationSearch = '?resource=storage-ceph-link';

    render(() => <StorageV2 />);

    await waitFor(() => {
      const row = document.querySelector('tr[data-row-id="storage-ceph-link"]');
      expect(row).toHaveAttribute('data-resource-highlighted', 'true');
      expect(document.querySelector('tr[data-row-id="storage-other"]')).toHaveAttribute(
        'data-resource-highlighted',
        'false',
      );
      expect(screen.getByRole('button', { name: 'Toggle Ceph details for Ceph-Link-Store' })).toHaveTextContent(
        'Hide',
      );
    });
  });

  it('shows zfs health indicators when legacy storage carries pool errors', async () => {
    wsState = {
      ...wsState,
      storage: [
        {
          id: 'storage-zfs',
          name: 'local-zfs',
          node: 'pve1',
          instance: 'cluster-main',
          type: 'zfspool',
          status: 'available',
          total: 1_000,
          used: 600,
          free: 400,
          usage: 60,
          content: 'images,rootdir',
          shared: false,
          enabled: true,
          active: true,
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
      ],
    };

    render(() => <StorageV2 />);

    await waitFor(() => {
      expect(screen.getAllByText('DEGRADED').length).toBeGreaterThan(0);
    });
    expect(screen.getByText('ERRORS')).toBeInTheDocument();
    expect(screen.getByText('ZFS Pool Status:')).toBeInTheDocument();
  });

  it('shows disconnected waiting state while loading with no initial data', () => {
    hookLoading = true;
    wsConnected = false;
    wsInitialDataReceived = false;

    render(() => <StorageV2 />);

    expect(screen.getByText('Waiting for storage data from connected platforms.')).toBeInTheDocument();
  });

  it('shows reconnect banner and retries on demand', () => {
    wsReconnecting = true;

    render(() => <StorageV2 />);

    expect(screen.getByText('Reconnecting to backend data stream…')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry now' }));
    expect(reconnectSpy).toHaveBeenCalledTimes(1);
  });

  it('shows disconnected stale-data state after initial load', () => {
    wsConnected = false;
    wsInitialDataReceived = true;
    wsReconnecting = false;

    render(() => <StorageV2 />);

    expect(screen.getByText('Storage data stream disconnected. Data may be stale.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }));
    expect(reconnectSpy).toHaveBeenCalledTimes(1);
  });

  it('shows warning when v2 fetch reports an error', () => {
    hookError = new Error('network');

    render(() => <StorageV2 />);

    expect(
      screen.getByText('Unable to refresh storage resources. Showing latest available data.'),
    ).toBeInTheDocument();
  });

  it('switches to physical disks view', () => {
    render(() => <StorageV2 />);

    fireEvent.change(screen.getByLabelText('View'), {
      target: { value: 'disks' },
    });

    expect(screen.getByTestId('disk-list')).toBeInTheDocument();
  });

  it('shows loading placeholder when pool resources are loading', () => {
    hookLoading = true;

    render(() => <StorageV2 />);

    expect(screen.getByText('Loading storage resources...')).toBeInTheDocument();
  });

  it('GA contract: StorageV2 served at /storage is the only canonical path', async () => {
    hookResources = [buildStorageResource('storage-ga', 'GA-Store', 'pve1', { status: 'degraded' })];
    mockLocationPath = '/storage';
    mockLocationSearch = '?source=proxmox-pve&status=warning';

    render(() => <StorageV2 />);

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
});
