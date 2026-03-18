import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Recovery from '@/components/Recovery/Recovery';
import { parseRecoveryDateKey } from '@/utils/recoveryDatePresentation';

let mockLocationSearch = '';
let mockLocationPath = '/recovery';
const navigateSpy = vi.hoisted(() => vi.fn());

const apiFetchMock = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockLocationPath, search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

const rollupsPayload = [
  {
    rollupId: 'res:vm-123',
    subjectResourceId: 'vm-123',
    lastAttemptAt: '2026-02-14T10:00:00.000Z',
    lastSuccessAt: '2026-02-14T10:00:00.000Z',
    lastOutcome: 'success',
    providers: ['proxmox-pve'],
  },
  {
    rollupId: 'ext:truenas-1',
    subjectRef: { type: 'truenas-dataset', name: 'tank/apps', id: 'tank/apps' },
    lastAttemptAt: '2026-02-13T09:00:00.000Z',
    lastSuccessAt: null,
    lastOutcome: 'failed',
    providers: ['truenas'],
  },
];

const pointsByRollupId: Record<string, any[]> = {
  'res:vm-123': [
    {
      id: 'p1',
      provider: 'proxmox-pve',
      kind: 'backup',
      mode: 'local',
      outcome: 'success',
      completedAt: '2026-02-14T10:00:00.000Z',
      sizeBytes: 1234,
    },
  ],
  'ext:truenas-1': [
    {
      id: 't1',
      provider: 'truenas',
      kind: 'snapshot',
      mode: 'snapshot',
      outcome: 'failed',
      completedAt: '2026-02-13T09:00:00.000Z',
      sizeBytes: 0,
    },
  ],
};

