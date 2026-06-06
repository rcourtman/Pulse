import { describe, expect, it } from 'vitest';
import { getAssistantActiveTurnStatus } from '../activeTurnStatus';
import type { ChatMessage } from '../types';

const assistantMessage = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'assistant-1',
  role: 'assistant',
  content: '',
  timestamp: new Date(),
  ...overrides,
});

describe('getAssistantActiveTurnStatus', () => {
  it('stays quiet when no Assistant turn is active', () => {
    expect(getAssistantActiveTurnStatus([], false)).toBeNull();
  });

  it('shows a waiting state before the first assistant event arrives', () => {
    expect(getAssistantActiveTurnStatus([], true)).toEqual({
      type: 'thinking',
      text: 'Waiting for assistant',
    });
  });

  it('surfaces the current pending tool from active message state', () => {
    const startedAt = Date.now() - 5_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [{ id: 'tool-1', name: 'pulse_get_nodes', input: '{}', startedAt }],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Running get nodes',
      startedAt,
    });
  });

  it('carries workflow start time so the live footer can show elapsed wait', () => {
    const startedAt = Date.now() - 8_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'provider_start',
              message: 'Sent request to OpenRouter; waiting for the first token.',
              startedAt,
            },
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Sent request to OpenRouter; waiting for the first token.',
      startedAt,
    });
  });

  it('prefers live tool progress over generic tool labels', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'pending_tool',
                pendingTool: {
                  id: 'tool-1',
                  name: 'pulse_get_inventory',
                  input: '{}',
                  progress: 'Reading Proxmox inventory',
                  status: 'running',
                },
                toolId: 'tool-1',
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Reading Proxmox inventory',
    });
  });

  it('uses latest pending tool activity without requiring transcript reordering', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [
              {
                id: 'tool-a',
                name: 'pulse_get_nodes',
                input: '{}',
                progress: 'Reading node inventory',
                status: 'running',
                startedAt: 100,
                updatedAt: 300,
              },
              {
                id: 'tool-b',
                name: 'pulse_read',
                input: '{}',
                status: 'running',
                startedAt: 200,
                updatedAt: 200,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Reading node inventory',
      startedAt: 100,
    });
  });

  it('keeps showing a remaining pending tool when another parallel tool completes', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'pending_tool',
                pendingTool: {
                  id: 'tool-a',
                  name: 'pulse_get_nodes',
                  input: '{}',
                  status: 'running',
                },
                toolId: 'tool-a',
              },
              {
                type: 'pending_tool',
                pendingTool: {
                  id: 'tool-b',
                  name: 'pulse_read',
                  input: '{}',
                  progress: 'Reading storage layout',
                  status: 'running',
                },
                toolId: 'tool-b',
              },
              {
                type: 'tool',
                toolId: 'tool-a',
                tool: {
                  name: 'pulse_get_nodes',
                  input: '{}',
                  output: 'ok',
                  success: true,
                },
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Reading storage layout',
    });
  });

  it('surfaces neutral workflow progress while a turn is active', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'preflight',
              message: 'Preparing governed tools',
              tool: 'pulse_exec',
            },
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Preparing governed tools · exec',
    });
  });

  it('shows generating status after visible assistant output starts streaming', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            content: 'Partial answer',
            isStreaming: true,
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'generating',
      text: 'Generating response',
    });
  });

  it('surfaces fresh workflow progress after completed tool evidence', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: true,
            workflowStatus: {
              phase: 'model_thinking',
              message: 'Model is reasoning before responding.',
              startedAt: 1000,
            },
            streamEvents: [
              {
                type: 'tool',
                toolId: 'tool-1',
                tool: {
                  name: 'pulse_alerts',
                  input: '{}',
                  output: '11 active alerts',
                  success: true,
                },
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Model is reasoning before responding.',
      startedAt: 1000,
    });
  });

  it('ignores stale workflow status on a completed assistant turn', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: false,
            content: 'Finished answer',
            workflowStatus: {
              phase: 'model_thinking',
              message: 'Model is reasoning before responding.',
              startedAt: 1000,
            },
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'generating',
      text: 'Generating response',
    });
  });
});
