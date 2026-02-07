import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import BackupsV2 from '@/components/Backups/BackupsV2';

let mockLocationSearch = '';
let mockLocationPath = '/backups-v2';
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

const wsMock = {
  state: {
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
  },
  connected: () => true,
  initialDataReceived: () => true,
};

vi.mock('@/App', () => ({
  useWebSocket: () => wsMock,
}));

describe('BackupsV2', () => {
  beforeEach(() => {
    localStorage.clear();
    navigateSpy.mockReset();
    mockLocationSearch = '';
    mockLocationPath = '/backups-v2';
  });

  afterEach(() => {
    cleanup();
  });

  it('renders a dense backup operations view with key indicators', async () => {
    render(() => <BackupsV2 />);

    expect(screen.getByRole('button', { name: '30d' })).toBeInTheDocument();

    expect(screen.getByText('Daily VM101')).toBeInTheDocument();
    expect(screen.getByText('pmg-config-backup.tar.zst')).toBeInTheDocument();

    expect(screen.getAllByText(/Protected.*Encrypted/i).length).toBeGreaterThan(0);
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

  it('does not render the legacy issues-only focus toggle', async () => {
    render(() => <BackupsV2 />);

    expect(screen.queryByRole('button', { name: 'Issues Only' })).not.toBeInTheDocument();
  });

  it('canonicalizes query params for v2 route contracts', async () => {
    mockLocationSearch = '?search=vm-101&source=pbs&backupType=remote&status=verified';

    render(() => <BackupsV2 />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('vm-101');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('backupType')).toBe('remote');
    expect(params.get('status')).toBe('verified');
    expect(params.get('search')).toBeNull();
    expect(options?.replace).toBe(true);
  });

  it('syncs query params when served from /backups route', async () => {
    mockLocationPath = '/backups';
    mockLocationSearch = '?search=vm-101&source=pbs&backupType=remote&status=verified';

    render(() => <BackupsV2 />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path] = navigateSpy.mock.calls.at(-1) as [string];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('vm-101');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('backupType')).toBe('remote');
    expect(params.get('status')).toBe('verified');
    expect(params.get('search')).toBeNull();
  });
});
