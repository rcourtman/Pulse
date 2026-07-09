import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import proxmoxBackupServersTableSource from '../ProxmoxBackupServersTable.tsx?raw';
import proxmoxBackupsTableSource from '../ProxmoxBackupsTable.tsx?raw';
import proxmoxPageSurfaceSource from '../ProxmoxPageSurface.tsx?raw';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
} from '@/features/platformPage/sharedPlatformPage';
import { TABLE_CARD_FRAME_CLASS } from '@/components/shared/TableCard';
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

const pbsServerResource = {
  id: 'pbs-main',
  type: 'pbs',
  name: 'pbs-main',
  displayName: 'pbs-main',
  platformId: 'pbs-main',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.parse('2026-05-25T00:00:00Z'),
  cpu: { current: 12 },
  memory: { current: 40, total: 8_000, used: 3_200, free: 4_800 },
  uptime: 86_400,
  pbs: {
    instanceId: 'pbs-main',
    version: '3.2.1',
    connectionHealth: 'healthy',
    datastores: [{ name: 'main', total: 10_000, used: 4_000, available: 6_000, usagePercent: 40 }],
  },
} as Resource;

const expectClassTokens = (element: Element | null, className: string): void => {
  expect(element).not.toBeNull();
  for (const token of className.split(/\s+/).filter(Boolean)) {
    expect(element).toHaveClass(token);
  }
};

const expectCanonicalPlatformTableShell = (table: HTMLElement): void => {
  expectClassTokens(table.querySelector('thead tr'), PLATFORM_TABLE_HEADER_ROW_CLASS);
  expectClassTokens(table.querySelector('tbody'), PLATFORM_TABLE_BODY_CLASS);
};

afterEach(() => {
  cleanup();
  apiFetchMock.mockReset();
});

