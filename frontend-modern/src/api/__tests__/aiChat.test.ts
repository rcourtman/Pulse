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
import { logger } from '@/utils/logger';

describe('AIChatAPI', () => {
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it('clears read timeout timers when chat stream reads complete', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();
    const onEvent = vi.fn();
    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIChatAPI.chat('hello', undefined, undefined, onEvent);

    expect(read).toHaveBeenCalledTimes(1);
    expect(releaseLock).toHaveBeenCalledTimes(1);
    expect(onEvent).toHaveBeenCalledWith({ type: 'done' });
    expect(clearTimeoutSpy).toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });

  it('ignores invalid chat stream events through the shared JSON-text helper', async () => {
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

    await AIChatAPI.chat('hello', undefined, undefined, onEvent);

    expect(logger.error).toHaveBeenCalledWith('[AI Chat] Failed to parse event', {
      line: 'data: not valid json',
    });
    expect(onEvent).toHaveBeenCalledWith({ type: 'done' });
    expect(releaseLock).toHaveBeenCalledTimes(1);
  });

  it('includes a per-request autonomous override when supplied', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIChatAPI.chat(
      'summarize dashboard',
      'session-1',
      undefined,
      vi.fn(),
      undefined,
      undefined,
      undefined,
      false,
    );

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/chat',
      expect.objectContaining({
        body: JSON.stringify({
          prompt: 'summarize dashboard',
          session_id: 'session-1',
          model: undefined,
          autonomous_mode: false,
        }),
      }),
    );
  });

  it('includes a Patrol finding id when supplied for Assistant context', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIChatAPI.chat(
      'inspect this finding',
      'session-2',
      undefined,
      vi.fn(),
      undefined,
      undefined,
      'finding-123',
    );

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/chat',
      expect.objectContaining({
        body: JSON.stringify({
          prompt: 'inspect this finding',
          session_id: 'session-2',
          model: undefined,
          finding_id: 'finding-123',
        }),
      }),
    );
  });

  it('includes model-only handoff context and resource references when supplied', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIChatAPI.chat(
      'discuss incident',
      'session-3',
      undefined,
      vi.fn(),
      undefined,
      undefined,
      undefined,
      false,
      '[Alert Incident Context]\nIncident ID: incident-1',
      [
        {
          id: 'storage-1',
          name: 'tank',
          type: 'storage',
          node: 'nas-1',
        },
      ],
      [
        {
          findingId: 'finding-1',
          approvalId: 'approval-1',
          approvalStatus: 'pending',
          approvalRequestedAt: '2026-05-06T12:00:00Z',
          approvalExpiresAt: '2026-05-06T12:10:00Z',
          actionId: 'action-1',
          actionApprovalPolicy: 'admin',
          actionRequiresApproval: true,
          actionPlanExpiresAt: '2026-05-06T12:10:00Z',
          actionDryRunSummary: 'No provider-supported dry run is available for this action.',
          riskLevel: 'high',
          targetResourceId: 'vm-100',
          targetResourceName: 'web-server',
          targetResourceType: 'vm',
        },
      ],
    );

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/chat',
      expect.objectContaining({
        body: JSON.stringify({
          prompt: 'discuss incident',
          session_id: 'session-3',
          model: undefined,
          autonomous_mode: false,
          handoff_context: '[Alert Incident Context]\nIncident ID: incident-1',
          handoff_resources: [
            {
              id: 'storage-1',
              name: 'tank',
              type: 'storage',
              node: 'nas-1',
            },
          ],
          handoff_actions: [
            {
              finding_id: 'finding-1',
              record_id: undefined,
              approval_id: 'approval-1',
              approval_status: 'pending',
              approval_requested_at: '2026-05-06T12:00:00Z',
              approval_expires_at: '2026-05-06T12:10:00Z',
              approval_decided_at: undefined,
              approval_consumed: undefined,
              action_id: 'action-1',
              action_state: undefined,
              action_updated_at: undefined,
              action_requested_by: undefined,
              action_capability: undefined,
              action_approval_policy: 'admin',
              action_requires_approval: true,
              action_plan_expires_at: '2026-05-06T12:10:00Z',
              action_plan_message: undefined,
              action_preflight: undefined,
              action_dry_run_summary: 'No provider-supported dry run is available for this action.',
              action_result: undefined,
              fix_id: undefined,
              description: undefined,
              risk_level: 'high',
              destructive: undefined,
              target_host: undefined,
              target_resource_id: 'vm-100',
              target_resource_name: 'web-server',
              target_resource_type: 'vm',
              target_node: undefined,
            },
          ],
        }),
      }),
    );
  });
});