let facetsPayload: any;
type RecoveryPointsResponse = {
  data: any[];
  meta: { page: number; limit: number; total: number; totalPages: number };
};

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
  apiFetchJSON: apiFetchMock,
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useStorageRecoveryResources: () => ({
    resources: () => [{ id: 'vm-123', name: 'VM 123' }],
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

describe('Recovery', () => {
  beforeEach(() => {
    localStorage.clear();
    navigateSpy.mockReset();
    apiFetchMock.mockClear();
    mockLocationSearch = '';
    mockLocationPath = '/recovery';

    facetsPayload = {
      clusters: [],
      nodesAgents: [],
      namespaces: [],
      hasSize: true,
      hasVerification: false,
      hasEntityId: false,
    };

    apiFetchMock.mockImplementation(async (url: string) => {
      const u = new URL(url, 'http://localhost');
      if (u.pathname === '/api/recovery/rollups') {
        return {
          data: rollupsPayload,
          meta: { page: 1, limit: 500, total: rollupsPayload.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/points') {
        const rid = u.searchParams.get('rollupId') || '';
        const data = pointsByRollupId[rid] || [];
        return {
          data,
          meta: { page: 1, limit: 500, total: data.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/facets') {
        return {
          data: facetsPayload,
        };
      }
      if (u.pathname === '/api/recovery/series') {
        return {
          data: [
            { day: '2026-02-13', total: 1, snapshot: 1, local: 0, remote: 0 },
            { day: '2026-02-14', total: 1, snapshot: 0, local: 1, remote: 0 },
          ],
        };
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('renders protected rollups and resolves unified resource names', async () => {
    render(() => <Recovery />);

    expect(await screen.findByText('VM 123')).toBeInTheDocument();
    expect(screen.getByText('tank/apps')).toBeInTheDocument();
  });

  it('focuses history when a rollup is clicked', async () => {
    render(() => <Recovery />);

    const subject = await screen.findByText('VM 123');
    fireEvent.click(subject);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?rollupId=res%3Avm-123', {
        replace: true,
      });
    });

    expect(await screen.findByText('Focused')).toBeInTheDocument();
    expect(screen.getAllByText('VM 123').length).toBeGreaterThan(0);
    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);
    const tables = await screen.findAllByRole('table');
    const table = tables[tables.length - 1];
    expect(within(table).getAllByText('Local').length).toBeGreaterThan(0);
    expect(within(table).getAllByText('Success').length).toBeGreaterThan(0);
  });

  it('filters protected rollups by provider', async () => {
    render(() => <Recovery />);

    expect(await screen.findByText('VM 123')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'truenas' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?provider=truenas', { replace: true });
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
    });
    expect(screen.getByText('tank/apps')).toBeInTheDocument();

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('provider=truenas'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('provider=truenas'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('provider=truenas'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('provider=truenas'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('uses one canonical free-text query across protected items and history transport', async () => {
    render(() => <Recovery />);

    const protectedSearch = await screen.findByPlaceholderText('Search protected items...');
    fireEvent.input(protectedSearch, { target: { value: 'tank' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?q=tank', { replace: true });
    });

    await waitFor(() => {
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
      expect(screen.getByText('tank/apps')).toBeInTheDocument();
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('q=tank'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('q=tank'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('q=tank'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('q=tank'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('uses one canonical outcome filter across protected items and history transport', async () => {
    render(() => <Recovery />);

    const protectedStatus = await screen.findByLabelText('Latest status');
    fireEvent.change(protectedStatus, { target: { value: 'failed' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?status=failed', { replace: true });
    });

    await waitFor(() => {
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
      expect(screen.getByText('tank/apps')).toBeInTheDocument();
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('outcome=failed'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('outcome=failed'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('outcome=failed'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('outcome=failed'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('persists and restores the protected stale-only filter through the canonical recovery URL', async () => {
    mockLocationSearch = '?stale=%20TRUE%20';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?stale=1', { replace: true });
    });

    expect(screen.getByRole('button', { name: 'Stale only' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('normalizes legacy provider aliases from the URL into canonical history filters', async () => {
    mockLocationSearch = '?provider=proxmox';
    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?provider=proxmox-pve', {
        replace: true,
      });
    });
  });

  it('collapses unknown recovery provider values back to canonical unset state', async () => {
    mockLocationSearch = '?provider=%20custom-provider%20';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', { replace: true });
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const filteredUrls = urls.filter(
        (url) =>
          url.includes('/api/recovery/rollups') ||
          url.includes('/api/recovery/points') ||
          url.includes('/api/recovery/facets') ||
          url.includes('/api/recovery/series'),
      );
      expect(filteredUrls.some((url) => url.includes('provider=custom-provider'))).toBe(false);
    });
  });

  it('adds cluster to the URL and API queries when the cluster filter changes', async () => {
    facetsPayload.clusters = ['dev-cluster', 'prod-cluster'];

    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: /^filter$/i }));

    const clusterSelect = await screen.findByLabelText('Cluster');
    fireEvent.change(clusterSelect, { target: { value: 'dev-cluster' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?cluster=dev-cluster', {
        replace: true,
      });
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('cluster=dev-cluster'),
      );
      const hasPoints = urls.some(
        (url) => url.includes('/api/recovery/points') && url.includes('cluster=dev-cluster'),
      );
      const hasSeries = urls.some(
        (url) => url.includes('/api/recovery/series') && url.includes('cluster=dev-cluster'),
      );
      const hasFacets = urls.some(
        (url) => url.includes('/api/recovery/facets') && url.includes('cluster=dev-cluster'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('collapses explicit all recovery filters back to canonical route state', async () => {
    mockLocationSearch =
      '?provider=%20ALL%20&scope=%20ALL%20&mode=%20all%20&status=%20ALL%20&verification=%20All%20&cluster=%20ALL%20&node=%20all%20&namespace=%20All%20';

    render(() => <Recovery />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', { replace: true });
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const filteredUrls = urls.filter(
        (url) =>
          url.includes('/api/recovery/rollups') ||
          url.includes('/api/recovery/points') ||
          url.includes('/api/recovery/facets') ||
          url.includes('/api/recovery/series'),
      );
      expect(
        filteredUrls.some(
          (url) =>
            url.includes('provider=all') ||
            url.includes('provider=ALL') ||
            url.includes('scope=all') ||
            url.includes('scope=ALL') ||
            url.includes('mode=all') ||
            url.includes('mode=ALL') ||
            url.includes('status=all') ||
            url.includes('status=ALL') ||
            url.includes('verification=all') ||
            url.includes('verification=ALL') ||
            url.includes('cluster=all') ||
            url.includes('cluster=ALL') ||
            url.includes('node=all') ||
            url.includes('node=ALL') ||
            url.includes('namespace=all') ||
            url.includes('namespace=All'),
        ),
      ).toBe(false);
    });
  });

  it('keeps facets aligned with node and namespace history filters', async () => {
    facetsPayload.nodesAgents = ['node-agent-1', 'node-agent-2'];
    facetsPayload.namespaces = ['tenant-a', 'tenant-b'];

    render(() => <Recovery />);

    fireEvent.click(await screen.findByRole('button', { name: /^filter$/i }));

    fireEvent.change(await screen.findByLabelText('Node or agent'), {
      target: { value: 'node-agent-1' },
    });
    fireEvent.change(await screen.findByLabelText('Namespace'), {
      target: { value: 'tenant-a' },
    });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        '/recovery?namespace=tenant-a&node=node-agent-1',
        { replace: true },
      );
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasRollups = urls.some(
        (url) =>
          url.includes('/api/recovery/rollups') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      const hasPoints = urls.some(
        (url) =>
          url.includes('/api/recovery/points') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      const hasSeries = urls.some(
        (url) =>
          url.includes('/api/recovery/series') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      const hasFacets = urls.some(
        (url) =>
          url.includes('/api/recovery/facets') &&
          url.includes('node=node-agent-1') &&
          url.includes('namespace=tenant-a'),
      );
      expect(hasRollups && hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });

  it('keeps the history card mounted while filter refetches are in flight', async () => {
    mockLocationSearch = '?rollupId=res%3Avm-123';
    facetsPayload.clusters = ['dev-cluster'];

    let delayedPointsReady = false;
    let resolveDelayedPoints!: (value: RecoveryPointsResponse) => void;

    apiFetchMock.mockImplementation(async (url: string) => {
      const u = new URL(url, 'http://localhost');
      if (u.pathname === '/api/recovery/rollups') {
        return {
          data: rollupsPayload,
          meta: { page: 1, limit: 500, total: rollupsPayload.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/points') {
        const rid = u.searchParams.get('rollupId') || '';
        const data = pointsByRollupId[rid] || [];
        if (u.searchParams.get('cluster') === 'dev-cluster') {
          return await new Promise<RecoveryPointsResponse>((resolve) => {
            delayedPointsReady = true;
            resolveDelayedPoints = resolve as (value: RecoveryPointsResponse) => void;
          });
        }
        return {
          data,
          meta: { page: 1, limit: 500, total: data.length, totalPages: 1 },
        };
      }
      if (u.pathname === '/api/recovery/facets') {
        return {
          data: facetsPayload,
        };
      }
      if (u.pathname === '/api/recovery/series') {
        return {
          data: [
            { day: '2026-02-13', total: 1, snapshot: 1, local: 0, remote: 0 },
            { day: '2026-02-14', total: 1, snapshot: 0, local: 1, remote: 0 },
          ],
        };
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    render(() => <Recovery />);

    await screen.findByText(/Showing 1 - 1 of 1 recovery points/i);

    fireEvent.click(screen.getByRole('button', { name: /^filter$/i }));
    fireEvent.change(await screen.findByLabelText('Cluster'), {
      target: { value: 'dev-cluster' },
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      expect(
        urls.some(
          (url) => url.includes('/api/recovery/points') && url.includes('cluster=dev-cluster'),
        ),
      ).toBe(true);
    });

    expect(screen.getByText('Backups By Date')).toBeInTheDocument();
    expect(screen.getByText(/Showing 1 - 1 of 1 recovery points/i)).toBeInTheDocument();

    if (!delayedPointsReady) {
      throw new Error('Expected delayed recovery points request to be pending');
    }
    resolveDelayedPoints({
      data: pointsByRollupId['res:vm-123'],
      meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
    });

    await waitFor(() => {
      expect(screen.getByText(/Showing 1 - 1 of 1 recovery points/i)).toBeInTheDocument();
    });
  });

  it('narrows recovery point history to the selected timeline day', async () => {
    render(() => <Recovery />);

    await screen.findByText('Backups By Date');
    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      expect(pointUrls.length).toBeGreaterThan(0);
    });

    const initialPointUrls = apiFetchMock.mock.calls
      .map((call) => String(call[0] || ''))
      .filter((url) => url.includes('/api/recovery/points'));
    const initialPointsUrl = initialPointUrls[initialPointUrls.length - 1];

    const timelineButtons = await screen.findAllByRole('button', { name: /recovery points/i });
    fireEvent.click(timelineButtons[0]);

    const selectedDay = '2026-02-13';
    const selectedStart = parseRecoveryDateKey(selectedDay);
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).not.toBe(initialPointsUrl);
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });

    await waitFor(() => {
      const facetUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/facets'));
      const latestFacetsUrl = facetUrls[facetUrls.length - 1];
      expect(latestFacetsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestFacetsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith(
        expect.stringContaining('day=2026-02-13'),
        { replace: true },
      );
    });
  });

  it('restores selected timeline day from the recovery URL', async () => {
    mockLocationSearch = '?day=2026-02-13';

    render(() => <Recovery />);

    await screen.findByText('Backups By Date');

    const selectedStart = parseRecoveryDateKey('2026-02-13');
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });
  });

  it('canonicalizes whitespace-padded timeline day and range from the recovery URL', async () => {
    mockLocationSearch = '?day=%202026-02-13%20&range=%207%20';

    render(() => <Recovery />);

    await screen.findByText('Backups By Date');

    const selectedStart = parseRecoveryDateKey('2026-02-13');
    selectedStart.setHours(0, 0, 0, 0);
    const selectedEnd = new Date(selectedStart);
    selectedEnd.setHours(23, 59, 59, 999);

    await waitFor(() => {
      const pointUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/points'));
      const latestPointsUrl = pointUrls[pointUrls.length - 1];
      expect(latestPointsUrl).toContain(`from=${encodeURIComponent(selectedStart.toISOString())}`);
      expect(latestPointsUrl).toContain(`to=${encodeURIComponent(selectedEnd.toISOString())}`);
    });

    await waitFor(() => {
      const rollupUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/rollups'));
      const latestRollupsUrl = rollupUrls[rollupUrls.length - 1];
      expect(latestRollupsUrl).toContain('from=');
      expect(latestRollupsUrl).toContain('to=');
    });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?range=7&day=2026-02-13', {
        replace: true,
      });
    });
  });

  it('persists the selected timeline range in the recovery URL', async () => {
    render(() => <Recovery />);

    await screen.findByText('Backups By Date');

    fireEvent.click(await screen.findByRole('button', { name: '7d' }));

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery?range=7', { replace: true });
    });
  });

  it('restores the selected timeline range from the recovery URL', async () => {
    mockLocationSearch = '?range=7';

    render(() => <Recovery />);

    await screen.findByText('Backups By Date');

    const end = new Date();
    end.setHours(23, 59, 59, 999);
    const start = new Date(end);
    start.setDate(start.getDate() - 6);
    start.setHours(0, 0, 0, 0);

    await waitFor(() => {
      const rollupUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/rollups'));
      const seriesUrls = apiFetchMock.mock.calls
        .map((call) => String(call[0] || ''))
        .filter((url) => url.includes('/api/recovery/series'));
      const latestRollupsUrl = rollupUrls[rollupUrls.length - 1];
      const latestSeriesUrl = seriesUrls[seriesUrls.length - 1];
      expect(latestRollupsUrl).toContain(`from=${encodeURIComponent(start.toISOString())}`);
      expect(latestRollupsUrl).toContain(`to=${encodeURIComponent(end.toISOString())}`);
      expect(latestSeriesUrl).toContain(`from=${encodeURIComponent(start.toISOString())}`);
      expect(latestSeriesUrl).toContain(`to=${encodeURIComponent(end.toISOString())}`);
    });
  });

  it('clears the shared recovery search query on Escape', async () => {
    mockLocationSearch = '?q=gdfdgd';
    render(() => <Recovery />);

    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/recovery', { replace: true });
    });
  });
});
