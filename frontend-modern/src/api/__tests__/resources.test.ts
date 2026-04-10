import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { ResourceAPI } from '@/api/resources';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('ResourceAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the compact dashboard summary payload from the dashboard-summary endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      health: { totalResources: 4, byStatus: { online: 3, degraded: 1 } },
      infrastructure: {
        total: 2,
        byStatus: { online: 2 },
        byType: { agent: 1, 'docker-host': 1 },
        topCPU: [
          {
            id: 'infra-a',
            name: 'Infra A',
            percent: 91,
            metricsTarget: { resourceType: 'agent', resourceId: 'host-a' },
          },
        ],
        topMemory: [
          {
            id: 'infra-a',
            name: 'Infra A',
            percent: 82,
            metricsTarget: { resourceType: 'agent', resourceId: 'host-a' },
          },
        ],
      },
      workloads: { total: 1, running: 1, stopped: 0, byType: { vm: 1 } },
      storage: {
        total: 1,
        totalCapacity: 1_000,
        totalUsed: 850,
        warningCount: 1,
        criticalCount: 0,
      },
      problemResources: [],
    } as any);

    const result = await ResourceAPI.getDashboardSummary();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/resources/dashboard-summary', {
      cache: 'no-store',
    });
    expect(result.health.totalResources).toBe(4);
    expect(result.infrastructure.topCPU[0]?.id).toBe('infra-a');
    expect(result.infrastructure.topCPU[0]?.metricsTarget).toEqual({
      resourceType: 'agent',
      resourceId: 'host-a',
    });
  });

  it('fetches the resource history bundle from the facet endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      recentChanges: [{ id: 'change-1' }],
      counts: {
        recentChanges: 3,
        recentChangeKinds: {
          metric_anomaly: 2,
          restart: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 2,
          proxmox_adapter: 1,
        },
      },
    } as any);

    const result = await ResourceAPI.getFacetBundle('vm:42', {
      since: '2026-03-18T12:00:00Z',
      limit: 25,
      kind: 'restart',
      sourceType: 'platform_event',
      sourceAdapter: 'proxmox_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/vm%3A42/facets?since=2026-03-18T12%3A00%3A00.000Z&limit=25&kind=restart&sourceType=platform_event&sourceAdapter=proxmox_adapter',
      {
        cache: 'no-store',
      },
    );
    expect(result).toEqual({
      resourceId: 'vm:42',
      recentChanges: [{ id: 'change-1' }],
      counts: {
        recentChanges: 3,
        recentChangeKinds: {
          metric_anomaly: 2,
          restart: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 2,
          proxmox_adapter: 1,
        },
      },
    });
    expect(result.counts).toStrictEqual({
      recentChanges: 3,
      recentChangeKinds: {
        metric_anomaly: 2,
        restart: 1,
      },
      recentChangeSourceTypes: {
        platform_event: 1,
        pulse_diff: 2,
      },
      recentChangeSourceAdapters: {
        docker_adapter: 2,
        proxmox_adapter: 1,
      },
    });
  });

  it('omits invalid timeline query values instead of emitting broken URLs', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      recentChanges: [],
      count: 0,
    } as any);

    await ResourceAPI.getTimeline('vm:42', {
      since: 'not-a-date',
      limit: -1,
      kind: 'metric_anomaly',
      sourceType: 'pulse_diff',
      sourceAdapter: 'docker_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/vm%3A42/timeline?kind=metric_anomaly&sourceType=pulse_diff&sourceAdapter=docker_adapter',
      {
        cache: 'no-store',
      },
    );
  });

  it('preserves timeline filters when the time window is valid', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      recentChanges: [],
      count: 0,
    } as any);

    await ResourceAPI.getTimeline('vm:42', {
      since: '2026-03-18T12:00:00Z',
      limit: 10,
      kind: 'state_transition',
      sourceType: 'platform_event',
      sourceAdapter: 'proxmox_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/vm%3A42/timeline?since=2026-03-18T12%3A00%3A00.000Z&limit=10&kind=state_transition&sourceType=platform_event&sourceAdapter=proxmox_adapter',
      {
        cache: 'no-store',
      },
    );
  });
});
