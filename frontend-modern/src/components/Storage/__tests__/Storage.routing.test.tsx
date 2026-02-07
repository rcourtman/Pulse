import { render, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import Storage from '@/components/Storage/Storage';

let mockLocationSearch = '';
const navigateSpy = vi.fn();

type StorageFilterMock = {
  setSearch: (value: string) => void;
  setGroupBy: (value: 'node' | 'storage') => void;
  setSourceFilter: (value: 'all' | 'proxmox' | 'pbs' | 'ceph') => void;
  setStatusFilter: (value: 'all' | 'available' | 'offline') => void;
};

let lastStorageFilter: StorageFilterMock | undefined;

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/storage', search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: {
      temperatureMonitoringEnabled: false,
      nodes: [
        {
          id: 'cluster-main-pve1',
          instance: 'cluster-main',
          name: 'pve1',
          type: 'node',
          status: 'online',
          uptime: 100,
        },
      ],
      storage: [
        {
          id: 'storage-1',
          instance: 'cluster-main',
          node: 'pve1',
          name: 'local-lvm',
          type: 'lvm',
          content: 'images',
          status: 'available',
          shared: false,
          usage: 25,
          total: 1000,
          used: 250,
          free: 750,
        },
      ],
      cephClusters: [],
      physicalDisks: [],
    },
    connected: () => true,
    activeAlerts: () => [],
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
  }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => 'active',
  }),
}));

vi.mock('@/components/shared/UnifiedNodeSelector', () => ({
  UnifiedNodeSelector: () => <div data-testid="storage-node-selector">node-selector</div>,
}));

vi.mock('@/components/Storage/StorageFilter', () => ({
  StorageFilter: (props: StorageFilterMock) => {
    lastStorageFilter = props;
    return <div data-testid="storage-filter">filter</div>;
  },
}));

vi.mock('@/components/ErrorBoundary', () => ({
  ComponentErrorBoundary: (props: { children: unknown }) => <>{props.children}</>,
}));

vi.mock('@/components/Storage/DiskList', () => ({
  DiskList: () => <div data-testid="disk-list">disk-list</div>,
}));

vi.mock('@/components/Storage/ZFSHealthMap', () => ({
  ZFSHealthMap: () => <div data-testid="zfs-map">zfs</div>,
}));

vi.mock('@/components/Storage/EnhancedStorageBar', () => ({
  EnhancedStorageBar: () => <div data-testid="storage-bar">bar</div>,
}));

vi.mock('@/components/shared/NodeGroupHeader', () => ({
  NodeGroupHeader: () => <tr data-testid="node-group-header" />,
}));

describe('Storage routing contract', () => {
  beforeEach(() => {
    mockLocationSearch = '';
    navigateSpy.mockReset();
    lastStorageFilter = undefined;
  });

  it('canonicalizes legacy search query params to q', async () => {
    mockLocationSearch = '?search=local&migrated=1&from=proxmox-overview';

    render(() => <Storage />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('local');
    expect(params.get('search')).toBeNull();
    expect(params.get('migrated')).toBe('1');
    expect(params.get('from')).toBe('proxmox-overview');
    expect(options?.replace).toBe(true);
  });

  it('syncs storage filter state into URL query params', async () => {
    render(() => <Storage />);

    await waitFor(() => {
      expect(lastStorageFilter).toBeDefined();
    });

    lastStorageFilter!.setSourceFilter('pbs');
    lastStorageFilter!.setStatusFilter('offline');
    lastStorageFilter!.setGroupBy('storage');
    lastStorageFilter!.setSearch('ceph');

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('status')).toBe('offline');
    expect(params.get('group')).toBe('storage');
    expect(params.get('q')).toBe('ceph');
    expect(options?.replace).toBe(true);
  });
});
