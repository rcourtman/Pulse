import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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

import { AIChatAPI, createAIChatStreamPaintCheckpointPredicate } from '@/api/aiChat';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

const flushMicrotasks = async () => {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await Promise.resolve();
  }
};

describe('AIChatAPI', () => {
  const apiFetchMock = vi.mocked(apiFetch);
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);
  const originalRequestAnimationFrame = window.requestAnimationFrame;
  const originalCancelAnimationFrame = window.cancelAnimationFrame;

  beforeEach(() => {
    apiFetchMock.mockReset();
    apiFetchJSONMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: originalRequestAnimationFrame,
    });
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: originalCancelAnimationFrame,
    });
    vi.useRealTimers();
  });

  const installAnimationFrame = () => {
    const requestAnimationFrame = vi.fn((callback: FrameRequestCallback) => {
      return window.setTimeout(() => callback(performance.now()), 16);
    });
    const cancelAnimationFrame = vi.fn((id: number) => window.clearTimeout(id));
    vi.stubGlobal('requestAnimationFrame', requestAnimationFrame);
    vi.stubGlobal('cancelAnimationFrame', cancelAnimationFrame);
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: requestAnimationFrame,
    });
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: cancelAnimationFrame,
    });
    return requestAnimationFrame;
  };

  const advancePaintCheckpoint = async () => {
    await vi.advanceTimersToNextTimerAsync();
    await vi.advanceTimersToNextTimerAsync();
    await flushMicrotasks();
  };

  it('uses immediate and periodic paint checkpoints for text while yielding every progress event', () => {
    const shouldYield = createAIChatStreamPaintCheckpointPredicate();

    expect(shouldYield({ type: 'content' })).toBe(true);
    for (let index = 0; index < 14; index += 1) {
      expect(shouldYield({ type: 'content' })).toBe(false);
    }
    expect(shouldYield({ type: 'content' })).toBe(true);

    for (const type of [
      'session',
      'workflow_state',
      'tool_start',
      'tool_progress',
      'tool_cancel',
      'tool_end',
      'approval_needed',
      'question',
    ] as const) {
      expect(shouldYield({ type })).toBe(true);
    }

    expect(shouldYield({ type: 'thinking' })).toBe(true);
    expect(shouldYield({ type: 'thinking' })).toBe(false);
  });

  it('lets the browser paint the first visible text delta before draining a coalesced chat chunk', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const requestAnimationFrame = installAnimationFrame();
    const encoder = new TextEncoder();
    const read = vi
      .fn()
      .mockResolvedValueOnce({
        done: false,
        value: encoder.encode(
          [
            'data: {"type":"content","data":{"text":"First"}}',
            '',
            'data: {"type":"content","data":{"text":" second"}}',
            '',
            'data: {"type":"done"}',
            '',
            '',
          ].join('\n'),
        ),
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

    const streamPromise = AIChatAPI.chat('hello', undefined, undefined, onEvent);

    await flushMicrotasks();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual(['content']);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);

    await advancePaintCheckpoint();
    await streamPromise;

    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual(['content', 'content', 'done']);
    expect(releaseLock).toHaveBeenCalledTimes(1);
  });

  it('preserves safe handoff summaries on listed chat sessions', async () => {
    const session = {
      id: 'session-operator-briefing',
      title: 'High CPU follow-up',
      created_at: '2026-05-06T12:00:00Z',
      updated_at: '2026-05-06T12:08:00Z',
      message_count: 2,
      handoff_summary: {
        kind: 'patrol_finding',
        finding_id: 'finding-operator-briefing',
        has_model_context: true,
        resource_count: 1,
        primary_resource: {
          id: 'host:web-server',
          name: 'web-server',
          type: 'host',
          node: 'pve-1',
        },
        action_count: 1,
        requires_approval: true,
        last_known_approval_status: 'pending',
        last_known_action_state: 'awaiting_approval',
        last_known_action_risk: 'high',
        updated_at: '2026-05-06T12:08:00Z',
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce([session]);

    await expect(AIChatAPI.listSessions()).resolves.toEqual([session]);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/sessions');
  });

  it('passes session search and limit as query parameters', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([]);

    await expect(AIChatAPI.listSessions({ search: '  backup jobs  ', limit: 30 })).resolves.toEqual(
      [],
    );

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/sessions?search=backup+jobs&limit=30');
  });

  it('normalizes a null sessions payload to an empty array (#1149)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(null);
    await expect(AIChatAPI.listSessions()).resolves.toEqual([]);
  });

  it('normalizes a non-array sessions payload to an empty array (#1149)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ error: 'boom' });
    await expect(AIChatAPI.listSessions()).resolves.toEqual([]);
  });

  it('renames sessions through the session root endpoint', async () => {
    const renamed = {
      id: 'session/root',
      title: 'Renamed session',
      created_at: '2026-06-06T10:00:00Z',
      updated_at: '2026-06-06T10:05:00Z',
      message_count: 2,
    };
    apiFetchJSONMock.mockResolvedValueOnce(renamed);

    await expect(AIChatAPI.renameSession('session/root', 'Renamed session')).resolves.toEqual(
      renamed,
    );

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/sessions/session%2Froot', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Renamed session' }),
    });
  });

  it('preserves restored Assistant tool evidence on session messages', async () => {
    const messages = [
      {
        id: 'msg-user',
        role: 'user',
        content: 'show alerts',
        timestamp: '2026-06-06T05:00:00Z',
      },
      {
        id: 'msg-assistant',
        role: 'assistant',
        content: 'I checked alerts.',
        timestamp: '2026-06-06T05:00:01Z',
        model: 'openrouter:qwen/qwen3.7-plus',
        tool_calls: [
          {
            name: 'pulse_alerts',
            input: { action: 'list' },
            output: '{"count":11}',
            success: true,
          },
        ],
      },
    ];
    apiFetchJSONMock.mockResolvedValueOnce(messages);

    await expect(AIChatAPI.getMessages('session/tool-history')).resolves.toEqual(messages);
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/sessions/session%2Ftool-history/messages',
    );
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

  it('includes browser-safe Patrol run handoff metadata when supplied', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIChatAPI.chat(
      'discuss run',
      'session-run',
      undefined,
      vi.fn(),
      undefined,
      undefined,
      undefined,
      false,
      undefined,
      undefined,
      undefined,
      {
        kind: 'patrol_run',
        runId: 'run-runtime-error',
        runType: 'Scoped run',
        runStatus: 'error',
        runtimeFailure: true,
      },
    );

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/chat',
      expect.objectContaining({
        body: JSON.stringify({
          prompt: 'discuss run',
          session_id: 'session-run',
          model: undefined,
          autonomous_mode: false,
          handoff_metadata: {
            kind: 'patrol_run',
            run_id: 'run-runtime-error',
            run_type: 'Scoped run',
            run_status: 'error',
            runtime_failure: true,
          },
        }),
      }),
    );
  });

  it('includes browser-safe Patrol recommendation metadata when supplied', async () => {
    const read = vi.fn().mockResolvedValueOnce({ done: true, value: undefined });
    const releaseLock = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    await AIChatAPI.chat(
      'discuss finding',
      'session-finding',
      undefined,
      vi.fn(),
      undefined,
      undefined,
      'finding-provider-settings',
      false,
      undefined,
      undefined,
      undefined,
      {
        kind: 'patrol_finding',
      },
    );

    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/ai/chat',
      expect.objectContaining({
        body: JSON.stringify({
          prompt: 'discuss finding',
          session_id: 'session-finding',
          model: undefined,
          finding_id: 'finding-provider-settings',
          autonomous_mode: false,
          handoff_metadata: {
            kind: 'patrol_finding',
          },
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
