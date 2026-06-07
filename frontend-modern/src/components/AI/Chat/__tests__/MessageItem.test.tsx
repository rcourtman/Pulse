import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { MessageItem } from '../MessageItem';
import type { ChatMessage, PendingApproval, PendingQuestion, StreamDisplayEvent } from '../types';

// Mock child components to isolate MessageItem logic
vi.mock('../ThinkingBlock', () => ({
  ThinkingBlock: (props: {
    content: string;
    isStreaming?: boolean;
    startedAt?: number;
    updatedAt?: number;
  }) => (
    <div
      data-testid="thinking-block"
      data-streaming={props.isStreaming}
      data-started-at={props.startedAt}
      data-updated-at={props.updatedAt}
    >
      {props.isStreaming ? 'Thinking...' : 'Thinking complete'}
    </div>
  ),
}));

vi.mock('../ToolExecutionBlock', () => ({
  PendingToolBlock: (props: { tool: { name: string; input: string } }) => (
    <div data-testid="pending-tool-block" data-tool-name={props.tool.name}>
      {props.tool.input}
    </div>
  ),
  ToolCancellationBlock: (props: { tool: { name: string; input: string; reason?: string } }) => (
    <div role="status" aria-label="Assistant tool canceled">
      <span>{props.tool.name.replace(/^pulse_/, '')}</span>
      <span>skipped</span>
      <span>Inspect devices on current resource</span>
      <span>{props.tool.reason}</span>
    </div>
  ),
  ToolExecutionBlock: (props: {
    tool: { name: string; input: string; output: string; success: boolean };
    startedAt?: number;
    completedAt?: number;
  }) => (
    <div
      data-testid="tool-execution-block"
      data-tool-name={props.tool.name}
      data-started-at={props.startedAt}
      data-completed-at={props.completedAt}
    >
      {props.tool.output}
    </div>
  ),
}));

vi.mock('../ApprovalCard', () => ({
  ApprovalCard: (props: {
    approval: PendingApproval;
    onApprove: () => void;
    onSkip: () => void;
  }) => (
    <div data-testid="approval-card">
      <span>{props.approval.command}</span>
      <button data-testid="approve-btn" onClick={props.onApprove}>
        Approve
      </button>
      <button data-testid="skip-btn" onClick={props.onSkip}>
        Skip
      </button>
    </div>
  ),
}));

vi.mock('../QuestionCard', () => ({
  QuestionCard: (props: {
    question: PendingQuestion;
    onAnswer: (answers: Array<{ id: string; value: string }>) => void;
    onSkip: () => void;
  }) => (
    <div data-testid="question-card">
      <span>{props.question.questionId}</span>
      <button
        data-testid="answer-btn"
        onClick={() => props.onAnswer([{ id: 'q-1', value: 'answer' }])}
      >
        Answer
      </button>
      <button data-testid="skip-question-btn" onClick={props.onSkip}>
        Skip Question
      </button>
    </div>
  ),
}));

vi.mock('../../aiChatUtils', () => ({
  renderMarkdown: (text: string) => `<p>${text}</p>`,
}));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

function makeMessage(overrides?: Partial<ChatMessage>): ChatMessage {
  return {
    id: 'msg-1',
    role: 'assistant',
    content: 'Hello world',
    timestamp: new Date('2026-03-01T12:00:00Z'),
    ...overrides,
  };
}

function makeHandlers() {
  return {
    onApprove: vi.fn(),
    onSkip: vi.fn(),
    onAnswerQuestion: vi.fn(),
    onSkipQuestion: vi.fn(),
  };
}

