import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  AvailabilityHistoryAPI,
  type AvailabilityHistoryResponse,
} from '@/api/availabilityHistory';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

beforeEach(() => {
  mockedApiFetchJSON.mockReset();
});

describe('AvailabilityHistoryAPI', () => {
  it('deduplicates target ids and posts one bounded batch', async () => {
    const response: AvailabilityHistoryResponse = {
      start: '2026-08-29T12:00:00Z',
      end: '2026-08-30T12:00:00Z',
      targets: [],
    };
    mockedApiFetchJSON.mockResolvedValue(response);

    await AvailabilityHistoryAPI.batch(['one', ' one ', '', 'two']);

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/availability-history', {
      method: 'POST',
      body: JSON.stringify({ targetIds: ['one', 'two'], range: '24h' }),
    });
  });

  it('chunks fleets larger than the server bound without target-by-target reads', async () => {
    mockedApiFetchJSON.mockImplementation(async (_path, init) => {
      const request = JSON.parse(String(init?.body)) as { targetIds: string[] };
      return {
        start: '2026-08-29T12:00:00Z',
        end: '2026-08-30T12:00:00Z',
        targets: request.targetIds.map((targetId) => ({ targetId })),
      } satisfies AvailabilityHistoryResponse;
    });
    const ids = Array.from({ length: 450 }, (_, index) => `target-${index}`);

    const response = await AvailabilityHistoryAPI.batch(ids);

    expect(mockedApiFetchJSON).toHaveBeenCalledTimes(3);
    expect(response.targets).toHaveLength(450);
  });
});
