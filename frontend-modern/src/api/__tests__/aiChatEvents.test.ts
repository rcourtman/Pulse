import { describe, expect, it } from 'vitest';
import aiChatEventsSource from '@/api/generated/aiChatEvents.ts?raw';
import type {
  AIChatStreamEvent,
  DoneData,
  SessionData,
  ToolCancelData,
  ToolProgressData,
  ToolStartData,
  WorkflowStateData,
} from '@/api/generated/aiChatEvents';

describe('AI chat stream event contract', () => {
  it('does not expose the retired explore pre-pass stream event', () => {
    expect(aiChatEventsSource).not.toContain('ExploreStatusData');
    expect(aiChatEventsSource).not.toContain("type: 'explore_status'");
  });

  it('exposes the backend-created session event as a typed stream contract', () => {
    const session: SessionData = { id: 'sess-stream' };
    const event: AIChatStreamEvent = { type: 'session', data: session };

    expect(event.data.id).toBe('sess-stream');
    expect(aiChatEventsSource).toContain('export interface SessionData');
    expect(aiChatEventsSource).toContain("type: 'session'");
  });

  it('does not expose automatic provider fallback metadata on workflow state events', () => {
    expect(aiChatEventsSource).not.toContain('provider_fallback');
    expect(aiChatEventsSource).not.toContain('failed_provider?: string');
    expect(aiChatEventsSource).not.toContain('failed_model?: string');
    expect(aiChatEventsSource).not.toContain('next_provider?: string');
    expect(aiChatEventsSource).not.toContain('next_model?: string');
  });

  it('exposes selected provider and model metadata on workflow state events', () => {
    const workflow: WorkflowStateData = {
      phase: 'provider_start',
      message: 'Waiting for assistant.',
      provider: 'openrouter',
      model: 'openrouter:qwen/qwen3.7-plus',
    };
    const event: AIChatStreamEvent = { type: 'workflow_state', data: workflow };

    expect(event.data.provider).toBe('openrouter');
    expect(event.data.model).toBe('openrouter:qwen/qwen3.7-plus');
    expect(aiChatEventsSource).toContain('provider?: string');
    expect(aiChatEventsSource).toContain('model?: string');
  });

  it('exposes provider retry metadata on workflow state events', () => {
    const workflow: WorkflowStateData = {
      phase: 'provider_retry',
      message: 'Selected route connection failed before any output; retrying.',
      state: 'investigating',
      attempt: 2,
      max_attempts: 2,
      retry_after_ms: 200,
    };
    const event: AIChatStreamEvent = { type: 'workflow_state', data: workflow };

    expect(event.data.attempt).toBe(2);
    expect(event.data.max_attempts).toBe(2);
    expect(event.data.retry_after_ms).toBe(200);
    expect(aiChatEventsSource).toContain('attempt?: number');
    expect(aiChatEventsSource).toContain('max_attempts?: number');
    expect(aiChatEventsSource).toContain('retry_after_ms?: number');
  });

  it('exposes live tool progress as a typed stream contract', () => {
    const progress: ToolProgressData = {
      id: 'tool-1',
      name: 'pulse_query',
      input: '{"action":"topology"}',
      raw_input: '{"action":"topology"}',
      phase: 'running',
      message: 'Reading inventory.',
    };
    const event: AIChatStreamEvent = { type: 'tool_progress', data: progress };

    expect(event.data.phase).toBe('running');
    expect(event.data.message).toBe('Reading inventory.');
    expect(aiChatEventsSource).toContain('export interface ToolProgressData');
    expect(aiChatEventsSource).toContain("type: 'tool_progress'");
  });

  it('exposes live tool start state as a typed stream contract', () => {
    const start: ToolStartData = {
      id: 'tool-1',
      name: 'pulse_read',
      input: '{"action":"exec","command":"ls /dev | wc -l"}',
      raw_input: '{"action":"exec","command":"ls /dev | wc -l"}',
      phase: 'running',
      message: 'Reading target.',
    };
    const event: AIChatStreamEvent = { type: 'tool_start', data: start };

    expect(event.data.phase).toBe('running');
    expect(event.data.message).toBe('Reading target.');
    expect(aiChatEventsSource).toContain('export interface ToolStartData');
    expect(aiChatEventsSource).toContain("type: 'tool_start'");
    expect(aiChatEventsSource).toContain('phase?: string');
    expect(aiChatEventsSource).toContain('message?: string');
  });

  it('exposes pending tool cancellation as a typed stream contract', () => {
    const cancel: ToolCancelData = {
      id: 'tool-1',
      name: 'pulse_read',
      reason: 'current_resource unavailable',
    };
    const event: AIChatStreamEvent = { type: 'tool_cancel', data: cancel };

    expect(event.data.reason).toBe('current_resource unavailable');
    expect(aiChatEventsSource).toContain('export interface ToolCancelData');
    expect(aiChatEventsSource).toContain("type: 'tool_cancel'");
  });

  it('exposes the effective completion model on done events', () => {
    const done: DoneData = {
      session_id: 'sess-stream',
      model: 'deepseek:deepseek-v4-pro',
      input_tokens: 904,
      output_tokens: 30,
    };
    const event: AIChatStreamEvent = { type: 'done', data: done };

    expect(event.data?.model).toBe('deepseek:deepseek-v4-pro');
    expect(aiChatEventsSource).toContain('model?: string');
  });
});
