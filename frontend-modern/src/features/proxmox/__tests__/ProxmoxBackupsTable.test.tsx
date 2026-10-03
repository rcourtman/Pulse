import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal, type JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import { buildBackupServerRows } from '../ProxmoxBackupServersTable';
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
import { eventBus } from '@/stores/events';
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
  it('uses a corroborated PBS host link for Backups History despite a PVE-only name collision', () => {
    const pbs = {
      ...pbsServerResource,
      name: 'backup-connection',
      displayName: 'Backup connection',
      metricsTarget: { resourceType: 'agent', resourceId: 'pbs-service' },
      pbs: {
        ...pbsServerResource.pbs!,
        hostname: '10.0.0.5',
        nodeName: undefined,
        linkedAgentId: 'agent-uuid',
      },
    } as Resource;
    const host = {
      ...workloadResource,
      id: 'host-merged-with-pve',
      type: 'agent',
      name: 'different-hostname',
      sources: ['proxmox', 'agent'],
      agent: { agentId: 'agent-uuid', hostname: 'different-hostname' },
      metricsTarget: { resourceType: 'agent', resourceId: 'agent-uuid' },
    } as Resource;
    const pveOnly = {
      ...host,
      id: 'pve-only',
      name: 'backup-connection',
      agent: undefined,
      sources: ['proxmox'],
      metricsTarget: { resourceType: 'agent', resourceId: 'pve-only' },
    } as Resource;

    const row = buildBackupServerRows([pbs, pveOnly, host])[0];
    expect(row.resource.id).toBe(pbs.id);
    expect(row.resource.pbs?.linkedAgentId).toBe('agent-uuid');
    expect(row.resource.metricsTarget).toEqual({ resourceType: 'agent', resourceId: 'agent-uuid' });
    expect(row.resource.agent?.agentId).toBe('agent-uuid');
  });

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
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pbs', {
      signal: expect.any(AbortSignal),
    });
    expect(apiFetchMock).toHaveBeenCalledWith('/api/backups/pve', {
      signal: expect.any(AbortSignal),
    });
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

  it('keeps coverage evidence expanded across repeated workload snapshots', async () => {
    mockBackupAPIs();
    const [workloads, setWorkloads] = createSignal<readonly Resource[]>([workloadResource]);
    renderInRouter(() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={workloads()} />);

    await screen.findAllByText('pbs-docker');
    await fireEvent.click(screen.getByRole('link', { name: /coverage/i }));
    await fireEvent.click(screen.getByRole('button', { name: /expand details for pbs-docker/i }));

    const toggle = screen.getByRole('button', { name: /collapse details for pbs-docker/i });
    toggle.focus();

    for (let snapshot = 1; snapshot <= 3; snapshot += 1) {
      const name = `pbs-docker-snapshot-${snapshot}`;
      setWorkloads([{ ...workloadResource, name, displayName: name }]);

      // Assert the new snapshot reached the rendered table, rather than merely
      // checking that a stale expanded row survived.
      await screen.findAllByText(name);
      expect(screen.getByRole('button', { name: /collapse details for pbs-docker/i })).toBe(toggle);
      expect(toggle).toHaveFocus();
      expect(screen.getByRole('columnheader', { name: /posture/i })).toBeInTheDocument();
      expect(screen.getByText('Restore evidence')).toBeInTheDocument();
      expect(screen.getAllByText('PVE file').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
    }
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
    // A single agent can surface as both a PVE guest and a standalone host row;
    // collapse those by agent identity, and keep declining genuinely ambiguous
    // matches.
    expect(proxmoxBackupServersTableSource).toContain('correlatedAgentKey');
    expect(proxmoxBackupServersTableSource).toContain(
      'if (byAgentKey.size !== 1) return undefined;',
    );
    expect(proxmoxBackupServersTableSource).toContain(
      'metricsTarget: agent.metricsTarget ?? server.metricsTarget',
    );
    // A refresh that briefly omits the correlated host row must not flip the
    // drawer target to the PBS service key; retain the resolved host per server
    // and reuse it only across the omission.
    expect(proxmoxBackupServersTableSource).toContain('createPbsCorrelationRetention');
    expect(proxmoxBackupServersTableSource).toContain('hasCorrelationCandidate');
    expect(proxmoxBackupServersTableSource).toContain(
      'buildBackupServerRows(props.servers, props.backups ?? [], retention)',
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

const emptyPVE = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
const emptyPBS = { data: { backups: [] } };
const readPvePayload = { data: { ...pvePayload.data, guestSnapshots: [] } };
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => (resolve = res));
  return { promise, resolve };
};
const mountBackupReadState = () =>
  renderInRouter(() => (
    <ProxmoxBackupsTable
      emptyIcon={<span />}
      workloads={[workloadResource]}
      servers={[pbsServerResource]}
    />
  ));
const recoverableRows = () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]');
const serverTable = () => document.querySelector('[data-proxmox-backups-table="servers"]')!;
const sourceCalls = (source: 'pve' | 'pbs') =>
  apiFetchMock.mock.calls.filter(([url]) => url === `/api/backups/${source}`);

describe('independent Proxmox backup inventory reads', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/proxmox/backups/date');
    apiFetchJSONMock.mockResolvedValue({ data: [], policy: {}, meta: {} });
  });
  it('does not present an unread PBS inventory as empty or a measured zero', async () => {
    const pending = deferred<Response>();
    apiFetchMock.mockImplementation((url) =>
      url.endsWith('/pbs') ? pending.promise : Promise.resolve(jsonResponse(emptyPVE)),
    );
    mountBackupReadState();
    await screen.findByText(/PBS backup inventory is loading/);
    expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
    expect(screen.getByText('Backup inventory is incomplete')).toBeInTheDocument();
    expect(serverTable()).toHaveTextContent('Loading');
    pending.resolve(jsonResponse(emptyPBS));
    await screen.findByText('No backups yet');
    expect(serverTable()).not.toHaveTextContent('Loading');
    expect(
      within(serverTable() as HTMLElement).getByText('0', { exact: true }),
    ).toBeInTheDocument();
  });

  it('shows fulfilled PBS evidence without waiting for PVE', async () => {
    const pending = deferred<Response>();
    apiFetchMock.mockImplementation((url) =>
      url.endsWith('/pve') ? pending.promise : Promise.resolve(jsonResponse(pbsPayload)),
    );
    mountBackupReadState();
    await screen.findByText(/PVE backup inventory is loading/);
    expect(recoverableRows()).toHaveLength(1);
    expect(screen.getByText(/1 restore points read/)).toBeInTheDocument();
    pending.resolve(jsonResponse(readPvePayload));
    await waitFor(() => expect(recoverableRows()).toHaveLength(2));
    expect(screen.queryByText(/inventory is loading/)).not.toBeInTheDocument();
  });

  it('keeps a source failure actionable even before the other read settles', async () => {
    const pending = deferred<Response>();
    apiFetchMock.mockImplementation((url) =>
      url.endsWith('/pve') ? pending.promise : Promise.resolve(new Response('{}', { status: 503 })),
    );
    mountBackupReadState();
    await screen.findByText(/PBS backup inventory is unavailable/);
    expect(screen.getByText(/PVE backup inventory is loading/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry PBS inventory' })).toBeEnabled();
    expect(screen.getByText('Backup inventory is incomplete')).toBeInTheDocument();
    expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
    pending.resolve(jsonResponse(readPvePayload));
    await waitFor(() => expect(recoverableRows()).toHaveLength(1));
  });

  it.each(['pve', 'pbs'] as const)(
    'contains a %s failure, preserves the other source and retries only the failed source',
    async (source) => {
      let response = Promise.resolve(new Response('{}', { status: 503 }));
      apiFetchMock.mockImplementation((url) =>
        url.endsWith(`/${source}`)
          ? response
          : Promise.resolve(jsonResponse(source === 'pve' ? pbsPayload : readPvePayload)),
      );
      mountBackupReadState();
      await screen.findByText(
        new RegExp(`${source.toUpperCase()} backup inventory is unavailable`),
      );
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      // A failed source can now be actionable before the independent read
      // settles. Wait for that read's evidence, not the failure notice.
      await waitFor(() => expect(recoverableRows()).toHaveLength(1));
      if (source === 'pbs') expect(serverTable()).toHaveTextContent('Unavailable');
      const retry = screen.getByRole('button', { name: `Retry ${source.toUpperCase()} inventory` });
      const pending = deferred<Response>();
      response = pending.promise;
      fireEvent.click(retry);
      expect(retry).toBeDisabled();
      fireEvent.click(retry);
      expect(sourceCalls(source)).toHaveLength(2);
      expect(recoverableRows()).toHaveLength(1);
      expect(screen.getByText(/inventory is unavailable/)).toBeInTheDocument();
      pending.resolve(jsonResponse(source === 'pbs' ? pbsPayload : readPvePayload));
      await waitFor(() => expect(recoverableRows()).toHaveLength(2));
      expect(screen.queryByText(/inventory is unavailable/)).not.toBeInTheDocument();
      expect(sourceCalls(source === 'pbs' ? 'pve' : 'pbs')).toHaveLength(1);
    },
  );

  it('retries both failed inventories and allows the first recovered source to render', async () => {
    apiFetchMock.mockResolvedValue(new Response('{}', { status: 500 }));
    mountBackupReadState();
    await screen.findByText('Could not load Proxmox backup inventory');
    expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
    const pending = deferred<Response>();
    apiFetchMock.mockImplementation((url) =>
      url.endsWith('/pve') ? pending.promise : Promise.resolve(jsonResponse(pbsPayload)),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    await screen.findByText(/PVE backup inventory is unavailable/);
    expect(recoverableRows()).toHaveLength(1);
    pending.resolve(jsonResponse(readPvePayload));
    await waitFor(() => expect(recoverableRows()).toHaveLength(2));
    expect(sourceCalls('pve')).toHaveLength(2);
    expect(sourceCalls('pbs')).toHaveLength(2);
  });

  it('does not turn a rejected empty source into proof that no backups exist', async () => {
    apiFetchMock.mockImplementation((url) =>
      Promise.resolve(
        url.endsWith('/pbs') ? new Response('{}', { status: 403 }) : jsonResponse(emptyPVE),
      ),
    );
    mountBackupReadState();
    await screen.findByText(/PBS backup inventory is unavailable/);
    expect(screen.getByText('Backup inventory is incomplete')).toBeInTheDocument();
    expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
    expect(serverTable()).toHaveTextContent('Unavailable');
    expect(screen.getByText(/Access denied/)).toBeInTheDocument();
  });

  it.each([401, 403])(
    'withdraws the old organisation before a new %s PBS denial',
    async (status) => {
      apiFetchMock.mockImplementation((url) =>
        Promise.resolve(jsonResponse(url.endsWith('/pbs') ? pbsPayload : readPvePayload)),
      );
      mountBackupReadState();
      await waitFor(() => expect(recoverableRows()).toHaveLength(2));
      const pending = deferred<Response>();
      apiFetchMock.mockImplementation((url) =>
        url.endsWith('/pbs') ? pending.promise : Promise.resolve(jsonResponse(emptyPVE)),
      );
      eventBus.emit('org_switched', 'other-fixture-org');
      expect(recoverableRows()).toHaveLength(0);
      await screen.findByText(/PBS backup inventory is loading/);
      expect(screen.queryByText('No backups yet')).not.toBeInTheDocument();
      pending.resolve(new Response('{}', { status }));
      await screen.findByText(/PBS backup inventory is unavailable/);
      expect(recoverableRows()).toHaveLength(0);
      expect(serverTable()).toHaveTextContent('Unavailable');
    },
  );

  it('aborts replaced org reads and ignores a late earlier response', async () => {
    const old = deferred<Response>();
    apiFetchMock.mockImplementation((url) =>
      url.endsWith('/pbs') ? old.promise : Promise.resolve(jsonResponse(emptyPVE)),
    );
    mountBackupReadState();
    await waitFor(() => expect(sourceCalls('pbs')).toHaveLength(1));
    const oldSignal = sourceCalls('pbs')[0][1].signal as AbortSignal;
    apiFetchMock.mockImplementation((url) =>
      Promise.resolve(jsonResponse(url.endsWith('/pbs') ? emptyPBS : emptyPVE)),
    );
    eventBus.emit('org_switched', 'other-fixture-org');
    expect(oldSignal.aborted).toBe(true);
    await screen.findByText('No backups yet');
    old.resolve(jsonResponse(pbsPayload));
    await old.promise;
    await waitFor(() => expect(recoverableRows()).toHaveLength(0));
    expect(
      within(serverTable() as HTMLElement).getByText('0', { exact: true }),
    ).toBeInTheDocument();
  });

  it('aborts pending reads on disposal and leaves no org-switch readers', async () => {
    const pending = deferred<Response>();
    apiFetchMock.mockReturnValue(pending.promise);
    const view = mountBackupReadState();
    await waitFor(() => expect(apiFetchMock).toHaveBeenCalledTimes(2));
    const signals = apiFetchMock.mock.calls.map(([, options]) => options.signal as AbortSignal);
    view.unmount();
    expect(signals.every((signal) => signal.aborted)).toBe(true);
    eventBus.emit('org_switched', 'after-disposal');
    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    pending.resolve(jsonResponse(emptyPVE));
  });
});
