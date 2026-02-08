import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import BackupsV2 from '@/components/Backups/BackupsV2';

let mockLocationSearch = '';
let mockLocationPath = '/backups';
const navigateSpy = vi.fn();
const nowMs = Date.now();
const isoHoursAgo = (hours: number) => new Date(nowMs - hours * 60 * 60 * 1000).toISOString();
const unixSecondsHoursAgo = (hours: number) => Math.floor((nowMs - hours * 60 * 60 * 1000) / 1000);

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockLocationPath, search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

const createWsState = () => ({
  resources: [],
  backups: {
    pve: {
      backupTasks: [],
      guestSnapshots: [],
      storageBackups: [
        {
          id: 'pve-1',
          storage: 'local',
          node: 'pve1',
          instance: 'pve-main',
          type: 'qemu',
          vmid: 102,
          time: isoHoursAgo(3),
          ctime: unixSecondsHoursAgo(3),
          size: 1024,
          format: 'vma.zst',
          notes: 'nightly',
          protected: true,
          volid: 'backup/vzdump-qemu-102.vma.zst',
          isPBS: false,
          verified: false,
          verification: 'failed',
          encryption: 'on',
        },
        {
          id: 'pve-2',
          storage: 'local',
          node: 'pve1',
          instance: 'pve-main',
          type: 'lxc',
          vmid: 103,
          time: isoHoursAgo(4),
          ctime: unixSecondsHoursAgo(4),
          size: 4096,
          format: 'tar.zst',
          notes: 'Daily LXC103',
          protected: false,
          volid: 'backup/vzdump-lxc-103.tar.zst',
          isPBS: false,
          verified: true,
          verification: 'ok',
          encryption: 'off',
        },
      ],
    },
    pbs: [
      {
        id: 'pbs-1',
        instance: 'pbs-main',
        datastore: 'primary',
        namespace: '',
        backupType: 'vm',
        vmid: '101',
        backupTime: isoHoursAgo(2),
        size: 2048,
        protected: true,
        verified: true,
        comment: 'Daily VM101',
        files: ['index.fidx', 'blob.enc'],
        owner: 'root@pam',
      },
      {
        id: 'pbs-2',
        instance: 'pbs-main',
        datastore: 'primary',
        namespace: 'tenant-a',
        backupType: 'vm',
        vmid: '202',
        backupTime: isoHoursAgo(5),
        size: 1024,
        protected: false,
        verified: true,
        comment: 'Tenant VM202',
        files: ['index.fidx'],
        owner: 'root@pam',
      },
    ],
    pmg: [
      {
        id: 'pmg-1',
        instance: 'pmg-main',
        node: 'pmg-node-1',
        filename: 'pmg-config-backup.tar.zst',
        backupTime: isoHoursAgo(1),
        size: 512,
      },
    ],
  },
  pveBackups: { backupTasks: [], storageBackups: [], guestSnapshots: [] },
  pbsBackups: [],
  pmgBackups: [],
});

const wsMock = {
  state: createWsState(),
  connected: () => true,
  initialDataReceived: () => true,
};

