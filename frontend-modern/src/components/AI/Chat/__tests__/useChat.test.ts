import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { createEffect, createRoot } from 'solid-js';

// Mock dependencies before importing
vi.mock('@/api/aiChat', () => ({
  AIChatAPI: {
    createSession: vi.fn(),
    chat: vi.fn(),
    getMessages: vi.fn(),
    abortSession: vi.fn(),
    answerQuestion: vi.fn(),
    undoLastTurn: vi.fn(),
    redoLastTurn: vi.fn(),
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
import { AIChatAPI, type ChatMention, type StreamEvent } from '@/api/aiChat';
import { notificationStore } from '@/stores/notifications';

const mockCreateSession = AIChatAPI.createSession as ReturnType<typeof vi.fn>;
const mockChat = AIChatAPI.chat as ReturnType<typeof vi.fn>;
const mockGetMessages = AIChatAPI.getMessages as ReturnType<typeof vi.fn>;
const mockAbortSession = AIChatAPI.abortSession as ReturnType<typeof vi.fn>;
const mockAnswerQuestion = AIChatAPI.answerQuestion as ReturnType<typeof vi.fn>;
const mockUndoLastTurn = AIChatAPI.undoLastTurn as ReturnType<typeof vi.fn>;
const mockRedoLastTurn = AIChatAPI.redoLastTurn as ReturnType<typeof vi.fn>;
const mockNotifyError = notificationStore.error as ReturnType<typeof vi.fn>;

type TestStreamEvent = StreamEvent | { type: string; data?: unknown };
type TestStreamDispatch = (event: TestStreamEvent) => void;

function dispatchTestStreamEvent(
  onEvent: (event: StreamEvent) => void,
  event: TestStreamEvent,
): void {
  onEvent(event as StreamEvent);
}

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
    mockAbortSession.mockResolvedValue(undefined);
    mockUndoLastTurn.mockResolvedValue({
      success: false,
      session_id: 's',
      can_redo: false,
      message: 'No user turn to undo.',
    });
    mockRedoLastTurn.mockResolvedValue({
      success: false,
      session_id: 's',
      can_redo: false,
      message: 'No undone turn to redo.',
    });
  });

  afterEach(() => {
    vi.useRealTimers();
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
      expect(chat.queuedFollowUps()).toEqual([]);
      expect(chat.queuedFollowUpsPaused()).toBe(false);
      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(typeof chat.sendMessage).toBe('function');
      expect(typeof chat.retryMessage).toBe('function');
      expect(typeof chat.undoLastTurn).toBe('function');
      expect(typeof chat.redoLastTurn).toBe('function');
      expect(typeof chat.stop).toBe('function');
      expect(typeof chat.cancelQueuedFollowUp).toBe('function');
      expect(typeof chat.takeQueuedFollowUp).toBe('function');
      expect(typeof chat.queuedFollowUpsPaused).toBe('function');
      expect(typeof chat.sendQueuedFollowUpNow).toBe('function');
      expect(typeof chat.clearQueuedFollowUps).toBe('function');
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

    it('streams cold messages without precreating a session and binds the stream session', async () => {
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string | undefined,
          _model: string | undefined,
          onEvent: (event: StreamEvent) => void,
        ) => {
          onEvent({ type: 'session', data: { id: 'new-sess' } } as StreamEvent);
          return Promise.resolve();
        },
      );
      const onConversationChanged = vi.fn();

      const { value: chat, dispose } = withRoot(() => useChat({ onConversationChanged }));
      const result = await chat.sendMessage('hello');

      expect(result).toBe(true);
      expect(mockCreateSession).not.toHaveBeenCalled();
      expect(mockChat).toHaveBeenCalledOnce();
      expect(mockChat.mock.calls[0][1]).toBeUndefined();
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

    it('starts the cold chat stream immediately with a pending assistant turn', async () => {
      let resolveChat!: () => void;
      mockChat.mockReturnValue(
        new Promise<void>((resolve) => {
          resolveChat = resolve;
        }),
      );

      const { value: chat, dispose } = withRoot(() => useChat());
      const result = chat.sendMessage('test');

      expect(chat.isLoading()).toBe(true);
      expect(chat.messages()).toHaveLength(2);
      expect(chat.messages()[0]).toMatchObject({ role: 'user', content: 'test' });
      expect(chat.messages()[1]).toMatchObject({
        role: 'assistant',
        content: '',
        isStreaming: true,
        pendingTools: [],
        workflowStatusHistory: [],
        workflowStatus: {
          phase: 'request_send',
          message: 'Sending prompt.',
        },
      });
      expect(mockCreateSession).not.toHaveBeenCalled();
      expect(mockChat).toHaveBeenCalledOnce();
      expect(mockChat.mock.calls[0][1]).toBeUndefined();

      resolveChat();
      await result;

      expect(chat.messages()).toHaveLength(2);
      expect(chat.messages()[1].role).toBe('assistant');
      dispose();
    });

    it('clears the visible active turn before conversation refresh finishes', async () => {
      let resolveRefresh!: () => void;
      let sendResolved = false;
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string | undefined,
          _model: string | undefined,
          onEvent: (event: StreamEvent) => void,
        ) => {
          onEvent({
            type: 'content',
            data: { text: 'Pulse currently sees 33 compute resources.' },
          } as StreamEvent);
          onEvent({
            type: 'done',
            data: { model: 'pulse:local-inventory' },
          } as StreamEvent);
          return Promise.resolve();
        },
      );
      const onConversationChanged = vi.fn(
        () =>
          new Promise<void>((resolve) => {
            resolveRefresh = resolve;
          }),
      );

      const { value: chat, dispose } = withRoot(() => useChat({ onConversationChanged }));
      const sendPromise = chat.sendMessage('how many devices in this').then((result) => {
        sendResolved = true;
        return result;
      });

      await new Promise((resolve) => setTimeout(resolve, 0));

      expect(chat.isLoading()).toBe(false);
      expect(chat.messages()[1]).toMatchObject({
        isStreaming: false,
        model: 'pulse:local-inventory',
      });
      expect(sendResolved).toBe(false);

      resolveRefresh();
      await expect(sendPromise).resolves.toBe(true);
      dispose();
    });

    it('uses the configured default model route for the request and assistant turn', async () => {
      mockChat.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() =>
        useChat({ defaultModel: () => 'deepseek:deepseek-v4-pro' }),
      );
      await chat.sendMessage('hello');

      expect(mockChat).toHaveBeenCalledOnce();
      expect(mockChat.mock.calls[0][2]).toBe('deepseek:deepseek-v4-pro');
      expect(chat.messages()[1]).toMatchObject({
        role: 'assistant',
        model: 'deepseek:deepseek-v4-pro',
      });
      dispose();
    });

    it('shows the selected model route immediately while the first backend event is pending', async () => {
      let resolveChat!: () => void;
      mockChat.mockReturnValue(
        new Promise<void>((resolve) => {
          resolveChat = resolve;
        }),
      );

      const { value: chat, dispose } = withRoot(() =>
        useChat({ defaultModel: () => 'openrouter:qwen/qwen3.7-plus' }),
      );
      const result = chat.sendMessage('hello');

      const assistant = chat.messages()[1];
      expect(assistant).toMatchObject({
        role: 'assistant',
        model: 'openrouter:qwen/qwen3.7-plus',
        isStreaming: true,
        workflowStatusHistory: [],
        workflowStatus: {
          phase: 'request_send',
          message: 'Sending prompt.',
        },
      });
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'model_switch',
          model: 'openrouter:qwen/qwen3.7-plus',
          modelEvent: 'selected',
        }),
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'request_send',
            message: 'Sending prompt.',
          }),
        }),
      ]);

      resolveChat();
      await result;
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

    it('handles cold chat API error without precreating a session', async () => {
      mockChat.mockRejectedValue(new Error('server error'));

      const { value: chat, dispose } = withRoot(() => useChat());
      const result = await chat.sendMessage('hello');

      expect(result).toBe(false);
      expect(mockCreateSession).not.toHaveBeenCalled();
      expect(mockNotifyError).toHaveBeenCalledWith('server error');
      expect(chat.messages()).toHaveLength(2);
      expect(chat.messages()[0]).toMatchObject({ role: 'user', content: 'hello' });
      expect(chat.messages()[1]).toMatchObject({
        role: 'assistant',
        error: 'server error',
        isStreaming: false,
      });
      expect(chat.sessionId()).toBe('');
      expect(chat.isLoading()).toBe(false);
      dispose();
    });

    it('handles chat API error gracefully', async () => {
      mockChat.mockRejectedValue(new Error('server error'));

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const result = await chat.sendMessage('hello');

      expect(result).toBe(false);
      expect(mockNotifyError).toHaveBeenCalledWith('server error');

      // Assistant message should carry the error in its dedicated error field
      const msgs = chat.messages();
      const assistant = msgs.find((m) => m.role === 'assistant');
      expect(assistant?.error).toContain('server error');
      expect(assistant?.isStreaming).toBe(false);
      expect(chat.isLoading()).toBe(false);
      dispose();
    });

    it('chat promise rejection removes unresolved interactive stream rows', async () => {
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string,
          _model: string | undefined,
          onEvent: (event: StreamEvent) => void,
        ) => {
          dispatchTestStreamEvent(onEvent, { type: 'content', data: 'partial response' });
          dispatchTestStreamEvent(onEvent, {
            type: 'tool_start',
            data: { id: 'tool-1', name: 'pulse_read', input: '{}' },
          });
          dispatchTestStreamEvent(onEvent, {
            type: 'approval_needed',
            data: {
              command: 'systemctl restart nginx',
              tool_id: 'tool-2',
              tool_name: 'pulse_control',
              run_on_host: true,
              approval_id: 'approval-1',
            },
          });
          dispatchTestStreamEvent(onEvent, {
            type: 'question',
            data: {
              question_id: 'question-1',
              questions: [{ id: 'target', question: 'Which node?' }],
            },
          });
          return Promise.reject(new Error('server error'));
        },
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const result = await chat.sendMessage('hello');

      expect(result).toBe(false);
      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.error).toBe('server error');
      expect(assistant.content).toBe('partial response');
      expect(assistant.pendingTools).toEqual([]);
      expect(assistant.pendingApprovals).toEqual([]);
      expect(assistant.pendingQuestions).toEqual([]);
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual(['content']);
      dispose();
    });

    it('retryMessage drops the failed turn and re-sends the prompt', async () => {
      mockChat.mockRejectedValueOnce(new Error('server error'));
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      await chat.sendMessage('hello');

      const failed = chat.messages().find((m) => m.role === 'assistant');
      expect(failed?.error).toContain('server error');
      expect(chat.messages()).toHaveLength(2);

      mockChat.mockResolvedValueOnce(undefined);
      chat.retryMessage(failed!.id);
      await new Promise((r) => setTimeout(r, 0));

      const msgs = chat.messages();
      // The failed user+assistant pair is replaced by a single clean attempt.
      expect(msgs.filter((m) => m.role === 'user')).toHaveLength(1);
      expect(msgs.some((m) => m.error)).toBe(false);
      expect(mockChat).toHaveBeenCalledTimes(2);
      dispose();
    });

    it('retryMessage preserves mentions, finding handoff, and scoped send options', async () => {
      mockChat.mockRejectedValueOnce(new Error('server error')).mockResolvedValueOnce(undefined);
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));

      await chat.sendMessage(
        'check this resource',
        [{ id: 'vm-100', name: 'web-1', type: 'vm', node: 'pve-1' }],
        'finding-42',
        {
          autonomousMode: false,
          handoffContext: '[Patrol Finding Context]\nFinding ID: finding-42',
          handoffResources: [
            {
              id: 'vm-100',
              name: 'web-1',
              type: 'vm',
              node: 'pve-1',
            },
          ],
          handoffActions: [
            {
              findingId: 'finding-42',
              approvalId: 'approval-1',
              approvalStatus: 'pending',
            },
          ],
          handoffMetadata: {
            kind: 'patrol_finding',
          },
        },
      );

      const failed = chat.messages().find((m) => m.role === 'assistant');
      expect(failed?.error).toContain('server error');

      chat.retryMessage(failed!.id);
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(2);
      const retryCall = mockChat.mock.calls[1];
      expect(retryCall[0]).toBe('check this resource');
      expect(retryCall[5]).toEqual([{ id: 'vm-100', name: 'web-1', type: 'vm', node: 'pve-1' }]);
      expect(retryCall[6]).toBe('finding-42');
      expect(retryCall[7]).toBe(false);
      expect(retryCall[8]).toBe('[Patrol Finding Context]\nFinding ID: finding-42');
      expect(retryCall[9]).toEqual([
        {
          id: 'vm-100',
          name: 'web-1',
          type: 'vm',
          node: 'pve-1',
        },
      ]);
      expect(retryCall[10]).toEqual([
        {
          findingId: 'finding-42',
          approvalId: 'approval-1',
          approvalStatus: 'pending',
        },
      ]);
      expect(retryCall[11]).toEqual({
        kind: 'patrol_finding',
      });
      dispose();
    });

    it('retryMessage can override the original turn model route', async () => {
      mockChat.mockRejectedValueOnce(new Error('server error')).mockResolvedValueOnce(undefined);
      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 'sess', model: 'deepseek:deepseek-v4-pro' }),
      );

      await chat.sendMessage('check provider');

      const failed = chat.messages().find((m) => m.role === 'assistant');
      expect(failed?.error).toContain('server error');

      chat.retryMessage(failed!.id, { model: 'openrouter:deepseek/deepseek-v4-pro' });
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(2);
      expect(mockChat.mock.calls[0][2]).toBe('deepseek:deepseek-v4-pro');
      expect(mockChat.mock.calls[1][2]).toBe('openrouter:deepseek/deepseek-v4-pro');
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
      const mentions: ChatMention[] = [
        { id: 'vm-100', name: 'test-vm', type: 'vm', node: 'node1' },
      ];
      await chat.sendMessage('check this', mentions, 'finding-42');

      const chatCall = mockChat.mock.calls[0];
      expect(chatCall[0]).toBe('check this');
      expect(chatCall[1]).toBe('sess');
      expect(chatCall[2]).toBe('claude-3');
      expect(chatCall[5]).toEqual(mentions);
      expect(chatCall[6]).toBe('finding-42');
      dispose();
    });

    it('passes a scoped autonomous-mode override to the API', async () => {
      mockChat.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      await chat.sendMessage('summarize dashboard', undefined, undefined, {
        autonomousMode: false,
      });

      const chatCall = mockChat.mock.calls[0];
      expect(chatCall[0]).toBe('summarize dashboard');
      expect(chatCall[7]).toBe(false);
      dispose();
    });

    it('passes model-only handoff context and resource references to the API', async () => {
      mockChat.mockResolvedValue(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      await chat.sendMessage('explain this incident', undefined, undefined, {
        autonomousMode: false,
        handoffContext: '[Alert Incident Context]\nIncident ID: incident-1',
        handoffResources: [
          {
            id: 'storage-1',
            name: 'tank',
            type: 'storage',
            node: 'nas-1',
          },
        ],
        handoffActions: [
          {
            findingId: 'finding-1',
            approvalId: 'approval-1',
            approvalStatus: 'pending',
          },
        ],
      });

      const chatCall = mockChat.mock.calls[0];
      expect(chatCall[7]).toBe(false);
      expect(chatCall[8]).toBe('[Alert Incident Context]\nIncident ID: incident-1');
      expect(chatCall[9]).toEqual([
        {
          id: 'storage-1',
          name: 'tank',
          type: 'storage',
          node: 'nas-1',
        },
      ]);
      expect(chatCall[10]).toEqual([
        {
          findingId: 'finding-1',
          approvalId: 'approval-1',
          approvalStatus: 'pending',
        },
      ]);
      dispose();
    });

    it('queues follow-up sends mid-stream without aborting the active response', async () => {
      let firstSignal: AbortSignal | undefined;
      let resolveFirst!: () => void;
      let resolveSecond!: () => void;

      mockChat
        .mockImplementationOnce(
          (
            _p: string,
            _s: string,
            _m: string | undefined,
            _onEvent: (e: StreamEvent) => void,
            signal?: AbortSignal,
          ) => {
            firstSignal = signal;
            return new Promise<void>((resolve) => {
              resolveFirst = resolve;
            });
          },
        )
        .mockImplementationOnce(
          () =>
            new Promise<void>((resolve) => {
              resolveSecond = resolve;
            }),
        );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const first = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 0));

      const queued = await chat.sendMessage('second');

      expect(queued).toBe(true);
      expect(firstSignal?.aborted).toBe(false);
      expect(mockAbortSession).not.toHaveBeenCalled();
      expect(mockChat).toHaveBeenCalledTimes(1);
      expect(chat.queuedFollowUpCount()).toBe(1);

      const queuedUser = chat.messages().find((message) => message.content === 'second');
      expect(queuedUser).toMatchObject({
        role: 'user',
        delivery: 'queued',
      });

      resolveFirst();
      await first;
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(2);
      expect(mockChat.mock.calls[1][0]).toBe('second');
      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.messages().find((message) => message.content === 'second')).toMatchObject({
        delivery: 'sent',
      });
      expect(chat.isLoading()).toBe(true);

      resolveSecond();
      await new Promise((r) => setTimeout(r, 0));
      expect(chat.isLoading()).toBe(false);
      dispose();
    });

    it('snapshots the selected model for queued follow-ups', async () => {
      const resolvers: Array<() => void> = [];
      mockChat.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolvers.push(resolve);
          }),
      );

      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 'sess', model: 'openrouter:qwen/qwen3.7-plus' }),
      );
      const first = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 0));

      chat.setModel('openrouter:deepseek/deepseek-v4-pro');
      await chat.sendMessage('second');

      const queuedUser = chat.messages().find((message) => message.content === 'second');
      expect(queuedUser?.request?.model).toBe('openrouter:deepseek/deepseek-v4-pro');

      chat.setModel('gemini:gemini-3.1-flash-lite');
      resolvers[0]();
      await first;
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(2);
      expect(mockChat.mock.calls[1][0]).toBe('second');
      expect(mockChat.mock.calls[1][2]).toBe('openrouter:deepseek/deepseek-v4-pro');
      dispose();
    });

    it('preserves queued follow-up order and drains one turn at a time', async () => {
      const resolvers: Array<() => void> = [];
      mockChat.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolvers.push(resolve);
          }),
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const first = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 0));

      await chat.sendMessage('second');
      await chat.sendMessage('third');

      expect(mockChat).toHaveBeenCalledTimes(1);
      expect(chat.queuedFollowUpCount()).toBe(2);

      resolvers[0]();
      await first;
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(2);
      expect(mockChat.mock.calls[1][0]).toBe('second');
      expect(chat.queuedFollowUpCount()).toBe(1);

      resolvers[1]();
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(3);
      expect(mockChat.mock.calls[2][0]).toBe('third');
      expect(chat.queuedFollowUpCount()).toBe(0);

      resolvers[2]();
      await new Promise((r) => setTimeout(r, 0));
      expect(chat.isLoading()).toBe(false);
      dispose();
    });

    it('promotes a queued follow-up to send next while the active response is streaming', async () => {
      const resolvers: Array<() => void> = [];
      mockChat.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolvers.push(resolve);
          }),
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const first = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 0));

      await chat.sendMessage('second');
      await chat.sendMessage('third');

      const third = chat.queuedFollowUps().find((entry) => entry.prompt === 'third');
      expect(third).toBeDefined();
      await expect(chat.sendQueuedFollowUpNow(third!.id)).resolves.toBe(true);

      expect(chat.queuedFollowUps().map((entry) => entry.prompt)).toEqual(['third', 'second']);
      expect(
        chat
          .messages()
          .filter((message) => message.role === 'user' && message.delivery === 'queued')
          .map((message) => message.content),
      ).toEqual(['third', 'second']);

      resolvers[0]();
      await first;
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(2);
      expect(mockChat.mock.calls[1][0]).toBe('third');
      expect(chat.queuedFollowUps().map((entry) => entry.prompt)).toEqual(['second']);

      resolvers[1]();
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(3);
      expect(mockChat.mock.calls[2][0]).toBe('second');

      resolvers[2]();
      await new Promise((r) => setTimeout(r, 0));
      expect(chat.isLoading()).toBe(false);
      dispose();
    });

    it('cancels a queued follow-up before it is sent', async () => {
      let resolveFirst!: () => void;
      mockChat.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolveFirst = resolve;
          }),
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const first = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 0));

      await chat.sendMessage('second');

      const queued = chat.queuedFollowUps()[0];
      chat.cancelQueuedFollowUp(queued.id);

      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.messages().some((message) => message.content === 'second')).toBe(false);

      resolveFirst();
      await first;
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(1);
      dispose();
    });

    it('takes a queued follow-up for composer editing before it is sent', async () => {
      let resolveFirst!: () => void;
      mockChat.mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolveFirst = resolve;
          }),
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const first = chat.sendMessage('first');
      await new Promise((r) => setTimeout(r, 0));

      await chat.sendMessage(
        'second',
        [{ id: 'vm-1', name: 'web-1', type: 'vm', node: 'pve-1' }],
        'finding-1',
        { autonomousMode: false, handoffContext: 'scoped context' },
      );

      const queued = chat.queuedFollowUps()[0];
      const taken = chat.takeQueuedFollowUp(queued.id);

      expect(taken).toMatchObject({
        id: queued.id,
        messageId: queued.messageId,
        prompt: 'second',
        mentions: [{ id: 'vm-1', name: 'web-1', type: 'vm', node: 'pve-1' }],
        findingId: 'finding-1',
        sendOptions: { autonomousMode: false, handoffContext: 'scoped context' },
      });
      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.messages().some((message) => message.content === 'second')).toBe(false);

      resolveFirst();
      await first;
      await new Promise((r) => setTimeout(r, 0));

      expect(mockChat).toHaveBeenCalledTimes(1);
      dispose();
    });

    it('stop aborts the browser stream and backend session', async () => {
      let capturedSignal: AbortSignal | undefined;
      const abortError = new Error('Aborted');
      abortError.name = 'AbortError';
      mockAbortSession.mockResolvedValue(undefined);
      mockChat.mockImplementation(
        (
          _p: string,
          _s: string,
          _m: string | undefined,
          _onEvent: (e: StreamEvent) => void,
          signal?: AbortSignal,
        ) => {
          capturedSignal = signal;
          return new Promise<void>((_resolve, reject) => {
            signal?.addEventListener('abort', () => reject(abortError));
          });
        },
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess' }));
      const send = chat.sendMessage('hello');
      await new Promise((r) => setTimeout(r, 0));

      chat.stop();
      await expect(send).resolves.toBe(false);

      expect(capturedSignal?.aborted).toBe(true);
      expect(mockAbortSession).toHaveBeenCalledWith('sess');
      expect(chat.isLoading()).toBe(false);
      const assistant = chat.messages().find((message) => message.role === 'assistant');
      expect(assistant).toMatchObject({
        content: '',
        interruption: 'stopped',
        isStreaming: false,
      });
      expect(assistant?.completedAt).toBeInstanceOf(Date);
      dispose();
    });

    it('stop during cold stream prevents deferred session binding', async () => {
      let capturedSignal: AbortSignal | undefined;
      let fireEvent!: TestStreamDispatch;
      const abortError = new Error('Aborted');
      abortError.name = 'AbortError';
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string | undefined,
          _model: string | undefined,
          onEvent: (event: StreamEvent) => void,
          signal?: AbortSignal,
        ) => {
          capturedSignal = signal;
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
          return new Promise<void>((_resolve, reject) => {
            signal?.addEventListener('abort', () => reject(abortError));
          });
        },
      );

      const { value: chat, dispose } = withRoot(() => useChat());
      const send = chat.sendMessage('cold start');

      expect(chat.isLoading()).toBe(true);
      chat.stop();
      fireEvent({ type: 'session', data: { id: 'late-session' } });

      await expect(send).resolves.toBe(false);
      expect(capturedSignal?.aborted).toBe(true);
      expect(mockAbortSession).not.toHaveBeenCalled();
      expect(chat.sessionId()).toBe('');
      expect(chat.isLoading()).toBe(false);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // Stream event processing (via sendMessage callback)
  // ──────────────────────────────────────────────
  describe('processEvent (via stream callback)', () => {
    /** Helper that captures the onEvent callback from AIChatAPI.chat */
    function setupWithEventCapture() {
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string,
          _model: string | undefined,
          onEvent: (e: StreamEvent) => void,
        ) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      expect(contentEvents[0].startedAt).toEqual(expect.any(Number));
      expect(contentEvents[0].updatedAt).toEqual(expect.any(Number));
      expect(contentEvents[0].updatedAt).toBeGreaterThanOrEqual(contentEvents[0].startedAt || 0);
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
      expect(thinkingEvents[0].startedAt).toEqual(expect.any(Number));
      expect(thinkingEvents[0].updatedAt).toEqual(expect.any(Number));
      expect(thinkingEvents[0].updatedAt).toBeGreaterThanOrEqual(thinkingEvents[0].startedAt || 0);
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

    it('strips serialized Pulse tool calls from streamed content', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('how many devices in this');
      const fire = getFireEvent();

      fire({
        type: 'content',
        data: 'I will inspect the device nodes.\npulse_read(target_host="current_resource", command="ls /dev | wc -l")',
      });
      fire({ type: 'content', data: 'raw arguments that should stay hidden' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('I will inspect the device nodes.');
      expect(assistant.content).not.toContain('pulse_read');
      expect(assistant.content).not.toContain('raw arguments');
      expect(assistant.streamEvents?.filter((e) => e.type === 'content')).toMatchObject([
        { type: 'content', content: 'I will inspect the device nodes.' },
      ]);
      dispose();
    });

    it('strips serialized Pulse tool calls split across streamed content deltas', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('how many devices in this');
      const fire = getFireEvent();

      fire({ type: 'content', data: 'I will check pu' });
      fire({
        type: 'content',
        data: 'lse_read(target_host="current_resource", command="ls /dev | wc -l")',
      });
      fire({ type: 'content', data: 'raw arguments that should stay hidden' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('I will check ');
      expect(assistant.content).not.toContain('pulse_read');
      expect(assistant.content).not.toContain('target_host');
      expect(assistant.content).not.toContain('raw arguments');
      expect(assistant.streamEvents?.filter((e) => e.type === 'content')).toMatchObject([
        { type: 'content', content: 'I will check ' },
      ]);
      dispose();
    });

    it('holds compacted internal prose so a split tool-call leak never becomes visible', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('how many devices in this');
      const fire = getFireEvent();
      const compacted =
        "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.";

      fire({ type: 'content', data: compacted });
      expect(chat.messages().find((m) => m.role === 'assistant')?.content).toBe('');
      expect(
        chat
          .messages()
          .find((m) => m.role === 'assistant')
          ?.streamEvents?.filter((e) => e.type === 'content'),
      ).toEqual([]);

      fire({
        type: 'content',
        data: 'pulse_read(target_host="current_resource", command="ls /dev | wc -l")',
      });
      fire({ type: 'content', data: 'raw arguments that should stay hidden' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('');
      expect(assistant.content).not.toContain('pulse_read');
      expect(assistant.content).not.toContain('raw arguments');
      expect(assistant.streamEvents?.filter((e) => e.type === 'content')).toEqual([]);
      dispose();
    });

    it('does not flush held compacted prose on done when no tool-call leak follows', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();
      const compacted =
        'Thisisbadmodelspacingbutitistheactualanswerbecauseitneverturnsintoatoolcall.';

      fire({ type: 'content', data: compacted });
      expect(chat.messages().find((m) => m.role === 'assistant')?.content).toBe('');

      fire({ type: 'done', data: {} });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('');
      expect(assistant.streamEvents?.filter((e) => e.type === 'content')).toEqual([]);
      expect(assistant.isStreaming).toBe(false);
      dispose();
    });

    it('resumes visible content after a governed tool boundary clears a raw leak', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('how many devices in this');
      const fire = getFireEvent();

      fire({
        type: 'content',
        data: 'I will inspect the device nodes.\npulse_read(target_host="current_resource", command="ls /dev | wc -l")',
      });
      fire({ type: 'content', data: 'raw arguments that should stay hidden' });
      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: 'exec' } });
      fire({
        type: 'tool_end',
        data: { id: 'tool-1', name: 'pulse_read', input: 'exec', output: '42', success: true },
      });
      fire({ type: 'content', data: 'There are 42 device entries.' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe(
        'I will inspect the device nodes. There are 42 device entries.',
      );
      expect(assistant.content).not.toContain('pulse_read');
      expect(assistant.content).not.toContain('raw arguments');
      expect(assistant.streamEvents?.map((e) => e.type)).toEqual(['content', 'tool', 'content']);
      dispose();
    });

    it('does not render compacted pre-tool artifact text when a governed tool row follows', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('how many devices in this');
      const fire = getFireEvent();
      const compacted =
        "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.";

      fire({ type: 'content', data: compacted });
      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      fire({
        type: 'tool_end',
        data: { id: 'tool-1', name: 'pulse_read', input: '{}', output: '4358', success: true },
      });
      fire({ type: 'content', data: 'There are 4,358 entries under /dev.' });
      fire({ type: 'done', data: {} });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('There are 4,358 entries under /dev.');
      expect(assistant.content).not.toContain("I'llcheck");
      expect(assistant.streamEvents?.filter((e) => e.type === 'content')).toMatchObject([
        { type: 'content', content: 'There are 4,358 entries under /dev.' },
      ]);
      expect(assistant.streamEvents?.map((e) => e.type)).toEqual(['tool', 'content']);
      dispose();
    });

    it('ignores product telemetry events that are not the model response', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'explore_status',
        data: {
          phase: 'started',
          message: 'Explore pre-pass running',
          model: 'claude-3',
        },
      } as any);
      fire({
        type: 'workflow_state',
        data: {
          phase: 'plan',
          message: 'Planning governed action and safety checks before execution.',
          state: 'READING',
          tool: 'pulse_exec',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('');
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'plan',
            message: 'Planning governed action and safety checks before execution.',
            state: 'READING',
            tool: 'pulse_exec',
          }),
        }),
      ]);
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          phase: 'plan',
          message: 'Planning governed action and safety checks before execution.',
          state: 'READING',
          tool: 'pulse_exec',
        }),
      );
      dispose();
    });

    it('ignores obsolete provider fallback workflow metadata', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 's', model: 'openrouter:openai/gpt-4o-mini' }),
      );

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_fallback',
          message: 'OpenRouter did not start a response; trying Gemini.',
          failed_model: 'openrouter:openai/gpt-4o-mini',
          next_model: 'gemini:gemini-3.1-flash-lite',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.model).toBe('openrouter:openai/gpt-4o-mini');
      expect(assistant.streamEvents).not.toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            type: 'model_switch',
            model: 'gemini:gemini-3.1-flash-lite',
          }),
        ]),
      );
      dispose();
    });

    it('stores provider retry metadata from workflow state events', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 's', model: 'openrouter:openai/gpt-4o-mini' }),
      );

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_retry',
          message: 'Provider connection failed before any output; retrying.',
          attempt: 2,
          max_attempts: 2,
          retry_after_ms: 200,
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          phase: 'provider_retry',
          message: 'Provider connection failed before any output; retrying.',
          attempt: 2,
          maxAttempts: 2,
          retryAfterMs: 200,
        }),
      );
      expect(assistant.streamEvents).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            type: 'model_switch',
            model: 'openrouter:openai/gpt-4o-mini',
            modelEvent: 'selected',
          }),
          expect.objectContaining({
            type: 'workflow_status',
            workflowStatus: expect.objectContaining({
              phase: 'provider_retry',
              attempt: 2,
              maxAttempts: 2,
              retryAfterMs: 200,
            }),
          }),
        ]),
      );
      dispose();
    });

    it('does not infer a model switch from legacy next-model workflow metadata', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 's', model: 'openrouter:openai/gpt-4o-mini' }),
      );

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_fallback',
          message: 'Trying Gemini.',
          next_model: 'gemini:gemini-3.1-flash-lite',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.model).toBe('openrouter:openai/gpt-4o-mini');
      expect(assistant.streamEvents).not.toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            type: 'model_switch',
            model: 'gemini:gemini-3.1-flash-lite',
          }),
        ]),
      );
      dispose();
    });

    it('replaces neutral workflow activity until a durable stream boundary appears', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'context',
          message: 'Reading current Pulse inventory with pulse_query.',
          tool: 'pulse_query',
        },
      });
      let assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            message: 'Reading current Pulse inventory with pulse_query.',
          }),
        }),
      ]);
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          message: 'Reading current Pulse inventory with pulse_query.',
        }),
      );

      fire({
        type: 'workflow_state',
        data: {
          phase: 'context',
          message: 'Built compact inventory context for the model.',
          tool: 'pulse_query',
        },
      });
      assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          message: 'Built compact inventory context for the model.',
        }),
      );
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            message: 'Built compact inventory context for the model.',
          }),
        }),
      ]);

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
        },
      });
      assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          message: 'OpenRouter is starting the response.',
        }),
      );
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            message: 'OpenRouter is starting the response.',
          }),
        }),
      ]);
      expect(assistant.workflowStatusHistory?.map((status) => status.message)).toEqual([
        'Reading current Pulse inventory with pulse_query.',
        'Built compact inventory context for the model.',
        'OpenRouter is starting the response.',
      ]);

      dispose();
    });

    it('replaces provider wait with visible idle progress when an idle heartbeat arrives', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
        },
      });
      fire({
        type: 'workflow_state',
        data: {
          phase: 'stream_idle',
          message: 'Assistant is still working; waiting for the next stream event.',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          phase: 'stream_idle',
          message: 'Assistant is still working; waiting for the next stream event.',
        }),
      );
      expect(assistant.streamEvents).toHaveLength(1);
      expect(assistant.streamEvents?.[0]).toEqual(
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'stream_idle',
            message: 'Assistant is still working; waiting for the next stream event.',
          }),
        }),
      );

      dispose();
    });

    it('records the initial provider model route as a typed stream event', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
          provider: 'openrouter',
          model: 'openrouter:qwen/qwen3.7-plus',
        },
      });
      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
          provider: 'openrouter',
          model: 'openrouter:qwen/qwen3.7-plus',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.model).toBe('openrouter:qwen/qwen3.7-plus');
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'model_switch',
        'workflow_status',
      ]);
      expect(assistant.streamEvents?.[0]).toEqual(
        expect.objectContaining({
          type: 'model_switch',
          model: 'openrouter:qwen/qwen3.7-plus',
          failedModel: undefined,
          modelEvent: 'selected',
        }),
      );
      expect(assistant.streamEvents?.[1]).toEqual(
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'provider_start',
            message: 'OpenRouter is starting the response.',
          }),
        }),
      );

      dispose();
    });

    it('replaces a provisional selected-model row when provider startup confirms a different route', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() =>
        useChat({
          sessionId: 's',
          defaultModel: () => 'openrouter:qwen/qwen3.7-plus',
        }),
      );

      await chat.sendMessage('hi');
      let assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'model_switch',
          model: 'openrouter:qwen/qwen3.7-plus',
          modelEvent: 'selected',
        }),
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'request_send',
            message: 'Sending prompt.',
          }),
        }),
      ]);

      const fire = getFireEvent();
      fire({
        type: 'workflow_state',
        data: {
          phase: 'prepare',
          message: 'Preparing Pulse context.',
        },
      });
      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'DeepSeek is starting the response.',
          provider: 'deepseek',
          model: 'deepseek:deepseek-chat',
        },
      });

      assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.model).toBe('deepseek:deepseek-chat');
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'model_switch',
        'workflow_status',
      ]);
      expect(assistant.streamEvents?.filter((event) => event.type === 'model_switch')).toEqual([
        expect.objectContaining({
          type: 'model_switch',
          model: 'deepseek:deepseek-chat',
          modelEvent: 'selected',
        }),
      ]);
      expect(
        assistant.streamEvents?.some((event) => event.model === 'openrouter:qwen/qwen3.7-plus'),
      ).toBe(false);

      dispose();
    });

    it('keeps prior selected-model evidence after durable assistant content starts', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() =>
        useChat({
          sessionId: 's',
          defaultModel: () => 'openrouter:qwen/qwen3.7-plus',
        }),
      );

      await chat.sendMessage('hi');
      const fire = getFireEvent();
      fire({ type: 'content', data: 'Partial answer.' });
      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'DeepSeek is starting the response.',
          provider: 'deepseek',
          model: 'deepseek:deepseek-chat',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'model_switch',
        'content',
        'model_switch',
        'workflow_status',
      ]);
      expect(assistant.streamEvents?.filter((event) => event.type === 'model_switch')).toEqual([
        expect.objectContaining({
          type: 'model_switch',
          model: 'openrouter:qwen/qwen3.7-plus',
          modelEvent: 'selected',
        }),
        expect.objectContaining({
          type: 'model_switch',
          model: 'deepseek:deepseek-chat',
          modelEvent: 'selected',
        }),
      ]);

      dispose();
    });

    it('replaces local request-start status with visible idle progress', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'stream_idle',
          message: 'Assistant is still working; waiting for the next stream event.',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          phase: 'stream_idle',
          message: 'Assistant is still working; waiting for the next stream event.',
        }),
      );
      expect(assistant.streamEvents).toEqual([
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'stream_idle',
            message: 'Assistant is still working; waiting for the next stream event.',
          }),
        }),
      ]);

      dispose();
    });

    it('keeps neutral workflow activity visible when answer content starts streaming', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
        },
      });
      fire({ type: 'content', data: 'Here is the answer.' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.content).toBe('Here is the answer.');
      expect(assistant.workflowStatus).toBeUndefined();
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'workflow_status',
        'content',
      ]);
      expect(assistant.streamEvents?.[0]).toEqual(
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'provider_start',
            message: 'OpenRouter is starting the response.',
          }),
        }),
      );
      dispose();
    });

    it('keeps neutral workflow activity visible when a governed tool block starts', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'context',
          message: 'Reading Pulse inventory.',
        },
      });
      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'get_logs', input: '{}' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toBeUndefined();
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'workflow_status',
        'pending_tool',
      ]);
      expect(assistant.streamEvents?.[0]).toEqual(
        expect.objectContaining({
          type: 'workflow_status',
          workflowStatus: expect.objectContaining({
            phase: 'context',
            message: 'Reading Pulse inventory.',
          }),
        }),
      );
      dispose();
    });

    it('keeps late workflow progress live after typed tool evidence in chronological order', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
        },
      });
      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_alerts', input: '{}' } });
      fire({
        type: 'tool_end',
        data: {
          id: 'tool-1',
          name: 'pulse_alerts',
          input: '{}',
          output: '11 active alerts',
          success: true,
        },
      });
      fire({
        type: 'workflow_state',
        data: {
          phase: 'model_thinking',
          message: 'Model is reasoning before responding.',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.workflowStatus).toEqual(
        expect.objectContaining({
          phase: 'model_thinking',
          message: 'Model is reasoning before responding.',
        }),
      );
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'workflow_status',
        'tool',
        'workflow_status',
      ]);
      expect(assistant.streamEvents?.[2]).toEqual(
        expect.objectContaining({
          workflowStatus: expect.objectContaining({
            phase: 'model_thinking',
            message: 'Model is reasoning before responding.',
          }),
        }),
      );
      expect(assistant.toolCalls).toHaveLength(1);
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
        status: 'pending',
        startedAt: expect.any(Number),
        updatedAt: expect.any(Number),
      });
      expect(assistant.streamEvents).toContainEqual(
        expect.objectContaining({
          type: 'pending_tool',
          toolId: 'tool-1',
          startedAt: expect.any(Number),
          updatedAt: expect.any(Number),
        }),
      );
      dispose();
    });

    it('processes tool_progress events by updating the pending tool in place', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'get_logs', input: '{}' } });
      fire({
        type: 'tool_progress',
        data: {
          id: 'tool-1',
          name: 'get_logs',
          input: '{}',
          raw_input: '{"action": "logs"',
          phase: 'running',
          message: 'Running.',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.pendingTools![0]).toMatchObject({
        id: 'tool-1',
        name: 'get_logs',
        input: '{}',
        rawInput: '{"action": "logs"',
        status: 'running',
        progress: 'Running.',
      });
      const pendingToolEvents = assistant.streamEvents?.filter((e) => e.type === 'pending_tool');
      expect(pendingToolEvents).toHaveLength(1);
      expect(pendingToolEvents![0]).toMatchObject({
        startedAt: expect.any(Number),
        updatedAt: expect.any(Number),
        pendingTool: {
          id: 'tool-1',
          rawInput: '{"action": "logs"',
          status: 'running',
          progress: 'Running.',
        },
      });
      dispose();
    });

    it('deduplicates repeated tool_start events without regressing live progress', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      fire({
        type: 'tool_progress',
        data: {
          id: 'tool-1',
          name: 'pulse_read',
          input: '{}',
          raw_input: '{"command":"ls /dev"}',
          phase: 'running',
          message: 'Running command.',
        },
      });
      fire({
        type: 'tool_start',
        data: {
          id: 'tool-1',
          name: 'pulse_read',
          input: '{"command":"ls /dev"}',
          raw_input: '{"command":"ls /dev"}',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.pendingTools![0]).toMatchObject({
        id: 'tool-1',
        name: 'pulse_read',
        input: '{"command":"ls /dev"}',
        rawInput: '{"command":"ls /dev"}',
        status: 'running',
        progress: 'Running command.',
      });
      const pendingToolEvents = assistant.streamEvents?.filter((e) => e.type === 'pending_tool');
      expect(pendingToolEvents).toHaveLength(1);
      expect(pendingToolEvents![0]).toMatchObject({
        toolId: 'tool-1',
        pendingTool: {
          id: 'tool-1',
          status: 'running',
          progress: 'Running command.',
        },
      });
      dispose();
    });

    it('creates a pending row when tool_progress arrives before tool_start', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'tool_progress',
        data: {
          id: 'tool-1',
          name: 'get_logs',
          input: '{"action":"logs"}',
          phase: 'running',
          message: 'Running.',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.streamEvents?.filter((e) => e.type === 'pending_tool')).toHaveLength(1);
      expect(assistant.pendingTools![0]).toMatchObject({
        id: 'tool-1',
        name: 'get_logs',
        input: '{"action":"logs"}',
        status: 'running',
        progress: 'Running.',
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
        data: {
          id: 'tool-1',
          name: 'get_logs',
          input: '{}',
          raw_input: '{"action": "logs"',
          output: 'log data',
          success: true,
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.toolCalls).toHaveLength(1);
      expect(assistant.toolCalls![0]).toEqual({
        name: 'get_logs',
        input: '{}',
        rawInput: '{"action": "logs"',
        output: 'log data',
        success: true,
      });
      expect(assistant.streamEvents).toContainEqual(
        expect.objectContaining({
          type: 'tool',
          toolId: 'tool-1',
          startedAt: expect.any(Number),
          updatedAt: expect.any(Number),
        }),
      );
      dispose();
    });

    it('stamps fast tool completions with a transient settle deadline', async () => {
      vi.useFakeTimers();
      vi.setSystemTime(20_000);
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      vi.setSystemTime(20_040);
      fire({
        type: 'tool_end',
        data: {
          id: 'tool-1',
          name: 'pulse_read',
          input: '{}',
          output: '4358',
          success: true,
        },
      });
      fire({ type: 'content', data: 'There are 4,358 entries.' });
      fire({ type: 'done', data: {} });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      const toolEvent = assistant.streamEvents?.find((event) => event.type === 'tool');
      expect(assistant.isStreaming).toBe(false);
      expect(toolEvent).toEqual(
        expect.objectContaining({
          type: 'tool',
          toolId: 'tool-1',
          startedAt: 20_000,
          updatedAt: 20_040,
          settleUntil: 20_420,
        }),
      );
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual(['tool', 'content']);
      dispose();
    });

    it('does not stamp slow tool completions with a settle deadline', async () => {
      vi.useFakeTimers();
      vi.setSystemTime(30_000);
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      vi.setSystemTime(31_000);
      fire({
        type: 'tool_end',
        data: {
          id: 'tool-1',
          name: 'pulse_read',
          input: '{}',
          output: '4358',
          success: true,
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      const toolEvent = assistant.streamEvents?.find((event) => event.type === 'tool');
      expect(toolEvent).toEqual(
        expect.objectContaining({
          type: 'tool',
          toolId: 'tool-1',
          startedAt: 30_000,
          updatedAt: 31_000,
        }),
      );
      expect(toolEvent?.settleUntil).toBeUndefined();
      dispose();
    });

    it('preserves pending tool identity when terminal updates omit name and input', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();
      const input =
        '{"action":"exec","command":"ls /dev | wc -l","target_host":"current_resource"}';

      fire({
        type: 'tool_start',
        data: {
          id: 'tool-1',
          name: 'pulse_read',
          input,
          raw_input: input,
        },
      });
      fire({
        type: 'tool_end',
        data: {
          id: 'tool-1',
          output: '4358',
          success: true,
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(0);
      expect(assistant.toolCalls).toEqual([
        {
          name: 'pulse_read',
          input,
          rawInput: input,
          output: '4358',
          success: true,
        },
      ]);
      expect(assistant.streamEvents?.filter((event) => event.type === 'pending_tool')).toHaveLength(
        0,
      );
      expect(assistant.streamEvents?.filter((event) => event.type === 'tool')).toEqual([
        expect.objectContaining({
          type: 'tool',
          toolId: 'tool-1',
          tool: {
            name: 'pulse_read',
            input,
            rawInput: input,
            output: '4358',
            success: true,
          },
        }),
      ]);
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
          target_type: 'agent',
          target_id: 'agent-1',
          risk: 'high',
          description: 'Restart web service',
          audit_id: 'action-5',
          plan: {
            action_id: 'action-5',
            request_id: 'appr-5',
            summary: 'Restart web service',
            requires_approval: true,
            approval_policy: 'admin',
            blast_radius: 'service interruption on target',
            rollback_available: true,
            plan_hash: 'hash-5',
            expires_at: '2026-04-23T12:30:00Z',
          },
          context_confidence: {
            level: 'verified',
            summary: 'Target was resolved to a concrete resource before approval.',
            evidence: ['Target identifier bound to agent-1.'],
          },
          preflight: {
            target: 'agent:web1 (agent-1)',
            current_state: 'Resolved approval target: agent:web1 (agent-1).',
            intended_change: 'Restart web service',
            dry_run_available: false,
            dry_run_summary: 'No provider-supported dry run is available for this action.',
            safety_checks: ['Approval is scoped to this organization.'],
            verification_steps: ['Read back the target state after execution.'],
            generated_at: '2026-04-23T12:29:00Z',
          },
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
        targetType: 'agent',
        targetId: 'agent-1',
        risk: 'high',
        description: 'Restart web service',
        auditId: 'action-5',
        plan: {
          action_id: 'action-5',
          request_id: 'appr-5',
          summary: 'Restart web service',
          requires_approval: true,
          approval_policy: 'admin',
          blast_radius: 'service interruption on target',
          rollback_available: true,
          plan_hash: 'hash-5',
          expires_at: '2026-04-23T12:30:00Z',
        },
        contextConfidence: {
          level: 'verified',
          summary: 'Target was resolved to a concrete resource before approval.',
          evidence: ['Target identifier bound to agent-1.'],
        },
        preflight: {
          target: 'agent:web1 (agent-1)',
          current_state: 'Resolved approval target: agent:web1 (agent-1).',
          intended_change: 'Restart web service',
          dry_run_available: false,
          dry_run_summary: 'No provider-supported dry run is available for this action.',
          safety_checks: ['Approval is scoped to this organization.'],
          verification_steps: ['Read back the target state after execution.'],
          generated_at: '2026-04-23T12:29:00Z',
        },
        isExecuting: false,
        approvalId: 'appr-5',
      });
      expect(assistant.streamEvents).toContainEqual(
        expect.objectContaining({
          type: 'approval',
          startedAt: expect.any(Number),
          updatedAt: expect.any(Number),
        }),
      );
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
      expect(assistant.streamEvents).toContainEqual(
        expect.objectContaining({
          type: 'question',
          startedAt: expect.any(Number),
          updatedAt: expect.any(Number),
        }),
      );
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
      fire({
        type: 'done',
        data: {
          model: 'gemini:gemini-3.1-flash-lite',
          input_tokens: 100,
          output_tokens: 50,
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.isStreaming).toBe(false);
      expect(assistant.completedAt).toBeInstanceOf(Date);
      expect(assistant.model).toBe('gemini:gemini-3.1-flash-lite');
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
      expect(assistant.completedAt).toBeInstanceOf(Date);
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

    it('done event removes unresolved interactive rows and neutral workflow activity', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
        },
      });
      fire({ type: 'content', data: 'partial response' });
      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      fire({
        type: 'approval_needed',
        data: {
          command: 'systemctl restart nginx',
          tool_id: 'tool-2',
          tool_name: 'pulse_control',
          run_on_host: true,
          approval_id: 'approval-1',
        },
      });
      fire({
        type: 'question',
        data: {
          question_id: 'question-1',
          questions: [{ id: 'target', question: 'Which node?' }],
        },
      });
      fire({ type: 'done', data: { input_tokens: 1, output_tokens: 2 } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.isStreaming).toBe(false);
      expect(assistant.content).toBe('partial response');
      expect(assistant.pendingTools).toEqual([]);
      expect(assistant.pendingApprovals).toEqual([]);
      expect(assistant.pendingQuestions).toEqual([]);
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual(['content']);
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
      expect(assistant.completedAt).toBeInstanceOf(Date);
      expect(assistant.error).toBe('Rate limited');
      expect(assistant.pendingTools).toHaveLength(0);
      dispose();
    });

    it('error event removes unresolved interactive rows and neutral workflow activity', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({
        type: 'workflow_state',
        data: {
          phase: 'provider_start',
          message: 'OpenRouter is starting the response.',
        },
      });
      fire({ type: 'content', data: 'partial response' });
      fire({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      fire({
        type: 'approval_needed',
        data: {
          command: 'systemctl restart nginx',
          tool_id: 'tool-2',
          tool_name: 'pulse_control',
          run_on_host: true,
          approval_id: 'approval-1',
        },
      });
      fire({
        type: 'question',
        data: {
          question_id: 'question-1',
          questions: [{ id: 'target', question: 'Which node?' }],
        },
      });
      fire({ type: 'error', data: { message: 'Provider disconnected' } });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.isStreaming).toBe(false);
      expect(assistant.error).toBe('Provider disconnected');
      expect(assistant.content).toBe('partial response');
      expect(assistant.pendingTools).toEqual([]);
      expect(assistant.pendingApprovals).toEqual([]);
      expect(assistant.pendingQuestions).toEqual([]);
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual(['content']);
      dispose();
    });

    it('processes error event with string data', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'error', data: 'Something went wrong' });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.error).toBe('Something went wrong');
      dispose();
    });

    it('error event with empty data produces fallback message', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'error', data: null });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.error).toBe('Request failed');
      dispose();
    });

    it('ignores unknown event types gracefully', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

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
      expect(assistant?.content).toBe('');
      expect(assistant?.interruption).toBe('stopped');
      dispose();
    });

    it('preserves existing content when stopping', async () => {
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      expect(assistant?.interruption).toBe('stopped');
      dispose();
    });

    it('pauses queued follow-ups when stopping the active response', async () => {
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
          ) =>
            new Promise<void>((_resolve, reject) => {
              signal?.addEventListener('abort', () => reject(abortError));
            }),
        )
        .mockResolvedValueOnce(undefined);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      const send = chat.sendMessage('hello');
      await new Promise((r) => setTimeout(r, 0));

      await chat.sendMessage('queued follow-up');
      const queued = chat.queuedFollowUps()[0];
      expect(chat.queuedFollowUpCount()).toBe(1);
      expect(chat.queuedFollowUpsPaused()).toBe(false);
      expect(chat.messages().some((message) => message.delivery === 'queued')).toBe(true);

      chat.stop();
      await expect(send).resolves.toBe(false);

      expect(mockAbortSession).toHaveBeenCalledWith('s');
      expect(chat.queuedFollowUpCount()).toBe(1);
      expect(chat.queuedFollowUpsPaused()).toBe(true);
      expect(
        chat.messages().find((message) => message.content === 'queued follow-up'),
      ).toMatchObject({
        delivery: 'queued',
      });
      expect(mockChat).toHaveBeenCalledTimes(1);

      await expect(chat.waitForIdle(50)).resolves.toBe(true);
      await expect(chat.sendQueuedFollowUpNow(queued.id)).resolves.toBe(true);

      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.queuedFollowUpsPaused()).toBe(false);
      expect(
        chat.messages().find((message) => message.content === 'queued follow-up'),
      ).toMatchObject({
        delivery: 'sent',
      });
      expect(mockChat).toHaveBeenCalledTimes(2);
      expect(mockChat.mock.calls[1][0]).toBe('queued follow-up');
      dispose();
    });

    it('removes unresolved interactive stream rows when stopping', async () => {
      let fireEvent!: TestStreamDispatch;
      const abortError = new Error('Aborted');
      abortError.name = 'AbortError';
      mockChat.mockImplementation(
        (
          _prompt: string,
          _session: string,
          _model: string | undefined,
          onEvent: (event: StreamEvent) => void,
          signal?: AbortSignal,
        ) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
          return new Promise<void>((_resolve, reject) => {
            signal?.addEventListener('abort', () => reject(abortError));
          });
        },
      );

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));
      const send = chat.sendMessage('hello');
      await new Promise((r) => setTimeout(r, 0));

      fireEvent({ type: 'content', data: 'partial response' });
      fireEvent({ type: 'tool_start', data: { id: 'tool-1', name: 'pulse_read', input: '{}' } });
      fireEvent({
        type: 'approval_needed',
        data: {
          command: 'systemctl restart nginx',
          tool_id: 'tool-2',
          tool_name: 'pulse_control',
          run_on_host: true,
          approval_id: 'approval-1',
        },
      });
      fireEvent({
        type: 'question',
        data: {
          question_id: 'question-1',
          questions: [{ id: 'target', question: 'Which node?' }],
        },
      });

      let assistant = chat.messages().find((message) => message.role === 'assistant')!;
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual([
        'content',
        'pending_tool',
        'approval',
        'question',
      ]);

      chat.stop();
      await expect(send).resolves.toBe(false);

      assistant = chat.messages().find((message) => message.role === 'assistant')!;
      expect(assistant.content).toBe('partial response');
      expect(assistant.interruption).toBe('stopped');
      expect(assistant.pendingTools).toEqual([]);
      expect(assistant.pendingApprovals).toEqual([]);
      expect(assistant.pendingQuestions).toEqual([]);
      expect(assistant.streamEvents?.map((event) => event.type)).toEqual(['content']);
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

    it('clears queued follow-ups', async () => {
      mockChat.mockImplementation(() => new Promise(() => {}));

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess-1' }));
      void chat.sendMessage('active');
      await new Promise((r) => setTimeout(r, 0));
      await chat.sendMessage('queued');

      expect(chat.queuedFollowUpCount()).toBe(1);

      chat.clearMessages();

      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.messages()).toEqual([]);
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
      const loaded = await chat.loadSession('sess-42');

      expect(loaded).toBe(true);
      expect(chat.sessionId()).toBe('sess-42');
      const msgs = chat.messages();
      expect(msgs).toHaveLength(2);
      expect(msgs[0].role).toBe('user');
      expect(msgs[0].content).toBe('hello');
      expect(msgs[1].toolCalls).toHaveLength(1);
      expect(msgs[1].streamEvents).toEqual([
        {
          type: 'tool',
          tool: { name: 'test', input: '{}', output: 'ok', success: true },
        },
        { type: 'content', content: 'hi there' },
      ]);
      expect(msgs[1].timestamp).toBeInstanceOf(Date);
      dispose();
    });

    it('restores the latest explicit model route before publishing the loaded session id', async () => {
      mockGetMessages.mockResolvedValue([
        {
          id: 'msg-1',
          role: 'user',
          content: 'first',
          timestamp: '2024-01-01T00:00:00Z',
          model: 'deepseek:deepseek-v4-pro',
        },
        {
          id: 'msg-2',
          role: 'assistant',
          content: 'first answer',
          timestamp: '2024-01-01T00:00:01Z',
          model: 'deepseek:deepseek-v4-pro',
        },
        {
          id: 'msg-3',
          role: 'user',
          content: 'continue through OpenRouter',
          timestamp: '2024-01-01T00:00:02Z',
          model: 'openrouter:qwen/qwen3.7-plus',
        },
        {
          id: 'msg-4',
          role: 'assistant',
          content: 'latest answer',
          timestamp: '2024-01-01T00:00:03Z',
        },
      ]);

      const sessionModelSnapshots: Array<{ sessionId: string; model: string }> = [];
      const { value: chat, dispose } = withRoot(() => {
        const chat = useChat({ model: 'openai:gpt-4o' });
        createEffect(() => {
          const loadedSessionId = chat.sessionId();
          if (loadedSessionId) {
            sessionModelSnapshots.push({ sessionId: loadedSessionId, model: chat.model() });
          }
        });
        return chat;
      });

      const loaded = await chat.loadSession('sess-42');
      await Promise.resolve();

      expect(loaded).toBe(true);
      expect(chat.model()).toBe('openrouter:qwen/qwen3.7-plus');
      expect(sessionModelSnapshots).toContainEqual({
        sessionId: 'sess-42',
        model: 'openrouter:qwen/qwen3.7-plus',
      });
      dispose();
    });

    it('keeps the current model when loaded history only has legacy model names', async () => {
      mockGetMessages.mockResolvedValue([
        {
          id: 'msg-1',
          role: 'user',
          content: 'hello',
          timestamp: '2024-01-01T00:00:00Z',
          model: 'gpt-4o',
        },
        {
          id: 'msg-2',
          role: 'assistant',
          content: 'hi there',
          timestamp: '2024-01-01T00:00:01Z',
          model: 'openai/gpt-4o-mini',
        },
      ]);

      const { value: chat, dispose } = withRoot(() =>
        useChat({ model: 'deepseek:deepseek-v4-pro' }),
      );

      const loaded = await chat.loadSession('sess-legacy-models');

      expect(loaded).toBe(true);
      expect(chat.model()).toBe('deepseek:deepseek-v4-pro');
      dispose();
    });

    it('hydrates saved assistant tool calls into visible transcript events', async () => {
      mockGetMessages.mockResolvedValue([
        {
          id: 'msg-1',
          role: 'assistant',
          content: 'I checked the resource.',
          timestamp: '2024-01-01T00:00:01Z',
          model: 'openai:gpt-4',
          tool_calls: [
            {
              name: 'pulse_read',
              input: { target: 'vm-100' },
              output: 'Resource is running',
              success: true,
            },
          ],
        },
      ]);

      const { value: chat, dispose } = withRoot(() => useChat());
      await chat.loadSession('sess-42');

      const assistant = chat.messages()[0];
      expect(assistant.model).toBe('openai:gpt-4');
      expect(assistant.content).toBe('I checked the resource.');
      expect(assistant.streamEvents).toEqual([
        {
          type: 'tool',
          tool: {
            name: 'pulse_read',
            input: '{"target":"vm-100"}',
            output: 'Resource is running',
            success: true,
          },
        },
        { type: 'content', content: 'I checked the resource.' },
      ]);
      dispose();
    });

    it('clears queued follow-ups before loading another session', async () => {
      mockChat.mockImplementation(() => new Promise(() => {}));
      mockGetMessages.mockResolvedValue([]);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess-1' }));
      void chat.sendMessage('active');
      await new Promise((r) => setTimeout(r, 0));
      await chat.sendMessage('queued');

      expect(chat.queuedFollowUpCount()).toBe(1);

      const loaded = await chat.loadSession('sess-2');

      expect(loaded).toBe(true);
      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.messages()).toEqual([]);
      dispose();
    });

    it('handles load error gracefully', async () => {
      mockGetMessages.mockRejectedValue(new Error('not found'));

      const { value: chat, dispose } = withRoot(() => useChat());
      const loaded = await chat.loadSession('bad-id');

      expect(loaded).toBe(false);
      expect(mockNotifyError).toHaveBeenCalledWith('Failed to load session');
      expect(chat.messages()).toEqual([]);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // undo / redo
  // ──────────────────────────────────────────────
  describe('undo and redo', () => {
    it('undoes the latest turn, reloads the session, and returns the editable prompt draft', async () => {
      const mentions: ChatMention[] = [
        { id: 'vm:pve:101', name: 'vm-101', type: 'vm', node: 'pve' },
      ];
      mockChat.mockResolvedValue(undefined);
      mockUndoLastTurn.mockResolvedValue({
        success: true,
        session_id: 'sess-undo',
        restored_prompt: 'inspect vm-101',
        removed_messages: 2,
        can_redo: true,
      });
      mockGetMessages.mockResolvedValue([
        {
          id: 'msg-previous',
          role: 'assistant',
          content: 'Previous answer',
          timestamp: '2026-06-06T12:00:00Z',
        },
      ]);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess-undo' }));
      await chat.sendMessage('inspect vm-101', mentions, 'finding-101', {
        model: 'openrouter:deepseek/deepseek-chat',
        autonomousMode: false,
        handoffContext: '[Patrol Finding Context]\nVM 101 has stale guest tools',
      });

      const draft = await chat.undoLastTurn();

      expect(mockUndoLastTurn).toHaveBeenCalledWith('sess-undo');
      expect(mockGetMessages).toHaveBeenCalledWith('sess-undo');
      expect(chat.messages()).toHaveLength(1);
      expect(draft).toEqual({
        prompt: 'inspect vm-101',
        request: {
          mentions,
          findingId: 'finding-101',
          model: 'openrouter:deepseek/deepseek-chat',
          autonomousMode: false,
          handoffContext: '[Patrol Finding Context]\nVM 101 has stale guest tools',
        },
      });
      dispose();
    });

    it('redoes an undone turn and returns remaining redo availability', async () => {
      mockRedoLastTurn.mockResolvedValue({
        success: true,
        session_id: 'sess-redo',
        restored_messages: 2,
        can_redo: true,
      });
      mockGetMessages.mockResolvedValue([
        { id: 'msg-1', role: 'user', content: 'hello', timestamp: '2026-06-06T12:00:00Z' },
      ]);

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'sess-redo' }));
      const result = await chat.redoLastTurn();

      expect(mockRedoLastTurn).toHaveBeenCalledWith('sess-redo');
      expect(mockGetMessages).toHaveBeenCalledWith('sess-redo');
      expect(result).toEqual({ success: true, canRedo: true });
      expect(chat.messages()).toHaveLength(1);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // newSession
  // ──────────────────────────────────────────────
  describe('newSession', () => {
    it('starts a blank conversation without precreating a backend session', async () => {
      mockChat.mockResolvedValue(undefined);
      const onConversationChanged = vi.fn();

      const { value: chat, dispose } = withRoot(() =>
        useChat({ sessionId: 'old', onConversationChanged }),
      );
      await chat.sendMessage('setup');

      const session = await chat.newSession();

      expect(session).toBe(true);
      expect(mockCreateSession).not.toHaveBeenCalled();
      expect(chat.sessionId()).toBe('');
      expect(chat.messages()).toEqual([]);
      expect(onConversationChanged).toHaveBeenCalledTimes(1);
      dispose();
    });

    it('clears queued follow-ups when starting a blank conversation', async () => {
      mockChat.mockImplementation(() => new Promise(() => {}));

      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 'old' }));
      void chat.sendMessage('active');
      await new Promise((r) => setTimeout(r, 0));
      await chat.sendMessage('queued');

      expect(chat.queuedFollowUpCount()).toBe(1);

      await chat.newSession();

      expect(chat.queuedFollowUpCount()).toBe(0);
      expect(chat.sessionId()).toBe('');
      expect(chat.messages()).toEqual([]);
      dispose();
    });
  });

  // ──────────────────────────────────────────────
  // updateApproval
  // ──────────────────────────────────────────────
  describe('updateApproval', () => {
    async function setupWithApproval() {
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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

    it('does not start a blank follow-up stream when idle after answering', async () => {
      let fireEvent!: TestStreamDispatch;
      let chatCallCount = 0;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          chatCallCount++;
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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

      expect(chatCallCount).toBe(chatCallsBefore);
      dispose();
    });

    it('handles answer failure — resets isAnswering, shows notification', async () => {
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      const toolEvents = assistant.streamEvents?.filter((event) => event.type === 'tool');
      expect(toolEvents).toHaveLength(1);
      expect(toolEvents![0]).toEqual({
        type: 'tool',
        tool: {
          name: 'mystery_tool',
          input: '{}',
          output: 'ok',
          success: true,
        },
        toolId: 'orphan',
        updatedAt: expect.any(Number),
      });
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
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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
      let fireEvent!: TestStreamDispatch;
      mockChat.mockImplementation(
        (_p: string, _s: string, _m: string | undefined, onEvent: (e: StreamEvent) => void) => {
          fireEvent = (event) => dispatchTestStreamEvent(onEvent, event);
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

    it('keeps pending tool transcript order when an earlier tool reports progress', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'a', name: 'tool_a', input: '{}' } });
      fire({ type: 'tool_start', data: { id: 'b', name: 'tool_b', input: '{}' } });
      fire({
        type: 'tool_progress',
        data: {
          id: 'a',
          name: 'tool_a',
          input: '{}',
          phase: 'running',
          message: 'Reading node inventory',
        },
      });

      const assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools?.map((tool) => tool.id)).toEqual(['a', 'b']);
      expect(assistant.pendingTools?.[0]).toMatchObject({
        id: 'a',
        progress: 'Reading node inventory',
        status: 'running',
      });
      expect(assistant.pendingTools?.[0].updatedAt).toBeGreaterThanOrEqual(
        assistant.pendingTools?.[0].startedAt || 0,
      );
      expect(assistant.streamEvents?.filter((event) => event.type === 'pending_tool')).toHaveLength(
        2,
      );
      expect(
        assistant.streamEvents
          ?.filter((event) => event.type === 'pending_tool')
          .map((event) => event.pendingTool?.id),
      ).toEqual(['a', 'b']);
      dispose();
    });

    it('replaces a canceled pending tool with durable skipped activity', async () => {
      const { getFireEvent } = setupWithEventCapture();
      const { value: chat, dispose } = withRoot(() => useChat({ sessionId: 's' }));

      await chat.sendMessage('hi');
      const fire = getFireEvent();

      fire({ type: 'tool_start', data: { id: 'a', name: 'pulse_read', input: '{}' } });

      let assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toHaveLength(1);
      expect(assistant.streamEvents?.filter((event) => event.type === 'pending_tool')).toHaveLength(
        1,
      );

      fire({
        type: 'tool_cancel',
        data: { id: 'a', name: 'pulse_read', reason: 'current_resource unavailable' },
      });

      assistant = chat.messages().find((m) => m.role === 'assistant')!;
      expect(assistant.pendingTools).toEqual([]);
      expect(assistant.toolCalls).toEqual([]);
      expect(assistant.streamEvents?.filter((event) => event.type === 'pending_tool')).toHaveLength(
        0,
      );
      expect(assistant.streamEvents?.filter((event) => event.type === 'tool_cancel')).toEqual([
        expect.objectContaining({
          type: 'tool_cancel',
          toolId: 'a',
          toolCancel: {
            id: 'a',
            name: 'pulse_read',
            input: '{}',
            rawInput: undefined,
            reason: 'current_resource unavailable',
          },
        }),
      ]);
      dispose();
    });
  });
});
