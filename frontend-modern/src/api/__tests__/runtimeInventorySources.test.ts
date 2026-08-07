import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { RuntimeInventorySourcesAPI } from '../runtimeInventorySources';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedFetch = vi.mocked(apiFetchJSON);

describe('RuntimeInventorySourcesAPI', () => {
  beforeEach(() => {
    mockedFetch.mockReset();
  });

  it('reads the monitoring-tier route, not the admin connections ledger', async () => {
    mockedFetch.mockResolvedValue({ sources: [] });

    await RuntimeInventorySourcesAPI.list();

    expect(mockedFetch).toHaveBeenCalledWith('/api/runtime/inventory-sources');
  });

  it('returns the sources the backend sent', async () => {
    mockedFetch.mockResolvedValue({
      sources: [
        {
          id: 'pve:delly',
          type: 'pve',
          name: 'delly',
          state: 'unreachable',
          surfaces: ['vms'],
          credentialsInvalid: false,
        },
      ],
    });

    const response = await RuntimeInventorySourcesAPI.list();

    expect(response.sources).toHaveLength(1);
    expect(response.sources[0]?.name).toBe('delly');
  });

  it('defaults to an empty list when the envelope omits sources', async () => {
    mockedFetch.mockResolvedValue({});

    await expect(RuntimeInventorySourcesAPI.list()).resolves.toEqual({ sources: [] });
  });
});
