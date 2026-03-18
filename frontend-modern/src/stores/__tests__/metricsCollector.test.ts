import { describe, expect, it, vi, beforeEach } from 'vitest';

const mockRecordDiskMetric = vi.fn();
let mockResources: unknown[] = [];
let mockCachedUnifiedResources: unknown[] = [];

vi.mock('@/stores/websocket-global', () => ({
  getGlobalWebSocketStore: () => ({
    state: {
      resources: mockResources,
    },
  }),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    info: vi.fn(),
  },
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: vi.fn(),
  },
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  getCachedUnifiedResources: () => mockCachedUnifiedResources,
}));

vi.mock('../diskMetricsHistory', () => ({
  recordDiskMetric: (...args: unknown[]) => mockRecordDiskMetric(...args),
}));

describe('metricsCollector', () => {
  beforeEach(() => {
    mockResources = [];
    mockCachedUnifiedResources = [];
    mockRecordDiskMetric.mockReset();
    vi.useRealTimers();
  });

  it('collects diskIo counters from unified agent resources using camel-case diskIo', async () => {
    mockResources = [
      {
        id: 'agent-node',
        agent: {
          agentId: 'agent-123',
          diskIo: [
            {
              device: 'nvme0n1',
              readBytes: 1000,
              writeBytes: 2000,
              readOps: 10,
              writeOps: 20,
              ioTimeMs: 100,
            },
          ],
        },
      },
    ];

    vi.useFakeTimers();
    const { startMetricsCollector, stopMetricsCollector } = await import('../metricsCollector');

    startMetricsCollector();

    mockResources = [
      {
        id: 'agent-node',
        agent: {
          agentId: 'agent-123',
          diskIo: [
            {
              device: 'nvme0n1',
              readBytes: 3000,
              writeBytes: 6000,
              readOps: 30,
              writeOps: 60,
              ioTimeMs: 300,
            },
          ],
        },
      },
    ];

    vi.advanceTimersByTime(2000);

    expect(mockRecordDiskMetric).toHaveBeenCalledWith('agent-123:nvme0n1', 1000, 2000, 10, 20, 10);

    stopMetricsCollector();
  });

  it('falls back to cached unified resources when websocket resources do not include diskIo', async () => {
    mockResources = [
      {
        id: 'node-1',
        type: 'node',
        name: 'delly',
      },
    ];
    mockCachedUnifiedResources = [
      {
        id: 'node-1',
        agent: {
          agentId: 'agent-456',
          diskIo: [
            {
              device: 'sda',
              readBytes: 100,
              writeBytes: 200,
              readOps: 1,
              writeOps: 2,
              ioTimeMs: 50,
            },
          ],
        },
      },
    ];

    vi.useFakeTimers();
    const { startMetricsCollector, stopMetricsCollector } = await import('../metricsCollector');

    startMetricsCollector();

    mockCachedUnifiedResources = [
      {
        id: 'node-1',
        agent: {
          agentId: 'agent-456',
          diskIo: [
            {
              device: 'sda',
              readBytes: 2100,
              writeBytes: 4200,
              readOps: 21,
              writeOps: 42,
              ioTimeMs: 250,
            },
          ],
        },
      },
    ];

    vi.advanceTimersByTime(2000);

    expect(mockRecordDiskMetric).toHaveBeenCalledWith('agent-456:sda', 1000, 2000, 10, 20, 10);

    stopMetricsCollector();
  });
});
