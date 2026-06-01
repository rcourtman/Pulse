import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import type { Resource } from '@/types/resource';
import { getRecoveryFullDateLabel } from '@/utils/recoveryDatePresentation';

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
  it('defaults to the By date chronological feed with source/location/state columns', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    // Default view is the v5-parity backup feed: one row PER backup, so the
    // guest with PBS + archive + snapshot artifacts appears on multiple rows,
    // sourced and located, not collapsed to a single coverage-posture summary.
    expect((await screen.findAllByText('pbs-docker')).length).toBeGreaterThan(1);
    expect(screen.getByRole('columnheader', { name: /location/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /source/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /type/i })).toBeInTheDocument();
    expect(screen.getAllByText('LXC').length).toBeGreaterThan(0);
    expect(screen.getAllByText('PBS').length).toBeGreaterThan(0);
    expect(screen.getByText('main / minipc')).toBeInTheDocument();
    expect(
      screen.getByRole('cell', {
        name: `${getRecoveryFullDateLabel('2026-05-25')} 3 backups`,
      }),
    ).toBeInTheDocument();
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pbs');
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pve');
  });

  it('offers By date / Coverage views and no legacy sub-tab tree', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');

    // The two top-level views exist...
    expect(screen.getByRole('button', { name: /by date/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /coverage/i })).toBeInTheDocument();
    // ...and the old four-tab + sub-tab tree does not.
    expect(screen.queryByRole('button', { name: /source details/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /job history/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /pbs artifacts/i })).not.toBeInTheDocument();
  });

  it('switches to Coverage showing posture, and keeps per-source evidence in the row expansion', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');
    await fireEvent.click(screen.getByRole('button', { name: /coverage/i }));

    // Coverage is the posture view; the workload's recent backup reads
    // "Current".
    expect(screen.getByRole('columnheader', { name: /posture/i })).toBeInTheDocument();
    expect(screen.getAllByText('Current').length).toBeGreaterThan(0);

    // Per-source detail is one click down inside the workload's row.
    await fireEvent.click(
      screen.getByRole('button', { name: /show restore evidence for pbs-docker/i }),
    );
    expect(screen.getByText('Restore evidence')).toBeInTheDocument();
    expect(screen.getAllByText('PVE archive').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
  });

  it('filters the backup feed by search term', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');

    const searchInput = screen.getByPlaceholderText(/search backups by workload/i);
    await fireEvent.input(searchInput, { target: { value: 'no-such-guest' } });

    expect(screen.queryByText('pbs-docker')).not.toBeInTheDocument();
  });
});
