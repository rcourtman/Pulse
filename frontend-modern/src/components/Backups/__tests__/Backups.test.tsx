import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Backups from '@/components/Backups/Backups';

let mockLocationSearch = '';
let mockLocationPath = '/backups';
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

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
  apiFetchJSON: apiFetchMock,
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useStorageBackupsResources: () => ({
    resources: () => [{ id: 'vm-123', name: 'VM 123' }],
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
    mutate: vi.fn(),
  }),
}));

describe('Backups', () => {
  beforeEach(() => {
    localStorage.clear();
    navigateSpy.mockReset();
    apiFetchMock.mockClear();
    mockLocationSearch = '';
    mockLocationPath = '/backups';

    facetsPayload = {
      clusters: [],
      nodesHosts: [],
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
    mockLocationSearch = '?view=protected';
    render(() => <Backups />);

    expect(await screen.findByText('VM 123')).toBeInTheDocument();
    expect(screen.getByText('tank/apps')).toBeInTheDocument();
  });

  it('drills down into events when a rollup is clicked', async () => {
    mockLocationSearch = '?view=protected';
    render(() => <Backups />);

    const subject = await screen.findByText('VM 123');
    fireEvent.click(subject);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/backups?view=events&rollupId=res%3Avm-123', { replace: true });
    });

    expect(await screen.findByText('Backup events')).toBeInTheDocument();
    expect(await screen.findByText('VM 123')).toBeInTheDocument();
    await screen.findByText(/Showing 1 - 1 of 1 events/i);
    const table = await screen.findByRole('table');
    expect(within(table).getAllByText('Local').length).toBeGreaterThan(0);
    expect(within(table).getAllByText('Success').length).toBeGreaterThan(0);
  });

  it('filters protected rollups by provider', async () => {
    mockLocationSearch = '?view=protected';
    render(() => <Backups />);

    expect(await screen.findByText('VM 123')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'truenas' } });

    await waitFor(() => {
      expect(screen.queryByText('VM 123')).not.toBeInTheDocument();
    });
    expect(screen.getByText('tank/apps')).toBeInTheDocument();
  });

  it('adds cluster to the URL and API queries when the cluster filter changes', async () => {
    mockLocationSearch = '?view=events';
    facetsPayload.clusters = ['dev-cluster', 'prod-cluster'];

    render(() => <Backups />);

    fireEvent.click(await screen.findByRole('button', { name: /more filters/i }));

    const clusterSelect = await screen.findByLabelText('Cluster');
    fireEvent.change(clusterSelect, { target: { value: 'dev-cluster' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/backups?view=events&cluster=dev-cluster', { replace: true });
    });

    await waitFor(() => {
      const urls = apiFetchMock.mock.calls.map((call) => String(call[0] || ''));
      const hasPoints = urls.some((url) => url.includes('/api/recovery/points') && url.includes('cluster=dev-cluster'));
      const hasSeries = urls.some((url) => url.includes('/api/recovery/series') && url.includes('cluster=dev-cluster'));
      const hasFacets = urls.some((url) => url.includes('/api/recovery/facets') && url.includes('cluster=dev-cluster'));
      expect(hasPoints && hasSeries && hasFacets).toBe(true);
    });
  });
});
