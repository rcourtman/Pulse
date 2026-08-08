import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { RuntimeInventorySourcesAPI } from '../runtimeInventorySources';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const apiFetchJSONMock = vi.mocked(apiFetchJSON);

describe('RuntimeInventorySourcesAPI', () => {
  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('reads the monitoring-tier runtime projection', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      sources: [
        {
          type: 'vmware',
          name: 'Primary vCenter',
          state: 'unreachable',
          surfaces: ['vms'],
        },
      ],
    });

    await expect(RuntimeInventorySourcesAPI.list()).resolves.toEqual({
      sources: [
        {
          type: 'vmware',
          name: 'Primary vCenter',
          state: 'unreachable',
          surfaces: ['vms'],
        },
      ],
    });
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/runtime/inventory-sources');
    expect(apiFetchJSONMock).not.toHaveBeenCalledWith('/api/connections');
  });

  it('normalizes a missing source array to an empty list', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({});

    await expect(RuntimeInventorySourcesAPI.list()).resolves.toEqual({ sources: [] });
  });
});
