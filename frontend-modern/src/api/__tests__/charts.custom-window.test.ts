import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import * as apiClient from '@/utils/apiClient';

afterEach(() => {
  vi.restoreAllMocks();
});

const start = Date.UTC(2026, 9, 10, 12, 0, 0);
const end = start + 3_600_000;

describe('ChartsAPI.getMetricsHistory custom window contract', () => {
  it('sends start and end without range and validates echoed bounds and point range', async () => {
    const payload: AllMetricsHistoryResponse = {
      resourceType: 'vm',
      resourceId: 'pve1-101',
      range: 'custom',
      start,
      end,
      source: 'store',
      metrics: {
        cpu: [
          { timestamp: start, value: 12, min: 10, max: 14 },
          { timestamp: end, value: 34, min: 30, max: 38 },
        ],
      },
    };
    const spy = vi.spyOn(apiClient, 'apiFetchJSON').mockResolvedValue(payload);

    const result = await ChartsAPI.getMetricsHistory({
      resourceType: 'vm',
      resourceId: 'pve1-101',
      start,
      end,
      maxPoints: 240,
    });

    expect(spy).toHaveBeenCalledTimes(1);
    const url = spy.mock.calls[0][0];
    expect(url).toContain(`start=${start}`);
    expect(url).toContain(`end=${end}`);
    expect(url).not.toContain('range=');
    expect(result).toEqual(payload);
  });

  it('rejects out-of-window points, mismatched bounds, and live source on custom replies', async () => {
    const base: AllMetricsHistoryResponse = {
      resourceType: 'vm',
      resourceId: 'pve1-101',
      range: 'custom',
      start,
      end,
      source: 'store',
      metrics: { cpu: [{ timestamp: start + 60_000, value: 20, min: 20, max: 20 }] },
    };

    const invalidCases: AllMetricsHistoryResponse[] = [
      { ...base, range: '24h' },
      { ...base, start: start + 1 },
      { ...base, end: end - 1 },
      { ...base, source: 'live' },
      { ...base, metrics: { cpu: [{ timestamp: start - 1, value: 10, min: 10, max: 10 }] } },
      { ...base, metrics: { cpu: [{ timestamp: end + 1, value: 10, min: 10, max: 10 }] } },
    ];

    for (const bad of invalidCases) {
      vi.spyOn(apiClient, 'apiFetchJSON').mockResolvedValueOnce(bad);
      await expect(
        ChartsAPI.getMetricsHistory({
          resourceType: 'vm',
          resourceId: 'pve1-101',
          start,
          end,
        }),
      ).rejects.toThrow('Invalid metrics history response.');
    }
  });

  it('rejects invalid custom window arguments before fetching', async () => {
    const spy = vi.spyOn(apiClient, 'apiFetchJSON');
    await expect(
      ChartsAPI.getMetricsHistory({
        resourceType: 'vm',
        resourceId: 'pve1-101',
        start: end,
        end: start,
      }),
    ).rejects.toThrow('Invalid metrics history window.');
    expect(spy).not.toHaveBeenCalled();
  });
});
