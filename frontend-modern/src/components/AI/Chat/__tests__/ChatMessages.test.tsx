import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { ChatMessages } from '../ChatMessages';
import type { QueuedFollowUp } from '../hooks/useChat';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
} from '../types';

// Capture callback props passed to MessageItem so we can invoke them in tests
let capturedMessageItemProps: Array<{
  message: ChatMessage;
  onApprove: (approval: PendingApproval) => void;
  onSkip: (toolId: string) => void;
  onAnswerQuestion: (
    question: PendingQuestion,
    answers: Array<{ id: string; value: string }>,
  ) => void;
  onSkipQuestion: (questionId: string) => void;
  onChangeModel?: () => void;
  getModelRouteLabel?: (modelId: string) => string;
  modelRouteAlternative?: ModelRouteRecoveryOption | null;
  onUseModelRoute?: (modelId: string, messageId?: string) => void;
  queuedPosition?: number;
  queuedCount?: number;
  queuedPaused?: boolean;
  onEditQueued?: () => void;
  onCancelQueued?: () => void;
}> = [];

vi.mock('../MessageItem', () => ({
  MessageItem: (props: {
    message: ChatMessage;
    onApprove: (approval: PendingApproval) => void;
    onSkip: (toolId: string) => void;
    onAnswerQuestion: (
      question: PendingQuestion,
      answers: Array<{ id: string; value: string }>,
    ) => void;
    onSkipQuestion: (questionId: string) => void;
    onChangeModel?: () => void;
    getModelRouteLabel?: (modelId: string) => string;
    modelRouteAlternative?: ModelRouteRecoveryOption | null;
    onUseModelRoute?: (modelId: string, messageId?: string) => void;
    queuedPosition?: number;
    queuedCount?: number;
    queuedPaused?: boolean;
    onEditQueued?: () => void;
    onCancelQueued?: () => void;
  }) => {
    capturedMessageItemProps.push(props);
    return (
      <div data-testid={`message-item-${props.message.id}`} data-role={props.message.role}>
        {props.message.content}
      </div>
    );
  },
}));

