import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { MessageItem } from '../MessageItem';
import type { ChatMessage, PendingApproval, PendingQuestion, StreamDisplayEvent } from '../types';

// Mock child components to isolate MessageItem logic
vi.mock('../ThinkingBlock', () => ({
  ThinkingBlock: (props: { content: string; isStreaming?: boolean }) => (
    <div data-testid="thinking-block" data-streaming={props.isStreaming}>
      {props.content}
    </div>
  ),
}));

vi.mock('../ExploreStatusBlock', () => ({
  ExploreStatusBlock: (props: { status: { phase: string; message: string } }) => (
    <div data-testid="explore-status-block">{props.status.message}</div>
  ),
}));

vi.mock('../ToolExecutionBlock', () => ({
  ToolExecutionBlock: (props: {
    tool: { name: string; input: string; output: string; success: boolean };
  }) => (
    <div data-testid="tool-execution-block" data-tool-name={props.tool.name}>
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

afterEach(cleanup);

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

      expect(screen.queryByText('Assistant')).not.toBeInTheDocument();
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
  });

  describe('assistant message rendering', () => {
    it('renders assistant indicator with label', () => {
      render(() => (
        <MessageItem message={makeMessage({ role: 'assistant' })} {...makeHandlers()} />
      ));

      expect(screen.getByText('Assistant')).toBeInTheDocument();
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

    it('does not show model name when message is streaming', () => {
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

      expect(screen.queryByText('claude-3.5-sonnet')).not.toBeInTheDocument();
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
      const assistantLabel = screen.getByText('Assistant');
      const parent = assistantLabel.parentElement;
      expect(parent?.querySelector('.font-mono')).not.toBeInTheDocument();
    });
  });

  describe('token display', () => {
    it('shows token counts when tokens are provided and not streaming', () => {
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

      expect(screen.getByText('500 in · 200 out')).toBeInTheDocument();
    });

    it('does not show token counts when streaming', () => {
      render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            tokens: { input: 500, output: 200 },
            isStreaming: true,
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
    it('renders thinking blocks from stream events', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'thinking', thinking: 'Let me analyze this...' },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('thinking-block')).toBeInTheDocument();
      expect(screen.getByText('Let me analyze this...')).toBeInTheDocument();
    });

    it('passes isStreaming to ThinkingBlock', () => {
      const events: StreamDisplayEvent[] = [{ type: 'thinking', thinking: 'Thinking...' }];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events, isStreaming: true })}
          {...makeHandlers()}
        />
      ));

      const block = screen.getByTestId('thinking-block');
      expect(block.getAttribute('data-streaming')).toBe('true');
    });

    it('renders explore status blocks', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'explore_status',
          exploreStatus: { phase: 'scanning', message: 'Scanning infrastructure...' },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      expect(screen.getByTestId('explore-status-block')).toBeInTheDocument();
      expect(screen.getByText('Scanning infrastructure...')).toBeInTheDocument();
    });

    it('renders tool execution blocks', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: { name: 'pulse_get_nodes', input: '{}', output: 'node1, node2', success: true },
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

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      const prose = container.querySelector('.prose');
      expect(prose).toBeInTheDocument();
      expect(prose!.innerHTML).toContain('<p>Here is the analysis</p>');
    });

    it('renders pending_tool events as empty fragments (no visible output)', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'pending_tool',
          pendingTool: { id: 'pt-1', name: 'run_command', input: '{}' },
        },
      ];

      render(() => (
        <MessageItem
          message={makeMessage({ role: 'assistant', streamEvents: events })}
          {...makeHandlers()}
        />
      ));

      // pending_tool renders <></> (empty fragment) - no tool block visible
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
        { type: 'thinking', thinking: 'Analyzing...' },
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

      expect(screen.getByTestId('thinking-block')).toBeInTheDocument();
      expect(screen.getByTestId('tool-execution-block')).toBeInTheDocument();

      // Verify DOM order: thinking → content(Step 1) → tool → content(Step 2)
      const contentArea = container.querySelector('.pl-1')!;
      const allBlocks = Array.from(
        contentArea.querySelectorAll(
          '[data-testid="thinking-block"], .prose, [data-testid="tool-execution-block"]',
        ),
      );
      expect(allBlocks.length).toBe(4); // thinking + 2 prose + 1 tool
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

    it('does not merge content events separated by other event types', () => {
      const events: StreamDisplayEvent[] = [
        { type: 'content', content: 'Part 1' },
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
      expect(proseBlocks.length).toBe(2);
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

    it('suppresses fallback content when streamEvents is non-empty but all events are non-renderable', () => {
      // All content events have empty content (falsy), so nothing renders from them.
      // But hasStreamEvents() is true because the array is non-empty,
      // so the fallback content path is also suppressed.
      const events: StreamDisplayEvent[] = [
        { type: 'content', content: '' },
        { type: 'content', content: '' },
      ];

      const { container } = render(() => (
        <MessageItem
          message={makeMessage({
            role: 'assistant',
            content: 'Fallback that should NOT appear',
            streamEvents: events,
          })}
          {...makeHandlers()}
        />
      ));

      // No prose blocks rendered (empty content events are skipped by groupedEvents)
      const proseBlocks = container.querySelectorAll('.prose');
      expect(proseBlocks.length).toBe(0);
      // Fallback is also suppressed because hasStreamEvents() is true
      expect(screen.queryByText('Fallback that should NOT appear')).not.toBeInTheDocument();
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
  });

  describe('context tools display', () => {
    it('shows "Context used" with unique tool names when not streaming', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: { name: 'pulse_get_nodes', input: '{}', output: 'ok', success: true },
        },
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

      expect(screen.getByText('Context used')).toBeInTheDocument();
      // formatToolName strips 'pulse_' and replaces underscores
      expect(screen.getByText('get nodes')).toBeInTheDocument();
      expect(screen.getByText('get metrics')).toBeInTheDocument();
    });

    it('deduplicates tool names in context footer', () => {
      const events: StreamDisplayEvent[] = [
        {
          type: 'tool',
          tool: { name: 'pulse_get_nodes', input: '{}', output: 'first', success: true },
        },
        {
          type: 'tool',
          tool: { name: 'pulse_get_nodes', input: '{}', output: 'second', success: true },
        },
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

      // Should show 2 unique tool names, not 3
      const toolLabels = screen.getAllByText(/get (nodes|metrics)/);
      expect(toolLabels.length).toBe(2);
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
