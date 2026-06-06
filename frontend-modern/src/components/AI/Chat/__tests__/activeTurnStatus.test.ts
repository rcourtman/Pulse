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
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [{ id: 'tool-1', name: 'pulse_get_nodes', input: '{}' }],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Running get nodes',
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
});
