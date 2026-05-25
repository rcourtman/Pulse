import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';

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

afterEach(() => {
  cleanup();
  apiFetchMock.mockReset();
});

describe('ProxmoxBackupsTable', () => {
  it('shows PBS artifacts from the PBS backup endpoint when PBS is present', async () => {
    mockBackupAPIs();

    render(() => <ProxmoxBackupsTable emptyIcon={<span />} hasPBS />);

    expect(await screen.findByText('CT 112')).toBeInTheDocument();
    expect(screen.getByText('main / minipc')).toBeInTheDocument();
    expect(screen.getByText('2 files')).toBeInTheDocument();
    expect(screen.getAllByText('Verified').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Protected').length).toBeGreaterThan(0);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pbs');
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pve');
  });

  it('omits PVE columns when the source cannot populate them', async () => {
    mockBackupAPIs();

    render(() => <ProxmoxBackupsTable emptyIcon={<span />} hasPBS={false} />);

    expect(await screen.findByText('CT 112')).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /total size/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /^ram$/i })).not.toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: /backup files/i }));
    expect(
      screen.getByText('local:backup/vzdump-lxc-112-2026_05_25-02_00_00.tar.zst'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /protection/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /verified/i })).not.toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: /recent tasks/i }));
    expect(screen.getAllByText('OK').length).toBeGreaterThan(0);
    expect(screen.queryByRole('columnheader', { name: /^size$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /error/i })).not.toBeInTheDocument();
  });
});
