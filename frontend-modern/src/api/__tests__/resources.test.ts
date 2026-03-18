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

  it('fetches capabilities with the canonical facet endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      capabilities: [],
      count: 0,
    } as any);

    const result = await ResourceAPI.getCapabilities(' vm:42 ');

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/resources/vm%3A42/capabilities', {
      cache: 'no-store',
    });
    expect(result).toEqual({
      resourceId: 'vm:42',
      capabilities: [],
      count: 0,
    });
  });

  it('fetches the resource history bundle from the dedicated facet endpoints', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      capabilities: [{ name: 'restart' }],
      relationships: [{ sourceId: 'node:1', targetId: 'vm:42' }],
      recentChanges: [{ id: 'change-1' }],
      counts: {
        capabilities: 1,
        relationships: 1,
        recentChanges: 3,
        recentChangeKinds: {
          metric_anomaly: 2,
          restart: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
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
      capabilities: [{ name: 'restart' }],
      relationships: [{ sourceId: 'node:1', targetId: 'vm:42' }],
      recentChanges: [{ id: 'change-1' }],
      counts: {
        capabilities: 1,
        relationships: 1,
        recentChanges: 3,
        recentChangeKinds: {
          metric_anomaly: 2,
          restart: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
        },
      },
    });
    expect(result.counts).toStrictEqual({
      capabilities: 1,
      relationships: 1,
      recentChanges: 3,
      recentChangeKinds: {
        metric_anomaly: 2,
        restart: 1,
      },
      recentChangeSourceTypes: {
        platform_event: 1,
        pulse_diff: 2,
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
