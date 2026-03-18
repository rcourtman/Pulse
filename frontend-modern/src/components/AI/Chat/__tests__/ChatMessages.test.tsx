import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { ChatMessages } from '../ChatMessages';
import type { ChatMessage, PendingApproval, PendingQuestion } from '../types';

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

describe('ChatMessages', () => {
  describe('empty state', () => {
    it('shows empty state when no messages and emptyState provided', () => {
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{ title: 'Welcome to Pulse AI' }}
        />
      ));

      expect(screen.getByText('Welcome to Pulse AI')).toBeInTheDocument();
    });

    it('shows subtitle when provided in emptyState', () => {
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            subtitle: 'Ask me anything about your infrastructure',
          }}
        />
      ));

      expect(screen.getByText('Ask me anything about your infrastructure')).toBeInTheDocument();
    });

    it('does not show subtitle when not provided', () => {
      render(() => (
        <ChatMessages messages={[]} {...makeHandlers()} emptyState={{ title: 'Welcome' }} />
      ));

      expect(screen.getByText('Welcome')).toBeInTheDocument();
      expect(screen.queryByText('Ask me anything')).not.toBeInTheDocument();
    });

    it('renders suggestion buttons when suggestions provided', () => {
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            suggestions: ['Show disk usage', 'Check CPU load'],
          }}
        />
      ));

      expect(screen.getByText('Show disk usage')).toBeInTheDocument();
      expect(screen.getByText('Check CPU load')).toBeInTheDocument();
    });

    it('calls onSuggestionClick when a suggestion is clicked', () => {
      const onSuggestionClick = vi.fn();
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            suggestions: ['Show disk usage', 'Check CPU load'],
            onSuggestionClick,
          }}
        />
      ));

      fireEvent.click(screen.getByText('Show disk usage'));
      expect(onSuggestionClick).toHaveBeenCalledWith('Show disk usage');
      expect(onSuggestionClick).toHaveBeenCalledTimes(1);
    });

    it('calls onSuggestionClick with the correct suggestion text', () => {
      const onSuggestionClick = vi.fn();
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            suggestions: ['Show disk usage', 'Check CPU load'],
            onSuggestionClick,
          }}
        />
      ));

      fireEvent.click(screen.getByText('Check CPU load'));
      expect(onSuggestionClick).toHaveBeenCalledWith('Check CPU load');
    });

    it('does not render suggestions section when suggestions array is empty', () => {
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            suggestions: [],
          }}
        />
      ));

      expect(screen.queryByText('Or try asking')).not.toBeInTheDocument();
    });

    it('renders "Or try asking" label when suggestions exist', () => {
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            suggestions: ['Check nodes'],
          }}
        />
      ));

      expect(screen.getByText('Or try asking')).toBeInTheDocument();
    });

    it('does not show empty state when there are messages', () => {
      render(() => (
        <ChatMessages
          messages={[makeMessage()]}
          {...makeHandlers()}
          emptyState={{ title: 'Welcome to Pulse AI' }}
        />
      ));

      expect(screen.queryByText('Welcome to Pulse AI')).not.toBeInTheDocument();
    });

    it('does not show empty state when emptyState prop is not provided', () => {
      render(() => <ChatMessages messages={[]} {...makeHandlers()} />);

      expect(screen.queryByText('Welcome')).not.toBeInTheDocument();
      expect(screen.queryByText('Or try asking')).not.toBeInTheDocument();
    });

    it('each suggestion button is individually clickable', () => {
      const onSuggestionClick = vi.fn();
      render(() => (
        <ChatMessages
          messages={[]}
          {...makeHandlers()}
          emptyState={{
            title: 'Welcome',
            suggestions: ['A', 'B', 'C'],
            onSuggestionClick,
          }}
        />
      ));

      fireEvent.click(screen.getByText('A'));
      fireEvent.click(screen.getByText('B'));
      fireEvent.click(screen.getByText('C'));

      expect(onSuggestionClick).toHaveBeenCalledTimes(3);
      expect(onSuggestionClick).toHaveBeenNthCalledWith(1, 'A');
      expect(onSuggestionClick).toHaveBeenNthCalledWith(2, 'B');
      expect(onSuggestionClick).toHaveBeenNthCalledWith(3, 'C');
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

    it('renders the scroll anchor element', () => {
      const { container } = render(() => (
        <ChatMessages messages={[makeMessage()]} {...makeHandlers()} />
      ));

      const anchor = container.querySelector('.h-1');
      expect(anchor).toBeInTheDocument();
    });

    it('renders no message items when messages is empty and no emptyState', () => {
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

      const scrollContainer = container.firstElementChild;
      expect(scrollContainer).toHaveClass('flex-1', 'overflow-y-auto');
    });
  });
});
