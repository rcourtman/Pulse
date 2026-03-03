import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { apiFetchJSON } from '@/utils/apiClient';
import { dismissFinding, undismissFinding } from '@/api/patrol';

describe('patrol api helpers', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchJSONMock.mockResolvedValue({ success: true, message: 'ok' } as any);
  });

  it('dismissFinding calls dismiss endpoint', async () => {
    await dismissFinding('finding-1', 'not_an_issue', 'false positive');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/dismiss', {
      method: 'POST',
      body: JSON.stringify({ finding_id: 'finding-1', reason: 'not_an_issue', note: 'false positive' }),
    });
  });

  it('undismissFinding calls undismiss endpoint', async () => {
    await undismissFinding('finding-1');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/undismiss', {
      method: 'POST',
      body: JSON.stringify({ finding_id: 'finding-1' }),
    });
  });
});
