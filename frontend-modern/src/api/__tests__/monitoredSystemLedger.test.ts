import { beforeEach, describe, expect, it, vi } from 'vitest';

import { MonitoredSystemLedgerAPI } from '../monitoredSystemLedger';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('MonitoredSystemLedgerAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the canonical monitored-system ledger endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [],
      total: 0,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/monitored-system-ledger');
    expect(result).toEqual({
      systems: [],
      total: 0,
      limit: 5,
    });
  });
});
