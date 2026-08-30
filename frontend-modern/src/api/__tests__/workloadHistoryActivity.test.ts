import { beforeEach, describe, expect, it, vi } from 'vitest';

import { apiFetchJSON } from '@/utils/apiClient';
import {
  recordWorkloadHistoryActivity,
  resetWorkloadHistoryActivityForTest,
} from '../workloadHistoryActivity';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));

describe('recordWorkloadHistoryActivity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
    resetWorkloadHistoryActivityForTest();
    vi.mocked(apiFetchJSON).mockResolvedValue(null);
  });

  it('sends only the closed milestone and deduplicates it for the browser session', () => {
    recordWorkloadHistoryActivity('preview');
    recordWorkloadHistoryActivity('preview');

    expect(apiFetchJSON).toHaveBeenCalledTimes(1);
    expect(apiFetchJSON).toHaveBeenCalledWith('/api/usage/workload-history', {
      method: 'POST',
      body: '{"activity":"preview"}',
    });
  });

  it('allows a later retry when the local intake failed', async () => {
    vi.mocked(apiFetchJSON).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(null);

    recordWorkloadHistoryActivity('scrub');
    await Promise.resolve();
    await Promise.resolve();
    recordWorkloadHistoryActivity('scrub');

    expect(apiFetchJSON).toHaveBeenCalledTimes(2);
  });
});
