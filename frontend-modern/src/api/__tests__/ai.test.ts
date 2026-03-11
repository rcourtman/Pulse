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
import { logger } from '@/utils/logger';

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

  it('normalizes unified finding alert identifiers', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      findings: [
        {
          id: 'f1',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          source: 'threshold',
          severity: 'warning',
          category: 'performance',
          title: 'CPU',
          description: 'high',
          detected_at: '2026-03-01T00:00:00Z',
          alert_identifier: 'canonical-alert-1',
        },
      ],
      count: 1,
    } as any);

    const result = await AIAPI.getUnifiedFindings();

    expect(result.findings[0]).toMatchObject({
      alertIdentifier: 'canonical-alert-1',
    });
  });

  it('encodes dynamic provider, approval, finding, and plan identifiers', async () => {
    apiFetchJSONMock.mockResolvedValue({} as any);

    await AIAPI.testProvider('openai/internal');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/test/openai%2Finternal', {
      method: 'POST',
    });

    await AIAPI.getRemediationPlan('plan/1?x=1');
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/remediation/plan?plan_id=plan%2F1%3Fx%3D1',
    );

    await AIAPI.approveInvestigationFix('approval/root');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/approvals/approval%2Froot/approve', {
      method: 'POST',
    });

    await AIAPI.reapproveInvestigationFix('finding/root');
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/findings/finding%2Froot/reapprove', {
      method: 'POST',
    });
  });

  it('treats 402 responses from optional AI paywalled collections as empty state', async () => {
    const paymentRequiredError = Object.assign(new Error('Approval management requires Pulse Pro'), {
      status: 402,
    });

    apiFetchJSONMock.mockRejectedValueOnce(paymentRequiredError);
    await expect(AIAPI.getPendingApprovals()).resolves.toEqual([]);

    apiFetchJSONMock.mockRejectedValueOnce(paymentRequiredError);
    await expect(AIAPI.getRemediationPlans()).resolves.toEqual({ plans: [] });
  });

  it('does not treat status text without canonical error status as payment required', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(new Error('402'));

    await expect(AIAPI.getPendingApprovals()).rejects.toThrow('402');
  });

  it('normalizes missing or malformed approval collections to empty arrays', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ approvals: 'bad' } as any);

    await expect(AIAPI.getPendingApprovals()).resolves.toEqual([]);
  });

  it('normalizes remediation plans from optional or legacy collection payloads', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ plans: [{ id: 'plan-1' }] } as any);
    await expect(AIAPI.getRemediationPlans()).resolves.toEqual({
      plans: [{ id: 'plan-1' }],
    });

    apiFetchJSONMock.mockResolvedValueOnce({ executions: [{ id: 'exec-1' }] } as any);
    await expect(AIAPI.getRemediationPlans()).resolves.toEqual({ plans: [] });
  });

  it('returns null only for canonical not-found investigation lookups', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(Object.assign(new Error('Not Found'), { status: 404 }));

    await expect(AIAPI.getInvestigation('finding-1')).resolves.toBeNull();
  });

  it('does not swallow non-404 investigation lookup failures', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(Object.assign(new Error('Payment Required'), { status: 402 }));
    await expect(AIAPI.getInvestigation('finding-2')).rejects.toThrow('Payment Required');

    apiFetchJSONMock.mockRejectedValueOnce(new Error('backend down'));
    await expect(AIAPI.getInvestigation('finding-3')).rejects.toThrow('backend down');
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
          alertIdentifier: 'a1',
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
        () => undefined,
      ),
    ).rejects.toThrow('backend error');
  });

  it('throws when investigateAlert has no streaming body', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 200 }));
    await expect(
      AIAPI.investigateAlert(
        {
          alertIdentifier: 'a1',
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
        () => undefined,
      ),
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
        alertIdentifier: 'a1',
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
      () => undefined,
    );

    expect(read).toHaveBeenCalledTimes(1);
    expect(releaseLock).toHaveBeenCalledTimes(1);
    expect(clearTimeoutSpy).toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });

  it('ignores invalid investigateAlert stream events through the shared JSON-text helper', async () => {
    const encoder = new TextEncoder();
    const read = vi
      .fn()
      .mockResolvedValueOnce({
        done: false,
        value: encoder.encode('data: not valid json\n\n'),
      })
      .mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();
    const onEvent = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIAPI.investigateAlert(
      {
        alertIdentifier: 'a1',
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
      onEvent,
    );

    expect(onEvent).not.toHaveBeenCalled();
    expect(logger.error).toHaveBeenCalledWith('[AI] Failed to parse investigation event:', {
      line: 'data: not valid json',
    });
    expect(releaseLock).toHaveBeenCalledTimes(1);
  });

  it('sends canonical alertIdentifier for investigateAlert', async () => {
    apiFetchMock.mockResolvedValueOnce(new Response(null, { status: 200 }));

    await expect(
      AIAPI.investigateAlert(
        {
          alertIdentifier: 'instance:node:100::metric/cpu',
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
        () => undefined,
      ),
    ).rejects.toThrow('No response body');

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/investigate-alert',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          alertIdentifier: 'instance:node:100::metric/cpu',
          resource_id: 'r1',
          resource_name: 'res',
          resource_type: 'vm',
          alert_type: 'cpu',
          level: 'warning',
          value: 1,
          threshold: 2,
          message: 'msg',
          duration: '1m',
        }),
      }),
    );
  });
});
