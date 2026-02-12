import { describe, expect, it, beforeEach, vi } from 'vitest';

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

import { AIAPI } from '@/api/ai';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

describe('AIAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchMock.mockReset();
  });

  it('calls the expected endpoints for settings and models', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ configured: true } as any);
    await AIAPI.getSettings();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/settings/ai');

    apiFetchJSONMock.mockResolvedValueOnce({ configured: true } as any);
    await AIAPI.updateSettings({ enabled: true });
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/settings/ai/update', {
      method: 'PUT',
      body: JSON.stringify({ enabled: true }),
    });

    apiFetchJSONMock.mockResolvedValueOnce({ models: [] } as any);
    await AIAPI.getModels();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/models');
  });

  it('includes query parameters for cost endpoints', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getCostSummary();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/cost/summary?days=30');

    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    await AIAPI.getCostSummary(7);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/cost/summary?days=7');
  });

  it('sanitizes runCommand payload consistently', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ output: 'ok', success: true } as any);
    await AIAPI.runCommand({
      command: 'echo hi',
      target_type: 'vm',
      target_id: 'vm-101',
      run_on_host: undefined as any,
      vmid: 101,
      target_host: 'delly',
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/run-command', {
      method: 'POST',
      body: JSON.stringify({
        command: 'echo hi',
        target_type: 'vm',
        target_id: 'vm-101',
        run_on_host: false,
        vmid: '101',
        target_host: 'delly',
      }),
    });
  });

  it('throws useful errors for investigateAlert failures', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response('backend error', { status: 500 }));
    await expect(
      AIAPI.investigateAlert(
        {
          alert_id: 'a1',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        },
        () => undefined
      )
    ).rejects.toThrow('backend error');
  });

  it('throws when investigateAlert has no streaming body', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 200 }));
    await expect(
      AIAPI.investigateAlert(
        {
          alert_id: 'a1',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        },
        () => undefined
      )
    ).rejects.toThrow('No response body');
  });

  it('clears read timeout timers when investigateAlert stream reads complete', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();
    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIAPI.investigateAlert(
      {
        alert_id: 'a1',
        resource_id: 'r1',
        resource_name: 'res',
        resource_type: 'vm',
        alert_type: 'cpu',
        level: 'warning',
        value: 1,
        threshold: 2,
        message: 'msg',
        duration: '1m',
      },
      () => undefined
    );

    expect(read).toHaveBeenCalledTimes(1);
    expect(releaseLock).toHaveBeenCalledTimes(1);
    expect(clearTimeoutSpy).toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });
});