describe('MessageItem', () => {
  describe('user message rendering', () => {
    it('renders user message in a compact bubble', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'user', content: 'How is the cluster?' })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('How is the cluster?')).toBeInTheDocument();
      // User messages should be right-aligned (flex justify-end)
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper.className).toContain('justify-end');
    });

    it('does not render assistant indicator for user messages', () => {
      render(() => (
        <MessageItem
          message={makeMessage({ role: 'user', content: 'Hello' })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByText('Pulse Assistant')).not.toBeInTheDocument();
    });

    it('renders user content directly (not via markdown)', () => {
      render(() => (
        <MessageItem
          message={makeMessage({ role: 'user', content: 'raw text here' })}
          {...makeHandlers()}
        />
      ));

      // User messages use a plain <p> tag, not innerHTML with markdown
      const p = screen.getByText('raw text here');
      expect(p.tagName).toBe('P');
    });

    it('renders queued user messages with queue position and row actions', () => {
      const onEditQueued = vi.fn();
      const onCancelQueued = vi.fn();
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'user',
            content: 'follow up after this',
            delivery: 'queued',
          })}
          {...makeHandlers()}
          queuedPosition={2}
          queuedCount={3}
          onEditQueued={onEditQueued}
          onCancelQueued={onCancelQueued}
        />
      ));

      expect(screen.getByText('follow up after this')).toBeInTheDocument();
      expect(screen.getByRole('status')).toHaveTextContent('Queued 2 of 3');

      fireEvent.click(screen.getByRole('button', { name: 'Edit queued follow-up' }));
      expect(onEditQueued).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByRole('button', { name: 'Remove queued follow-up' }));
      expect(onCancelQueued).toHaveBeenCalledTimes(1);
    });

    it('renders paused queued user messages distinctly', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'user',
            content: 'follow up after stop',
            delivery: 'queued',
          })}
          {...makeHandlers()}
          queuedPosition={2}
          queuedCount={3}
          queuedPaused
        />
      ));

      expect(screen.getByText('follow up after stop')).toBeInTheDocument();
      expect(screen.getByRole('status')).toHaveTextContent('Paused 2 of 3');
    });
  });

  describe('error block', () => {
    it('renders a distinct error block with the message and a retry button', () => {
      const onRetry = vi.fn();
      const onChangeModel = vi.fn();
      const onUseModelRoute = vi.fn();
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            error: 'The AI provider rejected the request for billing or quota reasons.',
          })}
          {...makeHandlers()}
          onRetry={onRetry}
          onChangeModel={onChangeModel}
          modelRouteAlternative={{
            id: 'openrouter:deepseek/deepseek-v4-pro',
            label: 'DeepSeek: DeepSeek V4 Pro via OpenRouter',
            provider: 'openrouter',
            providerLabel: 'OpenRouter',
          }}
          onUseModelRoute={onUseModelRoute}
        />
      ));

      const alert = screen.getByRole('alert');
      expect(alert).toBeInTheDocument();
      expect(alert.textContent).toContain('billing or quota reasons');

      const retryViaOpenRouter = screen.getByRole('button', {
        name: 'Retry via OpenRouter provider route',
      });
      expect(retryViaOpenRouter).toHaveTextContent('Retry via OpenRouter');
      fireEvent.click(retryViaOpenRouter);
      expect(onUseModelRoute).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro', 'msg-1');

      const changeModel = screen.getByRole('button', { name: /change model/i });
      fireEvent.click(changeModel);
      expect(onChangeModel).toHaveBeenCalledTimes(1);

      const retry = screen.getByRole('button', { name: /try again/i });
      fireEvent.click(retry);
      expect(onRetry).toHaveBeenCalledWith('msg-1');
    });

    it('does not show a retry button when no onRetry handler is provided', () => {
      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', content: '', error: 'Something failed.' })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /try again/i })).not.toBeInTheDocument();
    });

    it('renders no error block when there is no error', () => {
      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', content: 'All good.' })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  describe('interruption marker', () => {
    it('renders a neutral stopped marker without treating it as answer content', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            interruption: 'stopped',
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      const marker = screen.getByRole('status');
      expect(marker).toHaveTextContent('Stopped');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /copy message/i })).not.toBeInTheDocument();
    });

    it('renders a replacement marker when a new user message interrupts the old turn', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            interruption: 'replaced',
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByRole('status')).toHaveTextContent(
        'Stopped when you sent the next message',
      );
    });

    it('keeps partial content visible before the stopped marker', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Partial answer',
            interruption: 'stopped',
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('Partial answer')).toBeInTheDocument();
      expect(screen.getByRole('status')).toHaveTextContent('Stopped');
    });
  });

  describe('copy button', () => {
    it('copies the message content when clicked', () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        configurable: true,
      });

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', content: 'The cluster is healthy.' })}
          {...makeHandlers()}
        />
      ));

      const copy = screen.getByRole('button', { name: /copy message/i });
      fireEvent.click(copy);
      expect(writeText).toHaveBeenCalledWith('The cluster is healthy.');
    });

    it('does not show a copy button while streaming', () => {
      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', content: 'partial', isStreaming: true })}
          {...makeHandlers()}
        />
      ));
      expect(screen.queryByRole('button', { name: /copy message/i })).not.toBeInTheDocument();
    });

    it('does not show a copy button when there is no content', () => {
      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', content: '', error: 'failed' })}
          {...makeHandlers()}
        />
      ));
      expect(screen.queryByRole('button', { name: /copy message/i })).not.toBeInTheDocument();
    });
  });

  describe('assistant message rendering', () => {
    it('renders assistant indicator with label', () => {
      render(() => (
        <MessageItem message={makeMessage({ role: 'assistant' })} {...makeHandlers()} />
      ));

      expect(screen.getByText('Pulse Assistant')).toBeInTheDocument();
    });

    it('renders completed assistant model routes with provider labels', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Done.',
            model: 'openrouter:deepseek/deepseek-v4-pro',
          })}
          getModelRouteLabel={(modelId) =>
            modelId === 'openrouter:deepseek/deepseek-v4-pro'
              ? 'DeepSeek: DeepSeek V4 Pro via OpenRouter'
              : modelId
          }
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('DeepSeek: DeepSeek V4 Pro via OpenRouter')).toBeInTheDocument();
    });

    it('renders completed assistant turn duration without token counts', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            model: 'openrouter:deepseek/deepseek-v4-pro',
            timestamp: new Date('2026-03-01T12:00:00Z'),
            completedAt: new Date('2026-03-01T12:00:04Z'),
            tokens: { input: 500, output: 200 },
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByLabelText('Turn duration 4s')).toBeInTheDocument();
      expect(screen.queryByText('500 in · 200 out')).not.toBeInTheDocument();
    });

    it('does not show turn duration while the assistant is still streaming', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            timestamp: new Date('2026-03-01T12:00:00Z'),
            completedAt: new Date('2026-03-01T12:00:04Z'),
            isStreaming: true,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByLabelText('Turn duration 4s')).not.toBeInTheDocument();
    });

    it('does not use right-alignment for assistant messages', () => {
      const { container } = render(() => (
        <MessageItem message={makeMessage({ role: 'assistant' })} {...makeHandlers()} />
      ));

      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper.className).not.toContain('justify-end');
    });

    it('renders content via markdown when no stream events', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Some **bold** text',
            streamEvents: undefined,
          })}
          {...makeHandlers()}
        />
      ));

      // renderMarkdown mock wraps in <p>
      const rendered = container.querySelector('.prose');
      expect(rendered).toBeInTheDocument();
      expect(rendered!.innerHTML).toContain('<p>Some **bold** text</p>');
    });

    it('shows model name when message is not streaming', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            model: 'claude-3.5-sonnet',
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('claude-3.5-sonnet')).toBeInTheDocument();
    });

    it('keeps the active model route visible while streaming', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            model: 'claude-3.5-sonnet',
            isStreaming: true,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('claude-3.5-sonnet')).toBeInTheDocument();
    });

    it('does not show model name when model is not provided', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            model: undefined,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      // No model text should appear anywhere in the assistant header area
      const assistantLabel = screen.getByText('Pulse Assistant');
      const parent = assistantLabel.parentElement;
      expect(parent?.querySelector('.font-mono')).not.toBeInTheDocument();
    });
  });

  describe('token display', () => {
    it('keeps token counts out of the visible transcript', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            tokens: { input: 500, output: 200 },
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByText('500 in · 200 out')).not.toBeInTheDocument();
    });

    it('does not show token counts when tokens are not provided', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            tokens: undefined,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByText(/in ·/)).not.toBeInTheDocument();
    });
  });

  describe('streaming cursor', () => {
    it('shows cursor when streaming with no pending tools', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            isStreaming: true,
            pendingTools: [],
          })}
          {...makeHandlers()}
        />
      ));

      const cursor = container.querySelector('.animate-pulse');
      expect(cursor).toBeInTheDocument();
    });

    it('does not show cursor when not streaming', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            isStreaming: false,
            pendingTools: [],
          })}
          {...makeHandlers()}
        />
      ));

      const cursor = container.querySelector('.animate-pulse');
      expect(cursor).not.toBeInTheDocument();
    });

    it('shows the first-token thinking indicator before content arrives', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [],
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('Thinking...')).toBeInTheDocument();
      expect(container.querySelector('.animate-bounce')).toBeInTheDocument();
      expect(container.querySelector('.animate-pulse')).not.toBeInTheDocument();
    });

    it('shows workflow progress inline before content arrives', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [],
            workflowStatus: {
              phase: 'plan',
              message: 'Planning governed action and safety checks before execution.',
              state: 'READING',
              tool: 'pulse_exec',
            },
          })}
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByText('Planning governed action and safety checks before execution. · command'),
      ).toBeInTheDocument();
      expect(screen.queryByText('Thinking...')).not.toBeInTheDocument();
    });

    it('renders workflow progress without internal tool identifiers', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'context',
                  message: 'Reading current Pulse inventory with pulse_query.',
                  tool: 'pulse_query',
                },
              },
            ],
            workflowStatus: {
              phase: 'context',
              message: 'Reading current Pulse inventory with pulse_query.',
              tool: 'pulse_query',
            },
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('Reading current Pulse inventory.')).toBeInTheDocument();
      expect(screen.queryByText(/pulse_query/)).not.toBeInTheDocument();
    });

    it('shows live workflow progress as a transcript activity row', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'provider_retry',
                  message: 'Provider connection failed before any output; retrying.',
                  attempt: 2,
                  maxAttempts: 3,
                  retryAfterMs: 1200,
                },
              },
            ],
            workflowStatus: {
              phase: 'provider_retry',
              message: 'Provider connection failed before any output; retrying.',
              attempt: 2,
              maxAttempts: 3,
              retryAfterMs: 1200,
            },
          })}
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByText(
          'Provider connection failed before any output; retrying. · attempt 2/3 · retrying in 1.2s',
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText('Thinking...')).not.toBeInTheDocument();
    });

    it('counts down provider retry workflow rows while the turn is live', () => {
      vi.useFakeTimers();
      vi.setSystemTime(2_300);

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'provider_retry',
                  message: 'Provider connection failed before any output; retrying.',
                  attempt: 2,
                  maxAttempts: 3,
                  retryAfterMs: 3200,
                  startedAt: 1_000,
                },
              },
            ],
            workflowStatus: {
              phase: 'provider_retry',
              message: 'Provider connection failed before any output; retrying.',
              attempt: 2,
              maxAttempts: 3,
              retryAfterMs: 3200,
              startedAt: 1_000,
            },
          })}
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByText(
          /Provider connection failed before any output; retrying\. · attempt 2\/3 · retrying in 1\.9s/,
        ),
      ).toBeInTheDocument();
    });

    it('shows the latest burst workflow activity through one live transcript row', () => {
      const workflowStatusHistory = [
        {
          phase: 'request_start',
          message: 'Preparing Pulse context.',
          startedAt: 1_000,
        },
        {
          phase: 'context',
          message: 'Reading current Pulse inventory with pulse_query.',
          tool: 'pulse_query',
          startedAt: 1_100,
        },
        {
          phase: 'provider_start',
          message: 'Sent request to OpenRouter; waiting for the first token.',
          startedAt: 1_200,
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            pendingTools: [],
            workflowStatusHistory,
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: workflowStatusHistory[2],
              },
            ],
            workflowStatus: workflowStatusHistory[2],
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByText(/Preparing Pulse context\./)).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Reading current Pulse inventory with pulse_query\./),
      ).not.toBeInTheDocument();
      expect(
        screen.getByText(/Sent request to OpenRouter; waiting for the first token\./),
      ).toBeInTheDocument();
    });

    it('hides stale workflow progress after visible content starts', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Partial answer',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [{ type: 'content', content: 'Partial answer' }],
            workflowStatus: {
              phase: 'provider_start',
              message: 'Sent request to OpenRouter; waiting for the first token.',
            },
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('Partial answer')).toBeInTheDocument();
      expect(
        screen.queryByText('Sent request to OpenRouter; waiting for the first token.'),
      ).not.toBeInTheDocument();
      expect(screen.queryByText('Thinking...')).not.toBeInTheDocument();
    });

    it('keeps transcript workflow activity visible before streamed content', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Partial answer',
            isStreaming: true,
            pendingTools: [],
            streamEvents: [
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'provider_start',
                  message: 'Sent request to OpenRouter; waiting for the first token.',
                },
              },
              { type: 'content', content: 'Partial answer' },
            ],
          })}
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByText('Sent request to OpenRouter; waiting for the first token.'),
      ).toBeInTheDocument();
      expect(screen.getByText('Partial answer')).toBeInTheDocument();
      expect(screen.queryByText('Thinking...')).not.toBeInTheDocument();
    });

    it('does not show cursor when streaming but there are pending tools', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            isStreaming: true,
            pendingTools: [{ id: 'tool-1', name: 'run_command', input: '{}' }],
          })}
          {...makeHandlers()}
        />
      ));

      const cursor = container.querySelector('.animate-pulse');
      expect(cursor).not.toBeInTheDocument();
    });
  });

  describe('stream events rendering', () => {
    it('renders neutral thinking progress without raw reasoning text', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'thinking', thinking: 'We need to inspect the user prompt before answering.' },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('thinking-block')).toHaveTextContent('Thinking complete');
      expect(screen.queryByText(/inspect the user prompt/i)).not.toBeInTheDocument();
    });

    it('keeps neutral thinking progress when only thinking has streamed', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'thinking', thinking: 'Hidden reasoning should not be visible.' },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            streamEvents: events,
            isStreaming: true,
            pendingTools: [],
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('Thinking...')).toBeInTheDocument();
      expect(screen.getByTestId('thinking-block')).toHaveAttribute('data-streaming', 'true');
      expect(screen.queryByText(/Hidden reasoning/i)).not.toBeInTheDocument();
    });

    it('renders provider fallback model switches as typed transcript status', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'model_switch', model: 'openrouter:deepseek/deepseek-v4-pro' },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          getModelRouteLabel={(model) =>
            model === 'openrouter:deepseek/deepseek-v4-pro'
              ? 'DeepSeek V4 Pro via OpenRouter'
              : model
          }
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByRole('status', { name: 'Assistant model route changed' }),
      ).toHaveTextContent('Switched to DeepSeek V4 Pro via OpenRouter');
    });

    it('renders the initially selected provider model route as active model use', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'model_switch',
          model: 'openrouter:qwen/qwen3.7-plus',
          modelEvent: 'selected',
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          getModelRouteLabel={(model) =>
            model === 'openrouter:qwen/qwen3.7-plus' ? 'Qwen 3.7 Plus via OpenRouter' : model
          }
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByRole('status', { name: 'Assistant model route selected' }),
      ).toHaveTextContent('Using Qwen 3.7 Plus via OpenRouter');
    });

    it('keeps request-start progress visible next to selected model route evidence', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'model_switch',
          model: 'openrouter:qwen/qwen3.7-plus',
          modelEvent: 'selected',
          startedAt: 1_000,
          updatedAt: 1_000,
        },
        {
          type: 'workflow_status',
          workflowStatus: {
            phase: 'request_start',
            message: 'Preparing Pulse context.',
            startedAt: 1_000,
          },
          startedAt: 1_000,
          updatedAt: 1_050,
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            isStreaming: true,
            streamEvents: events,
            workflowStatus: events[1].workflowStatus,
          })}
          getModelRouteLabel={(model) =>
            model === 'openrouter:qwen/qwen3.7-plus' ? 'Qwen 3.7 Plus via OpenRouter' : model
          }
          {...makeHandlers()}
        />
      ));

      expect(
        screen.getByRole('status', { name: 'Assistant model route selected' }),
      ).toHaveTextContent('Using Qwen 3.7 Plus via OpenRouter');
      expect(screen.getByText(/Preparing Pulse context\./)).toBeInTheDocument();
    });

    it('renders provider fallback model switches with failed and next routes', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'model_switch',
          failedModel: 'openrouter:openai/gpt-4o-mini',
          model: 'gemini:gemini-3.1-flash-lite',
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          getModelRouteLabel={(model) =>
            model === 'openrouter:openai/gpt-4o-mini'
              ? 'OpenAI GPT-4o Mini via OpenRouter'
              : model === 'gemini:gemini-3.1-flash-lite'
                ? 'Gemini 3.1 Flash Lite'
                : model
          }
          {...makeHandlers()}
        />
      ));

      const status = screen.getByRole('status', {
        name: 'Assistant provider fallback route changed',
      });
      expect(status).toHaveTextContent('Provider fallback');
      expect(status).toHaveTextContent('OpenAI GPT-4o Mini via OpenRouter');
      expect(status).toHaveTextContent('Gemini 3.1 Flash Lite');
      expect(status).toHaveAttribute(
        'title',
        'OpenAI GPT-4o Mini via OpenRouter -> Gemini 3.1 Flash Lite',
      );
    });

    it('renders tool execution blocks', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: {
            name: 'pulse_get_nodes',
            input: '{}',
            output: 'node1, node2',
            success: true,
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const block = screen.getByTestId('tool-execution-block');
      expect(block).toBeInTheDocument();
      expect(block.getAttribute('data-tool-name')).toBe('pulse_get_nodes');
    });

    it('passes completed stream timing into completed tool rows', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          startedAt: 1_000,
          updatedAt: 4_200,
          tool: {
            name: 'pulse_get_nodes',
            input: '{}',
            output: 'node1, node2',
            success: true,
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const block = screen.getByTestId('tool-execution-block');
      expect(block).toHaveAttribute('data-started-at', '1000');
      expect(block).toHaveAttribute('data-completed-at', '4200');
    });

    it('renders canceled tool activity as a skipped transcript row', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool_cancel',
          toolId: 'tool-1',
          toolCancel: {
            id: 'tool-1',
            name: 'pulse_read',
            input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
            reason: 'current_resource unavailable',
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', content: '', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const row = screen.getByRole('status', { name: 'Assistant tool canceled' });
      expect(row).toHaveTextContent('read');
      expect(row).toHaveTextContent('skipped');
      expect(row).toHaveTextContent('Inspect devices on current resource');
      expect(row).toHaveTextContent('current_resource unavailable');
    });

    it('does not render tool block when tool property is undefined', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: [{ type: 'tool', tool: undefined } as unknown as StreamDisplayEvent],
          })}
          {...makeHandlers()}
        />
      ));

      // When tool is undefined, the Match condition (evt.type === 'tool' && evt.tool) is falsy
      expect(screen.queryByTestId('tool-execution-block')).not.toBeInTheDocument();
    });

    it('uses fallback values for missing tool fields', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: { name: '', input: '', output: '', success: true },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      // Tool with empty name still renders — the component uses || 'unknown' fallback
      const block = screen.getByTestId('tool-execution-block');
      // name is '' which is falsy, so component falls back to 'unknown'
      expect(block.getAttribute('data-tool-name')).toBe('unknown');
    });

    it('renders content blocks via markdown', () => {
      const events: StreamDisplayEvent[] = [{ type: 'content', content: 'Here is the analysis' }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const prose = screen.getByText('Here is the analysis').closest('.prose');
      expect(prose).toBeInTheDocument();
      expect(prose!.innerHTML).toContain('<p>Here is the analysis</p>');
    });

    it('paces appended streaming content instead of dumping the whole delta at once', async () => {
      vi.useFakeTimers();
      const [content, setContent] = createSignal('A');
      const message = () =>
        makeMessage({
          role: 'assistant',
          content: content(),
          isStreaming: true,
          streamEvents: [{ type: 'content', content: content() }],
        });

      const { container } = render(() => <MessageItem message={message()} {...makeHandlers()} />);
      const prose = () => container.querySelector('.prose') as HTMLElement;

      expect(prose().innerHTML).toContain('<p>A</p>');

      setContent('A readable streaming answer lands over a few frames.');
      await Promise.resolve();

      expect(prose().innerHTML).toContain('<p>A</p>');
      expect(prose().innerHTML).not.toContain('streaming answer lands');

      await vi.runAllTimersAsync();

      expect(prose().innerHTML).toContain('A readable streaming answer lands over a few frames.');
    });

    it('strips serialized tool-call text from content blocks', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'content',
          content:
            'I will inspect the device nodes.\npulse_read(target_host="current_resource", command="lsblk")',
        },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const prose = container.querySelector('.prose');
      expect(prose).toBeInTheDocument();
      expect(prose!.innerHTML).toContain('I will inspect the device nodes.');
      expect(prose!.innerHTML).not.toContain('pulse_read');
      expect(screen.queryByText(/target_host/)).not.toBeInTheDocument();
    });

    it('renders pending_tool events as inline running tool rows', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'pending_tool',
          pendingTool: { id: 'pt-1', name: 'run_command', input: '{"command":"ls /dev"}' },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('pending-tool-block')).toHaveAttribute(
        'data-tool-name',
        'run_command',
      );
      expect(screen.queryByTestId('tool-execution-block')).not.toBeInTheDocument();
    });

    it('renders approval cards with correct callbacks', () => {
      const handlers = makeHandlers();
      const approval: PendingApproval = {
        command: 'systemctl restart nginx',
        toolId: 'tool-abc',
        toolName: 'run_command',
        runOnHost: false,
      };
      const events: StreamDisplayEvent[] = [{ type: 'approval', approval }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...handlers}
        />
      ));

      expect(screen.getByTestId('approval-card')).toBeInTheDocument();
      expect(screen.getByText('systemctl restart nginx')).toBeInTheDocument();
    });

    it('renders question cards', () => {
      const question: PendingQuestion = {
        questionId: 'q-42',
        questions: [{ id: 'q-1', type: 'text', question: 'Which host?' }],
      };
      const events: StreamDisplayEvent[] = [{ type: 'question', question }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('question-card')).toBeInTheDocument();
      expect(screen.getByText('q-42')).toBeInTheDocument();
    });

    it('renders mixed event types in correct DOM order', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'thinking', thinking: 'Analyzing...', startedAt: 1_000, updatedAt: 2_000 },
        { type: 'content', content: 'Step 1' },
        {
          type: 'tool',
          tool: { name: 'pulse_get_nodes', input: '{}', output: 'ok', success: true },
        },
        { type: 'content', content: 'Step 2' },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('thinking-block')).toHaveTextContent('Thinking complete');
      expect(screen.getByTestId('thinking-block')).toHaveAttribute('data-started-at', '1000');
      expect(screen.getByTestId('thinking-block')).toHaveAttribute('data-updated-at', '2000');
      expect(screen.getByTestId('tool-execution-block')).toBeInTheDocument();

      // Verify DOM order: thinking → content(Step 1) → tool → content(Step 2).
      const allBlocks = Array.from(
        container.querySelectorAll(
          '[data-testid="thinking-block"], .prose, [data-testid="tool-execution-block"]',
        ),
      );
      expect(allBlocks.length).toBe(4);
      expect(allBlocks[0].getAttribute('data-testid')).toBe('thinking-block');
      expect(allBlocks[1].classList.contains('prose')).toBe(true);
      expect(allBlocks[1].innerHTML).toContain('Step 1');
      expect(allBlocks[2].getAttribute('data-testid')).toBe('tool-execution-block');
      expect(allBlocks[3].classList.contains('prose')).toBe(true);
      expect(allBlocks[3].innerHTML).toContain('Step 2');
    });
  });

  describe('groupedEvents logic (content merging)', () => {
    it('merges consecutive content events into a single block', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'content', content: 'Hello ' },
        { type: 'content', content: 'world' },
        { type: 'content', content: '!' },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      // All three content chunks should be merged into one prose block
      const proseBlocks = container.querySelectorAll('.prose');
      expect(proseBlocks.length).toBe(1);
      expect(proseBlocks[0].innerHTML).toContain('Hello world!');
    });

    it('merges content into one block across interleaved thinking events', () => {
      // Reasoning models reached via gateways like OpenRouter interleave
      // reasoning and answer tokens. Thinking must not fragment the answer into
      // separate markdown blocks, or whitespace and table structure are lost.
      const events: StreamDisplayEvent[] = [
        { type: 'content', content: 'Part 1 ' },
        { type: 'thinking', thinking: 'hmm...' },
        { type: 'content', content: 'Part 2' },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const proseBlocks = container.querySelectorAll('.prose');
      expect(proseBlocks.length).toBe(1);
      expect(proseBlocks[0].innerHTML).toContain('Part 1 Part 2');
      expect(screen.queryByTestId('thinking-block')).not.toBeInTheDocument();
    });

    it('ignores content events with empty content', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'content', content: '' },
        { type: 'content', content: 'Actual content' },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      // Empty content events are skipped (content is falsy),
      // only the "Actual content" event is added
      const proseBlocks = container.querySelectorAll('.prose');
      expect(proseBlocks.length).toBe(1);
      expect(proseBlocks[0].innerHTML).toContain('Actual content');
    });

    it('handles empty stream events array', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: [],
            content: 'Fallback text',
          })}
          {...makeHandlers()}
        />
      ));

      // With empty streamEvents, hasStreamEvents() is false → shows fallback content
      const prose = container.querySelector('.prose');
      expect(prose).toBeInTheDocument();
      expect(prose!.innerHTML).toContain('Fallback text');
    });

    it('uses fallback content when streamEvents are present but all events are non-renderable', () => {
      // All content events have empty content (falsy), so nothing renders from them.
      const events: StreamDisplayEvent[] = [
        { type: 'content', content: '' },
        { type: 'content', content: '' },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Fallback answer text',
            streamEvents: events,
          })}
          {...makeHandlers()}
        />
      ));

      const proseBlocks = container.querySelectorAll('.prose');
      expect(proseBlocks.length).toBe(1);
      expect(proseBlocks[0].innerHTML).toContain('Fallback answer text');
    });

    it('handles undefined stream events (uses fallback content)', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: undefined,
            content: 'Regular content',
          })}
          {...makeHandlers()}
        />
      ));

      const prose = container.querySelector('.prose');
      expect(prose!.innerHTML).toContain('Regular content');
    });

    it('strips serialized tool-call text from fallback content', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            streamEvents: undefined,
            content:
              'I will inspect the device nodes.\npulse_read(target_host="current_resource", command="lsblk")',
          })}
          {...makeHandlers()}
        />
      ));

      const prose = container.querySelector('.prose');
      expect(prose).toBeInTheDocument();
      expect(prose!.innerHTML).toContain('I will inspect the device nodes.');
      expect(prose!.innerHTML).not.toContain('pulse_read');
    });
  });

  describe('context tools display', () => {
    it('groups consecutive context checks into one expandable transcript row', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
            output: 'ok',
            success: true,
          },
        },
        {
          type: 'tool',
          tool: {
            name: 'pulse_get_metrics',
            input: '{"action":"history","resource":"vm-101"}',
            output: 'ok',
            success: true,
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('context-tool-group')).toHaveTextContent('Context gathered');
      expect(screen.getByTestId('context-tool-group')).toHaveTextContent('2 context checks');
      expect(screen.queryByTestId('tool-execution-block')).not.toBeInTheDocument();
      expect(screen.queryByText('Context used')).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: /Context gathered/i }));

      expect(screen.getAllByTestId('tool-execution-block')).toHaveLength(2);
    });

    it('shows consecutive pending context checks as one active expandable transcript row', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'pending_tool',
          toolId: 'context-resource',
          pendingTool: {
            id: 'context-resource',
            name: 'pulse_get_resource_details',
            input: '{"resource_id":"vm-101"}',
          },
        },
        {
          type: 'pending_tool',
          toolId: 'context-metrics',
          pendingTool: {
            id: 'context-metrics',
            name: 'pulse_get_metrics_history',
            input: '{"resource_id":"vm-101","metric":"cpu"}',
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: true,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('context-tool-group')).toHaveTextContent('Gathering context');
      expect(screen.getByTestId('context-tool-group')).toHaveTextContent('2 context checks');
      expect(screen.queryByTestId('pending-tool-block')).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: /Gathering context/i }));

      expect(screen.getAllByTestId('pending-tool-block')).toHaveLength(2);
    });

    it('keeps action tools visible as individual transcript rows', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: {
            name: 'pulse_run_command',
            input: '{"target_host":"pve-1","command":"systemctl restart pvedaemon"}',
            output: 'queued',
            success: true,
          },
        },
        {
          type: 'tool',
          tool: {
            name: 'pulse_apply_fix',
            input: '{"resource_id":"vm-101","action":"restart_service"}',
            output: 'approval required',
            success: true,
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByTestId('context-tool-group')).not.toBeInTheDocument();
      expect(screen.getAllByTestId('tool-execution-block')).toHaveLength(2);
    });

    it('shows "Context used" with unique tool summaries when context checks are not grouped', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
            output: 'ok',
            success: true,
          },
        },
        { type: 'content', content: 'I checked the device node count.' },
        {
          type: 'tool',
          tool: {
            name: 'pulse_get_metrics',
            input: '{"action":"history","resource":"vm-101"}',
            output: 'ok',
            success: true,
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('Context used')).toBeInTheDocument();
      expect(screen.getByText('Inspect devices on current resource')).toBeInTheDocument();
      expect(screen.getByText('history')).toBeInTheDocument();
      expect(screen.queryByText('read')).not.toBeInTheDocument();
    });

    it('deduplicates tool summaries in context footer', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
            output: 'first',
            success: true,
          },
        },
        { type: 'content', content: 'First context check complete.' },
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: '{"action":"exec","target_host":"current_resource","command":"ls /dev | wc -l"}',
            output: 'second',
            success: true,
          },
        },
        { type: 'content', content: 'Second context check complete.' },
        {
          type: 'tool',
          tool: { name: 'pulse_get_metrics', input: '{}', output: 'ok', success: true },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getAllByText('Inspect devices on current resource')).toHaveLength(1);
      expect(screen.getByText('get metrics')).toBeInTheDocument();
    });

    it('does not show context tools section when streaming', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: { name: 'pulse_get_nodes', input: '{}', output: 'ok', success: true },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: true,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByText('Context used')).not.toBeInTheDocument();
    });

    it('does not show context tools section when no tools used', () => {
      const events: StreamDisplayEvent[] = [{ type: 'content', content: 'Just text, no tools' }];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.queryByText('Context used')).not.toBeInTheDocument();
    });

    it('formats tool names correctly (strips pulse_ prefix and replaces underscores)', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: {
            name: 'pulse_get_container_status',
            input: '{}',
            output: 'ok',
            success: true,
          },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('get container status')).toBeInTheDocument();
    });

    it('handles tool names without pulse_ prefix', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: { name: 'run_command', input: '{}', output: 'ok', success: true },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            streamEvents: events,
            isStreaming: false,
          })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByText('run command')).toBeInTheDocument();
    });
  });

  describe('callback forwarding from approval/question cards', () => {
    it('calls onApprove when approval card approve button is clicked', async () => {
      const handlers = makeHandlers();
      const approval: PendingApproval = {
        command: 'reboot server',
        toolId: 'tool-1',
        toolName: 'run_command',
        runOnHost: true,
        targetHost: 'web-01',
      };
      const events: StreamDisplayEvent[] = [{ type: 'approval', approval }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...handlers}
        />
      ));

      screen.getByTestId('approve-btn').click();
      expect(handlers.onApprove).toHaveBeenCalledWith(approval);
    });

    it('calls onSkip with toolId when approval card skip button is clicked', () => {
      const handlers = makeHandlers();
      const approval: PendingApproval = {
        command: 'dangerous command',
        toolId: 'tool-42',
        toolName: 'run_command',
        runOnHost: false,
      };
      const events: StreamDisplayEvent[] = [{ type: 'approval', approval }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...handlers}
        />
      ));

      screen.getByTestId('skip-btn').click();
      expect(handlers.onSkip).toHaveBeenCalledWith('tool-42');
    });

    it('calls onAnswerQuestion when question card answer button is clicked', () => {
      const handlers = makeHandlers();
      const question: PendingQuestion = {
        questionId: 'q-10',
        questions: [{ id: 'q-1', type: 'text', question: 'Which host?' }],
      };
      const events: StreamDisplayEvent[] = [{ type: 'question', question }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...handlers}
        />
      ));

      screen.getByTestId('answer-btn').click();
      expect(handlers.onAnswerQuestion).toHaveBeenCalledWith(question, [
        { id: 'q-1', value: 'answer' },
      ]);
    });

    it('calls onSkipQuestion with questionId when question card skip is clicked', () => {
      const handlers = makeHandlers();
      const question: PendingQuestion = {
        questionId: 'q-99',
        questions: [{ id: 'q-1', type: 'select', question: 'Pick one' }],
      };
      const events: StreamDisplayEvent[] = [{ type: 'question', question }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...handlers}
        />
      ));

      screen.getByTestId('skip-question-btn').click();
      expect(handlers.onSkipQuestion).toHaveBeenCalledWith('q-99');
    });
  });

  describe('fallback content', () => {
    it('renders fallback content when no stream events exist', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Fallback message',
            streamEvents: undefined,
          })}
          {...makeHandlers()}
        />
      ));

      const prose = container.querySelector('.prose');
      expect(prose).toBeInTheDocument();
      expect(prose!.innerHTML).toContain('Fallback message');
    });

    it('does not render fallback content when stream events exist', () => {
      const events: StreamDisplayEvent[] = [{ type: 'content', content: 'Streamed content' }];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Should NOT appear',
            streamEvents: events,
          })}
          {...makeHandlers()}
        />
      ));

      // Only 1 prose block (from stream event), not the fallback
      const proseBlocks = container.querySelectorAll('.prose');
      expect(proseBlocks.length).toBe(1);
      expect(proseBlocks[0].innerHTML).toContain('Streamed content');
      expect(proseBlocks[0].innerHTML).not.toContain('Should NOT appear');
    });

    it('does not render fallback when content is empty and no stream events', () => {
      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: '',
            streamEvents: undefined,
          })}
          {...makeHandlers()}
        />
      ));

      const prose = container.querySelector('.prose');
      expect(prose).not.toBeInTheDocument();
    });
  });
});
