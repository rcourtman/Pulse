import { createSignal } from 'solid-js';
import { render, waitFor, cleanup } from '@solidjs/testing-library';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { InfrastructureSummary } from './InfrastructureSummary';
import type { Resource } from '@/types/resource';
import type { TimeRange } from '@/api/charts';
import { __resetInfrastructureSummaryFetchesForTests } from '@/utils/infrastructureSummaryCache';

const mockGetCharts = vi.fn();
const INFRA_SUMMARY_CACHE_KEY_PREFIX = 'pulse.infrastructureSummaryCharts.';

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getInfrastructureSummaryCharts: (...args: unknown[]) => mockGetCharts(...args),
    },
  };
});

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    workloads: () => [],
  }),
}));

vi.mock('@/stores/websocket-global', () => ({
  getGlobalWebSocketStore: () => ({
    state: {
      hosts: [],
    },
  }),
}));

const makeHost = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'node-1',
  type: 'node',
  name: 'node-1',
  displayName: 'node-1',
  platformId: 'node-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  ...overrides,
});

const makeChartsResponse = (ids: string[] = ['node-1']) => ({
  nodeData: Object.fromEntries(
    ids.map((id, index) => [
      id,
      {
      cpu: [
        { timestamp: Date.now() - 60_000, value: 10 + index },
        { timestamp: Date.now(), value: 15 + index },
      ],
      memory: [
        { timestamp: Date.now() - 60_000, value: 45 + index },
        { timestamp: Date.now(), value: 50 + index },
      ],
      disk: [
        { timestamp: Date.now() - 60_000, value: 30 + index },
        { timestamp: Date.now(), value: 35 + index },
      ],
      },
    ]),
  ),
  dockerHostData: {},
  hostData: {},
  timestamp: Date.now(),
  stats: {
    oldestDataTimestamp: Date.now() - 60_000,
  },
});

const countSparklinePaths = (container: HTMLElement): number =>
  container.querySelectorAll('path[vector-effect="non-scaling-stroke"]').length;

