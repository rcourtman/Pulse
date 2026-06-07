import { describe, expect, it } from 'vitest';
import {
  formatAssistantWorkflowStatus,
  getAssistantActiveTurnStatus,
  withAssistantQueuedFollowUpStatus,
} from '../activeTurnStatus';
import type { ChatMessage } from '../types';

const assistantMessage = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'assistant-1',
  role: 'assistant',
  content: '',
  timestamp: new Date(),
  ...overrides,
});

const userMessage = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'user-1',
  role: 'user',
  content: 'Check the current node',
  timestamp: new Date(),
  ...overrides,
});

describe('getAssistantActiveTurnStatus', () => {
  it('stays quiet when no Assistant turn is active', () => {
    expect(getAssistantActiveTurnStatus([], false)).toBeNull();
  });

  it('shows an active send state before the first assistant event arrives', () => {
    expect(getAssistantActiveTurnStatus([], true)).toEqual({
      type: 'thinking',
      text: 'Sending prompt',
    });
  });

  it('tracks startup timing from the submitted user turn', () => {
    expect(
      getAssistantActiveTurnStatus([userMessage({ timestamp: new Date(1_000) })], true, 4_000),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt',
      startedAt: 1_000,
    });
  });

  it('does not let queued follow-ups reset the active turn startup time', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          userMessage({ id: 'user-1', timestamp: new Date(1_000), delivery: 'sent' }),
          userMessage({
            id: 'user-2',
            content: 'Follow up',
            timestamp: new Date(5_000),
            delivery: 'queued',
          }),
        ],
        true,
        6_000,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Sending prompt · 1 follow-up queued',
      startedAt: 1_000,
    });
  });

  it('applies queued follow-up pressure to an existing active status', () => {
    expect(
      withAssistantQueuedFollowUpStatus(
        {
          type: 'thinking',
          text: 'OpenRouter is starting the response.',
          startedAt: 1_000,
        },
        [
          assistantMessage({ isStreaming: true }),
          userMessage({ id: 'user-2', content: 'First follow-up', delivery: 'queued' }),
          userMessage({ id: 'user-3', content: 'Second follow-up', delivery: 'queued' }),
        ],
      ),
    ).toEqual({
      type: 'thinking',
      text: 'OpenRouter is starting the response. · 2 follow-ups queued',
      startedAt: 1_000,
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
      text: 'Running $ ls /dev | wc -l',
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
      text: 'Running $ ls /dev | wc -l',
    });
  });

  it('redacts sensitive read-only command previews in the active turn status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            pendingTools: [
              {
                id: 'tool-1',
                name: 'pulse_read',
                input:
                  '{"action":"exec","target_host":"current_resource","command":"curl -H \\"Authorization: Bearer secret-token\\" --password hunter2 https://example.local"}',
                status: 'running',
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Running $ curl -H "Authorization: Bearer [redacted-secret]" --password [redacted-secret] https://example.local',
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
              message: 'OpenRouter is starting the response.',
              startedAt,
            },
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'OpenRouter is starting the response.',
      startedAt,
    });
  });

  it('surfaces stream-idle liveness over stale provider-start progress', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: true,
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'provider_start',
                  message: 'OpenRouter is starting the response.',
                  startedAt: 1_000,
                },
                startedAt: 1_000,
                updatedAt: 1_000,
              },
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'stream_idle',
                  message: 'Assistant is still working; waiting for the next stream event.',
                  startedAt: 2_000,
                },
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
      text: 'Assistant is still working; waiting for the next stream event.',
      startedAt: 2_000,
    });
  });

  it('prefers workflow progress over selected model route evidence', () => {
    const startedAt = Date.now() - 1_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: true,
            workflowStatus: {
              phase: 'request_start',
              message: 'Preparing Pulse context.',
              startedAt,
            },
            streamEvents: [
              {
                type: 'model_switch',
                model: 'openrouter:qwen/qwen3.7-plus',
                modelEvent: 'selected',
                startedAt,
                updatedAt: startedAt,
              },
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'request_start',
                  message: 'Preparing Pulse context.',
                  startedAt,
                },
                startedAt,
                updatedAt: startedAt + 50,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Preparing Pulse context.',
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

  it('counts provider retry delay down from workflow start time', () => {
    const startedAt = 1_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'provider_retry',
              message: 'Provider connection failed before any output; retrying.',
              attempt: 2,
              maxAttempts: 2,
              retryAfterMs: 3200,
              startedAt,
            },
          }),
        ],
        true,
        2_300,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Provider connection failed before any output; retrying. · attempt 2/2 · retrying in 1.9s',
      startedAt,
    });
  });

  it('keeps provider retry status explicit when the retry delay has elapsed', () => {
    const startedAt = 1_000;
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'provider_retry',
              message: 'Provider connection failed before any output; retrying.',
              attempt: 2,
              maxAttempts: 2,
              retryAfterMs: 3200,
              startedAt,
            },
          }),
        ],
        true,
        4_500,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Provider connection failed before any output; retrying. · attempt 2/2 · retrying now',
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

  it('surfaces completed tool activity over stale provider-start progress', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: true,
            workflowStatus: {
              phase: 'provider_start',
              message: 'OpenRouter is starting the response.',
              startedAt: 1_000,
            },
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'provider_start',
                  message: 'OpenRouter is starting the response.',
                  startedAt: 1_000,
                },
                startedAt: 1_000,
                updatedAt: 1_000,
              },
              {
                type: 'pending_tool',
                toolId: 'tool-1',
                pendingTool: {
                  id: 'tool-1',
                  name: 'pulse_read',
                  input:
                    '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
                  status: 'running',
                  startedAt: 1_100,
                  updatedAt: 1_100,
                },
                startedAt: 1_100,
                updatedAt: 1_100,
              },
              {
                type: 'tool',
                toolId: 'tool-1',
                tool: {
                  name: 'pulse_read',
                  input:
                    '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
                  output: '4358',
                  success: true,
                },
                startedAt: 1_100,
                updatedAt: 1_300,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Completed $ ls /dev | wc -l',
    });
  });

  it('surfaces failed completed-tool activity over stale provider-start progress', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: true,
            workflowStatus: {
              phase: 'provider_start',
              message: 'OpenRouter is starting the response.',
              startedAt: 1_000,
            },
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'provider_start',
                  message: 'OpenRouter is starting the response.',
                  startedAt: 1_000,
                },
                startedAt: 1_000,
                updatedAt: 1_000,
              },
              {
                type: 'tool',
                toolId: 'tool-1',
                tool: {
                  name: 'pulse_get_nodes',
                  input: '{}',
                  output: 'permission denied',
                  success: false,
                },
                startedAt: 1_100,
                updatedAt: 1_300,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Failed get nodes',
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
      text: 'Preparing governed tools · command',
    });
  });

  it('normalizes internal workflow tool identifiers before display', () => {
    expect(
      formatAssistantWorkflowStatus({
        phase: 'context',
        message: 'Reading current Pulse inventory with pulse_query.',
        tool: 'pulse_query',
      }),
    ).toBe('Reading current Pulse inventory.');

    expect(
      formatAssistantWorkflowStatus({
        phase: 'preflight',
        message: 'Preparing governed tools',
        tool: 'pulse_exec',
      }),
    ).toBe('Preparing governed tools · command');
  });

  it('lets a newer provider switch replace stale waiting progress', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            workflowStatus: {
              phase: 'provider_start',
              message: 'OpenRouter is starting the response.',
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

  it('shows previous and next model routes for explicit route switches', () => {
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
      text: 'Switched from OpenAI: GPT 4o Mini via OpenRouter to DeepSeek: DeepSeek V4 Pro via OpenRouter',
      startedAt: 2_000,
    });
  });

  it('shows the initial selected model route without calling it a fallback', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'model_switch',
                model: 'openrouter:qwen/qwen3.7-plus',
                modelEvent: 'selected',
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
      text: 'Using Qwen: Qwen3.7 Plus via OpenRouter',
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

  it('surfaces skipped tool activity in the active turn status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            streamEvents: [
              {
                type: 'pending_tool',
                toolId: 'tool-1',
                pendingTool: {
                  id: 'tool-1',
                  name: 'pulse_read',
                  input: '{}',
                  status: 'pending',
                  startedAt: 1_000,
                },
                startedAt: 1_000,
                updatedAt: 1_000,
              },
              {
                type: 'tool_cancel',
                toolId: 'tool-1',
                toolCancel: {
                  id: 'tool-1',
                  name: 'pulse_read',
                  input: '{}',
                  reason: 'current_resource unavailable',
                },
                startedAt: 1_000,
                updatedAt: 1_500,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'tool',
      text: 'Skipped read: current_resource unavailable',
      startedAt: 1_000,
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

  it('surfaces queued follow-up pressure in the active status headline', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            content: 'Partial answer',
            isStreaming: true,
          }),
          userMessage({
            id: 'queued-1',
            content: 'Follow up once',
            delivery: 'queued',
          }),
          userMessage({
            id: 'queued-2',
            content: 'Follow up twice',
            delivery: 'queued',
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'generating',
      text: 'Generating response · 2 follow-ups queued',
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

  it('surfaces the latest workflow activity row when it replaces older hidden status', () => {
    expect(
      getAssistantActiveTurnStatus(
        [
          assistantMessage({
            isStreaming: true,
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
                startedAt: 1000,
                updatedAt: 1100,
              },
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'model_thinking',
                  message: 'Model is reasoning before responding.',
                  startedAt: 2000,
                },
                startedAt: 2000,
                updatedAt: 2000,
              },
            ],
          }),
        ],
        true,
      ),
    ).toEqual({
      type: 'thinking',
      text: 'Model is reasoning before responding.',
      startedAt: 2000,
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