vi.mock('@/App', () => ({
  useWebSocket: () => wsMock,
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useStorageBackupsResources: () => ({
    resources: () => [],
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

describe('BackupsV2', () => {
  beforeEach(() => {
    localStorage.clear();
    navigateSpy.mockReset();
    mockLocationSearch = '';
    mockLocationPath = '/backups';
    wsMock.state = createWsState();
  });

  afterEach(() => {
    cleanup();
  });

  it('renders a dense backup operations view with key indicators', async () => {
    render(() => <BackupsV2 />);

    expect(screen.getByRole('button', { name: '30d' })).toBeInTheDocument();

    expect(screen.getByText('Daily VM101')).toBeInTheDocument();
    expect(screen.getByText('Tenant VM202')).toBeInTheDocument();
    expect(screen.getByText('pmg-config-backup.tar.zst')).toBeInTheDocument();

    expect(screen.getAllByText(/Protected.*Encrypted/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText('VM').some((node) => node.className.includes('bg-blue-100'))).toBe(true);
    expect(screen.getByText('LXC').className).toContain('bg-green-100');
    expect(screen.getAllByText('PBS').some((node) => node.className.includes('bg-indigo-100'))).toBe(true);
    expect(screen.getAllByText('PVE').some((node) => node.className.includes('bg-orange-100'))).toBe(true);
    expect(screen.getAllByText('PMG').some((node) => node.className.includes('bg-rose-100'))).toBe(true);
  });

  it('filters by source platform', async () => {
    render(() => <BackupsV2 />);

    fireEvent.change(screen.getByLabelText('Source'), {
      target: { value: 'proxmox-pbs' },
    });

    await waitFor(() => {
      expect(screen.getByText('Daily VM101')).toBeInTheDocument();
    });

    expect(screen.queryByText('pmg-config-backup.tar.zst')).not.toBeInTheDocument();
  });

  it('hides optional columns when records do not expose those capabilities', async () => {
    wsMock.state = {
      ...createWsState(),
      backups: {
        pve: { backupTasks: [], guestSnapshots: [], storageBackups: [] },
        pbs: [],
        pmg: [
          {
            id: 'pmg-only',
            instance: 'pmg-main',
            node: 'pmg-node-1',
            filename: 'pmg-config-backup.tar.zst',
            backupTime: isoHoursAgo(1),
            size: 0,
          },
        ],
      },
    };

    render(() => <BackupsV2 />);

    await waitFor(() => {
      expect(screen.getByText('pmg-config-backup.tar.zst')).toBeInTheDocument();
    });

    expect(screen.queryByRole('columnheader', { name: 'Namespace' })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Verified' })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Details' })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Size' })).not.toBeInTheDocument();
  });

  it('filters by namespace', async () => {
    render(() => <BackupsV2 />);

    fireEvent.change(screen.getByLabelText('Namespace'), {
      target: { value: 'tenant-a' },
    });

    await waitFor(() => {
      expect(screen.getByText('Tenant VM202')).toBeInTheDocument();
    });

    expect(screen.queryByText('Daily VM101')).not.toBeInTheDocument();
  });

  it('shows active namespace chip and clears namespace filter from chip action', async () => {
    render(() => <BackupsV2 />);

    fireEvent.change(screen.getByLabelText('Namespace'), {
      target: { value: 'tenant-a' },
    });

    const chip = await screen.findByTestId('active-namespace-chip');
    expect(chip).toHaveTextContent('tenant-a');

    fireEvent.click(within(chip).getByRole('button', { name: 'Clear' }));

    await waitFor(() => {
      expect(screen.queryByTestId('active-namespace-chip')).not.toBeInTheDocument();
    });

    expect(screen.getByText('Daily VM101')).toBeInTheDocument();
  });

  it('does not render the legacy issues-only focus toggle', async () => {
    render(() => <BackupsV2 />);

    expect(screen.queryByRole('button', { name: 'Issues Only' })).not.toBeInTheDocument();
  });

  it('canonicalizes query params for v2 route contracts', async () => {
    mockLocationSearch = '?search=vm-101&source=pbs&namespace=tenant-a&backupType=remote&status=verified';

    render(() => <BackupsV2 />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('vm-101');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('namespace')).toBe('tenant-a');
    expect(params.get('backupType')).toBe('remote');
    expect(params.get('status')).toBe('verified');
    expect(params.get('search')).toBeNull();
    expect(options?.replace).toBe(true);
  });

  it('syncs query params when served from /backups route', async () => {
    mockLocationPath = '/backups';
    mockLocationSearch = '?search=vm-101&source=pbs&namespace=tenant-a&backupType=remote&status=verified';

    render(() => <BackupsV2 />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path] = navigateSpy.mock.calls.at(-1) as [string];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('vm-101');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('namespace')).toBe('tenant-a');
    expect(params.get('backupType')).toBe('remote');
    expect(params.get('status')).toBe('verified');
    expect(params.get('search')).toBeNull();
  });

  it('GA contract: BackupsV2 served at /backups is the only canonical path', async () => {
    mockLocationPath = '/backups';
    mockLocationSearch = '?source=pbs&status=verified';

    render(() => <BackupsV2 />);

    await waitFor(() => {
      expect(screen.getByTestId('backups-v2-page')).toBeInTheDocument();
    });

    const path =
      navigateSpy.mock.calls.length > 0
        ? (navigateSpy.mock.calls.at(-1) as [string])[0]
        : `${mockLocationPath}${mockLocationSearch}`;
    expect(path.startsWith('/backups')).toBe(true);
    expect(path).not.toContain('/backups-v2');
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('status')).toBe('verified');
  });
});
