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

const makeHost = (): Resource => ({
  id: 'node-1',
  type: 'node',
  name: 'node-1',
  displayName: 'node-1',
  platformId: 'node-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
});

const makeChartsResponse = () => ({
  nodeData: {
    'node-1': {
      cpu: [
        { timestamp: Date.now() - 60_000, value: 10 },
        { timestamp: Date.now(), value: 15 },
      ],
      memory: [
        { timestamp: Date.now() - 60_000, value: 45 },
        { timestamp: Date.now(), value: 50 },
      ],
      disk: [
        { timestamp: Date.now() - 60_000, value: 30 },
        { timestamp: Date.now(), value: 35 },
      ],
    },
  },
  dockerHostData: {},
  hostData: {},
  timestamp: Date.now(),
  stats: {
    oldestDataTimestamp: Date.now() - 60_000,
  },
});

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
