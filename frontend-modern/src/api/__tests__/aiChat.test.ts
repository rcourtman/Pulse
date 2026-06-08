import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import aiChatSource from '../aiChat.ts?raw';

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
import {
  AI_CHAT_DEV_STREAM_FIXTURE_ALIAS_NAMES,
  AI_CHAT_DEV_STREAM_FIXTURE_NAMES,
  maybeRunAIChatDevStreamFixture,
} from '@/api/aiChatDevStreamFixture';
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

  it('uses low-latency paint checkpoints for text while yielding every progress event', () => {
    const shouldYield = createAIChatStreamPaintCheckpointPredicate();

    expect(shouldYield({ type: 'content' })).toBe(true);
    for (let index = 0; index < 2; index += 1) {
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

  it('runs the local dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '  /fixture   devices  ',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'thinking',
      'tool_start',
      'tool_progress',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[1][0]).toMatchObject({
      type: 'workflow_state',
      data: { phase: 'request_start', message: 'Preparing Pulse context.' },
    });
    expect(onEvent.mock.calls[4][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-devices',
        name: 'pulse_read',
        input: '{}',
        raw_input: 'pulse_read(target_host="current_resource", command="ls /dev | wc',
      },
    });
    expect(onEvent.mock.calls[7][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('4,358 entries'),
      },
    });
    expect(onEvent.mock.calls[8][0]).toMatchObject({
      type: 'done',
      data: {
        model: 'openrouter:deepseek/deepseek-chat',
        input_tokens: 4358,
        output_tokens: 943,
      },
    });
  });

  it('runs the skipped-tool dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture skipped-tool',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'tool_start',
      'tool_cancel',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-skipped',
        name: 'pulse_read',
        input: expect.stringContaining('current_resource'),
        raw_input: expect.stringContaining('ls /dev | wc -l'),
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_cancel',
      data: {
        id: 'fixture-tool-skipped',
        name: 'pulse_read',
        reason: 'current_resource unavailable',
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-skipped-tool',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
  });

  it('runs the tool-burst dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture tool-burst',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'provider_start',
        message: 'OpenRouter is starting the response.',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-burst',
        name: 'pulse_read',
        input: expect.stringContaining('ls /dev | wc -l'),
      },
    });
    expect(onEvent.mock.calls[4][0]).toMatchObject({
      type: 'tool_end',
      data: {
        id: 'fixture-tool-burst',
        name: 'pulse_read',
        output: '4358',
        success: true,
      },
    });
  });

  it('runs the reasoning-leak dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture reasoning-leak',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'content',
      'content',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('Thinking'),
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('pulse_read'),
      },
    });
    expect(onEvent.mock.calls[4][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-reasoning-leak',
        name: 'pulse_read',
        input: expect.stringContaining('ls /dev | wc -l'),
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'content',
      data: {
        text: 'There are 4,358 entries under `/dev`.',
      },
    });
  });

  it('runs the workflow-burst dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture workflow-burst',
      undefined,
      'openrouter:qwen/qwen3.7-plus',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'workflow_state',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'context',
        message: 'Reading current Pulse inventory with pulse_query.',
        tool: 'pulse_query',
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'provider_start',
        message: 'OpenRouter is starting the response.',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-workflow-burst',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('keeps the tool-burst fixture running state visible when fixture pacing is enabled', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const onEvent = vi.fn();

    const streamPromise = maybeRunAIChatDevStreamFixture({
      prompt: '/fixture tool-burst',
      model: 'openrouter:deepseek/deepseek-chat',
      onEvent,
      stepDelayMs: 25,
    });

    await flushMicrotasks();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual(['session']);

    await vi.advanceTimersByTimeAsync(25);
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual(['session', 'workflow_state']);

    await vi.advanceTimersByTimeAsync(25);
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
    ]);

    await vi.advanceTimersByTimeAsync(25);
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
    ]);

    await vi.advanceTimersByTimeAsync(419);
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
    ]);

    await vi.advanceTimersByTimeAsync(1);
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
      'tool_end',
    ]);

    await vi.runAllTimersAsync();
    await expect(streamPromise).resolves.toBe(true);
  });

  it('runs the send-hold dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat('/fixture send-hold', undefined, 'openrouter:qwen/qwen3.7-plus', onEvent);

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[0][0]).toMatchObject({
      type: 'session',
      data: { id: 'dev-fixture-send-hold' },
    });
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'provider_start',
        message: 'OpenRouter is starting the response.',
        provider: 'openrouter',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('inspect the local prompt-send state'),
      },
    });
    expect(onEvent.mock.calls[4][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-send-hold',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('accepts the burst-tool alias for the tool-burst dev stream fixture', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat('/fixture burst-tool', undefined, 'openrouter:qwen/qwen3.7-plus', onEvent);

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[0][0]).toMatchObject({
      type: 'session',
      data: { id: 'dev-fixture-tool-burst' },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-tool-burst',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('runs the context-group dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture context-group',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
      'tool_end',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-context-resource',
        name: 'pulse_get_resource_details',
        input: expect.stringContaining('vm-101'),
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'tool_end',
      data: {
        id: 'fixture-tool-context-metrics',
        name: 'pulse_get_metrics_history',
        success: true,
      },
    });
    expect(onEvent.mock.calls[7][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('separate visible activity rows'),
      },
    });
    expect(onEvent.mock.calls[7][0].data.text).not.toContain('one compact context activity row');
    expect(onEvent.mock.calls[8][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-context-group',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
  });

  it('runs the status-boundary dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture status-boundary',
      undefined,
      'openrouter:qwen/qwen3.7-plus',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
      'tool_end',
      'workflow_state',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'context',
        message: 'Reading current Pulse inventory with pulse_query.',
        tool: 'pulse_query',
      },
    });
    expect(onEvent.mock.calls[4][0]).toMatchObject({
      type: 'tool_end',
      data: {
        id: 'fixture-tool-status-boundary',
        name: 'pulse_query',
        output: expect.stringContaining('"devices":3'),
        success: true,
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'provider_start',
        message: 'OpenRouter is starting the response.',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('inventory status row stable'),
      },
    });
    expect(onEvent.mock.calls[7][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-status-boundary',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('fails unknown dev fixture prompts locally instead of sending them to a provider', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat('/fixture typo-check', undefined, 'openrouter:qwen/qwen3.7-plus', onEvent);

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'error',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'error',
      data: {
        message: expect.stringContaining('Unknown Assistant fixture "/fixture typo-check"'),
      },
    });
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'error',
      data: {
        message: expect.stringContaining('tool-burst'),
      },
    });
  });

  it('exports fixture names and aliases for command discovery', () => {
    expect(AI_CHAT_DEV_STREAM_FIXTURE_NAMES).toContain('provider-retry');
    expect(AI_CHAT_DEV_STREAM_FIXTURE_NAMES).toContain('send-hold');
    expect(AI_CHAT_DEV_STREAM_FIXTURE_NAMES).toContain('command-tool');
    expect(AI_CHAT_DEV_STREAM_FIXTURE_NAMES).toContain('reasoning-leak');
    expect(AI_CHAT_DEV_STREAM_FIXTURE_NAMES).not.toContain('/fixture provider-retry');
    expect(AI_CHAT_DEV_STREAM_FIXTURE_ALIAS_NAMES).toEqual(['burst-tool', 'queued-follow-up']);
  });

  it('runs the pending-tool dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture pending-tool',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'tool_start',
      'tool_progress',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-pending',
        name: 'pulse_read',
        input: '{}',
        raw_input: expect.stringContaining('ls /dev | wc'),
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_progress',
      data: {
        id: 'fixture-tool-pending',
        name: 'pulse_read',
        phase: 'running',
        message: 'Running command.',
        raw_input: expect.stringContaining('lsblk -d -o NAME,TYPE,SIZE'),
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-pending-tool',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
  });

  it('runs the command-tool dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture command-tool',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'tool_start',
      'tool_progress',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-command',
        name: 'pulse_run_command',
        input: '{}',
        raw_input: expect.stringContaining('systemctl restart nginx'),
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_progress',
      data: {
        id: 'fixture-tool-command',
        name: 'pulse_run_command',
        phase: 'running',
        message: 'Running command.',
        input: expect.stringContaining('systemctl restart nginx'),
      },
    });
    expect(onEvent.mock.calls[4][0]).toMatchObject({
      type: 'tool_end',
      data: {
        id: 'fixture-tool-command',
        name: 'pulse_run_command',
        output: 'queued',
        success: true,
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-command-tool',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
  });

  it('runs the long-output dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture long-output',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_end',
      data: {
        id: 'fixture-tool-long-output',
        name: 'pulse_read',
        output: expect.stringContaining('line 6'),
        success: true,
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-long-output',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
  });

  it('runs the provider-retry dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture provider-retry',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'workflow_state',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'provider_retry',
        message: 'Provider connection failed before any output; retrying.',
        provider: 'openrouter',
        model: 'openrouter:deepseek/deepseek-chat',
        attempt: 2,
        max_attempts: 3,
        retry_after_ms: 3200,
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-provider-retry',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
  });

  it('runs the stream-idle dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture stream-idle',
      undefined,
      'openrouter:qwen/qwen3.7-plus',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'workflow_state',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'provider_start',
        message: 'OpenRouter is starting the response.',
        provider: 'openrouter',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'stream_idle',
        message: 'Assistant is still working; waiting for the next stream event.',
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-stream-idle',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('runs the queue-hold dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat('/fixture queue-hold', undefined, 'openrouter:qwen/qwen3.7-plus', onEvent);

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'workflow_state',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'stream_idle',
        message: 'Holding the Assistant turn open so queued follow-ups can be reordered.',
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-queue-hold',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('runs the queued follow-up drain fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture queued-follow-up',
      undefined,
      'openrouter:qwen/qwen3.7-plus',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'workflow_state',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[0][0]).toMatchObject({
      type: 'session',
      data: { id: 'dev-fixture-queue-drain' },
    });
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'workflow_state',
      data: {
        phase: 'context',
        message: 'Replaying queued turn without opening a provider request.',
        tool: 'pulse_query',
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-queue-drain',
        name: 'pulse_query',
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-queue-drain',
        model: 'openrouter:qwen/qwen3.7-plus',
      },
    });
  });

  it('runs the compacted-artifact dev stream fixture without opening a provider request', async () => {
    const onEvent = vi.fn();

    await AIChatAPI.chat(
      '/fixture compacted-artifact',
      undefined,
      'openrouter:deepseek/deepseek-chat',
      onEvent,
    );

    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual([
      'session',
      'workflow_state',
      'content',
      'tool_start',
      'tool_end',
      'content',
      'done',
    ]);
    expect(onEvent.mock.calls[2][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining("I'llcheckthedevicenodes"),
      },
    });
    expect(onEvent.mock.calls[3][0]).toMatchObject({
      type: 'tool_start',
      data: {
        id: 'fixture-tool-compacted',
        name: 'pulse_read',
      },
    });
    expect(onEvent.mock.calls[5][0]).toMatchObject({
      type: 'content',
      data: {
        text: expect.stringContaining('4,358 entries'),
      },
    });
    expect(onEvent.mock.calls[6][0]).toMatchObject({
      type: 'done',
      data: {
        session_id: 'dev-fixture-compacted-artifact',
        model: 'openrouter:deepseek/deepseek-chat',
      },
    });
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
      can_redo: true,
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

  it('undoes and redoes chat turns through explicit session endpoints', async () => {
    const undoResult = {
      success: true,
      session_id: 'session/root',
      restored_prompt: 'show me the affected hosts',
      removed_messages: 2,
      can_redo: true,
    };
    const redoResult = {
      success: true,
      session_id: 'session/root',
      restored_messages: 2,
      can_redo: false,
    };
    apiFetchJSONMock.mockResolvedValueOnce(undoResult).mockResolvedValueOnce(redoResult);

    await expect(AIChatAPI.undoLastTurn('session/root')).resolves.toEqual(undoResult);
    await expect(AIChatAPI.redoLastTurn('session/root')).resolves.toEqual(redoResult);

    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(1, '/api/ai/sessions/session%2Froot/undo', {
      method: 'POST',
    });
    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(2, '/api/ai/sessions/session%2Froot/redo', {
      method: 'POST',
    });
  });

  it('compacts a session through the summarize endpoint', async () => {
    const result = {
      success: true,
      status: 'compacted',
      message: 'Compacted 8 older messages into a session summary.',
      session_id: 'session/root',
      compacted_messages: 8,
      kept_recent_messages: 4,
    };
    apiFetchJSONMock.mockResolvedValueOnce(result);

    await expect(AIChatAPI.summarizeSession('session/root')).resolves.toEqual(result);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/sessions/session%2Froot/summarize', {
      method: 'POST',
    });
  });

  it('does not expose unsupported file-change session operations from the browser client', () => {
    expect('getSessionDiff' in AIChatAPI).toBe(false);
    expect('revertSession' in AIChatAPI).toBe(false);
    expect('unrevertSession' in AIChatAPI).toBe(false);

    expect(aiChatSource).toContain('/summarize');
    expect(aiChatSource).toContain('/fork');
    expect(aiChatSource).toContain('/undo');
    expect(aiChatSource).toContain('/redo');
    expect(aiChatSource).not.toContain('SessionDiff');
    expect(aiChatSource).not.toContain('FileChange');
    expect(aiChatSource).not.toContain('/diff');
    expect(aiChatSource).not.toContain('/revert');
    expect(aiChatSource).not.toContain('/unrevert');
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

  it('rejects stalled chat stream reads instead of emitting synthetic completion', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const read = vi.fn(
      () => new Promise<ReadableStreamReadResult<Uint8Array>>(() => undefined),
    );
    const releaseLock = vi.fn();
    const onEvent = vi.fn();

    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => ({ read, releaseLock }),
      },
    } as unknown as Response);

    const streamPromise = AIChatAPI.chat('hello', undefined, undefined, onEvent);
    const expectedRejection = expect(streamPromise).rejects.toThrow(
      'Pulse Assistant stream timed out waiting for provider data.',
    );
    await flushMicrotasks();
    await vi.advanceTimersByTimeAsync(300000);

    await expectedRejection;
    expect(read).toHaveBeenCalledTimes(1);
    expect(releaseLock).toHaveBeenCalledTimes(1);
    expect(onEvent).not.toHaveBeenCalledWith({ type: 'done' });
    expect(logger.warn).toHaveBeenCalledWith('[AI Chat] Stream timeout');
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
