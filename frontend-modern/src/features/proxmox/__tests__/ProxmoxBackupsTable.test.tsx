import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';

// ProxmoxBackupsTable reads URL search params (node/type scope filters), so it
// must render inside a Router context.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

const apiFetchMock = vi.hoisted(() => vi.fn());
const apiFetchJSONMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
  apiFetchJSON: apiFetchJSONMock,
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

function mockBackupAPIs(
  state: 'protected' | 'attention' = 'protected',
  pbsResponse: typeof pbsPayload = pbsPayload,
) {
  apiFetchMock.mockImplementation((url: string) => {
    if (url === '/api/backups/pbs') return Promise.resolve(jsonResponse(pbsResponse));
    if (url === '/api/backups/pve') return Promise.resolve(jsonResponse(pvePayload));
    return Promise.resolve(jsonResponse({}));
  });
  apiFetchJSONMock.mockResolvedValue({
    data: [
      {
        subjectResourceId: 'ct-112',
        state,
        lastAttemptAt: '2026-05-25T02:00:00Z',
        lastSuccessfulPointAt: '2026-05-25T01:34:25Z',
        lastVerifiedAt: '2026-05-25T01:34:25Z',
        freshness: 'current',
        verification: 'verified',
        coverage: 'complete',
        providerStates: [],
        repositoryResourceIds: [],
        evidenceIds: ['evidence-1'],
        explanation:
          state === 'protected'
            ? 'A current verified backup is available.'
            : 'The latest provider job needs attention.',
        evaluatedAt: '2026-05-25T02:05:00Z',
      },
    ],
    policy: {
      freshnessWindowSeconds: 604800,
      verificationWindowSeconds: 604800,
      requireVerification: true,
    },
    meta: { page: 1, limit: 200, total: 1, totalPages: 1 },
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
  vi.unstubAllGlobals();
  window.history.replaceState({}, '', '/');
  apiFetchMock.mockReset();
  apiFetchJSONMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
});

beforeEach(() => {
  vi.stubGlobal('scrollTo', vi.fn());
});

describe('ProxmoxBackupsTable', () => {
  it('shows Checking until the canonical posture request resolves', async () => {
    mockBackupAPIs();
    let resolvePosture: ((value: unknown) => void) | undefined;
    apiFetchJSONMock.mockReturnValue(
      new Promise((resolve) => {
        resolvePosture = resolve;
      }),
    );
    window.history.replaceState({}, '', '/?view=coverage');

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');
    expect(screen.getAllByText('Checking').length).toBeGreaterThan(0);

    resolvePosture?.({
      data: [
        {
          subjectResourceId: 'ct-112',
          state: 'protected',
          freshness: 'current',
          verification: 'verified',
          coverage: 'complete',
          providerStates: [],
          repositoryResourceIds: [],
          evidenceIds: ['evidence-1'],
          explanation: 'A current verified backup is available.',
          evaluatedAt: '2026-05-25T02:05:00Z',
        },
      ],
      policy: {
        freshnessWindowSeconds: 604800,
        verificationWindowSeconds: 604800,
        requireVerification: true,
      },
      meta: { page: 1, limit: 200, total: 1, totalPages: 1 },
    });

    await waitFor(() =>
      expect(
        screen
          .getAllByTitle('A current verified backup is available.')
          .some((element) => element.textContent === 'Protected'),
      ).toBe(true),
    );
    expect(screen.queryByText('Checking')).not.toBeInTheDocument();
  });

  it('defaults to coverage when protection needs attention and keeps the dated feed one click away', async () => {
    mockBackupAPIs('attention');

    renderInRouter(() => (
      <ProxmoxBackupsTable
        emptyIcon={<span />}
        workloads={[workloadResource]}
        servers={[pbsServerResource]}
      />
    ));

    await screen.findAllByText('pbs-docker');
    expect(screen.getByRole('link', { name: /coverage/i })).toHaveAttribute('aria-current', 'page');

    await fireEvent.click(screen.getByRole('link', { name: /by date/i }));

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
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    const postureURL = new URL(apiFetchJSONMock.mock.calls[0][0], 'https://pulse.invalid');
    expect(postureURL.pathname).toBe('/api/recovery/postures');
    expect(postureURL.searchParams.getAll('resourceId')).toEqual(['ct-112']);
  });

  it('offers By date / Coverage views and no legacy sub-tab tree', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');

    const healthSummary = screen.getByText(/targets · .*restore points/);
    expect(healthSummary.parentElement).toHaveClass('w-full', 'sm:ml-auto', 'sm:w-auto');
    expect(healthSummary.parentElement).not.toHaveClass('ml-auto');

    expect(proxmoxBackupsTableSource).toContain('<PlatformSectionTabs');
    expect(proxmoxBackupsTableSource).not.toContain('FilterSegmentedControl');
    expect(proxmoxBackupsTableSource).toContain('buildProxmoxBackupsPath');
    expect(proxmoxBackupsTableSource).toContain('trailingControls={');
    expect(proxmoxBackupsTableSource).toContain('<PlatformResourceCounter');
    expect(proxmoxBackupServersTableSource).toContain(
      '<PlatformResponsiveTableLabel\n                    compact="Bkps"',
    );
    expect(proxmoxBackupServersTableSource).not.toContain('compact="#"');

    // The two route-backed sections exist...
    expect(screen.getByRole('navigation', { name: /backup views/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /by date/i })).toHaveAttribute(
      'href',
      '/proxmox/backups/date',
    );
    expect(screen.getByRole('link', { name: /coverage/i })).toHaveAttribute(
      'href',
      '/proxmox/backups/coverage',
    );
    // ...and the old four-tab + sub-tab tree does not.
    expect(screen.queryByRole('button', { name: /source details/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /job history/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /pbs artifacts/i })).not.toBeInTheDocument();
  });

  it('hydrates the complete saved-view filter state from the URL and clears it atomically', async () => {
    mockBackupAPIs();
    window.history.replaceState(
      {},
      '',
      '/?view=date&q=pbs-docker&source=pbs&location=pbs%3Apbs-main%3Amain&node=pve-a&type=ct&day=2026-05-25',
    );

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');
    expect(screen.getByRole('link', { name: /by date/i })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByPlaceholderText(/search backups by workload/i)).toHaveValue('pbs-docker');
    expect(screen.getByRole('button', { name: /pbs snapshots/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: /clear date filter/i })).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Backup location: pbs-main / main' }),
    ).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: /clear filters/i }));

    await waitFor(() => {
      const params = new URLSearchParams(window.location.search);
      expect(params.get('view')).toBeNull();
      expect(params.get('q')).toBeNull();
      expect(params.get('source')).toBeNull();
      expect(params.get('location')).toBeNull();
      expect(params.get('node')).toBeNull();
      expect(params.get('type')).toBeNull();
      expect(params.get('day')).toBeNull();
    });
    expect(screen.getByPlaceholderText(/search backups by workload/i)).toHaveValue('');
  });

  it('filters restore points and coverage by PBS server and datastore', async () => {
    const offsitePayload = {
      ...pbsPayload,
      data: {
        backups: [
          ...pbsPayload.data.backups,
          {
            ...pbsPayload.data.backups[0],
            id: 'pbs-offsite/offsite/minipc/ct/112/2026-05-24T01:34:25Z',
            instance: 'pbs-offsite',
            datastore: 'offsite',
            backupTime: '2026-05-24T01:34:25Z',
          },
        ],
      },
    };
    mockBackupAPIs('protected', offsitePayload);

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findByText('offsite / minipc');
    const filterSelect = screen.getByRole('combobox', { name: 'Filter' });
    const offsiteOption = screen.getByRole('option', {
      name: 'Backup location: pbs-offsite / offsite',
    }) as HTMLOptionElement;
    await fireEvent.change(filterSelect, { target: { value: offsiteOption.value } });

    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get('location')).toBe(
        'pbs:pbs-offsite:offsite',
      ),
    );
    expect(screen.getByText('offsite / minipc')).toBeInTheDocument();
    expect(screen.queryByText('main / minipc')).not.toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Backup location: pbs-offsite / offsite' }),
    ).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('link', { name: /coverage/i }));
    await waitFor(() => expect(window.location.pathname).toBe('/proxmox/backups/coverage'));
    expect(new URLSearchParams(window.location.search).get('location')).toBe(
      'pbs:pbs-offsite:offsite',
    );
    expect(screen.getAllByText('pbs-docker').length).toBeGreaterThan(0);
    await fireEvent.click(screen.getByRole('button', { name: /expand details for pbs-docker/i }));
    expect(screen.getByText('offsite / minipc')).toBeInTheDocument();
    expect(screen.queryByText('main / minipc')).not.toBeInTheDocument();
  });

  it('clears incompatible URL facets when switching backup views', async () => {
    mockBackupAPIs();
    window.history.replaceState({}, '', '/?view=date&source=pbs&day=2026-05-25');

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');
    await fireEvent.click(screen.getByRole('link', { name: /coverage/i }));

    await waitFor(() => {
      const params = new URLSearchParams(window.location.search);
      expect(window.location.pathname).toBe('/proxmox/backups/coverage');
      expect(params.get('view')).toBeNull();
      expect(params.get('source')).toBeNull();
      expect(params.get('day')).toBeNull();
    });

    await fireEvent.click(screen.getByRole('button', { name: /^protected$/i }));
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get('posture')).toBe('protected'),
    );

    await fireEvent.click(screen.getByRole('link', { name: /by date/i }));
    await waitFor(() => {
      const params = new URLSearchParams(window.location.search);
      expect(window.location.pathname).toBe('/proxmox/backups/date');
      expect(params.get('view')).toBeNull();
      expect(params.get('posture')).toBeNull();
    });
  });

  it('switches to Coverage showing posture, and keeps per-source evidence in the row expansion', async () => {
    mockBackupAPIs();

    renderInRouter(() => (
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[workloadResource]} />
    ));

    await screen.findAllByText('pbs-docker');
    await fireEvent.click(screen.getByRole('link', { name: /coverage/i }));

    // Coverage is the posture view; the server-owned workload posture reads
    // "Protected".
    expect(screen.getByRole('columnheader', { name: /posture/i })).toBeInTheDocument();
    expect(screen.getAllByText('Protected').length).toBeGreaterThan(0);

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
    await fireEvent.click(screen.getByRole('link', { name: /by date/i }));

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

  it('routes PBS server expansion through the canonical resource drawer', () => {
    expect(proxmoxBackupServersTableSource).toContain('PlatformResourceDetailTableRow');
    expect(proxmoxBackupServersTableSource).toContain('resource={row.resource}');
    expect(proxmoxBackupServersTableSource).toContain('initialShowHostDetails');
    expect(proxmoxBackupServersTableSource).toContain('uniquelyCorrelatedAgent');
    expect(proxmoxBackupServersTableSource).toContain(
      'return matches.length === 1 ? matches[0] : undefined;',
    );
    expect(proxmoxBackupServersTableSource).toContain(
      'metricsTarget: agent.metricsTarget ?? server.metricsTarget',
    );
    expect(proxmoxBackupServersTableSource).not.toContain(
      '<span class="font-medium text-base-content">Server:</span>',
    );
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

  it('keeps Overview guest totals aligned with the filtered Workloads collection', () => {
    // Guest totals now render as the shared toolbar inventory counts, fed by
    // the workloads state whose collection already excludes demoted app
    // containers. They must never come from the raw model summary counts.
    expect(proxmoxPageSurfaceSource).toContain('inventoryStats={workloadsState.inventoryStats}');
    expect(proxmoxPageSurfaceSource).not.toContain(
      'currentModel().summary.runningGuestCount} running',
    );
    expect(proxmoxPageSurfaceSource).not.toContain(
      'currentModel().summary.stoppedGuestCount} stopped',
    );
  });

  it('keeps the overview node and guest regions on one canonical resource snapshot', () => {
    expect(proxmoxPageSurfaceSource).toContain('const overviewResources = useUnifiedResources({');
    expect(proxmoxPageSurfaceSource).toContain("cacheKey: 'proxmox-overview'");
    expect(proxmoxPageSurfaceSource).toContain('resourceSnapshot={() =>');
    expect(proxmoxPageSurfaceSource).toContain(
      'resourceSnapshotRefetch={() => overviewResources.refetch()}',
    );
    expect(proxmoxPageSurfaceSource).toContain('useWorkloadsState({');
    expect(proxmoxPageSurfaceSource).toContain('resourceSnapshot: props.resourceSnapshot');
    expect(proxmoxPageSurfaceSource).not.toContain(
      'useWorkloads({ enabled: () => workloadsEnabled() })',
    );
  });

  it('keeps the shared storage surface scoped to the whole Proxmox product family', () => {
    expect(proxmoxPageSurfaceSource).toContain("const PROXMOX_PLATFORM_FILTER = 'proxmox-all';");
    expect(proxmoxPageSurfaceSource).toContain('forcedSourceFilter={PROXMOX_PLATFORM_FILTER}');
    expect(proxmoxPageSurfaceSource).not.toContain(
      "const PROXMOX_PLATFORM_FILTER = 'proxmox-pve';",
    );
  });

  it('keeps Patrol coverage out of Proxmox evidence surfaces', () => {
    expect(proxmoxPageSurfaceSource).not.toContain('getMonitorContextPatrolProtectionPosture');
    expect(proxmoxPageSurfaceSource).not.toContain('getPatrolRunHistory(1)');
    expect(proxmoxPageSurfaceSource).not.toContain('aria-label="Proxmox Patrol coverage"');
    expect(proxmoxPageSurfaceSource).not.toContain('aria-label="Patrol protection posture"');
    expect(proxmoxBackupsTableSource).not.toContain('Proxmox Patrol coverage');
  });
});
