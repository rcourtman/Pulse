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
    vi.mocked(apiFetchJSON)
      .mockResolvedValueOnce({
        resourceId: 'vm:42',
        capabilities: [{ name: 'restart' }],
        count: 1,
      } as any)
      .mockResolvedValueOnce({
        resourceId: 'vm:42',
        relationships: [{ sourceId: 'node:1', targetId: 'vm:42' }],
        count: 1,
      } as any)
      .mockResolvedValueOnce({
        resourceId: 'vm:42',
        recentChanges: [{ id: 'change-1' }],
        count: 1,
      } as any);

    const result = await ResourceAPI.getFacetBundle('vm:42', {
      since: '2026-03-18T12:00:00Z',
      limit: 25,
    });

    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      1,
      '/api/resources/vm%3A42/capabilities',
      {
        cache: 'no-store',
      },
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      2,
      '/api/resources/vm%3A42/relationships',
      {
        cache: 'no-store',
      },
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      3,
      '/api/resources/vm%3A42/timeline?since=2026-03-18T12%3A00%3A00.000Z&limit=25',
      {
        cache: 'no-store',
      },
    );
    expect(result).toEqual({
      capabilities: [{ name: 'restart' }],
      relationships: [{ sourceId: 'node:1', targetId: 'vm:42' }],
      recentChanges: [{ id: 'change-1' }],
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
    });

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/resources/vm%3A42/timeline', {
      cache: 'no-store',
    });
  });
});
