import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import type { Resource } from '@/types/resource';

// ProxmoxBackupsTable reads URL search params (node/type scope filters), so it
// must render inside a Router context.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/" component={component} />
    </Router>
  ));

const apiFetchMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

const jsonResponse = (payload: unknown) =>
  new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const pvePayload = {
  data: {
    guestSnapshots: [
      {
        id: 'snap-112',
        name: 'pre-upgrade',
        node: 'pve-a',
        instance: 'pve-a',
        type: 'ct',
        vmid: 112,
        time: '2026-05-25T01:00:00Z',
        vmstate: false,
      },
    ],
    storageBackups: [
      {
        id: 'archive-112',
        storage: 'local',
        node: 'pve-a',
        instance: 'pve-a',
        type: 'ct',
        vmid: 112,
        time: '2026-05-25T02:00:00Z',
        ctime: 1_769_390_400,
        size: 1_048_576,
        format: 'zst',
        protected: false,
        volid: 'local:backup/vzdump-lxc-112-2026_05_25-02_00_00.tar.zst',
        isPBS: false,
        verified: false,
      },
    ],
    backupTasks: [
      {
        id: 'task-112',
        node: 'pve-a',
        instance: 'pve-a',
        type: 'ct',
        vmid: 112,
        status: 'OK',
        startTime: '2026-05-25T02:00:00Z',
        endTime: '2026-05-25T02:05:00Z',
      },
    ],
  },
  meta: {
    totalBackupTasks: 1,
    totalStorageBackups: 1,
    totalGuestSnapshots: 1,
  },
};

const pbsPayload = {
  data: {
    backups: [
      {
        id: 'pbs-main/main/minipc/ct/112/2026-05-25T01:34:25Z',
        instance: 'pbs-main',
        datastore: 'main',
        namespace: 'minipc',
        backupType: 'ct',
        vmid: '112',
        backupTime: '2026-05-25T01:34:25Z',
        size: 8_589_934_592,
        protected: true,
        verified: true,
        files: ['index.json.blob', 'root.pxar.didx'],
        owner: 'backup@pbs',
      },
    ],
  },
  meta: { totalBackups: 1 },
};

function mockBackupAPIs() {
  apiFetchMock.mockImplementation((url: string) => {
    if (url === '/api/backups/pbs') return Promise.resolve(jsonResponse(pbsPayload));
    if (url === '/api/backups/pve') return Promise.resolve(jsonResponse(pvePayload));
    return Promise.resolve(jsonResponse({}));
  });
}

const workloadResource = {
  id: 'ct-112',
  type: 'system-container',
  name: 'pbs-docker',
  displayName: 'pbs-docker',
  platformId: 'pve-a',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.parse('2026-05-25T00:00:00Z'),
  proxmox: { vmid: 112, node: 'pve-a', instance: 'pve-a' },
} as Resource;

afterEach(() => {
  cleanup();
  apiFetchMock.mockReset();
});

describe('ProxmoxBackupsTable', () => {
  it('renders one coverage row per workload with restore posture', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    expect(await screen.findByText('pbs-docker')).toBeInTheDocument();
    // The workload has a recent restore point, so its posture reads "Current".
    expect(screen.getAllByText('Current').length).toBeGreaterThan(0);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pbs');
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pve');
  });

  it('collapses the surface to a single table — no inner source/job/restore tabs', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findByText('pbs-docker');

    // The old four-tab + sub-tab tree is gone; the surface is the coverage
    // table alone.
    expect(screen.queryByRole('button', { name: /workload coverage/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /restore points/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /source details/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /job history/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /pbs artifacts/i })).not.toBeInTheDocument();
  });

  it('keeps PBS / archive / snapshot evidence in the row expansion (demote, not delete)', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findByText('pbs-docker');

    await fireEvent.click(
      screen.getByRole('button', { name: /show restore evidence for pbs-docker/i }),
    );

    // The per-source detail that used to live behind Source details is now one
    // click down, inside the workload's row.
    expect(screen.getByText('Restore evidence')).toBeInTheDocument();
    expect(screen.getAllByText('PBS').length).toBeGreaterThan(0);
    expect(screen.getAllByText('PVE archive').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
  });

  it('filters coverage rows by search term', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findByText('pbs-docker');

    const searchInput = screen.getByPlaceholderText(/search backups by workload/i);
    await fireEvent.input(searchInput, { target: { value: 'no-such-guest' } });

    expect(screen.queryByText('pbs-docker')).not.toBeInTheDocument();
  });
});