// jsdom does not implement scrollIntoView
beforeEach(() => {
  capturedMessageItemProps = [];
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(cleanup);

function makeMessage(overrides?: Partial<ChatMessage>): ChatMessage {
  return {
    id: 'msg-1',
    role: 'user',
    content: 'Hello, AI!',
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

function setScrollMetrics(
  element: Element,
  metrics: {
    scrollTop: number;
    scrollHeight: number;
    clientHeight: number;
  },
) {
  Object.defineProperty(element, 'scrollTop', {
    configurable: true,
    value: metrics.scrollTop,
    writable: true,
  });
  Object.defineProperty(element, 'scrollHeight', {
    configurable: true,
    value: metrics.scrollHeight,
  });
  Object.defineProperty(element, 'clientHeight', {
    configurable: true,
    value: metrics.clientHeight,
  });
}

describe('ChatMessages', () => {
  describe('empty transcript', () => {
    it('keeps the transcript blank when there are no messages or resume actions', () => {
      const { container } = render(() => <ChatMessages messages={[]} {...makeHandlers()} />);

      expect(screen.queryByText('Ask about your infrastructure')).not.toBeInTheDocument();
      expect(screen.queryByText('Chat with your configured model')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Recent Assistant sessions')).not.toBeInTheDocument();
      expect(container.querySelectorAll('[data-testid^="message-item-"]')).toHaveLength(0);
    });

    it('does not show empty transcript copy when there are messages', () => {
      render(() => <ChatMessages messages={[makeMessage()]} {...makeHandlers()} />);

      expect(screen.queryByText('Ask about your infrastructure')).not.toBeInTheDocument();
    });

    it('shows recent sessions as resume actions in an empty transcript', () => {
      const onLoadSession = vi.fn();
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          recentSessions={[
            {
              id: 'session-1',
              title: 'Storage follow-up',
              created_at: '',
              updated_at: '',
              message_count: 4,
              handoff_summary: {
                kind: 'patrol_finding',
                finding_id: 'finding-1',
                has_model_context: true,
              },
            },
            {
              id: 'session-2',
              title: 'Router question',
              created_at: '',
              updated_at: '',
              message_count: 1,
            },
          ]}
          onLoadSession={onLoadSession}
        />
      ));

      expect(screen.getByLabelText('Recent Assistant sessions')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Resume Storage follow-up' })).toHaveTextContent(
        '4 messages',
      );
      expect(screen.getByRole('button', { name: 'Resume Storage follow-up' })).toHaveTextContent(
        'Patrol Finding',
      );
      expect(screen.getByRole('button', { name: 'Resume Router question' })).toHaveTextContent(
        '1 message',
      );

      fireEvent.click(screen.getByRole('button', { name: 'Resume Storage follow-up' }));
      expect(onLoadSession).toHaveBeenCalledWith('session-1');
    });

    it('does not show recent session resume actions when messages are present', () => {
      render(() => (
        <ChatMessages
          messages={[makeMessage()]}
          {...makeHandlers()}
          recentSessions={[
            {
              id: 'session-1',
              title: 'Storage follow-up',
              created_at: '',
              updated_at: '',
              message_count: 4,
            },
          ]}
          onLoadSession={vi.fn()}
        />
      ));

      expect(screen.queryByLabelText('Recent Assistant sessions')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Resume Storage follow-up' }),
      ).not.toBeInTheDocument();
    });
  });

  describe('message rendering', () => {
    it('renders a single message via MessageItem', () => {
      render(() => (
        <ChatMessages
          messages={[makeMessage({ id: 'msg-1', content: 'Hello world' })]}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('message-item-msg-1')).toBeInTheDocument();
      expect(screen.getByText('Hello world')).toBeInTheDocument();
    });

    it('renders multiple messages in DOM order', () => {
      const messages = [
        makeMessage({ id: 'msg-1', role: 'user', content: 'Question one' }),
        makeMessage({ id: 'msg-2', role: 'assistant', content: 'Answer one' }),
        makeMessage({ id: 'msg-3', role: 'user', content: 'Question two' }),
      ];

      const { container } = render(() => <ChatMessages messages={messages} {...makeHandlers()} />);

      const items = container.querySelectorAll('[data-testid^="message-item-"]');
      expect(items).toHaveLength(3);
      // Verify DOM order matches message order
      expect(items[0]).toHaveAttribute('data-testid', 'message-item-msg-1');
      expect(items[1]).toHaveAttribute('data-testid', 'message-item-msg-2');
      expect(items[2]).toHaveAttribute('data-testid', 'message-item-msg-3');
    });

    it('renders both user and assistant messages', () => {
      const messages = [
        makeMessage({ id: 'user-1', role: 'user', content: 'User message' }),
        makeMessage({ id: 'asst-1', role: 'assistant', content: 'Assistant message' }),
      ];

      render(() => <ChatMessages messages={messages} {...makeHandlers()} />);

      const userMsg = screen.getByTestId('message-item-user-1');
      const asstMsg = screen.getByTestId('message-item-asst-1');
      expect(userMsg).toHaveAttribute('data-role', 'user');
      expect(asstMsg).toHaveAttribute('data-role', 'assistant');
    });

    it('passes queue position and row actions to queued message items', () => {
      const onEditQueuedFollowUp = vi.fn();
      const onCancelQueuedFollowUp = vi.fn();
      const queuedFollowUps: QueuedFollowUp[] = [
        {
          id: 'queue-1',
          messageId: 'queued-user-1',
          prompt: 'first queued turn',
          timestamp: new Date('2026-03-01T12:01:00Z'),
        },
        {
          id: 'queue-2',
          messageId: 'queued-user-2',
          prompt: 'second queued turn',
          timestamp: new Date('2026-03-01T12:02:00Z'),
        },
      ];

      render(() => (
        <ChatMessages
          messages={[
            makeMessage({
              id: 'queued-user-1',
              role: 'user',
              content: 'first queued turn',
              delivery: 'queued',
            }),
            makeMessage({
              id: 'queued-user-2',
              role: 'user',
              content: 'second queued turn',
              delivery: 'queued',
            }),
          ]}
          {...makeHandlers()}
          queuedFollowUps={queuedFollowUps}
          onEditQueuedFollowUp={onEditQueuedFollowUp}
          onCancelQueuedFollowUp={onCancelQueuedFollowUp}
        />
      ));

      const firstQueued = capturedMessageItemProps.find((p) => p.message.id === 'queued-user-1');
      const secondQueued = capturedMessageItemProps.find((p) => p.message.id === 'queued-user-2');

      expect(firstQueued).toMatchObject({ queuedPosition: 1, queuedCount: 2 });
      expect(secondQueued).toMatchObject({ queuedPosition: 2, queuedCount: 2 });
      expect(firstQueued?.queuedPaused).toBe(false);
      expect(secondQueued?.queuedPaused).toBe(false);

      firstQueued?.onEditQueued?.();
      secondQueued?.onCancelQueued?.();

      expect(onEditQueuedFollowUp).toHaveBeenCalledWith('queue-1');
      expect(onCancelQueuedFollowUp).toHaveBeenCalledWith('queue-2');
    });

    it('passes paused queue state to queued message items', () => {
      const queuedFollowUps: QueuedFollowUp[] = [
        {
          id: 'queue-1',
          messageId: 'queued-user-1',
          prompt: 'paused queued turn',
          timestamp: new Date('2026-03-01T12:01:00Z'),
        },
      ];

      render(() => (
        <ChatMessages
          messages={[
            makeMessage({
              id: 'queued-user-1',
              role: 'user',
              content: 'paused queued turn',
              delivery: 'queued',
            }),
          ]}
          {...makeHandlers()}
          queuedFollowUps={queuedFollowUps}
          queuedFollowUpsPaused
        />
      ));

      const queuedItem = capturedMessageItemProps.find((p) => p.message.id === 'queued-user-1');
      expect(queuedItem).toMatchObject({
        queuedPosition: 1,
        queuedCount: 1,
        queuedPaused: true,
      });
    });

    it('renders the scroll anchor element', () => {
      const { container } = render(() => (
        <ChatMessages messages={[makeMessage()]} {...makeHandlers()} />
      ));

      const anchor = container.querySelector('.h-1');
      expect(anchor).toBeInTheDocument();
    });

    it('renders no message items when messages is empty', () => {
      const { container } = render(() => <ChatMessages messages={[]} {...makeHandlers()} />);

      expect(container.querySelectorAll('[data-testid^="message-item-"]')).toHaveLength(0);
    });

    it('renders a large number of messages', () => {
      const messages = Array.from({ length: 50 }, (_, i) =>
        makeMessage({ id: `msg-${i}`, content: `Message ${i}` }),
      );

      const { container } = render(() => <ChatMessages messages={messages} {...makeHandlers()} />);

      expect(container.querySelectorAll('[data-testid^="message-item-"]')).toHaveLength(50);
    });
  });

  describe('callback forwarding', () => {
    it('forwards onApprove with message.id prepended', () => {
      const handlers = makeHandlers();
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-42' })]} {...handlers} />);

      const propsForMsg = capturedMessageItemProps.find((p) => p.message.id === 'msg-42');
      expect(propsForMsg).toBeDefined();

      const fakeApproval: PendingApproval = {
        command: 'systemctl restart nginx',
        toolId: 'tool-abc',
        toolName: 'run_command',
        runOnHost: false,
      };
      propsForMsg!.onApprove(fakeApproval);

      expect(handlers.onApprove).toHaveBeenCalledWith('msg-42', fakeApproval);
    });

    it('forwards onSkip with message.id prepended', () => {
      const handlers = makeHandlers();
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-99' })]} {...handlers} />);

      const propsForMsg = capturedMessageItemProps.find((p) => p.message.id === 'msg-99');
      propsForMsg!.onSkip('tool-xyz');

      expect(handlers.onSkip).toHaveBeenCalledWith('msg-99', 'tool-xyz');
    });

    it('forwards onAnswerQuestion with message.id prepended', () => {
      const handlers = makeHandlers();
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-7' })]} {...handlers} />);

      const propsForMsg = capturedMessageItemProps.find((p) => p.message.id === 'msg-7');
      const fakeQuestion: PendingQuestion = {
        questionId: 'q-1',
        questions: [{ id: 'q-1', type: 'text', question: 'What is the host?' }],
      };
      const fakeAnswers = [{ id: 'q-1', value: 'web-01' }];
      propsForMsg!.onAnswerQuestion(fakeQuestion, fakeAnswers);

      expect(handlers.onAnswerQuestion).toHaveBeenCalledWith('msg-7', fakeQuestion, fakeAnswers);
    });

    it('forwards onSkipQuestion with message.id prepended', () => {
      const handlers = makeHandlers();
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-5' })]} {...handlers} />);

      const propsForMsg = capturedMessageItemProps.find((p) => p.message.id === 'msg-5');
      propsForMsg!.onSkipQuestion('q-77');

      expect(handlers.onSkipQuestion).toHaveBeenCalledWith('msg-5', 'q-77');
    });

    it('forwards onChangeModel unchanged for failed-turn recovery', () => {
      const onChangeModel = vi.fn();
      render(() => (
        <ChatMessages
          messages={[makeMessage({ id: 'msg-model', role: 'assistant', error: 'failed' })]}
          {...makeHandlers()}
          onChangeModel={onChangeModel}
        />
      ));

      const propsForMsg = capturedMessageItemProps.find((p) => p.message.id === 'msg-model');
      expect(propsForMsg?.onChangeModel).toBe(onChangeModel);
    });

    it('forwards message-specific route recovery options for failed turns', () => {
      const routeAlternative: ModelRouteRecoveryOption = {
        id: 'openrouter:deepseek/deepseek-v4-pro',
        label: 'DeepSeek: DeepSeek V4 Pro via OpenRouter',
        provider: 'openrouter',
        providerLabel: 'OpenRouter',
      };
      const getModelRouteAlternative = vi.fn(() => routeAlternative);
      const onUseModelRoute = vi.fn();
      const failedMessage = makeMessage({
        id: 'msg-route',
        role: 'assistant',
        error: 'failed',
      });
      render(() => (
        <ChatMessages
          messages={[failedMessage]}
          {...makeHandlers()}
          getModelRouteAlternative={getModelRouteAlternative}
          onUseModelRoute={onUseModelRoute}
        />
      ));

      const propsForMsg = capturedMessageItemProps.find((p) => p.message.id === 'msg-route');
      const forwardedAlternative = propsForMsg?.modelRouteAlternative;

      expect(getModelRouteAlternative).toHaveBeenCalledWith(failedMessage);
      expect(forwardedAlternative).toBe(routeAlternative);
      expect(propsForMsg?.onUseModelRoute).toBe(onUseModelRoute);
    });

    it('forwards correct message.id when multiple messages exist', () => {
      const handlers = makeHandlers();
      render(() => (
        <ChatMessages
          messages={[makeMessage({ id: 'msg-a' }), makeMessage({ id: 'msg-b' })]}
          {...handlers}
        />
      ));

      const propsA = capturedMessageItemProps.find((p) => p.message.id === 'msg-a');
      const propsB = capturedMessageItemProps.find((p) => p.message.id === 'msg-b');

      propsA!.onSkip('tool-1');
      propsB!.onSkip('tool-2');

      expect(handlers.onSkip).toHaveBeenNthCalledWith(1, 'msg-a', 'tool-1');
      expect(handlers.onSkip).toHaveBeenNthCalledWith(2, 'msg-b', 'tool-2');
    });
  });

  describe('auto-scroll behavior', () => {
    it('calls scrollIntoView when messages are present', () => {
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-1' })]} {...makeHandlers()} />);

      // scrollIntoView should have been called by the createEffect
      expect(Element.prototype.scrollIntoView).toHaveBeenCalled();
    });

    it('reacts to in-place pending tool progress without a new stream event', async () => {
      const [messages, setMessages] = createSignal<ChatMessage[]>([
        makeMessage({
          id: 'assistant-1',
          role: 'assistant',
          content: '',
          isStreaming: true,
          pendingTools: [
            {
              id: 'tool-1',
              name: 'pulse_read',
              input: '{"command":"ls /dev | wc -l"}',
              status: 'running',
              progress: 'Starting command',
              startedAt: 1_000,
              updatedAt: 1_000,
            },
          ],
          streamEvents: [
            {
              type: 'pending_tool',
              toolId: 'tool-1',
              pendingTool: {
                id: 'tool-1',
                name: 'pulse_read',
                input: '{"command":"ls /dev | wc -l"}',
                status: 'running',
                progress: 'Starting command',
                startedAt: 1_000,
                updatedAt: 1_000,
              },
            },
          ],
        }),
      ]);
      render(() => <ChatMessages messages={messages()} {...makeHandlers()} />);

      const scrollIntoView = Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>;
      scrollIntoView.mockClear();

      setMessages([
        makeMessage({
          id: 'assistant-1',
          role: 'assistant',
          content: '',
          isStreaming: true,
          pendingTools: [
            {
              id: 'tool-1',
              name: 'pulse_read',
              input: '{"command":"ls /dev | wc -l"}',
              status: 'running',
              progress: 'Reading device nodes',
              startedAt: 1_000,
              updatedAt: 2_000,
            },
          ],
          streamEvents: [
            {
              type: 'pending_tool',
              toolId: 'tool-1',
              pendingTool: {
                id: 'tool-1',
                name: 'pulse_read',
                input: '{"command":"ls /dev | wc -l"}',
                status: 'running',
                progress: 'Reading device nodes',
                startedAt: 1_000,
                updatedAt: 2_000,
              },
            },
          ],
        }),
      ]);
      await Promise.resolve();

      expect(scrollIntoView).toHaveBeenCalled();
    });

    it('keeps following live output when a large streaming update grows from the bottom', async () => {
      const [messages, setMessages] = createSignal<ChatMessage[]>([
        makeMessage({
          id: 'assistant-1',
          role: 'assistant',
          content: 'Starting',
          isStreaming: true,
        }),
      ]);
      render(() => <ChatMessages messages={messages()} {...makeHandlers()} />);
      const scrollContainer = screen.getByTestId('assistant-message-list');
      const scrollIntoView = Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>;

      setScrollMetrics(scrollContainer, {
        scrollTop: 800,
        scrollHeight: 1000,
        clientHeight: 200,
      });
      fireEvent.scroll(scrollContainer);
      scrollIntoView.mockClear();

      setScrollMetrics(scrollContainer, {
        scrollTop: 800,
        scrollHeight: 1400,
        clientHeight: 200,
      });
      setMessages([
        makeMessage({
          id: 'assistant-1',
          role: 'assistant',
          content: `${'Streaming update. '.repeat(80)}`,
          isStreaming: true,
        }),
      ]);
      await Promise.resolve();

      expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'instant' });
    });

    it('does not pull the transcript back down after the user scrolls away from live output', async () => {
      const [messages, setMessages] = createSignal<ChatMessage[]>([
        makeMessage({
          id: 'assistant-1',
          role: 'assistant',
          content: 'Starting',
          isStreaming: true,
        }),
      ]);
      render(() => <ChatMessages messages={messages()} {...makeHandlers()} />);
      const scrollContainer = screen.getByTestId('assistant-message-list');
      const scrollIntoView = Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>;

      setScrollMetrics(scrollContainer, {
        scrollTop: 100,
        scrollHeight: 1000,
        clientHeight: 200,
      });
      fireEvent.scroll(scrollContainer);
      scrollIntoView.mockClear();

      setMessages([
        makeMessage({
          id: 'assistant-1',
          role: 'assistant',
          content: `${'Streaming update. '.repeat(80)}`,
          isStreaming: true,
        }),
      ]);
      await Promise.resolve();

      expect(scrollIntoView).not.toHaveBeenCalled();
    });

    it('shows a jump to latest control when the user scrolls away from messages', () => {
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-1' })]} {...makeHandlers()} />);
      const scrollContainer = screen.getByTestId('assistant-message-list');

      expect(
        screen.queryByRole('button', { name: 'Jump to latest Assistant message' }),
      ).not.toBeInTheDocument();

      setScrollMetrics(scrollContainer, {
        scrollTop: 100,
        scrollHeight: 1000,
        clientHeight: 200,
      });
      fireEvent.scroll(scrollContainer);

      expect(
        screen.getByRole('button', { name: 'Jump to latest Assistant message' }),
      ).toBeInTheDocument();
    });

    it('jumps back to live output and hides the control when selected', () => {
      render(() => <ChatMessages messages={[makeMessage({ id: 'msg-1' })]} {...makeHandlers()} />);
      const scrollContainer = screen.getByTestId('assistant-message-list');
      const scrollIntoView = Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>;

      setScrollMetrics(scrollContainer, {
        scrollTop: 100,
        scrollHeight: 1000,
        clientHeight: 200,
      });
      fireEvent.scroll(scrollContainer);
      scrollIntoView.mockClear();

      fireEvent.click(screen.getByRole('button', { name: 'Jump to latest Assistant message' }));

      expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth' });
      expect(
        screen.queryByRole('button', { name: 'Jump to latest Assistant message' }),
      ).not.toBeInTheDocument();
    });

    it('does not show the jump to latest control in an empty transcript', () => {
      render(() => <ChatMessages messages={[]} {...makeHandlers()} />);

      expect(
        screen.queryByRole('button', { name: 'Jump to latest Assistant message' }),
      ).not.toBeInTheDocument();
    });

    it('does not call scrollIntoView when messages list is empty', () => {
      // Reset the mock to clear any prior calls
      (Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>).mockClear();

      render(() => <ChatMessages messages={[]} {...makeHandlers()} />);

      expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
    });
  });

  describe('container structure', () => {
    it('renders the scrollable container with correct classes', () => {
      const { container } = render(() => <ChatMessages messages={[]} {...makeHandlers()} />);

      const viewport = container.firstElementChild;
      const scrollContainer = screen.getByTestId('assistant-message-list');
      expect(viewport).toHaveClass('relative', 'flex-1', 'min-h-0');
      expect(scrollContainer).toHaveClass('h-full', 'overflow-y-auto');
      expect(scrollContainer).toHaveAttribute('data-testid', 'assistant-message-list');
    });
  });
});
