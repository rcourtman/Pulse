import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { AIChatAPI } from '@/api/aiChat';
import { apiFetch } from '@/utils/apiClient';

describe('AIChatAPI mention payloads', () => {
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it('preserves canonical storage mention ids in the shared chat request body', async () => {
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockResolvedValueOnce({ done: true, value: undefined }),
          releaseLock: vi.fn(),
        }),
      },
    } as unknown as Response);

    await AIChatAPI.chat(
      'check @NVMe Primary',
      undefined,
      undefined,
      vi.fn(),
      undefined,
      [
        {
          id: 'storage-vmware-1',
          name: 'NVMe Primary',
          type: 'storage',
          node: 'Lab VC',
        },
      ],
    );

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/chat',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream',
        },
        body: JSON.stringify({
          prompt: 'check @NVMe Primary',
          session_id: undefined,
          model: undefined,
          mentions: [
            {
              id: 'storage-vmware-1',
              name: 'NVMe Primary',
              type: 'storage',
              node: 'Lab VC',
            },
          ],
        }),
      }),
    );
  });
});