describe('ProxmoxBackupsTable', () => {
  it('defaults to coverage when protection needs attention and keeps the dated feed one click away', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable
        emptyIcon={<span />}
        workloads={[workloadResource]}
        servers={[pbsServerResource]}
      />
    ));

    await screen.findAllByText('pbs-docker');
    expect(screen.getByRole('button', { name: /coverage/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );

    await fireEvent.click(screen.getByRole('button', { name: /by date/i }));

    // The chronological feed remains available for forensic review: one row
    // per restore point, sourced and located.
    expect(screen.getAllByText('pbs-docker').length).toBeGreaterThan(1);
    expect(screen.getByRole('columnheader', { name: /location/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /source/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /type/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /target id/i })).toBeInTheDocument();
    expect(screen.getAllByText('LXC').length).toBeGreaterThan(0);
    expect(screen.getAllByText('PBS').length).toBeGreaterThan(0);
    expect(screen.getByText('2 PBS files')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pbs snapshots/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pve backup files/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /guest snapshots/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Archives' })).not.toBeInTheDocument();
    expect(screen.getByText('main / minipc')).toBeInTheDocument();
    expect(
      screen.getByRole('cell', {
        name: `${getRecoveryFullDateLabel('2026-05-25')} 3 backups`,
      }),
    ).toBeInTheDocument();
    const tables = screen.getAllByRole('table');
    expect(tables).toHaveLength(2);
    for (const table of tables) {
      expectCanonicalPlatformTableShell(table);
    }
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pbs');
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pve');
  });

  it('offers By date / Coverage views and no legacy sub-tab tree', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');

    expect(proxmoxBackupsTableSource).toContain(
      "import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';",
    );
    expect(proxmoxBackupsTableSource).toContain('<FilterSegmentedControl');
    expect(proxmoxBackupsTableSource).not.toContain('variant="segmented"');
    expect(proxmoxBackupsTableSource).not.toContain('const viewButtonClass');
    expect(proxmoxBackupsTableSource).not.toContain(
      'inline-flex items-center gap-1 rounded-md border border-border bg-surface p-1',
    );
    expect(proxmoxBackupsTableSource).not.toContain(
      'inline-flex min-h-8 items-center gap-1.5 rounded-sm px-3 text-xs font-medium transition-colors',
    );

    // The two top-level views exist...
    expect(screen.getByRole('group', { name: /backups view/i })).toBeInTheDocument();
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
    await fireEvent.click(screen.getByRole('button', { name: /expand details for pbs-docker/i }));
    expect(screen.getByText('Restore evidence')).toBeInTheDocument();
    expect(screen.getAllByText('PVE file').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
  });

  it('filters the backup feed by search term', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');
    await fireEvent.click(screen.getByRole('button', { name: /by date/i }));

    const searchInput = screen.getByPlaceholderText(/search backups by workload/i);
    await fireEvent.input(searchInput, { target: { value: 'no-such-guest' } });

    expect(screen.queryByText('pbs-docker')).not.toBeInTheDocument();
    const emptyStateHeading = screen.getByRole('heading', {
      name: /no recoverable artifacts match current filters/i,
    });
    expect(emptyStateHeading).toBeInTheDocument();
    expect(
      screen.getByText('Adjust the search, source filter, or selected day to see more artifacts.'),
    ).toBeInTheDocument();
    expectClassTokens(
      emptyStateHeading.closest(`.${TABLE_CARD_FRAME_CLASS}`),
      TABLE_CARD_FRAME_CLASS,
    );
    expect(emptyStateHeading.closest('.border-dashed')).not.toBeNull();
  });

  it('routes top-level loading and error states through shared platform primitives', () => {
    expect(proxmoxBackupsTableSource).toContain('PlatformErrorState');
    expect(proxmoxBackupsTableSource).toContain('PlatformTableLoadingState');
    expect(proxmoxBackupsTableSource).toContain('title="Could not load Proxmox backup inventory"');
    expect(proxmoxBackupsTableSource).toContain('title="Loading Proxmox backup inventory"');
    expect(proxmoxBackupsTableSource).not.toContain(
      'inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover',
    );
  });

  it('keeps PBS backup count and uptime cells on shared platform primitives', () => {
    const directLocaleCountCall = 'row.backupCount.' + 'toLocale' + 'String()';
    const directCpuPercentRound = 'Math.round(row.cpuPercent ?? 0)}' + '%';
    const directMemoryPercentRound = 'Math.round(row.memoryPercent ?? 0)}' + '%';
    const directDatastorePercentRound = 'Math.round(pct() ?? 0)}' + '%';
    const directUptimeCall = 'formatUptime(row.uptimeSeconds ?? 0)';

    expect(proxmoxBackupServersTableSource).toContain('PlatformTableNumberValue');
    expect(proxmoxBackupServersTableSource).toContain('formatPlatformTableIntegerValue');
    expect(proxmoxBackupServersTableSource).toContain('PlatformTablePercentValue');
    expect(proxmoxBackupServersTableSource).toContain('formatPlatformTablePercentValue');
    expect(proxmoxBackupServersTableSource).toContain('formatPlatformTableUptimeValue');
    expect(proxmoxBackupServersTableSource).not.toContain(directLocaleCountCall);
    expect(proxmoxBackupServersTableSource).not.toContain(directCpuPercentRound);
    expect(proxmoxBackupServersTableSource).not.toContain(directMemoryPercentRound);
    expect(proxmoxBackupServersTableSource).not.toContain(directDatastorePercentRound);
    expect(proxmoxBackupServersTableSource).not.toContain(directUptimeCall);
  });

  it('keeps backup coverage fed by Proxmox VM/LXC guests when Overview demotes app containers', () => {
    expect(proxmoxPageSurfaceSource).toContain(
      'excludedWorkloadTypes: PROXMOX_WORKLOAD_EXCLUDED_TYPES',
    );
    expect(proxmoxPageSurfaceSource).toContain('showNestedExcludedWorkloads: true');
    expect(proxmoxPageSurfaceSource).toContain(
      'excludedWorkloadTypes={PROXMOX_WORKLOAD_EXCLUDED_TYPES}',
    );
    expect(proxmoxPageSurfaceSource).toContain('showNestedExcludedWorkloads');
    expect(proxmoxPageSurfaceSource).toContain('workloads={model().guests}');
    expect(proxmoxPageSurfaceSource).not.toContain('workloads={workloadsState.allGuests');
  });

  it('keeps Patrol coverage out of Proxmox evidence surfaces', () => {
    expect(proxmoxPageSurfaceSource).not.toContain('getMonitorContextPatrolProtectionPosture');
    expect(proxmoxPageSurfaceSource).not.toContain('getPatrolRunHistory(1)');
    expect(proxmoxPageSurfaceSource).not.toContain('aria-label="Proxmox Patrol coverage"');
    expect(proxmoxPageSurfaceSource).not.toContain('aria-label="Patrol protection posture"');
    expect(proxmoxBackupsTableSource).not.toContain('Proxmox Patrol coverage');
  });
});
