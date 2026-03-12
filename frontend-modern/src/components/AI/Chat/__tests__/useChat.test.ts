import { describe, expect, it, vi, beforeEach } from 'vitest';
import { createRoot } from 'solid-js';

// Mock dependencies before importing
vi.mock('@/api/aiChat', () => ({
  AIChatAPI: {
    createSession: vi.fn(),
    chat: vi.fn(),
    getMessages: vi.fn(),
    answerQuestion: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { useChat } from '../hooks/useChat';
import { AIChatAPI, type StreamEvent } from '@/api/aiChat';
import { notificationStore } from '@/stores/notifications';

const mockCreateSession = AIChatAPI.createSession as ReturnType<typeof vi.fn>;
const mockChat = AIChatAPI.chat as ReturnType<typeof vi.fn>;
const mockGetMessages = AIChatAPI.getMessages as ReturnType<typeof vi.fn>;
const mockAnswerQuestion = AIChatAPI.answerQuestion as ReturnType<typeof vi.fn>;
const mockNotifyError = notificationStore.error as ReturnType<typeof vi.fn>;

/**
 * Helper: run a callback inside a SolidJS reactive root so
 * createSignal / onCleanup work correctly, then dispose.
 */
function withRoot<T>(fn: () => T): { value: T; dispose: () => void } {
  let value!: T;
  const dispose = createRoot((d) => {
    value = fn();
    return d;
  });
  return { value, dispose };
}

describe('useChat', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  // ──────────────────────────────────────────────
  // Initialization
  // ──────────────────────────────────────────────
  describe('initialization', () => {
    it('returns expected API shape', () => {
      const { value: chat, dispose } = withRoot(() => useChat());
      expect(chat.messages()).toEqual([]);
      expect(chat.isLoading()).toBe(false);
      expect(chat.sessionId()).toBe('');
      expect(chat.model()).toBe('');
      expect(typeof chat.sendMessage).toBe('function');
      expect(typeof chat.stop).toBe('function');
      expect(typeof chat.clearMessages).toBe('function');
      expect(typeof chat.loadSession).toBe('function');
      expect(typeof chat.newSession).toBe('function');
      expect(typeof chat.updateApproval).toBe('function');
      expect(typeof chat.addToolResult).toBe('function');
      expect(typeof chat.updateQuestion).toBe('function');
      expect(typeof chat.answerQuestion).toBe('function');
      expect(typeof chat.waitForIdle).toBe('function');
      dispose();
    });

    it('uses provided sessionId and model options', () => {
      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 'sess-123', model: 'gpt-4' }),
      );
      expect(chat.sessionId()).toBe('sess-123');
      expect(chat.model()).toBe('gpt-4');
      dispose();
    });

    it('setModel updates model signal', () => {
      const { value: chat, dispose } = withRoot(() => useChat());
      expect(chat.model()).toBe('');
      chat.setModel('claude-3');
      expect(chat.model()).toBe('claude-3');
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // sendMessage
  // ──────────────────────────────────────────────
  describe('sendMessage', () => {
    it('rejects empty/whitespace prompts', async () => {
      const { value: chat, dispose } = withRoot(() => useChat());
      const result = await chat.sendMessage('   ');
      expect(result).toBe(false);
      expect(mockCreateSession).not.toHaveBeenCalled();
      dispose();
    });

    it('creates session when none exists and sends message', async () => {
      mockCreateSession.mockResolvedValue({ id: 'new-sess' });
      mockChat.mockResolvedValue(undefined);
      const onConversationChanged = vi.fn();

      const { value: chat, dispose } = withRoot(() => useChat({ onConversationChanged }));
      const result = await chat.sendMessage('hello');

      expect(result).toBe(true);
      expect(mockCreateSession).toHaveBeenCalledOnce();
      expect(chat.sessionId()).toBe('new-sess');
      expect(onConversationChanged).toHaveBeenCalledTimes(2);

      // Should have user + assistant messages
      const msgs = chat.messages();
      expect(msgs).toHaveLength(2);
      expect(msgs[0].role).toBe('user');
      expect(msgs[0].content).toBe('hello');
      expect(msgs[1].role).toBe('assistant');
      dispose();
    });

    it('reuses existing session', async () => {
      mockChat.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'existing-sess' }));
      await chat.sendMessage('hello');

      expect(mockCreateSession).not.toHaveBeenCalled();
      expect(mockChat).toHaveBeenCalledOnce();
      expect(mockChat.mock.calls[0][1]).toBe('existing-sess');
      dispose();
    });

    it('returns false and notifies on session creation failure', async () => {
      mockCreateSession.mockRejectedValue(new Error('network error'));

      const { value: chat, dispose } = withRoot(() => useChat());
      const result = await chat.sendMessage('hello');

      expect(result).toBe(false);
      expect(mockNotifyError).toHaveBeenCalledWith('Failed to create chat session');
      expect(chat.messages()).toHaveLength(0);
      dispose();
    });

    it('handles chat API error gracefully', async () => {
      mockChat.mockRejectedValue(new Error('server error'));

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const result = await chat.sendMessage('hello');

      expect(result).toBe(false);
      expect(mockNotifyError).toHaveBeenCalledWith('server error');

      // Assistant message should have error content
      const msgs = chat.messages();
      const assistant = msgs.find((m) => m.role === 'assistant');
      expect(assistant?.content).toContain('Error: server error');
      expect(assistant?.isStreaming).toBe(false);
      expect(chat.isLoading()).toBe(false);
      dispose();
    });

    it('handles AbortError silently (returns false, no notification)', async () => {
      const abortError = new Error('Aborted');
      abortError.name = 'AbortError';
      mockChat.mockRejectedValue(abortError);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const result = await chat.sendMessage('hello');

      expect(result).toBe(false);
      expect(mockNotifyError).not.toHaveBeenCalled();
      dispose();
    });

    it('passes model, mentions, and findingId to API', async () => {
      mockChat.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 'sess', model: 'claude-3' }),
      );
      const mentions = [{ id: 'vm-100', name: 'test-vm', type: 'vm', node: 'node1' }];
      await chat.sendMessage('check this', mentions, 'finding-42');

      const chatCall = mockChat.mock.calls[0];
      expect(chatCall[0]).toBe('check this');
      expect(chatCall[1]).toBe('sess');
      expect(chatCall[2]).toBe('claude-3');
      expect(chatCall[5]).toEqual(mentions);
      expect(chatCall[6]).toBe('finding-42');
      dispose();
    });

    it('aborts current stream when sending mid-stream', async () => {
      // First call: capture signal so we can verify it was aborted
      let capturedSignal: AbortSignal | undefined;
      const abortError = new Error('Aborted');
      abortError.name = 'AbortError';

      mockChat
        .mockImplementationOnce(
          (
            _p: string,
            _s: string,
            _m: string | undefined,
            _onEvent: (e: StreamEvent) => void,
            signal?: AbortSignal,
          ) => {
            capturedSignal = signal;
            // Simulate the abort path: when aborted, reject with AbortError
            return new Promise<void>((_resolve, reject) => {
              signal?.addEventListener('abort', () => reject(abortError));
            });
          },
        )
        .mockResolvedValueOnce(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));

      // Start first message (will hang until aborted)
      const p1 = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 10));

      expect(chat.isLoading()).toBe(true);

      // Send second message which should abort the first
      const p2 = chat.sendMessage('second');

      // First call should have been aborted
      expect(capturedSignal?.aborted).toBe(true);

      const result1 = await p1;
      expect(result1).toBe(false); // Aborted → returns false

      await p2;

      // First assistant message should be marked non-streaming after abort
      const msgs = chat.messages();
      const firstAssistant = msgs.find((m) => m.role === 'assistant');
      expect(firstAssistant?.isStreaming).toBe(false);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // Stream event processing (via sendMessage callback)
  // ──────────────────────────────────────────────
  describe('processEvent (via stream callback)', () => {
    /** Helper that captures the onEvent callback from AIChatAPI.chat */
    function setupWithEventCapture() {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string,
          _model: string | undefined,
          onEvent: (e: StreamEvent) => void,
        ) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      return {
        getFireEvent: () => fireEvent,
      };
    }

    it('processes content events — merges consecutive content', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'content', data: 'Hello ' });
      fire({ type: 'content', data: 'world' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('Hello world');

      // streamEvents should merge consecutive content
      const contentEvents = assistant.streamEvents?.filter((e) => e.type === 'content') ?? [];
      expect(contentEvents).toHaveLength(1);
      expect(contentEvents[0].content).toBe('Hello world');
      dispose();
    });

    it('processes thinking events — merges consecutive thinking', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'thinking', data: 'Let me ' });
      fire({ type: 'thinking', data: 'think...' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.thinking).toBe('Let me think...');

      const thinkingEvents = assistant.streamEvents?.filter((e) => e.type === 'thinking') ?? [];
      expect(thinkingEvents).toHaveLength(1);
      expect(thinkingEvents[0].thinking).toBe('Let me think...');
      dispose();
    });

    it('processes content with nested text/content object', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'content', data: { text: 'from text field' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('from text field');
      dispose();
    });

    it('processes content with { content: string } object', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'content', data: { content: 'from content field' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('from content field');
      dispose();
    });

    it('ignores empty content events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'content', data: '' });
      fire({ type: 'content', data: null });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('');
      dispose();
    });

    it('processes explore_status events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'explore_status',
        data: {
          phase: 'investigating',
          message: 'Checking logs',
          model: 'claude-3',
          outcome: 'ok',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      const exploreEvents =
        assistant.streamEvents?.filter((e) => e.type === 'explore_status') ?? [];
      expect(exploreEvents).toHaveLength(1);
      expect(exploreEvents[0].exploreStatus).toEqual({
        phase: 'investigating',
        message: 'Checking logs',
        model: 'claude-3',
        outcome: 'ok',
      });
      dispose();
    });

    it('ignores explore_status with empty message', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'explore_status', data: { phase: 'x', message: '  ' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      const exploreEvents =
        assistant.streamEvents?.filter((e) => e.type === 'explore_status') ?? [];
      expect(exploreEvents).toHaveLength(0);
      dispose();
    });

    it('processes tool_start events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'get_logs', input: '{}' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.pendingTools![0]).toEqual({
        id: 'tool-1',
        name: 'get_logs',
        input: '{}',
        rawInput: undefined,
      });
      dispose();
    });

    it('skips tool_start for question tools', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'q-1', name: 'question', input: '{}' } });
      fire({ type: 'tool_start', data: { id: 'q-2', name: 'Question', input: '{}' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      dispose();
    });

    it('processes tool_end events — resolves pending tool by ID', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'get_logs', input: '{}' } });
      fire({
        type: 'tool_end',
        data: { id: 'tool-1', name: 'get_logs', input: '{}', output: 'log data', success: true },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.toolCalls).toHaveLength(1);
      expect(assistant.toolCalls![0]).toEqual({
        name: 'get_logs',
        input: '{}',
        output: 'log data',
        success: true,
      });
      dispose();
    });

    it('processes tool_end — resolves by normalized name when no ID', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      // tool_start without backend ID
      fire({ type: 'tool_start', data: { name: 'pulse_get_logs', input: '{}' } });
      // tool_end without ID, but matching normalized name
      fire({
        type: 'tool_end',
        data: { name: 'pulse_get_logs', input: '{}', output: 'ok', success: true },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.toolCalls).toHaveLength(1);
      dispose();
    });

    it('tool_end with approval removes pending_tool and approval from streamEvents', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'restart_vm', input: '{}' } });
      fire({
        type: 'approval_needed',
        data: {
          command: 'qm reboot 100',
          tool_id: 'tool-1',
          tool_name: 'restart_vm',
          run_on_host: true,
          target_host: 'node1',
          approval_id: 'appr-1',
        },
      });
      fire({
        type: 'tool_end',
        data: { id: 'tool-1', name: 'restart_vm', input: '{}', output: 'done', success: true },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.pendingApprovals).toHaveLength(0);
      // Should have tool event but no pending_tool or approval events
      const pendingToolEvents = assistant.streamEvents?.filter((e) => e.type === 'pending_tool');
      expect(pendingToolEvents).toHaveLength(0);
      const approvalEvents = assistant.streamEvents?.filter((e) => e.type === 'approval');
      expect(approvalEvents).toHaveLength(0);
      const toolEvents = assistant.streamEvents?.filter((e) => e.type === 'tool');
      expect(toolEvents).toHaveLength(1);
      dispose();
    });

    it('processes approval_needed events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'approval_needed',
        data: {
          command: 'systemctl restart nginx',
          tool_id: 'tool-99',
          tool_name: 'run_command',
          run_on_host: true,
          target_host: 'web1',
          approval_id: 'appr-5',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingApprovals).toHaveLength(1);
      expect(assistant.pendingApprovals![0]).toEqual({
        command: 'systemctl restart nginx',
        toolId: 'tool-99',
        toolName: 'run_command',
        runOnHost: true,
        targetHost: 'web1',
        isExecuting: false,
        approvalId: 'appr-5',
      });
      dispose();
    });

    it('processes question events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'question',
        data: {
          question_id: 'q-10',
          questions: [
            {
              id: 'q1',
              question: 'Which node?',
              type: 'select',
              options: [{ label: 'Node 1', value: 'n1', description: 'Primary' }],
            },
          ],
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions).toHaveLength(1);
      expect(assistant.pendingQuestions![0].questionId).toBe('q-10');
      expect(assistant.pendingQuestions![0].questions).toHaveLength(1);
      expect(assistant.pendingQuestions![0].questions[0]).toEqual({
        id: 'q1',
        type: 'select',
        question: 'Which node?',
        header: undefined,
        options: [{ label: 'Node 1', value: 'n1', description: 'Primary' }],
      });
      dispose();
    });

    it('infers select type when options are present but type is not "select"', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'question',
        data: {
          question_id: 'q-11',
          questions: [
            {
              id: 'q2',
              question: 'Pick one',
              // no type field, but has options
              options: [{ label: 'A' }, { label: 'B' }],
            },
          ],
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions![0].questions[0].type).toBe('select');
      dispose();
    });

    it('question defaults to text type when no options', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'question',
        data: {
          question_id: 'q-12',
          questions: [{ id: 'q3', question: 'What is the hostname?' }],
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions![0].questions[0].type).toBe('text');
      dispose();
    });

    it('processes done event with tokens', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'content', data: 'response' });
      fire({ type: 'done', data: { input_tokens: 100, output_tokens: 50 } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.isStreaming).toBe(false);
      expect(assistant.tokens).toEqual({ input: 100, output: 50 });
      expect(assistant.pendingTools).toHaveLength(0);
      dispose();
    });

    it('processes done event without tokens', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'done', data: {} });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.isStreaming).toBe(false);
      expect(assistant.tokens).toBeUndefined();
      dispose();
    });

    it('processes done event with alternative token key names', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'done', data: { inputTokens: 200, outputTokens: 80 } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.tokens).toEqual({ input: 200, output: 80 });
      dispose();
    });

    it('processes error events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'error', data: { message: 'Rate limited' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.isStreaming).toBe(false);
      expect(assistant.content).toBe('Error: Rate limited');
      expect(assistant.pendingTools).toHaveLength(0);
      dispose();
    });

    it('processes error event with string data', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'error', data: 'Something went wrong' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('Error: Something went wrong');
      dispose();
    });

    it('error event with empty data produces fallback message', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'error', data: null });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('Error: Request failed');
      dispose();
    });

    it('ignores unknown event types gracefully', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      // @ts-expect-error testing unknown type
      fire({ type: 'unknown_event', data: 'mystery' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      // Message should be unchanged
      expect(assistant.content).toBe('');
      dispose();
    });

    it('interleaves thinking, content, and tool events chronologically', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'thinking', data: 'hmm' });
      fire({ type: 'content', data: 'First, ' });
      fire({ type: 'tool_start', data: { id: 't1', name: 'check', input: '{}' } });
      fire({ type: 'content', data: 'then...' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      const types = assistant.streamEvents!.map((e) => e.type);
      // Thinking, content, pending_tool, content (new content after non-content breaks merging)
      expect(types).toEqual(['thinking', 'content', 'pending_tool', 'content']);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // stop
  // ──────────────────────────────────────────────
  describe('stop', () => {
    it('marks streaming messages as stopped', async () => {
      mockChat.mockImplementation(() => new Promise(() => {})); // never resolves

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      // Start but don't await (hangs)
      chat.sendMessage('hello');
      await new Promise((r) => setTimeout(r, 10));

      expect(chat.isLoading()).toBe(true);

      chat.stop();

      expect(chat.isLoading()).toBe(false);
      const assistant = chat.messages().find((m) => m.role === 'assistant');
      expect(assistant?.isStreaming).toBe(false);
      expect(assistant?.content).toBe('(Stopped)');
      dispose();
    });

    it('preserves existing content when stopping', async () => {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return new Promise(() => {}); // never resolves
        },
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      chat.sendMessage('hi');
      await new Promise((r) => setTimeout(r, 10));

      fireEvent({ type: 'content', data: 'partial response' });

      chat.stop();

      const assistant = chat.messages().find((m) => m.role === 'assistant');
      expect(assistant?.content).toBe('partial response');
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // clearMessages
  // ──────────────────────────────────────────────
  describe('clearMessages', () => {
    it('clears messages and resets session', async () => {
      mockChat.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess-1' }));
      await chat.sendMessage('hello');
      expect(chat.messages().length).toBeGreaterThan(0);

      chat.clearMessages();

      expect(chat.messages()).toEqual([]);
      expect(chat.sessionId()).toBe('');
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // loadSession
  // ──────────────────────────────────────────────
  describe('loadSession', () => {
    it('loads messages from API and sets sessionId', async () => {
      mockGetMessages.mockResolvedValue([
        { id: 'msg-1', role: 'user', content: 'hello', timestamp: '2024-01-01T00:00:00Z' },
        {
          id: 'msg-2',
          role: 'assistant',
          content: 'hi there',
          timestamp: '2024-01-01T00:00:01Z',
          tool_calls: [{ name: 'test', input: '{}', output: 'ok', success: true }],
        },
      ]);

      const { value: chat, dispose } = withRoot(() => useChat());
      await chat.loadSession('sess-42');

      expect(chat.sessionId()).toBe('sess-42');
      const msgs = chat.messages();
      expect(msgs).toHaveLength(2);
      expect(msgs[0].role).toBe('user');
      expect(msgs[0].content).toBe('hello');
      expect(msgs[1].toolCalls).toHaveLength(1);
      expect(msgs[1].timestamp).toBeInstanceOf(Date);
      dispose();
    });

    it('handles load error gracefully', async () => {
      mockGetMessages.mockRejectedValue(new Error('not found'));

      const { value: chat, dispose } = withRoot(() => useChat());
      await chat.loadSession('bad-id');

      expect(mockNotifyError).toHaveBeenCalledWith('Failed to load session');
      expect(chat.messages()).toEqual([]);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // newSession
  // ──────────────────────────────────────────────
  describe('newSession', () => {
    it('creates session and clears messages', async () => {
      mockCreateSession.mockResolvedValue({
        id: 'new-sess-99',
        title: '',
        created_at: '',
        updated_at: '',
        message_count: 0,
      });
      mockChat.mockResolvedValue(undefined);
      const onConversationChanged = vi.fn();

      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 'old', onConversationChanged }),
      );
      await chat.sendMessage('setup');

      const session = await chat.newSession();

      expect(session).toBeDefined();
      expect(session!.id).toBe('new-sess-99');
      expect(chat.sessionId()).toBe('new-sess-99');
      expect(chat.messages()).toEqual([]);
      expect(onConversationChanged).toHaveBeenCalledTimes(2);
      dispose();
    });

    it('returns null on failure and notifies', async () => {
      mockCreateSession.mockRejectedValue(new Error('fail'));

      const { value: chat, dispose } = withRoot(() => useChat());
      const session = await chat.newSession();

      expect(session).toBeNull();
      expect(mockNotifyError).toHaveBeenCalledWith('Failed to create session');
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // updateApproval
  // ──────────────────────────────────────────────
  describe('updateApproval', () => {
    async function setupWithApproval() {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );

      const root = withRoot(() => useChat({ sessionId: 's' }));
      await root.value.sendMessage('hi');

      fireEvent({
        type: 'approval_needed',
        data: {
          command: 'restart',
          tool_id: 'tool-5',
          tool_name: 'restart_vm',
          run_on_host: false,
          approval_id: 'appr-5',
        },
      });

      const msgId = root.value.messages().find((m) => m.role === 'assistant')!.id;
      return { ...root, msgId, fireEvent };
    }

    it('marks approval as executing', async () => {
      const { value: chat, dispose, msgId } = await setupWithApproval();

      chat.updateApproval(msgId, 'tool-5', { isExecuting: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingApprovals![0].isExecuting).toBe(true);
      // Also check streamEvents updated
      const approvalEvent = assistant.streamEvents?.find((e) => e.type === 'approval');
      expect(approvalEvent?.approval?.isExecuting).toBe(true);
      dispose();
    });

    it('removes approval when removed=true', async () => {
      const { value: chat, dispose, msgId } = await setupWithApproval();

      chat.updateApproval(msgId, 'tool-5', { removed: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingApprovals).toHaveLength(0);
      expect(assistant.streamEvents?.filter((e) => e.type === 'approval')).toHaveLength(0);
      dispose();
    });

    it('no-ops for non-matching messageId', async () => {
      const { value: chat, dispose } = await setupWithApproval();

      chat.updateApproval('wrong-id', 'tool-5', { isExecuting: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingApprovals![0].isExecuting).toBe(false);
      dispose();
    });

    it('no-ops for non-matching toolId', async () => {
      const { value: chat, dispose, msgId } = await setupWithApproval();

      chat.updateApproval(msgId, 'wrong-tool', { isExecuting: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingApprovals![0].isExecuting).toBe(false);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // addToolResult
  // ──────────────────────────────────────────────
  describe('addToolResult', () => {
    it('appends tool result to message', async () => {
      mockChat.mockResolvedValue(undefined);
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      await chat.sendMessage('hi');

      const msgId = chat.messages().find((m) => m.role === 'assistant')!.id;
      chat.addToolResult(msgId, {
        name: 'get_status',
        input: '{}',
        output: 'running',
        success: true,
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.toolCalls).toHaveLength(1);
      expect(assistant.toolCalls![0].name).toBe('get_status');
      // Also in streamEvents
      const toolEvents = assistant.streamEvents?.filter((e) => e.type === 'tool');
      expect(toolEvents).toHaveLength(1);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // updateQuestion
  // ──────────────────────────────────────────────
  describe('updateQuestion', () => {
    async function setupWithQuestion() {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );

      const root = withRoot(() => useChat({ sessionId: 's' }));
      await root.value.sendMessage('hi');

      fireEvent({
        type: 'question',
        data: {
          question_id: 'q-20',
          questions: [{ id: 'q1', question: 'Which one?', options: [{ label: 'A' }] }],
        },
      });

      const msgId = root.value.messages().find((m) => m.role === 'assistant')!.id;
      return { ...root, msgId };
    }

    it('marks question as answering', async () => {
      const { value: chat, dispose, msgId } = await setupWithQuestion();

      chat.updateQuestion(msgId, 'q-20', { isAnswering: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions![0].isAnswering).toBe(true);
      const questionEvent = assistant.streamEvents?.find((e) => e.type === 'question');
      expect(questionEvent?.question?.isAnswering).toBe(true);
      dispose();
    });

    it('removes question when removed=true', async () => {
      const { value: chat, dispose, msgId } = await setupWithQuestion();

      chat.updateQuestion(msgId, 'q-20', { removed: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions).toHaveLength(0);
      expect(assistant.streamEvents?.filter((e) => e.type === 'question')).toHaveLength(0);
      dispose();
    });

    it('no-ops for non-matching messageId', async () => {
      const { value: chat, dispose } = await setupWithQuestion();

      chat.updateQuestion('wrong-id', 'q-20', { isAnswering: true });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions![0].isAnswering).toBeFalsy();
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // answerQuestion
  // ──────────────────────────────────────────────
  describe('answerQuestion', () => {
    it('sends answer to API and removes question on success', async () => {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      mockAnswerQuestion.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      await chat.sendMessage('hi');

      fireEvent({
        type: 'question',
        data: {
          question_id: 'q-30',
          questions: [{ id: 'q1', question: 'Pick one' }],
        },
      });

      const msgId = chat.messages().find((m) => m.role === 'assistant')!.id;

      await chat.answerQuestion(msgId, 'q-30', [{ id: 'q1', value: 'yes' }]);

      expect(mockAnswerQuestion).toHaveBeenCalledWith('q-30', [{ id: 'q1', value: 'yes' }]);

      // Question should be removed
      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions).toHaveLength(0);
      dispose();
    });

    it('re-initiates stream when idle after answering (reconnection path)', async () => {
      let fireEvent!: (event: StreamEvent) => void;
      let chatCallCount = 0;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          chatCallCount++;
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      mockAnswerQuestion.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      await chat.sendMessage('hi');

      fireEvent({
        type: 'question',
        data: { question_id: 'q-50', questions: [{ id: 'q1', question: 'Ready?' }] },
      });

      // After sendMessage completes, isLoading should be false (stream ended)
      expect(chat.isLoading()).toBe(false);

      const msgId = chat.messages().find((m) => m.role === 'assistant')!.id;
      const chatCallsBefore = chatCallCount;

      await chat.answerQuestion(msgId, 'q-50', [{ id: 'q1', value: 'yes' }]);

      // Should have called chat again with empty prompt for reconnection
      expect(chatCallCount).toBeGreaterThan(chatCallsBefore);
      // The reconnection call should use empty prompt and same session
      const reconnectCall = mockChat.mock.calls[mockChat.mock.calls.length - 1];
      expect(reconnectCall[0]).toBe('');
      expect(reconnectCall[1]).toBe('s');
      dispose();
    });

    it('handles answer failure — resets isAnswering, shows notification', async () => {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      mockAnswerQuestion.mockRejectedValue(new Error('timeout'));

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      await chat.sendMessage('hi');

      fireEvent({
        type: 'question',
        data: {
          question_id: 'q-40',
          questions: [{ id: 'q1', question: 'Pick' }],
        },
      });

      const msgId = chat.messages().find((m) => m.role === 'assistant')!.id;
      await chat.answerQuestion(msgId, 'q-40', [{ id: 'q1', value: 'x' }]);

      expect(mockNotifyError).toHaveBeenCalledWith('Failed to answer question');

      // Question should still be there, not answering
      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingQuestions).toHaveLength(1);
      expect(assistant.pendingQuestions![0].isAnswering).toBe(false);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // waitForIdle
  // ──────────────────────────────────────────────
  describe('waitForIdle', () => {
    it('resolves immediately when not loading', async () => {
      const { value: chat, dispose } = withRoot(() => useChat());
      const result = await chat.waitForIdle();
      expect(result).toBe(true);
      dispose();
    });

    it('resolves when loading finishes', async () => {
      let chatResolve!: () => void;
      mockChat.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            chatResolve = resolve;
          }),
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      chat.sendMessage('hello');
      await new Promise((r) => setTimeout(r, 10));

      expect(chat.isLoading()).toBe(true);

      const idle = chat.waitForIdle(5000);

      // Resolve the chat call
      chatResolve();
      await new Promise((r) => setTimeout(r, 150));

      const result = await idle;
      expect(result).toBe(true);
      dispose();
    });

    it('resolves false on timeout', async () => {
      mockChat.mockImplementation(() => new Promise(() => {})); // never resolves

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      chat.sendMessage('hello');
      await new Promise((r) => setTimeout(r, 10));

      const result = await chat.waitForIdle(200);
      expect(result).toBe(false);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // extractTokens edge cases (tested via done event)
  // ──────────────────────────────────────────────
  describe('extractTokens edge cases', () => {
    function setupWithEventCapture() {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      return { getFireEvent: () => fireEvent };
    }

    it('handles NaN token values gracefully', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      getFireEvent()({
        type: 'done',
        data: { input_tokens: 'not-a-number', output_tokens: 'bad' },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      // Both NaN → extractTokens returns null → no tokens set
      expect(assistant.tokens).toBeUndefined();
      dispose();
    });

    it('handles zero tokens — no tokens set', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      getFireEvent()({ type: 'done', data: { input_tokens: 0, output_tokens: 0 } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      // 0 tokens → not > 0 → no tokens object
      expect(assistant.tokens).toBeUndefined();
      dispose();
    });

    it('handles negative token values', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      getFireEvent()({ type: 'done', data: { input_tokens: -5, output_tokens: 100 } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      // -5 not > 0 → clamped to 0, 100 > 0 → tokens set
      expect(assistant.tokens).toEqual({ input: 0, output: 100 });
      dispose();
    });

    it('uses shorthand "input"/"output" key names', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      getFireEvent()({ type: 'done', data: { input: 50, output: 25 } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.tokens).toEqual({ input: 50, output: 25 });
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // tool_end with no matching pending tool
  // ──────────────────────────────────────────────
  describe('tool_end with no matching pending tool', () => {
    it('still appends tool to toolCalls even without matching pending', async () => {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      await chat.sendMessage('hi');

      // tool_end without a prior tool_start
      fireEvent({
        type: 'tool_end',
        data: { id: 'orphan', name: 'mystery_tool', input: '{}', output: 'ok', success: true },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.toolCalls).toHaveLength(1);
      expect(assistant.toolCalls![0].name).toBe('mystery_tool');
      // pendingTools should remain empty (nothing to remove)
      expect(assistant.pendingTools).toHaveLength(0);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // Tool name normalization (pulse_ prefix stripping)
  // ──────────────────────────────────────────────
  describe('tool name normalization', () => {
    function setupWithEventCapture() {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      return { getFireEvent: () => fireEvent };
    }

    it('strips doubled pulse_ prefix for matching', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      // Start with doubled prefix, end with single prefix — should match
      fire({ type: 'tool_start', data: { name: 'pulse_pulse_get_logs', input: '{}' } });
      fire({
        type: 'tool_end',
        data: { name: 'pulse_get_logs', input: '{}', output: 'ok', success: true },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.toolCalls).toHaveLength(1);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // Multiple parallel tools
  // ──────────────────────────────────────────────
  describe('parallel tool handling', () => {
    function setupWithEventCapture() {
      let fireEvent!: (event: StreamEvent) => void;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = onEvent;
          return Promise.resolve();
        },
      );
      return { getFireEvent: () => fireEvent };
    }

    it('tracks multiple pending tools and resolves them independently', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'a', name: 'tool_a', input: '{}' } });
      fire({ type: 'tool_start', data: { id: 'b', name: 'tool_b', input: '{}' } });

      let assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(2);

      // Resolve tool B first (out of order)
      fire({
        type: 'tool_end',
        data: { id: 'b', name: 'tool_b', input: '{}', output: 'B done', success: true },
      });

      assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.pendingTools![0].id).toBe('a');
      expect(assistant.toolCalls).toHaveLength(1);
      expect(assistant.toolCalls![0].name).toBe('tool_b');

      // Resolve tool A
      fire({
        type: 'tool_end',
        data: { id: 'a', name: 'tool_a', input: '{}', output: 'A done', success: true },
      });

      assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.toolCalls).toHaveLength(2);
      dispose();
    });
  });
});
