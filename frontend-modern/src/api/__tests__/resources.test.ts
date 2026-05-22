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

  it('fetches the resource history bundle from the facet endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: 'vm:42',
      capabilities: [{ name: 'restart', type: 'common' }],
      relationships: [
        {
          sourceId: 'vm:42',
          targetId: 'node-1',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:00:00Z',
        },
      ],
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
      capabilities: [{ name: 'restart', type: 'common' }],
      relationships: [
        {
          sourceId: 'vm:42',
          targetId: 'node-1',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:00:00Z',
        },
      ],
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

  it('fetches the global resource timeline with canonical filters', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      resourceId: '',
      recentChanges: [{ id: 'activity-1' }],
      count: 1,
    } as any);

    const result = await ResourceAPI.getGlobalTimeline({
      limit: 100,
      kind: 'activity',
      sourceType: 'platform_event',
      sourceAdapter: 'vmware_adapter',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/resources/timeline?limit=100&kind=activity&sourceType=platform_event&sourceAdapter=vmware_adapter',
      {
        cache: 'no-store',
      },
    );
    expect(result).toEqual({
      resourceId: '',
      recentChanges: [{ id: 'activity-1' }],
      count: 1,
    });
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
