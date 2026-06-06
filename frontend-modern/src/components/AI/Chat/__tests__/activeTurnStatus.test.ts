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

  it('surfaces the active pending command instead of the generic tool name', () => {
    const startedAt = Date.now() - 5_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [
              {
                id: 'tool-1',
                name: 'pulse_read',
                input:
                  '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
                status: 'running',
                startedAt,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Running $ ls /dev | wc -l on current resource',
      startedAt,
    });
  });

  it('surfaces provider-style pending command input in the active turn status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'pending_tool',
                pendingTool: {
                  id: 'tool-1',
                  name: 'pulse_read',
                  input: 'pulse_read(target_host="current_resource", command="ls /dev | wc -l")',
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
      text: 'Running $ ls /dev | wc -l on current resource',
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

  it('surfaces provider retry attempt and backoff metadata in the active footer', () => {
    const startedAt = Date.now() - 2_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'provider_retry',
              message: 'Provider connection failed before any output; retrying.',
              attempt: 2,
              maxAttempts: 2,
              retryAfterMs: 200,
              startedAt,
            },
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Provider connection failed before any output; retrying. · attempt 2/2 · retrying in 200ms',
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

  it('lets a newer provider switch replace stale waiting progress', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'provider_start',
              message: 'Sent request to OpenRouter; waiting for the first token.',
              startedAt: 1_000,
            },
            streamEvents: [
              {
                type: 'model_switch',
                model: 'openrouter:deepseek/deepseek-v4-pro',
                startedAt: 2_000,
                updatedAt: 2_000,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Switched to DeepSeek: DeepSeek V4 Pro via OpenRouter',
      startedAt: 2_000,
    });
  });

  it('shows failed and next model routes for provider fallback status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'model_switch',
                failedModel: 'openrouter:openai/gpt-4o-mini',
                model: 'openrouter:deepseek/deepseek-v4-pro',
                startedAt: 2_000,
                updatedAt: 2_000,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Provider fallback: OpenAI: GPT 4o Mini via OpenRouter -> DeepSeek: DeepSeek V4 Pro via OpenRouter',
      startedAt: 2_000,
    });
  });

  it('carries approval event timing into the active waiting status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'content',
                content: 'I found the target service.',
                startedAt: 1_000,
                updatedAt: 1_200,
              },
              {
                type: 'approval',
                startedAt: 2_000,
                updatedAt: 2_000,
                approval: {
                  command: 'systemctl restart nginx',
                  toolId: 'tool-1',
                  toolName: 'pulse_control',
                  runOnHost: true,
                },
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Waiting for approval',
      startedAt: 2_000,
    });
  });

  it('carries question event timing into the active waiting status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'approval',
                startedAt: 1_000,
                updatedAt: 1_000,
                approval: {
                  command: 'systemctl restart nginx',
                  toolId: 'tool-1',
                  toolName: 'pulse_control',
                  runOnHost: true,
                },
              },
              {
                type: 'question',
                startedAt: 2_000,
                updatedAt: 2_000,
                question: {
                  questionId: 'question-1',
                  questions: [{ id: 'target', type: 'text', question: 'Which node?' }],
                },
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Waiting for answer',
      startedAt: 2_000,
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

  it('lets newer streamed content replace an older pending tool footer status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [
              {
                id: 'tool-1',
                name: 'pulse_get_nodes',
                input: '{}',
                progress: 'Reading node inventory',
                status: 'running',
                startedAt: 1_000,
                updatedAt: 1_500,
              },
            ],
            streamEvents: [
              {
                type: 'pending_tool',
                toolId: 'tool-1',
                pendingTool: {
                  id: 'tool-1',
                  name: 'pulse_get_nodes',
                  input: '{}',
                  progress: 'Reading node inventory',
                  status: 'running',
                  startedAt: 1_000,
                  updatedAt: 1_500,
                },
              },
              {
                type: 'content',
                content: 'The node inventory is healthy.',
                startedAt: 2_000,
                updatedAt: 2_500,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'generating',
      text: 'Generating response',
      startedAt: 2_000,
    });
  });

  it('lets a newer in-place tool progress patch replace content footer status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [
              {
                id: 'tool-1',
                name: 'pulse_read',
                input: '{}',
                progress: 'Reading storage layout',
                status: 'running',
                startedAt: 1_000,
                updatedAt: 3_000,
              },
            ],
            streamEvents: [
              {
                type: 'pending_tool',
                toolId: 'tool-1',
                pendingTool: {
                  id: 'tool-1',
                  name: 'pulse_read',
                  input: '{}',
                  progress: 'Reading storage layout',
                  status: 'running',
                  startedAt: 1_000,
                  updatedAt: 3_000,
                },
              },
              {
                type: 'content',
                content: 'I found the node list.',
                startedAt: 2_000,
                updatedAt: 2_500,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Reading storage layout',
      startedAt: 1_000,
    });
  });

  it('surfaces newer hidden reasoning metadata in the footer after content', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'content',
                content: 'The device count is',
                startedAt: 1_000,
                updatedAt: 1_500,
              },
              {
                type: 'thinking',
                thinking: '**Checking device nodes**\n\nHidden reasoning body.',
                startedAt: 2_000,
                updatedAt: 2_500,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Thinking: Checking device nodes',
      startedAt: 2_000,
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