describe('InfrastructureSummary range behavior', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    mockGetCharts.mockResolvedValue(makeChartsResponse());
    __resetInfrastructureSummaryFetchesForTests();
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('requests charts for initial and updated time ranges', async () => {
    const [range, setRange] = createSignal<TimeRange>('1h');
    render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange={range()} />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });

    setRange('24h');

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('24h');
    });
  });

  it('deduplicates concurrent summary fetches across component instances', async () => {
    let resolveFetch: ((value: ReturnType<typeof makeChartsResponse>) => void) | undefined;
    const pending = new Promise<ReturnType<typeof makeChartsResponse>>((resolve) => {
      resolveFetch = resolve;
    });
    mockGetCharts.mockReset();
    mockGetCharts.mockImplementation(() => pending);

    render(() => (
      <>
        <InfrastructureSummary hosts={[makeHost()]} timeRange="1h" />
        <InfrastructureSummary hosts={[makeHost()]} timeRange="1h" />
      </>
    ));

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledTimes(1);
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });

    resolveFetch?.(makeChartsResponse());

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledTimes(1);
    });
  });

  it('hydrates sparklines from cache immediately while live fetch is pending', async () => {
    const now = Date.now();
    const cachePayload = {
      version: 1,
      range: '1h',
      cachedAt: now,
      oldestDataTimestamp: now - 60_000,
      charts: {
        'node-1': {
          cpu: [
            { timestamp: now - 60_000, value: 20 },
            { timestamp: now, value: 25 },
          ],
          memory: [
            { timestamp: now - 60_000, value: 35 },
            { timestamp: now, value: 40 },
          ],
          disk: [
            { timestamp: now - 60_000, value: 45 },
            { timestamp: now, value: 50 },
          ],
          netin: [],
          netout: [],
        },
      },
    };
    localStorage.setItem(`${INFRA_SUMMARY_CACHE_KEY_PREFIX}1h`, JSON.stringify(cachePayload));

    mockGetCharts.mockReset();
    mockGetCharts.mockImplementationOnce(() => new Promise(() => {}));

    const { container } = render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange="1h" />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });

    await waitFor(() => {
      const path = container
        .querySelector('svg.cursor-crosshair')
        ?.querySelector('path[vector-effect="non-scaling-stroke"]');
      expect(path).toBeTruthy();
    });
  });

  it('clears cached-data status once live history is applied', async () => {
    const now = Date.now();
    const cachePayload = {
      version: 1,
      range: '1h',
      cachedAt: now,
      oldestDataTimestamp: now - 60_000,
      charts: {
        'node-1': {
          cpu: [
            { timestamp: now - 60_000, value: 20 },
            { timestamp: now, value: 25 },
          ],
          memory: [
            { timestamp: now - 60_000, value: 35 },
            { timestamp: now, value: 40 },
          ],
          disk: [
            { timestamp: now - 60_000, value: 45 },
            { timestamp: now, value: 50 },
          ],
          netin: [],
          netout: [],
        },
      },
    };
    localStorage.setItem(`${INFRA_SUMMARY_CACHE_KEY_PREFIX}1h`, JSON.stringify(cachePayload));

    let resolveFetch: ((value: ReturnType<typeof makeChartsResponse>) => void) | undefined;
    mockGetCharts.mockReset();
    mockGetCharts.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve as (value: ReturnType<typeof makeChartsResponse>) => void;
        }),
    );

    const { container } = render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange="1h" />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
      expect(container.querySelector('svg.cursor-crosshair')).toBeTruthy();
    });

    resolveFetch?.(makeChartsResponse());

    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeTruthy();
    });
  });

  it('does not render stale-range sparkline paths while new range data is loading', async () => {
    const firstResponse = makeChartsResponse();
    mockGetCharts.mockImplementationOnce(() => Promise.resolve(firstResponse));
    mockGetCharts.mockImplementationOnce(() => new Promise(() => {}));

    const [range, setRange] = createSignal<TimeRange>('1h');
    const { container } = render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange={range()} />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });

    await waitFor(() => {
      const sparklineSvg = container.querySelector('svg.cursor-crosshair');
      const sparklinePath = sparklineSvg?.querySelector('path[vector-effect="non-scaling-stroke"]');
      expect(sparklinePath).toBeTruthy();
    });

    setRange('24h');

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('24h');
    });

    expect(container.querySelector('svg.cursor-crosshair')).toBeNull();
    expect(container.textContent).toContain('Loading history...');
  });

  it('requests the new range while the previous range request is still pending', async () => {
    mockGetCharts.mockReset();
    mockGetCharts.mockImplementationOnce(() => new Promise(() => {}));
    mockGetCharts.mockImplementationOnce(() => Promise.resolve(makeChartsResponse()));

    const [range, setRange] = createSignal<TimeRange>('1h');
    render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange={range()} />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });

    setRange('24h');

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('24h');
    });
  });

  it('clears stale range data when a new range response is empty', async () => {
    mockGetCharts.mockReset();
    mockGetCharts.mockImplementationOnce(() => Promise.resolve(makeChartsResponse()));
    mockGetCharts.mockImplementationOnce(() =>
      Promise.resolve({
        nodeData: {},
        dockerHostData: {},
        hostData: {},
        timestamp: Date.now(),
        stats: {
          oldestDataTimestamp: Date.now() - 60_000,
        },
      })
    );

    const [range, setRange] = createSignal<TimeRange>('1h');
    const { container } = render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange={range()} />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });
    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeTruthy();
    });

    setRange('24h');

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('24h');
    });

    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeNull();
      expect(container.textContent).toContain('No history yet');
    });
  });

  it('keeps loading state when the newly selected range request fails', async () => {
    mockGetCharts.mockReset();
    mockGetCharts.mockImplementationOnce(() => Promise.resolve(makeChartsResponse()));
    mockGetCharts.mockImplementationOnce(() => Promise.reject(new Error('network error')));

    const [range, setRange] = createSignal<TimeRange>('1h');
    const { container } = render(() => <InfrastructureSummary hosts={[makeHost()]} timeRange={range()} />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });
    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeTruthy();
    });

    setRange('24h');

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('24h');
    });

    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeNull();
      expect(container.textContent).toContain('Loading history...');
    });
  });

  it('hydrates from cache after hosts are removed and re-added', async () => {
    mockGetCharts.mockReset();
    mockGetCharts.mockImplementationOnce(() => Promise.resolve(makeChartsResponse()));
    mockGetCharts.mockImplementationOnce(() => new Promise(() => {}));

    const [hosts, setHosts] = createSignal<Resource[]>([makeHost()]);
    const { container } = render(() => <InfrastructureSummary hosts={hosts()} timeRange="1h" />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });
    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeTruthy();
    });

    setHosts([]);
    setHosts([makeHost()]);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledTimes(2);
    });

    await waitFor(() => {
      expect(container.querySelector('svg.cursor-crosshair')).toBeTruthy();
    });
  });

  it('keeps per-chart rendered series bounded to the focused host', async () => {
    mockGetCharts.mockReset();
    mockGetCharts.mockResolvedValueOnce(makeChartsResponse(['node-1', 'node-2']));
    const hosts = [
      makeHost(),
      makeHost({
        id: 'node-2',
        name: 'node-2',
        displayName: 'node-2',
        platformId: 'node-2',
      }),
    ];
    const [focusedHostId, setFocusedHostId] = createSignal<string | null>(null);

    const { container } = render(() => (
      <InfrastructureSummary
        hosts={hosts}
        timeRange="1h"
        focusedHostId={focusedHostId()}
      />
    ));

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
      expect(countSparklinePaths(container)).toBeGreaterThan(0);
    });

    const allSeriesPathCount = countSparklinePaths(container);
    setFocusedHostId('node-2');

    await waitFor(() => {
      expect(countSparklinePaths(container)).toBeGreaterThan(0);
      expect(countSparklinePaths(container)).toBeLessThan(allSeriesPathCount);
    });
  });

  it('does not refetch charts on large host list growth for the same range', async () => {
    mockGetCharts.mockReset();
    mockGetCharts.mockResolvedValue(makeChartsResponse(['node-1']));

    const [hosts, setHosts] = createSignal<Resource[]>(
      Array.from({ length: 300 }, (_, i) =>
        makeHost({
          id: `node-${i}`,
          name: `node-${i}`,
          displayName: `node-${i}`,
          platformId: `node-${i}`,
        }),
      ),
    );

    render(() => <InfrastructureSummary hosts={hosts()} timeRange="1h" />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledTimes(1);
    });

    setHosts(
      Array.from({ length: 1200 }, (_, i) =>
        makeHost({
          id: `node-updated-${i}`,
          name: `node-updated-${i}`,
          displayName: `node-updated-${i}`,
          platformId: `node-updated-${i}`,
        }),
      ),
    );

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledTimes(1);
    });
  });

  it('maps hostData by agentId from unified platform data when websocket hosts are unavailable', async () => {
    mockGetCharts.mockReset();
    const now = Date.now();
    mockGetCharts.mockResolvedValueOnce({
      nodeData: {},
      dockerHostData: {},
      hostData: {
        'agent-host-1': {
          cpu: [],
          memory: [],
          disk: [],
          netin: [
            { timestamp: now - 60_000, value: 1024 },
            { timestamp: now, value: 2048 },
          ],
          netout: [
            { timestamp: now - 60_000, value: 512 },
            { timestamp: now, value: 1536 },
          ],
        },
      },
      timestamp: now,
      stats: {
        oldestDataTimestamp: now - 60_000,
      },
    });

    const agentOnlyHost: Resource = {
      id: 'unified-host-1',
      type: 'host',
      name: 'unraid-node',
      displayName: 'unraid-node',
      platformId: 'unraid-node',
      platformType: 'host-agent',
      sourceType: 'agent',
      status: 'online',
      lastSeen: now,
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-host-1',
          hostname: 'unraid-node',
        },
      },
    };

    const { container } = render(() => <InfrastructureSummary hosts={[agentOnlyHost]} timeRange="1h" />);

    await waitFor(() => {
      expect(mockGetCharts).toHaveBeenCalledWith('1h');
    });

    await waitFor(() => {
      const networkChart = container.querySelector('svg.cursor-crosshair');
      expect(networkChart).toBeTruthy();
    });

    expect(container.textContent).toContain('Network');
  });
});
