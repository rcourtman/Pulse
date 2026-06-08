import { describe, expect, it, vi, afterEach, beforeAll, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Show, createSignal } from 'solid-js';
import type { ChatMessage, ModelInfo, ModelRouteRecoveryOption } from '../types';
import type { QueuedFollowUp } from '../hooks/useChat';
import { WORKFLOW_STATUS_PACE_MS } from '../workflowStatusDisplay';

// ── Hoisted mocks (vi.mock factories reference these) ──────────────────────

const {
  mockChat,
  mockAIAPI,
  mockAIChatAPI,
  mockNotificationStore,
  mockAiChatStore,
  mockByType,
  mockResources,
  mockWebSocketState,
  mockChatMessagesProps,
  mockModelSelectorProps,
  mockNavigate,
} = vi.hoisted(() => {
  const mockChatMessagesProps: Array<{
    messages: ChatMessage[];
    onChangeModel?: () => void;
    getModelRouteLabel?: (modelId: string) => string;
    getModelRouteAlternative?: (message: ChatMessage) => ModelRouteRecoveryOption | null;
    onUseModelRoute?: (modelId: string, messageId?: string) => void;
    queuedFollowUps?: QueuedFollowUp[];
    queuedFollowUpsPaused?: boolean;
    onEditQueuedFollowUp?: (id: string) => void;
    onCancelQueuedFollowUp?: (id: string) => void;
  }> = [];
  const mockModelSelectorProps: Array<{
    selectedModel: string;
    models: ModelInfo[];
    recentModelIds?: string[];
    openRequest?: number;
    initialSearchQuery?: string;
    onManageProviders?: () => void;
    onModelSelect?: (modelId: string) => void;
  }> = [];
  const mockChat = {
    messages: vi.fn((): ChatMessage[] => []),
    isLoading: vi.fn(() => false),
    sessionId: vi.fn(() => ''),
    model: vi.fn(() => ''),
    setModel: vi.fn(),
    queuedFollowUps: vi.fn((): QueuedFollowUp[] => []),
    queuedFollowUpsPaused: vi.fn(() => false),
    queuedFollowUpCount: vi.fn(() => 0),
    sendMessage: vi.fn().mockResolvedValue(true),
    retryMessage: vi.fn(),
    undoLastTurn: vi.fn().mockResolvedValue(null),
    redoLastTurn: vi.fn().mockResolvedValue({ success: false, canRedo: false }),
    stop: vi.fn(),
    cancelQueuedFollowUp: vi.fn(),
    takeQueuedFollowUp: vi.fn((): QueuedFollowUp | undefined => undefined),
    sendQueuedFollowUpNow: vi.fn().mockResolvedValue(true),
    clearQueuedFollowUps: vi.fn(),
    clearMessages: vi.fn(),
    loadSession: vi.fn().mockResolvedValue(true),
    newSession: vi.fn().mockResolvedValue(true),
    updateApproval: vi.fn(),
    addToolResult: vi.fn(),
    updateQuestion: vi.fn(),
    answerQuestion: vi.fn().mockResolvedValue(undefined),
    waitForIdle: vi.fn().mockResolvedValue(true),
  };

  const mockAIAPI = {
    getModels: vi.fn().mockResolvedValue({ models: [] }),
    getSettings: vi.fn().mockResolvedValue({
      model: 'gpt-4',
      chat_model: '',
      control_level: 'read_only',
      autonomous_mode: false,
      discovery_enabled: true,
    }),
    testProvider: vi.fn().mockResolvedValue({
      success: true,
      message: 'Connection successful',
      provider: 'openai',
      model: 'openai:gpt-4',
    }),
    updateSettings: vi.fn().mockResolvedValue({ control_level: 'controlled' }),
  };

  const mockAIChatAPI = {
    getStatus: vi.fn().mockResolvedValue({ running: true }),
    listSessions: vi.fn().mockResolvedValue([]),
    createSession: vi.fn().mockResolvedValue({
      id: 'new',
      title: '',
      created_at: '',
      updated_at: '',
      message_count: 0,
    }),
    deleteSession: vi.fn().mockResolvedValue(undefined),
    renameSession: vi.fn().mockResolvedValue({
      id: 'renamed-session',
      title: 'Renamed session',
      created_at: '2026-06-06T12:30:00Z',
      updated_at: '2026-06-06T12:35:00Z',
      message_count: 2,
    }),
    forkSession: vi.fn().mockResolvedValue({
      id: 'forked-session',
      title: 'Forked session',
      created_at: '2026-06-06T12:45:00Z',
      updated_at: '2026-06-06T12:45:00Z',
      message_count: 2,
    }),
    summarizeSession: vi.fn().mockResolvedValue({
      success: true,
      status: 'compacted',
      message: 'Compacted 2 older messages into a session summary.',
      session_id: 'source-session',
      compacted_messages: 2,
      kept_recent_messages: 4,
    }),
    approveCommand: vi.fn().mockResolvedValue({ approved: true, message: 'ok' }),
    denyCommand: vi.fn().mockResolvedValue({ denied: true, message: 'ok' }),
  };

  const mockNotificationStore = {
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
    error: vi.fn(),
  };

  const emptyChatContext = () => ({
    targetType: undefined,
    targetId: undefined,
    findingId: undefined,
    autonomousMode: undefined,
    context: undefined,
    briefing: undefined,
    handoffContext: undefined,
    handoffResources: undefined,
    handoffActions: undefined,
    handoffMetadata: undefined,
  });

  const mockAiChatStore = {
    isOpenSignal: vi.fn(() => true),
    commandRequestSignal: vi.fn(
      (): {
        id: number;
        action:
          | 'compact'
          | 'help'
          | 'new'
          | 'sessions'
          | 'models'
          | 'providers'
          | 'status'
          | 'undo'
          | 'redo';
      } | null => null,
    ),
    ackCommandRequest: vi.fn(),
    context: emptyChatContext() as {
      targetType?: string;
      targetId?: string;
      findingId?: string;
      autonomousMode?: boolean;
      context?: Record<string, unknown>;
      handoffContext?: string;
      handoffResources?: Array<{
        id: string;
        name?: string;
        type?: string;
        node?: string;
      }>;
      handoffActions?: Array<{
        findingId?: string;
        approvalId?: string;
        approvalStatus?: string;
      }>;
      handoffMetadata?: {
        kind?: string;
        runId?: string;
        runType?: string;
        runStatus?: string;
        runtimeFailure?: boolean;
      };
      briefing?: {
        sourceLabel: string;
        title: string;
        subject?: string;
        statusLabel?: string;
        detailLines?: string[];
        evidence?: string[];
        actionLabel?: string;
        commandSummary?: string;
        safetyNote?: string;
        actionHref?: string;
      };
    },
    setContext: vi.fn((context: any) => {
      mockAiChatStore.context = context;
    }),
    clearContext: vi.fn(() => {
      mockAiChatStore.context = emptyChatContext();
    }),
    clearFindingId: vi.fn(() => {
      const { findingId: _findingId, ...rest } = mockAiChatStore.context;
      mockAiChatStore.context = rest;
    }),
    registerInput: vi.fn(),
    clearRequestHandoffPayload: vi.fn(() => {
      const {
        handoffContext: _handoffContext,
        handoffResources: _handoffResources,
        handoffActions: _handoffActions,
        handoffMetadata: _handoffMetadata,
        ...rest
      } = mockAiChatStore.context;
      mockAiChatStore.context = rest;
    }),
  };

  const mockByType = vi.fn((_type: string): Array<{ name: string }> => []);
  const mockResources = vi.fn((): Array<{ id: string; type: string }> => []);
  const mockWebSocketState = {
    resources: [] as Array<{ id: string; type: string }> | undefined,
  };
  const mockNavigate = vi.fn();

  return {
    mockChat,
    mockAIAPI,
    mockAIChatAPI,
    mockNotificationStore,
    mockAiChatStore,
    mockByType,
    mockResources,
    mockWebSocketState,
    mockChatMessagesProps,
    mockModelSelectorProps,
    mockNavigate,
  };
});

// ── Module mocks ───────────────────────────────────────────────────────────

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('../hooks/useChat', () => ({
  latestExplicitModelRouteFromTranscript: (messages: ChatMessage[]) => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      const requestRoute = message.request?.model?.trim() || '';
      if (/^[a-z][a-z0-9_-]*:.+/i.test(requestRoute) && !requestRoute.includes('://')) {
        return requestRoute;
      }

      const messageRoute = message.model?.trim() || '';
      const isRouteSwitch = message.streamEvents?.some(
        (event) =>
          event.type === 'model_switch' &&
          event.model?.trim() === messageRoute &&
          Boolean(event.failedModel?.trim()) &&
          event.modelEvent !== 'selected',
      );
      if (
        messageRoute &&
        /^[a-z][a-z0-9_-]*:.+/i.test(messageRoute) &&
        !messageRoute.includes('://') &&
        !isRouteSwitch
      ) {
        return messageRoute;
      }
    }
    return '';
  },
  useChat: () => mockChat,
}));

vi.mock('../ChatMessages', () => ({
  ChatMessages: (props: {
    messages: ChatMessage[];
    onChangeModel?: () => void;
    getModelRouteLabel?: (modelId: string) => string;
    getModelRouteAlternative?: (message: ChatMessage) => ModelRouteRecoveryOption | null;
    onUseModelRoute?: (modelId: string, messageId?: string) => void;
    queuedFollowUps?: QueuedFollowUp[];
    queuedFollowUpsPaused?: boolean;
    onEditQueuedFollowUp?: (id: string) => void;
    onCancelQueuedFollowUp?: (id: string) => void;
  }) => {
    const routeRecovery = () => {
      const failedMessage = props.messages.find((message) => message.error);
      const alternative = failedMessage ? props.getModelRouteAlternative?.(failedMessage) : null;
      return failedMessage && alternative ? { alternative, failedMessage } : null;
    };
    mockChatMessagesProps.push(props);
    return (
      <div data-testid="chat-messages" data-msg-count={props.messages.length}>
        <button
          type="button"
          data-testid="mock-change-model"
          onClick={() => props.onChangeModel?.()}
        >
          Change model
        </button>
        <Show when={routeRecovery()}>
          {(recovery) => (
            <button
              type="button"
              data-testid="mock-use-model-route"
              onClick={() =>
                props.onUseModelRoute?.(recovery().alternative.id, recovery().failedMessage.id)
              }
            >
              {recovery().alternative.kind === 'same-model-route'
                ? `Retry with ${recovery().alternative.providerLabel} route`
                : `Retry with ${recovery().alternative.providerLabel} model route`}
            </button>
          )}
        </Show>
      </div>
    );
  },
}));

vi.mock('../ModelSelector', () => ({
  ModelSelector: (props: {
    selectedModel: string;
    models: ModelInfo[];
    recentModelIds?: string[];
    openRequest?: number;
    initialSearchQuery?: string;
    onManageProviders?: () => void;
    onModelSelect?: (modelId: string) => void;
  }) => {
    mockModelSelectorProps.push(props);
    return (
      <div
        data-testid="model-selector"
        data-selected={props.selectedModel}
        data-count={props.models.length}
        data-open-request={String(props.openRequest || 0)}
        data-initial-search={props.initialSearchQuery || ''}
        data-recent-models={(props.recentModelIds || []).join('|')}
      />
    );
  },
}));

vi.mock('../MentionAutocomplete', () => ({
  MentionAutocomplete: (props: {
    visible: boolean;
    query: string;
    resources: Array<{ id: string; label: string; node?: string }>;
    onSelect: (resource: { id: string; label: string; node?: string }) => void;
  }) => (
    <div
      data-testid="mention-autocomplete"
      data-visible={String(props.visible)}
      data-query={props.query}
      data-resource-count={String(props.resources.length)}
      data-resource-ids={props.resources.map((resource) => resource.id).join('|')}
      data-resource-labels={props.resources.map((resource) => resource.label).join('|')}
    >
      <button
        type="button"
        data-testid="mention-select-first"
        onClick={() => {
          if (props.resources[0]) {
            props.onSelect(props.resources[0]);
          }
        }}
      >
        select-first
      </button>
      {props.resources.map((resource) => (
        <button
          type="button"
          data-testid={`mention-select-${resource.id}`}
          onClick={() => props.onSelect(resource)}
        >
          {`select-${resource.id}`}
        </button>
      ))}
    </div>
  ),
}));

vi.mock('@/api/ai', () => ({ AIAPI: mockAIAPI }));
vi.mock('@/api/aiChat', () => ({ AIChatAPI: mockAIChatAPI }));
vi.mock('@/stores/notifications', () => ({ notificationStore: mockNotificationStore }));
vi.mock('@/stores/aiChat', () => ({ aiChatStore: mockAiChatStore }));
vi.mock('@/utils/logger', () => ({
  logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));
vi.mock('@/hooks/useUnifiedResources', () => ({
  getCachedUnifiedResources: mockResources,
}));
vi.mock('@/stores/websocket-global', () => ({
  getGlobalWebSocketStore: () => ({ state: mockWebSocketState }),
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ byType: mockByType, resources: mockResources }),
}));

// ── Lazy import after mocks ────────────────────────────────────────────────

let AIChat: typeof import('../index').AIChat;
let resetAIChatComposerDraftStashForTests: typeof import('../index').resetAIChatComposerDraftStashForTests;
let resetAIRuntimeState: typeof import('@/stores/aiRuntimeState').resetAIRuntimeState;

beforeAll(async () => {
  const [chatModule, runtimeModule] = await Promise.all([
    import('../index'),
    import('@/stores/aiRuntimeState'),
  ]);
  AIChat = chatModule.AIChat;
  resetAIChatComposerDraftStashForTests = chatModule.resetAIChatComposerDraftStashForTests;
  resetAIRuntimeState = runtimeModule.resetAIRuntimeState;
});

// ── Helpers ────────────────────────────────────────────────────────────────

function renderChat(onClose = vi.fn()) {
  return render(() => <AIChat onClose={onClose} />);
}

function setViewportWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    value: width,
    writable: true,
    configurable: true,
  });
  window.dispatchEvent(new Event('resize'));
}

async function waitForProviderCheckSettled() {
  await waitFor(() => {
    expect(mockAIAPI.testProvider).toHaveBeenCalled();
  });
  await waitFor(() => {
    expect(screen.queryByText('Verifying selected model route')).not.toBeInTheDocument();
  });
  await waitFor(() => {
    expect(screen.getByRole('button', { name: 'Send message' })).not.toBeDisabled();
  });
}

const readBlobText = (blob: Blob): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(reader.error);
    reader.readAsText(blob);
  });

// ── Setup / teardown ───────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks();
  mockChatMessagesProps.length = 0;
  mockModelSelectorProps.length = 0;
  mockNavigate.mockReset();
  resetAIChatComposerDraftStashForTests();
  setViewportWidth(1440);
  resetAIRuntimeState();
  mockAiChatStore.isOpenSignal.mockReturnValue(true);
  mockAiChatStore.commandRequestSignal.mockReturnValue(null);
  mockAiChatStore.context = {
    findingId: undefined,
    autonomousMode: undefined,
    briefing: undefined,
  };
  mockChat.messages.mockReturnValue([]);
  mockChat.isLoading.mockReturnValue(false);
  mockChat.sessionId.mockReturnValue('');
  mockChat.model.mockReturnValue('');
  mockChat.queuedFollowUps.mockReturnValue([]);
  mockChat.queuedFollowUpsPaused.mockReturnValue(false);
  mockChat.queuedFollowUpCount.mockReturnValue(0);
  mockChat.sendMessage.mockResolvedValue(true);
  mockChat.sendQueuedFollowUpNow.mockResolvedValue(true);
  mockChat.undoLastTurn.mockResolvedValue(null);
  mockChat.redoLastTurn.mockResolvedValue({ success: false, canRedo: false });
  mockChat.takeQueuedFollowUp.mockReturnValue(undefined);
  mockByType.mockReturnValue([]);
  mockResources.mockReturnValue([]);
  mockWebSocketState.resources = [];
  mockAIAPI.getModels.mockResolvedValue({ models: [] });
  mockAIAPI.getSettings.mockResolvedValue({
    model: 'gpt-4',
    chat_model: '',
    control_level: 'read_only',
    autonomous_mode: false,
    discovery_enabled: true,
  });
  mockAIAPI.testProvider.mockResolvedValue({
    success: true,
    message: 'Connection successful',
    provider: 'openai',
    model: 'openai:gpt-4',
  });
  mockAIChatAPI.getStatus.mockResolvedValue({ running: true });
  mockAIChatAPI.listSessions.mockResolvedValue([]);
  mockAIChatAPI.renameSession.mockResolvedValue({
    id: 'renamed-session',
    title: 'Renamed session',
    created_at: '2026-06-06T12:30:00Z',
    updated_at: '2026-06-06T12:35:00Z',
    message_count: 2,
  });
  mockAIChatAPI.forkSession.mockResolvedValue({
    id: 'forked-session',
    title: 'Forked session',
    created_at: '2026-06-06T12:45:00Z',
    updated_at: '2026-06-06T12:45:00Z',
    message_count: 2,
  });
  mockAIChatAPI.summarizeSession.mockResolvedValue({
    success: true,
    status: 'compacted',
    message: 'Compacted 2 older messages into a session summary.',
    session_id: 'source-session',
    compacted_messages: 2,
    kept_recent_messages: 4,
  });
  Element.prototype.scrollIntoView = vi.fn();
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// ── Tests ──────────────────────────────────────────────────────────────────

describe('AIChat', () => {
  // ── Rendering ──────────────────────────────────────────────────────────

  describe('rendering', () => {
    it('renders the header with title when open', () => {
      renderChat();
      expect(screen.getByText('Pulse Assistant')).toBeInTheDocument();
    });

    it('disables transcript actions until the current session has messages', () => {
      renderChat();

      expect(screen.getByRole('button', { name: 'Fork current Assistant session' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Copy last Assistant answer' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Copy Assistant transcript' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Export Assistant transcript' })).toBeDisabled();
    });

    it('copies the latest Assistant answer from the header', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        configurable: true,
      });
      mockChat.messages.mockReturnValue([
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'Earlier answer',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'user-1',
          role: 'user',
          content: 'what changed now?',
          timestamp: new Date('2026-06-06T12:35:00Z'),
        },
        {
          id: 'assistant-2',
          role: 'assistant',
          content: '',
          isStreaming: true,
          streamEvents: [
            {
              type: 'content',
              content: 'Current streamed answer.',
            },
          ],
          timestamp: new Date('2026-06-06T12:35:01Z'),
        },
      ]);

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Copy last Assistant answer' }));

      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith('Current streamed answer.');
      });
      await waitFor(() => {
        expect(mockNotificationStore.success).toHaveBeenCalledWith('Assistant answer copied', 2000);
      });
    });

    it('copies the current Assistant transcript from the header', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        configurable: true,
      });
      mockChat.sessionId.mockReturnValue('session-123456789');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'how many devices in this',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'There are 4,358 entries in /dev.',
          timestamp: new Date('2026-06-06T12:35:01Z'),
          model: 'openrouter:deepseek/deepseek-chat',
          streamEvents: [
            {
              type: 'tool',
              tool: {
                name: 'pulse_read',
                input: JSON.stringify({
                  action: 'exec',
                  command: 'ls /dev | wc -l',
                  target_host: 'current_resource',
                }),
                output: '4358\n',
                success: true,
              },
            },
            {
              type: 'content',
              content: 'There are 4,358 entries in /dev.',
            },
          ],
        },
      ]);

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Copy Assistant transcript' }));

      await waitFor(() => {
        expect(writeText).toHaveBeenCalled();
      });
      const transcript = writeText.mock.calls[0][0] as string;
      expect(transcript).toContain('Session ID: session-123456789');
      expect(transcript).toContain('how many devices in this');
      expect(transcript).toContain('[tool:read]');
      expect(transcript).toContain('Inspect devices on current resource');
      expect(transcript).toContain('$ ls /dev | wc -l');
      expect(transcript).toContain('There are 4,358 entries in /dev.');
      expect(transcript).not.toContain('pulse_read');
      await waitFor(() => {
        expect(mockNotificationStore.success).toHaveBeenCalledWith(
          'Assistant transcript copied',
          2000,
        );
      });
    });

    it('opens a manual transcript panel when clipboard writes are blocked', async () => {
      const writeText = vi.fn().mockRejectedValue(new DOMException('Denied', 'NotAllowedError'));
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        configurable: true,
      });
      mockChat.sessionId.mockReturnValue('session-clipboard-denied');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'copy fallback check',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'fallback transcript body',
          timestamp: new Date('2026-06-06T12:35:01Z'),
        },
      ]);

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Copy Assistant transcript' }));

      const transcriptField = await screen.findByLabelText('Assistant transcript');
      const transcriptValue = (transcriptField as HTMLTextAreaElement).value;
      expect(transcriptValue).toContain('copy fallback check');
      expect(transcriptValue).toContain('fallback transcript body');
      expect(document.activeElement).toBe(transcriptField);
      expect(mockNotificationStore.warning).toHaveBeenCalledWith(
        'Clipboard blocked; transcript opened for manual copy',
        4000,
      );

      fireEvent.click(
        screen.getByRole('button', { name: 'Close Assistant transcript copy panel' }),
      );

      await waitFor(() => {
        expect(screen.queryByLabelText('Assistant transcript')).not.toBeInTheDocument();
      });
    });

    it('exports the current Assistant transcript from the header', async () => {
      const originalCreateObjectURL = URL.createObjectURL;
      const originalRevokeObjectURL = URL.revokeObjectURL;
      const createObjectURL = vi.fn((blob: Blob) => {
        void blob;
        return 'blob:assistant-transcript';
      });
      const revokeObjectURL = vi.fn();
      Object.defineProperty(URL, 'createObjectURL', {
        value: createObjectURL,
        configurable: true,
      });
      Object.defineProperty(URL, 'revokeObjectURL', {
        value: revokeObjectURL,
        configurable: true,
      });
      const originalCreateElement = document.createElement.bind(document);
      const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tagName) => {
        const element = originalCreateElement(tagName);
        if (tagName.toLowerCase() === 'a') {
          vi.spyOn(element as HTMLAnchorElement, 'click').mockImplementation(() => undefined);
        }
        return element;
      });
      mockChat.sessionId.mockReturnValue('session-abcdef123456');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'summarize the session',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
      ]);

      try {
        renderChat();
        fireEvent.click(screen.getByRole('button', { name: 'Export Assistant transcript' }));

        await waitFor(() => {
          expect(createObjectURL).toHaveBeenCalled();
        });
        const createdBlob = createObjectURL.mock.calls[0]?.[0] as unknown as Blob;
        expect(await readBlobText(createdBlob)).toContain('summarize the session');
        const anchor = createElementSpy.mock.results
          .map((result) => result.value)
          .find((element): element is HTMLAnchorElement => element instanceof HTMLAnchorElement);
        expect(anchor?.download).toBe('pulse-assistant-session-abcdef123456.md');
        expect(anchor?.click).toHaveBeenCalled();
        expect(revokeObjectURL).toHaveBeenCalledWith('blob:assistant-transcript');
        expect(mockNotificationStore.success).toHaveBeenCalledWith(
          'Assistant transcript exported',
          2000,
        );
      } finally {
        createElementSpy.mockRestore();
        if (originalCreateObjectURL) {
          Object.defineProperty(URL, 'createObjectURL', {
            value: originalCreateObjectURL,
            configurable: true,
          });
        }
        if (originalRevokeObjectURL) {
          Object.defineProperty(URL, 'revokeObjectURL', {
            value: originalRevokeObjectURL,
            configurable: true,
          });
        }
      }
    });

    it('renders the input textarea', () => {
      renderChat();
      expect(screen.getByPlaceholderText('Ask about your infrastructure...')).toBeInTheDocument();
    });

    it('registers and focuses the composer when opened', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      await waitFor(() => {
        expect(mockAiChatStore.registerInput).toHaveBeenCalledWith(textarea);
        expect(document.activeElement).toBe(textarea);
      });
    });

    it('renders the ChatMessages child component', () => {
      renderChat();
      expect(screen.getByTestId('chat-messages')).toBeInTheDocument();
    });

    it('renders the ModelSelector child component', () => {
      renderChat();
      expect(screen.getByTestId('model-selector')).toBeInTheDocument();
    });

    it('opens the model selector from failed-turn recovery', async () => {
      renderChat();

      expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '0');

      fireEvent.click(screen.getByTestId('mock-change-model'));

      await waitFor(() => {
        expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '1');
      });
    });

    it('passes catalog-grade model route labels to transcript rows', async () => {
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'openrouter:deepseek/deepseek-v4-pro',
            name: 'DeepSeek: DeepSeek V4 Pro',
            provider: 'openrouter',
            notable: true,
          },
        ],
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.getModels).toHaveBeenCalled();
      });
      await waitFor(() => {
        const props = mockChatMessagesProps[mockChatMessagesProps.length - 1];
        expect(props?.getModelRouteLabel?.('openrouter:deepseek/deepseek-v4-pro')).toBe(
          'DeepSeek: DeepSeek V4 Pro via OpenRouter',
        );
      });
    });

    it('switches a failed turn to an equivalent configured-provider route and retries it', async () => {
      mockChat.model.mockReturnValue('deepseek:deepseek-v4-pro');
      mockChat.messages.mockReturnValue([
        {
          id: 'assistant-error-1',
          role: 'assistant',
          content: '',
          error:
            'Pulse could not reach the AI provider endpoint. Check the selected provider URL and network connection, then retry.',
          timestamp: new Date('2026-06-05T10:00:00Z'),
          model: 'deepseek:deepseek-v4-pro',
        },
      ]);
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
        configured_providers: ['deepseek', 'openrouter'],
      });
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'deepseek:deepseek-v4-pro',
            name: 'DeepSeek V4 Pro',
            provider: 'deepseek',
            notable: true,
          },
          {
            id: 'openrouter:deepseek/deepseek-v4-pro',
            name: 'DeepSeek: DeepSeek V4 Pro',
            provider: 'openrouter',
            notable: true,
          },
        ],
      });

      renderChat();

      fireEvent.click(await screen.findByTestId('mock-use-model-route'));

      const props = mockChatMessagesProps[mockChatMessagesProps.length - 1];
      const alternative = props.getModelRouteAlternative?.(mockChat.messages()[0]);
      expect(alternative).toMatchObject({
        id: 'openrouter:deepseek/deepseek-v4-pro',
        kind: 'same-model-route',
        provider: 'openrouter',
      });
      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(mockChat.retryMessage).toHaveBeenCalledWith('assistant-error-1', {
        model: 'openrouter:deepseek/deepseek-v4-pro',
      });
      expect(document.activeElement).toBe(
        screen.getByPlaceholderText('Ask about your infrastructure...'),
      );
    });

    it('does not offer provider route recovery for local fixture failures', async () => {
      const localFixtureFailure: ChatMessage = {
        id: 'assistant-local-fixture-error',
        role: 'assistant',
        content: '',
        error:
          'Unknown Assistant fixture "/fixture typo-check". Available fixtures: devices, assistant-stream, send-hold, tool-burst, workflow-burst, context-group, status-boundary, pending-tool, command-tool, long-output, provider-retry, stream-idle, queue-hold, queue-drain, compacted-artifact, skipped-tool.',
        timestamp: new Date('2026-06-05T10:00:00Z'),
        model: 'openrouter:qwen/qwen3.7-plus',
        streamEvents: [
          {
            type: 'model_switch',
            model: 'openrouter:qwen/qwen3.7-plus',
            modelEvent: 'selected',
          },
        ],
      };
      mockChat.model.mockReturnValue('openrouter:qwen/qwen3.7-plus');
      mockChat.messages.mockReturnValue([localFixtureFailure]);
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'openrouter:qwen/qwen3.7-plus',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
        configured_providers: ['openrouter', 'openai'],
      });
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'openrouter:qwen/qwen3.7-plus',
            name: 'Qwen: Qwen3.7 Plus',
            provider: 'openrouter',
            notable: true,
          },
          {
            id: 'openai:gpt-4o',
            name: 'GPT-4o',
            provider: 'openai',
            notable: true,
          },
        ],
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.getModels).toHaveBeenCalled();
      });

      const props = mockChatMessagesProps[mockChatMessagesProps.length - 1];
      expect(props.getModelRouteAlternative?.(localFixtureFailure)).toBeNull();
      expect(screen.queryByTestId('mock-use-model-route')).not.toBeInTheDocument();
    });

    it('offers an explicit alternate model route after equivalent routes have already failed', async () => {
      const openRouterFailure: ChatMessage = {
        id: 'assistant-error-openrouter',
        role: 'assistant',
        content: '',
        error:
          'The AI provider rejected the credentials. Check your AI provider API key in Settings.',
        timestamp: new Date('2026-06-05T10:00:00Z'),
        model: 'openrouter:deepseek/deepseek-v4-pro',
      };
      const deepSeekFailure: ChatMessage = {
        id: 'assistant-error-deepseek',
        role: 'assistant',
        content: '',
        error:
          'Pulse could not reach the AI provider endpoint. Check the selected provider URL and network connection, then retry.',
        timestamp: new Date('2026-06-05T10:01:00Z'),
        model: 'deepseek:deepseek-v4-pro',
      };
      mockChat.model.mockReturnValue('deepseek:deepseek-v4-pro');
      mockChat.messages.mockReturnValue([openRouterFailure, deepSeekFailure]);
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
        configured_providers: ['deepseek', 'openrouter', 'openai'],
      });
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'deepseek:deepseek-v4-pro',
            name: 'DeepSeek V4 Pro',
            provider: 'deepseek',
            notable: true,
          },
          {
            id: 'openrouter:deepseek/deepseek-v4-pro',
            name: 'DeepSeek: DeepSeek V4 Pro',
            provider: 'openrouter',
            notable: true,
          },
          {
            id: 'openai:gpt-4o',
            name: 'GPT-4o',
            provider: 'openai',
            notable: true,
          },
        ],
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.getModels).toHaveBeenCalled();
      });

      const props = mockChatMessagesProps[mockChatMessagesProps.length - 1];
      const alternative = props.getModelRouteAlternative?.(deepSeekFailure);

      expect(alternative).toMatchObject({
        id: 'openai:gpt-4o',
        kind: 'alternate-model-route',
        provider: 'openai',
        providerLabel: 'OpenAI',
      });

      props.onUseModelRoute?.(alternative!.id, deepSeekFailure.id);

      expect(mockChat.setModel).toHaveBeenCalledWith('openai:gpt-4o');
      expect(mockChat.retryMessage).toHaveBeenCalledWith('assistant-error-deepseek', {
        model: 'openai:gpt-4o',
      });
    });

    it('checks the selected provider and shows a readiness issue before the first send', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockResolvedValueOnce({
        success: false,
        message: 'Provider connection issue',
        provider: 'deepseek',
        model: 'deepseek:deepseek-v4-pro',
        cause: 'provider_connection',
        summary: 'Pulse could not maintain a healthy connection to this provider.',
        recommendation: 'Check provider reachability, base URL, firewall or proxy rules.',
        action: 'open_provider_settings',
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledWith('deepseek', 'deepseek:deepseek-v4-pro');
        expect(screen.getByLabelText('Assistant selected model route status')).toHaveTextContent(
          'Selected model route issue',
        );
      });
      expect(screen.getByLabelText('Assistant selected model route status')).toHaveTextContent(
        'Pulse could not maintain a healthy connection to this provider.',
      );
      expect(screen.getByLabelText('Assistant selected model route status')).toHaveTextContent(
        'This route stays selected until you choose another route.',
      );
      expect(screen.getByRole('link', { name: /Open settings/ })).toHaveAttribute(
        'href',
        '/settings/system-ai',
      );
    });

    it('shows pending selected-route checks in the composer without blocking input', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'openai:gpt-4',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockReturnValue(new Promise(() => {}));

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledWith('openai', 'openai:gpt-4');
      });
      expect(screen.queryByText('Verifying selected model route')).not.toBeInTheDocument();
      expect(
        screen.queryByLabelText('Assistant selected model route status'),
      ).not.toBeInTheDocument();
      expect(screen.getByLabelText('Assistant selected model route health')).toHaveTextContent(
        'Checking OpenAI',
      );

      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });

      expect(screen.getByRole('button', { name: 'Send message' })).not.toBeDisabled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'summarize the cluster',
        undefined,
        undefined,
      );
      expect(textarea.value).toBe('');
      expect(document.activeElement).toBe(textarea);
    });

    it('sends user input while a selected model route issue is unresolved', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockResolvedValueOnce({
        success: false,
        message: 'Provider connection issue',
        provider: 'deepseek',
        model: 'deepseek:deepseek-v4-pro',
        cause: 'provider_connection',
        summary: 'Pulse could not maintain a healthy connection to this provider.',
        recommendation: 'Check provider reachability.',
        action: 'open_provider_settings',
      });

      renderChat();

      await screen.findByText('Selected model route issue');
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });

      expect(screen.getByRole('button', { name: 'Send message' })).not.toBeDisabled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'summarize the cluster',
        undefined,
        undefined,
      );
      expect(textarea.value).toBe('');
      expect(document.activeElement).toBe(textarea);
    });

    it('keeps a failed stored route selected until the user explicitly changes it', async () => {
      mockChat.model.mockReturnValue('openrouter:deepseek/deepseek-v4-pro');
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: 'gemini:gemini-3.1-flash-lite',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockResolvedValueOnce({
        success: false,
        message: 'Provider authentication issue',
        provider: 'openrouter',
        model: 'openrouter:deepseek/deepseek-v4-pro',
        cause: 'provider_auth',
        summary: 'The provider rejected the configured credentials or account access.',
        recommendation: 'Check the API key or provider authentication.',
        action: 'open_provider_settings',
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledWith(
          'openrouter',
          'openrouter:deepseek/deepseek-v4-pro',
        );
      });
      expect(mockChat.setModel).not.toHaveBeenCalledWith('gemini:gemini-3.1-flash-lite');
      expect(
        screen.queryByRole('status', { name: 'Assistant provider readiness route adopted' }),
      ).not.toBeInTheDocument();
      expect(screen.getByLabelText('Assistant selected model route status')).toHaveTextContent(
        'Selected model route issue',
      );
      expect(mockNotificationStore.success).not.toHaveBeenCalledWith(
        expect.stringContaining('after OpenRouter provider check'),
        expect.any(Number),
      );
    });

    it('sends user input when the readiness issue reports a provider-qualified model', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockResolvedValueOnce({
        success: false,
        message: 'Selected model unavailable',
        provider: 'openai',
        model: 'openai:gpt-4',
        cause: 'model_unavailable',
        summary: 'Pulse could not find the selected model on this provider route.',
        recommendation: 'Choose another OpenAI model.',
        action: 'open_provider_settings',
      });

      renderChat();

      await screen.findByText('Selected model route issue');
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });

      expect(screen.getByRole('button', { name: 'Send message' })).not.toBeDisabled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'summarize the cluster',
        undefined,
        undefined,
      );
      expect(textarea.value).toBe('');
      expect(document.activeElement).toBe(textarea);
    });

    it('offers an equivalent configured-provider route when the selected provider fails', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
        configured_providers: ['deepseek', 'openrouter'],
      });
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'deepseek:deepseek-v4-pro',
            name: 'DeepSeek V4 Pro',
            provider: 'deepseek',
            notable: true,
          },
          {
            id: 'openrouter:deepseek/deepseek-v4-pro',
            name: 'DeepSeek: DeepSeek V4 Pro',
            provider: 'openrouter',
            notable: true,
          },
          {
            id: 'openai:gpt-5.5',
            name: 'GPT-5.5',
            provider: 'openai',
            notable: true,
          },
        ],
      });
      mockAIAPI.testProvider.mockResolvedValueOnce({
        success: false,
        message: 'Provider connection issue',
        provider: 'deepseek',
        model: 'deepseek:deepseek-v4-pro',
        cause: 'provider_connection',
        summary: 'Pulse could not maintain a healthy connection to this provider.',
        recommendation: 'Check provider reachability.',
        action: 'open_provider_settings',
      });

      renderChat();

      const switchButton = await screen.findByRole('button', {
        name: 'Use OpenRouter route',
      });
      expect(switchButton).toHaveTextContent('Use OpenRouter route');

      fireEvent.click(switchButton);

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(document.activeElement).toBe(
        screen.getByPlaceholderText('Ask about your infrastructure...'),
      );
    });

    it('does not automatically change provider route when sending after a selected provider failure', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
        configured_providers: ['deepseek', 'openrouter'],
      });
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'deepseek:deepseek-v4-pro',
            name: 'DeepSeek V4 Pro',
            provider: 'deepseek',
            notable: true,
          },
          {
            id: 'openrouter:deepseek/deepseek-v4-pro',
            name: 'DeepSeek: DeepSeek V4 Pro',
            provider: 'openrouter',
            notable: true,
          },
        ],
      });
      mockAIAPI.testProvider.mockResolvedValueOnce({
        success: false,
        message: 'Provider connection issue',
        provider: 'deepseek',
        model: 'deepseek:deepseek-v4-pro',
        cause: 'provider_connection',
        summary: 'Pulse could not maintain a healthy connection to this provider.',
        recommendation: 'Check provider reachability.',
        action: 'open_provider_settings',
      });

      renderChat();

      await screen.findByRole('button', {
        name: 'Use OpenRouter route',
      });

      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'summarize the cluster',
        undefined,
        undefined,
      );
      expect(mockChat.setModel).not.toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(
        screen.queryByRole('status', { name: 'Assistant provider readiness route adopted' }),
      ).not.toBeInTheDocument();
      expect(mockNotificationStore.success).not.toHaveBeenCalledWith(
        expect.stringContaining('after DeepSeek provider check'),
        expect.any(Number),
      );
      expect(textarea.value).toBe('');
    });

    it('rechecks provider readiness from the drawer status banner', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider
        .mockResolvedValueOnce({
          success: false,
          message: 'Provider connection issue',
          provider: 'deepseek',
          model: 'deepseek:deepseek-v4-pro',
          cause: 'provider_connection',
          summary: 'Pulse could not maintain a healthy connection to this provider.',
          recommendation: 'Check provider reachability.',
          action: 'open_provider_settings',
        })
        .mockResolvedValueOnce({
          success: true,
          message: 'Connection successful',
          provider: 'deepseek',
          model: 'deepseek:deepseek-v4-pro',
        });

      renderChat();

      await waitFor(() => {
        expect(screen.getByText('Selected model route issue')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Retry route check' }));

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledTimes(2);
        expect(
          screen.queryByLabelText('Assistant selected model route status'),
        ).not.toBeInTheDocument();
      });
    });

    it('checks the explicitly selected chat model provider instead of the default provider', async () => {
      mockChat.model.mockReturnValue('openrouter:anthropic/claude-sonnet-4.5');
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledWith(
          'openrouter',
          'openrouter:anthropic/claude-sonnet-4.5',
        );
      });
      expect(mockAIAPI.testProvider).not.toHaveBeenCalledWith(
        'deepseek',
        'deepseek:deepseek-v4-pro',
      );
    });

    it('renders the compact composer send control', () => {
      renderChat();
      expect(screen.getByRole('button', { name: 'Send message' })).toBeInTheDocument();
    });

    it('renders attached context briefing without raw command text', () => {
      mockAiChatStore.context = {
        findingId: 'finding-1',
        autonomousMode: undefined,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Investigation record attached',
          subject: 'High CPU usage on web-server',
          statusLabel: 'Completed · Fix Queued · High confidence',
          detailLines: ['Backup job saturated CPU.'],
          evidence: ['CPU stayed above 95% for 10 minutes'],
          actionLabel: 'Restart the workload service',
          commandSummary: '1 command recorded for approval context',
          safetyNote: 'Command details stay in approval context.',
        },
      };

      renderChat();

      expect(screen.getByLabelText('Assistant context')).toBeInTheDocument();
      expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
      expect(screen.getByText('High CPU usage on web-server')).toBeInTheDocument();
      expect(screen.queryByText('Backup job saturated CPU.')).not.toBeInTheDocument();
      expect(screen.queryByText('1 command recorded for approval context')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Explain recent changes and correlations' }),
      ).not.toBeInTheDocument();
      expect(screen.queryByText('systemctl restart workload.service')).not.toBeInTheDocument();
    });

    it('renders safe briefing actions as links when a route is attached', () => {
      mockAiChatStore.context = {
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding attached',
          subject: 'Provider connection issue on Patrol runtime',
          actionLabel: 'Open Patrol provider settings',
          actionHref: '/settings/system-ai',
        },
      };

      renderChat();

      expect(screen.getByRole('link', { name: 'Open Patrol provider settings' })).toHaveAttribute(
        'href',
        '/settings/system-ai',
      );
    });

    it('renders Patrol configuration handoff details without replacing the attached headline', () => {
      mockAiChatStore.context = {
        targetType: 'patrol-configuration',
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol configuration failure attached',
          subject: 'patrol_autonomy_pro_required: Investigation and auto-fix require Pulse Pro.',
          statusLabel: 'HTTP 402 · model_unsupported_tools',
          detailLines: ['Provider: openrouter'],
          evidence: ['Command: sensitive or command detail withheld'],
          safetyNote:
            'Assistant can explain the configuration state; provider changes remain operator-controlled.',
        },
      };

      renderChat();

      const context = screen.getByLabelText('Assistant context');
      expect(context).toHaveTextContent('Patrol configuration failure attached');
      expect(context).toHaveTextContent('patrol_autonomy_pro_required');
      expect(context).toHaveTextContent('Provider: openrouter');
      expect(context).toHaveTextContent('Command: sensitive or command detail withheld');
      expect(context).toHaveTextContent('Approval required before any action.');
    });

    it('renders resource context handoff details without prompt text injection', () => {
      mockAiChatStore.context = {
        targetType: 'resource',
        autonomousMode: false,
        handoffResources: [
          {
            id: 'app-container:homeassistant',
            name: 'Home Assistant',
            type: 'app-container',
            node: 'ha-lxc',
          },
        ],
        handoffMetadata: {
          kind: 'resource_context',
        },
        briefing: {
          sourceLabel: 'Pulse resource context',
          title: 'Home Assistant',
          subject: 'app-container / online / docker',
          statusLabel: 'Read-only context attached',
          detailLines: [
            'Resource ID: app-container:homeassistant',
            'Parent: ha-lxc',
            'Discovery: app-container:homeassistant',
          ],
          safetyNote: 'Approval required before any action.',
        },
      };

      renderChat();

      const context = screen.getByLabelText('Assistant context');
      expect(context).toHaveTextContent('Pulse resource context');
      expect(context).toHaveTextContent('Home Assistant');
      expect(context).toHaveTextContent('app-container / online / docker');
      expect(context).toHaveTextContent('Resource ID: app-container:homeassistant');
      expect(context).toHaveTextContent('Parent: ha-lxc');
      expect(context).toHaveTextContent('Approval required before any action.');
      expect(context).not.toHaveTextContent('[Resource Context Pack]');
    });

    it('renders Patrol run handoff details without replacing the attached headline', () => {
      mockAiChatStore.context = {
        targetType: 'patrol-run',
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol run attached',
          subject: 'Scoped run run-alert-scoped',
          statusLabel: 'error · Alert fired · Checked 2 resources',
          detailLines: ['Runtime failure: Selected model does not support Patrol tools'],
          actionLabel: 'Review Patrol runtime failure',
          safetyNote:
            'Assistant can explain the Patrol run context; retries, configuration changes, and remediation remain operator-controlled.',
        },
      };

      renderChat();

      const context = screen.getByLabelText('Assistant context');
      expect(context).toHaveTextContent('Patrol run attached');
      expect(context).toHaveTextContent('Scoped run run-alert-scoped');
      expect(context).toHaveTextContent('Review Patrol runtime failure');
      expect(context).toHaveTextContent('Selected model does not support Patrol tools');
      expect(context).toHaveTextContent('Approval required before any action.');
    });

    it('keeps Patrol action-artifact briefings compact in the sidebar', () => {
      mockAiChatStore.context = {
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding attached',
          subject: 'Backup failed on delly (backup)',
          statusLabel: 'Pending · Medium risk',
          detailLines: ['Existing action artifact: Fix: Backup failed'],
          evidence: [
            'Review backup job logs for error details',
            'Check backup storage connectivity and space',
          ],
          commandSummary: '4 commands recorded for governed plan review',
          safetyNote:
            'Assistant should decide remediation from evidence; command execution requires governed approval.',
        },
      };

      renderChat();

      expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
      expect(screen.getByText('Pending · Medium risk')).toBeInTheDocument();
      expect(screen.getByText('Backup failed on delly (backup)')).toBeInTheDocument();
      expect(screen.getByText('Approval required before any action.')).toBeInTheDocument();
      expect(
        screen.queryByText('Existing action artifact: Fix: Backup failed'),
      ).not.toBeInTheDocument();
      expect(screen.queryByText('4 planned steps')).not.toBeInTheDocument();
      expect(
        screen.queryByText('Review backup job logs for error details'),
      ).not.toBeInTheDocument();
      expect(screen.queryByRole('link', { name: 'Fix: Backup failed' })).not.toBeInTheDocument();
      expect(
        screen.queryByText('4 commands recorded for governed plan review'),
      ).not.toBeInTheDocument();
    });

    it('does not expose legacy Patrol remediation-plan briefing internals', () => {
      mockAiChatStore.context = {
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Remediation plan attached',
          subject: 'Backup failed on delly (backup)',
          statusLabel: 'Pending · Medium risk',
          detailLines: ['Plan: Fix: Backup failed', '4 planned steps'],
          evidence: ['Review backup job logs for error details'],
          actionLabel: 'Fix: Backup failed',
          commandSummary: '4 commands recorded for approval context',
          safetyNote: 'Command details stay in governed remediation context.',
        },
      };

      renderChat();

      expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
      expect(screen.getByText('Pending · Medium risk')).toBeInTheDocument();
      expect(screen.getByText('Backup failed on delly (backup)')).toBeInTheDocument();
      expect(screen.getByText('Approval required before any action.')).toBeInTheDocument();
      expect(screen.queryByText('Plan: Fix: Backup failed')).not.toBeInTheDocument();
      expect(screen.queryByText('4 planned steps')).not.toBeInTheDocument();
      expect(
        screen.queryByText('Review backup job logs for error details'),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Review plan risk and prerequisites' }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText('4 commands recorded for approval context'),
      ).not.toBeInTheDocument();
    });

    it('uses the context strip instead of product-authored empty transcript prompts', () => {
      mockAiChatStore.context = {
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol assessment attached',
          subject: 'Coverage incomplete',
        },
      };

      renderChat();

      expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
      expect(screen.getByText('Coverage incomplete')).toBeInTheDocument();
      expect(screen.getByText('Approval required before any action.')).toBeInTheDocument();
      expect(screen.queryByText('Patrol assessment attached')).not.toBeInTheDocument();
      expect(screen.queryByText('Ask about your infrastructure')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Explain why coverage is incomplete' }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Summarize cluster health' }),
      ).not.toBeInTheDocument();
    });

    it('renders New button', () => {
      renderChat();
      expect(
        screen.getByRole('button', { name: 'Start new Assistant session' }),
      ).toBeInTheDocument();
    });

    it('uses docked layout on wide viewports', () => {
      renderChat();
      expect(screen.getByText('Pulse Assistant').closest('[data-layout-mode]')).toHaveAttribute(
        'data-layout-mode',
        'docked',
      );
      expect(screen.getByTitle('Collapse Pulse Assistant')).toBeInTheDocument();
    });

    it('switches to overlay layout below the dock threshold', () => {
      setViewportWidth(1024);
      renderChat();
      expect(screen.getByText('Pulse Assistant').closest('[data-layout-mode]')).toHaveAttribute(
        'data-layout-mode',
        'overlay',
      );
      expect(screen.queryByTitle('Collapse Pulse Assistant')).not.toBeInTheDocument();
      expect(screen.getByLabelText('Close Pulse Assistant')).toBeInTheDocument();
    });

    it('keeps phone route controls in the composer chrome instead of the header actions', () => {
      setViewportWidth(375);
      renderChat();

      const closeButton = screen.getByRole('button', { name: 'Close Pulse Assistant' });
      const headerActions = screen.getByTestId('assistant-header-actions');
      const composerChrome = screen.getByTestId('assistant-composer-chrome');
      const routeControls = screen.getByTestId('assistant-composer-route-controls');
      const modelSelector = screen.getByTestId('model-selector');
      const controlButton = screen.getByRole('button', {
        name: 'Assistant control mode: Read-only',
      });

      expect(closeButton).toHaveClass('order-2');
      expect(headerActions).toHaveClass('order-3');
      expect(headerActions).toHaveClass('w-full');
      expect(headerActions).not.toHaveClass('overflow-x-auto');
      expect(headerActions).not.toContainElement(closeButton);
      expect(headerActions).not.toContainElement(modelSelector);
      expect(headerActions).not.toContainElement(controlButton);
      expect(composerChrome).toHaveClass('flex-col');
      expect(composerChrome).toHaveClass('sm:flex-row');
      expect(routeControls).toHaveClass('flex-wrap');
      expect(routeControls).toContainElement(modelSelector);
      expect(routeControls).toContainElement(controlButton);
    });
  });

  // ── Close button ─────────────────────────────────────────────────────

  describe('close behavior', () => {
    it('calls onClose when close button is clicked', () => {
      const onClose = vi.fn();
      renderChat(onClose);
      const closeBtn = screen.getByRole('button', { name: 'Close Pulse Assistant' });
      fireEvent.click(closeBtn);
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when desktop floating close handle is clicked', () => {
      const onClose = vi.fn();
      renderChat(onClose);
      const floatingClose = screen.getByTitle('Collapse Pulse Assistant');
      fireEvent.click(floatingClose);
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  // ── Hidden when closed ───────────────────────────────────────────────

  describe('visibility', () => {
    it('hides content when isOpen is false', () => {
      mockAiChatStore.isOpenSignal.mockReturnValue(false);
      renderChat();
      expect(screen.queryByText('Pulse Assistant')).not.toBeInTheDocument();
    });

    it('does not initialize sessions or models while closed', async () => {
      mockAiChatStore.isOpenSignal.mockReturnValue(false);
      renderChat();

      await Promise.resolve();
      await Promise.resolve();

      expect(mockAIChatAPI.getStatus).not.toHaveBeenCalled();
      expect(mockAIChatAPI.listSessions).not.toHaveBeenCalled();
      expect(mockAIAPI.getSettings).not.toHaveBeenCalled();
      expect(mockAIAPI.getModels).not.toHaveBeenCalled();
    });
  });

  // ── Input & submit ───────────────────────────────────────────────────

  describe('input and submit', () => {
    it('sends the trimmed input as a message on Enter', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: '  hello world  ' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).toHaveBeenCalledWith('hello world', undefined, undefined);
    });

    it('shows slash command suggestions and runs the selected command with Enter', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/mo' } });

      expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: /Run \/models/ })).toBeInTheDocument();

      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '1');
      });
      expect(screen.queryByRole('listbox', { name: 'Assistant commands' })).not.toBeInTheDocument();
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('inserts the local fixture command from slash suggestions without sending yet', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/fi' } });

      expect(screen.getByRole('option', { name: /Insert \/fixture/ })).toBeInTheDocument();

      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => expect(textarea.value).toBe('/fixture '));
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('runs a local fixture slash command through the normal chat send path', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/fixture provider-retry' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockChat.sendMessage).toHaveBeenCalledWith(
          '/fixture provider-retry',
          undefined,
          undefined,
        );
      });
    });

    it('omits unavailable session commands from slash suggestions', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/compact' } });

      expect(screen.queryByRole('listbox', { name: 'Assistant commands' })).not.toBeInTheDocument();
      expect(screen.queryByRole('option', { name: /Run \/compact/ })).not.toBeInTheDocument();
    });

    it('keeps unknown slash command searches open without sending a provider prompt', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/not-a-command' } });

      expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();
      expect(screen.getByRole('status')).toHaveTextContent(
        'No Assistant commands match /not-a-command',
      );

      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      expect(textarea.value).toBe('/not-a-command');
      expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();
    });

    it('explains unavailable manual slash commands without sending a provider prompt', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/compact' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockAIChatAPI.summarizeSession).not.toHaveBeenCalled();
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      expect(mockNotificationStore.info).toHaveBeenCalledWith(
        'Requires a saved Assistant session.',
        2000,
      );
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('opens Assistant command help from the composer chrome without sending a provider prompt', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: 'count devices' } });
      fireEvent.click(screen.getByRole('button', { name: 'Open Assistant commands' }));

      expect(screen.getByRole('dialog', { name: 'Assistant commands' })).toBeInTheDocument();
      expect(textarea).toHaveValue('count devices');
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('opens Assistant command help from /help without sending a provider prompt', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/help' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(screen.getByRole('dialog', { name: 'Assistant commands' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: /\/models/ })).toHaveTextContent(
        'Open model search or set a route (/model openrouter/qwen or provider:model-id)',
      );
      expect(screen.getByRole('option', { name: /\/status/ })).toHaveTextContent(
        'Check the selected model route',
      );
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('runs commands from Assistant command help', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/commands' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(screen.getByRole('dialog', { name: 'Assistant commands' })).toBeInTheDocument();
      fireEvent.click(screen.getByRole('option', { name: /\/models/ }));

      await waitFor(() => {
        expect(
          screen.queryByRole('dialog', { name: 'Assistant commands' }),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '1');
      });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('opens Assistant provider settings from /connect without sending a provider prompt', async () => {
      const onClose = vi.fn();
      renderChat(onClose);
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/connect' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockNavigate).toHaveBeenCalledWith('/settings/system-ai');
      expect(onClose).toHaveBeenCalledTimes(1);
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('wires Assistant provider settings into the model selector', () => {
      const onClose = vi.fn();
      renderChat(onClose);

      const selectorProps = mockModelSelectorProps[mockModelSelectorProps.length - 1];
      expect(selectorProps.onManageProviders).toEqual(expect.any(Function));

      selectorProps.onManageProviders?.();

      expect(mockNavigate).toHaveBeenCalledWith('/settings/system-ai');
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('filters slash command suggestions by alias and runs clicked commands', async () => {
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
      });
      mockAIChatAPI.listSessions.mockClear();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/res' } });
      expect(screen.getByRole('option', { name: /Run \/sessions/ })).toBeInTheDocument();

      fireEvent.click(screen.getByRole('option', { name: /Run \/sessions/ }));

      await waitFor(() => {
        expect(
          screen.getByRole('dialog', { name: 'Pulse Assistant sessions' }),
        ).toBeInTheDocument();
      });
      expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('clears a transient slash command draft with Escape without submitting', async () => {
      const onClose = vi.fn();
      renderChat(onClose);
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/mo' } });
      expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();

      fireEvent.keyDown(textarea, { key: 'Escape' });

      await waitFor(() => {
        expect(
          screen.queryByRole('listbox', { name: 'Assistant commands' }),
        ).not.toBeInTheDocument();
      });
      expect(textarea.value).toBe('');
      expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '0');
      expect(onClose).not.toHaveBeenCalled();
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('clears a transient slash command draft when autocomplete closes from outside click', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/new' } });
      expect(screen.getByRole('listbox', { name: 'Assistant commands' })).toBeInTheDocument();

      fireEvent.click(document.body);

      await waitFor(() => {
        expect(
          screen.queryByRole('listbox', { name: 'Assistant commands' }),
        ).not.toBeInTheDocument();
      });
      expect(textarea.value).toBe('');
      expect(mockChat.newSession).not.toHaveBeenCalled();
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('runs /new locally without sending a provider prompt', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/new' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockChat.newSession).toHaveBeenCalled();
      });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('opens Assistant sessions from /sessions without sending a provider prompt', async () => {
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
      });
      mockAIChatAPI.listSessions.mockClear();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/sessions' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(
          screen.getByRole('dialog', { name: 'Pulse Assistant sessions' }),
        ).toBeInTheDocument();
      });
      expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('searches Assistant sessions from /sessions arguments without sending a provider prompt', async () => {
      mockAIChatAPI.listSessions
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce([
          {
            id: 's-backup',
            title: 'Backup Patrol Follow-up',
            created_at: '',
            updated_at: '',
            message_count: 3,
          },
        ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
      });
      mockAIChatAPI.listSessions.mockClear();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/sessions backup' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      const searchInput = await screen.findByPlaceholderText('Search sessions...');
      await waitFor(() => {
        expect(searchInput).toHaveValue('backup');
        expect(document.activeElement).toBe(searchInput);
      });
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ search: 'backup', limit: 30 });
        expect(screen.getByText('Backup Patrol Follow-up')).toBeInTheDocument();
      });
      expect(mockAIChatAPI.listSessions).not.toHaveBeenCalledWith({ limit: 30 });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('opens the model selector from /models without sending a provider prompt', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '0');

      fireEvent.input(textarea, { target: { value: '/models' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '1');
      });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('sets an explicit model route from /model without sending a provider prompt', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, {
        target: { value: '/model openrouter:deepseek/deepseek-v4-pro' },
      });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('sets an OpenCode-style provider/model route from /model without sending a provider prompt', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'openai:gpt-4o',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
        configured_providers: ['openai', 'openrouter'],
      });
      mockAIAPI.getModels.mockResolvedValue({
        models: [
          {
            id: 'openrouter:qwen/qwen3.7-plus',
            name: 'Qwen: Qwen3.7 Plus',
            provider: 'openrouter',
            notable: true,
          },
        ],
      });
      renderChat();
      await waitFor(() => {
        expect(mockAIAPI.getModels).toHaveBeenCalled();
      });
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, {
        target: { value: '/model openrouter/qwen/qwen3.7-plus' },
      });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:qwen/qwen3.7-plus');
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('returns to the configured default model from /model default', async () => {
      mockChat.model.mockReturnValue('openrouter:qwen/qwen3.7-plus');
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/model default' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.setModel).toHaveBeenCalledWith('');
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('cycles recent model routes from /model next without opening the picker', async () => {
      localStorage.setItem(
        'pulse:ai_chat_recent_models',
        JSON.stringify(['openrouter:anthropic/claude-sonnet-4.5', 'openrouter:qwen/qwen3.7-plus']),
      );
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/model next' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:anthropic/claude-sonnet-4.5');
      expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '0');
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('opens model search for partial /model route text instead of sending it', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: '/model qwen3.7-plus' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.setModel).not.toHaveBeenCalled();
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      await waitFor(() => {
        expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '1');
      });
      expect(screen.getByTestId('model-selector')).toHaveAttribute(
        'data-initial-search',
        'qwen3.7-plus',
      );
      expect(mockNotificationStore.error).not.toHaveBeenCalled();
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('checks selected provider status from /status without sending a provider prompt', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'openrouter:deepseek/deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockResolvedValue({
        success: true,
        message: 'Connection successful',
        provider: 'openrouter',
        model: 'openrouter:deepseek/deepseek-v4-pro',
      });

      renderChat();

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledWith(
          'openrouter',
          'openrouter:deepseek/deepseek-v4-pro',
        );
      });
      expect(
        screen.queryByLabelText('Assistant selected model route status'),
      ).not.toBeInTheDocument();
      mockAIAPI.testProvider.mockClear();

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: '/status' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledWith(
          'openrouter',
          'openrouter:deepseek/deepseek-v4-pro',
        );
      });

      await waitFor(() => {
        expect(screen.getByLabelText('Assistant selected model route status')).toHaveTextContent(
          'Selected model route ready',
        );
      });
      const status = screen.getByLabelText('Assistant selected model route status');
      expect(status).toHaveTextContent('Connection successful');
      expect(status).toHaveTextContent('Route:');
      expect(screen.getByRole('button', { name: 'Hide route status' })).toBeInTheDocument();
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('consumes pending command palette model requests without sending a provider prompt', async () => {
      mockAiChatStore.commandRequestSignal.mockReturnValue({ id: 42, action: 'models' });
      renderChat();

      await waitFor(() => {
        expect(screen.getByTestId('model-selector')).toHaveAttribute('data-open-request', '1');
      });
      expect(mockAiChatStore.ackCommandRequest).toHaveBeenCalledWith(42);
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('copies the transcript from /copy without sending a provider prompt', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        configurable: true,
      });
      mockChat.sessionId.mockReturnValue('session-slash-copy');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'count devices',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'There are three nodes.',
          timestamp: new Date('2026-06-06T12:35:01Z'),
        },
      ]);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/copy' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(writeText).toHaveBeenCalled();
      });
      expect(writeText.mock.calls[0][0]).toContain('There are three nodes.');
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('sends unknown slash text as a normal Assistant prompt', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.input(textarea, { target: { value: '/explain /dev devices' } });
      expect(screen.queryByRole('listbox', { name: 'Assistant commands' })).not.toBeInTheDocument();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        '/explain /dev devices',
        undefined,
        undefined,
      );
    });

    it('submits the live textarea value when composition text has not reached state', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      textarea.value = '  final composed text  ';
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'final composed text',
        undefined,
        undefined,
      );
    });

    it('ignores overlapping submit dispatches before the composer clear reaches the DOM', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      const form = textarea.closest('form');
      expect(form).toBeTruthy();

      fireEvent.input(textarea, { target: { value: 'duplicate prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      textarea.value = 'duplicate prompt';
      fireEvent.submit(form!);

      expect(mockChat.sendMessage).toHaveBeenCalledTimes(1);
      expect(mockChat.sendMessage).toHaveBeenCalledWith('duplicate prompt', undefined, undefined);
    });

    it('releases the submit guard for the next intentional follow-up', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'first prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      await Promise.resolve();

      fireEvent.input(textarea, { target: { value: 'follow-up prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledTimes(2);
      expect(mockChat.sendMessage).toHaveBeenLastCalledWith(
        'follow-up prompt',
        undefined,
        undefined,
      );
    });

    it('recalls submitted prompts with ArrowUp and ArrowDown from the empty composer', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'first prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      await Promise.resolve();
      fireEvent.input(textarea, { target: { value: 'second prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => expect(textarea.value).toBe(''));

      fireEvent.keyDown(textarea, { key: 'ArrowUp' });
      await waitFor(() => expect(textarea.value).toBe('second prompt'));

      fireEvent.keyDown(textarea, { key: 'ArrowUp' });
      await waitFor(() => expect(textarea.value).toBe('first prompt'));

      textarea.setSelectionRange(textarea.value.length, textarea.value.length);
      fireEvent.keyDown(textarea, { key: 'ArrowDown' });
      await waitFor(() => expect(textarea.value).toBe('second prompt'));

      fireEvent.keyDown(textarea, { key: 'ArrowDown' });
      await waitFor(() => expect(textarea.value).toBe(''));
    });

    it('recalls prompt history from the start of a draft and restores the draft', async () => {
      localStorage.setItem(
        'pulse:ai_chat_prompt_history',
        JSON.stringify([{ prompt: 'previous prompt', mentions: [] }]),
      );

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'draft prompt' } });
      textarea.setSelectionRange(0, 0);
      fireEvent.keyDown(textarea, { key: 'ArrowUp' });

      await waitFor(() => expect(textarea.value).toBe('previous prompt'));

      textarea.setSelectionRange(textarea.value.length, textarea.value.length);
      fireEvent.keyDown(textarea, { key: 'ArrowDown' });

      await waitFor(() => expect(textarea.value).toBe('draft prompt'));
    });

    it('keeps previous-history navigation at the start boundary only', async () => {
      localStorage.setItem(
        'pulse:ai_chat_prompt_history',
        JSON.stringify([
          { prompt: 'newer prompt', mentions: [] },
          { prompt: 'older prompt', mentions: [] },
        ]),
      );

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'draft prompt' } });
      textarea.setSelectionRange(0, 0);
      fireEvent.keyDown(textarea, { key: 'ArrowUp' });
      await waitFor(() => expect(textarea.value).toBe('newer prompt'));

      textarea.setSelectionRange(textarea.value.length, textarea.value.length);
      const arrowUpAtEnd = new KeyboardEvent('keydown', {
        key: 'ArrowUp',
        bubbles: true,
        cancelable: true,
      });
      textarea.dispatchEvent(arrowUpAtEnd);

      expect(textarea.value).toBe('newer prompt');
      expect(arrowUpAtEnd.defaultPrevented).toBe(false);

      textarea.setSelectionRange(0, 0);
      fireEvent.keyDown(textarea, { key: 'ArrowUp' });
      await waitFor(() => expect(textarea.value).toBe('older prompt'));
    });

    it('keeps next-history navigation at the end boundary only', async () => {
      localStorage.setItem(
        'pulse:ai_chat_prompt_history',
        JSON.stringify([
          { prompt: 'newer prompt', mentions: [] },
          { prompt: 'older prompt', mentions: [] },
        ]),
      );

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'draft prompt' } });
      textarea.setSelectionRange(0, 0);
      fireEvent.keyDown(textarea, { key: 'ArrowUp' });
      await waitFor(() => expect(textarea.value).toBe('newer prompt'));

      textarea.setSelectionRange(0, 0);
      const arrowDownAtStart = new KeyboardEvent('keydown', {
        key: 'ArrowDown',
        bubbles: true,
        cancelable: true,
      });
      textarea.dispatchEvent(arrowDownAtStart);

      expect(textarea.value).toBe('newer prompt');
      expect(arrowDownAtStart.defaultPrevented).toBe(false);

      textarea.setSelectionRange(textarea.value.length, textarea.value.length);
      fireEvent.keyDown(textarea, { key: 'ArrowDown' });
      await waitFor(() => expect(textarea.value).toBe('draft prompt'));
    });

    it('loads persisted prompt history for recall', async () => {
      localStorage.setItem(
        'pulse:ai_chat_prompt_history',
        JSON.stringify([{ prompt: 'persisted prompt', mentions: [] }]),
      );

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.keyDown(textarea, { key: 'ArrowUp' });

      await waitFor(() => expect(textarea.value).toBe('persisted prompt'));
    });

    it('does not send an empty message', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: '   ' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('queues a follow-up message when chat is loading', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'hello' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).toHaveBeenCalledWith('hello', undefined, undefined);
      expect(textarea).toHaveValue('');
    });

    it('clears input after successful submit', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'hello' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(textarea.value).toBe('');
    });

    it('restores the submitted draft when send returns false', async () => {
      mockChat.sendMessage.mockResolvedValue(false);
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'do not lose this draft' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(textarea.value).toBe('');
      await waitFor(() => expect(textarea.value).toBe('do not lose this draft'));
      expect(document.activeElement).toBe(textarea);
    });

    it('does not overwrite a newer draft when a failed send resolves late', async () => {
      let resolveSend!: (ok: boolean) => void;
      mockChat.sendMessage.mockReturnValue(
        new Promise<boolean>((resolve) => {
          resolveSend = resolve;
        }),
      );
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, { target: { value: 'first draft' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      fireEvent.input(textarea, { target: { value: 'new draft while waiting' } });

      resolveSend(false);

      await waitFor(() => expect(textarea.value).toBe('new draft while waiting'));
    });

    it('restores an unsent composer draft after the assistant drawer remounts', async () => {
      const firstRender = renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.input(textarea, {
        target: { value: 'keep this draft while I check another page' },
      });
      textarea.setSelectionRange(10, 10);

      firstRender.unmount();
      renderChat();

      const restoredTextarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      await waitFor(() =>
        expect(restoredTextarea.value).toBe('keep this draft while I check another page'),
      );
      await waitFor(() => expect(restoredTextarea.selectionStart).toBe(10));
      expect(document.activeElement).toBe(restoredTextarea);
    });

    it('restores queued follow-up edit metadata after remounting the composer', async () => {
      const queued: QueuedFollowUp = {
        id: 'queued-1',
        messageId: 'msg-queued-1',
        prompt: 'queued scoped prompt',
        mentions: [{ id: 'vm-1', name: 'web-1', type: 'vm', node: 'pve-1' }],
        findingId: 'finding-1',
        sendOptions: { autonomousMode: false, handoffContext: 'scoped context' },
        timestamp: new Date('2026-06-06T08:00:00Z'),
      };
      mockChat.queuedFollowUps.mockReturnValue([queued]);
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.takeQueuedFollowUp.mockReturnValue(queued);
      const firstRender = renderChat();

      mockChatMessagesProps.at(-1)?.onEditQueuedFollowUp?.('queued-1');
      let textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      await waitFor(() => expect(textarea.value).toBe('queued scoped prompt'));

      firstRender.unmount();
      mockChat.queuedFollowUps.mockReturnValue([]);
      mockChat.queuedFollowUpCount.mockReturnValue(0);
      renderChat();

      textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      await waitFor(() => expect(textarea.value).toBe('queued scoped prompt'));
      fireEvent.input(textarea, { target: { value: 'edited scoped prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'edited scoped prompt',
        [{ id: 'vm-1', name: 'web-1', type: 'vm', node: 'pve-1' }],
        'finding-1',
        { autonomousMode: false, handoffContext: 'scoped context' },
      );
    });

    it('allows newlines with Shift+Enter', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'hello' } });
      fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: true });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('submit button is disabled when input is empty', () => {
      renderChat();
      const submitBtn = screen.getByTitle('Send');
      expect(submitBtn).toBeDisabled();
    });

    it('submit button is enabled when input has text', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'hello' } });
      const submitBtn = screen.getByTitle('Send');
      expect(submitBtn).not.toBeDisabled();
    });

    it('shows stop button when loading', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      const activityDock = screen.getByTestId('assistant-activity-dock');
      expect(activityDock).toContainElement(screen.getByTitle('Stop'));
      expect(screen.getByRole('button', { name: 'Queue follow-up' })).toBeInTheDocument();
    });

    it('calls chat.stop when stop button is clicked', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      fireEvent.click(screen.getByTitle('Stop'));
      expect(mockChat.stop).toHaveBeenCalledTimes(1);
    });

    it('arms keyboard interruption on first Escape and stops on second Escape', async () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.keyDown(textarea, { key: 'Escape' });

      expect(mockChat.stop).not.toHaveBeenCalled();
      expect(screen.getByTestId('assistant-activity-dock')).toContainElement(
        screen.getByTitle('Stop response armed'),
      );

      fireEvent.keyDown(textarea, { key: 'Escape' });

      expect(mockChat.stop).toHaveBeenCalledTimes(1);
      await waitFor(() => expect(document.activeElement).toBe(textarea));
    });

    it('returns focus to the composer after stopping a response', async () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.click(screen.getByTitle('Stop'));

      await waitFor(() => expect(document.activeElement).toBe(textarea));
    });

    it('submits via form submit button click', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'test query' } });
      const submitBtn = screen.getByTitle('Send');
      fireEvent.click(submitBtn);
      expect(mockChat.sendMessage).toHaveBeenCalledWith('test query', undefined, undefined);
    });

    it('returns focus to the composer after submit button click', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'test query' } });

      fireEvent.click(screen.getByTitle('Send'));

      await waitFor(() => expect(document.activeElement).toBe(textarea));
    });

    it('shows queued follow-up count and clears queued follow-ups', () => {
      mockChat.queuedFollowUpCount.mockReturnValue(2);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-1',
          messageId: 'msg-queued-1',
          prompt: 'first queued prompt',
          timestamp: new Date(),
        },
        {
          id: 'queued-2',
          messageId: 'msg-queued-2',
          prompt: 'second queued prompt',
          timestamp: new Date(),
        },
      ]);
      renderChat();

      expect(screen.getByRole('status', { name: 'Queued follow-up messages' })).toHaveTextContent(
        '2 follow-ups queued',
      );
      expect(screen.getByText('first queued prompt')).toBeInTheDocument();
      expect(screen.getByText('second queued prompt')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Clear queued follow-up messages' }));

      expect(mockChat.clearQueuedFollowUps).toHaveBeenCalledTimes(1);
    });

    it('shows paused queued follow-ups and lets the first one resume', () => {
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.queuedFollowUpsPaused.mockReturnValue(true);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-paused-1',
          messageId: 'msg-queued-paused-1',
          prompt: 'paused queued prompt',
          timestamp: new Date(),
        },
      ]);
      renderChat();

      expect(screen.getByRole('status', { name: 'Queued follow-up messages' })).toHaveTextContent(
        '1 follow-up paused',
      );
      expect(mockChatMessagesProps.at(-1)?.queuedFollowUpsPaused).toBe(true);

      fireEvent.click(
        screen.getByRole('button', {
          name: 'Resume queued follow-up: paused queued prompt',
        }),
      );

      expect(mockChat.sendQueuedFollowUpNow).toHaveBeenCalledWith('queued-paused-1');
    });

    it('lets a later queued follow-up be promoted to send next', async () => {
      mockChat.queuedFollowUpCount.mockReturnValue(2);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-1',
          messageId: 'msg-queued-1',
          prompt: 'first queued prompt',
          timestamp: new Date(),
        },
        {
          id: 'queued-2',
          messageId: 'msg-queued-2',
          prompt: 'second queued prompt',
          timestamp: new Date(),
        },
      ]);
      renderChat();

      expect(
        screen.queryByRole('button', {
          name: 'Send queued follow-up next: first queued prompt',
        }),
      ).not.toBeInTheDocument();

      fireEvent.click(
        screen.getByRole('button', {
          name: 'Send queued follow-up next: second queued prompt',
        }),
      );

      expect(mockChat.sendQueuedFollowUpNow).toHaveBeenCalledWith('queued-2');
      await waitFor(() => expect(document.activeElement).toHaveAttribute('placeholder'));
    });

    it('passes queued follow-up metadata and row actions into the transcript', () => {
      const queuedFollowUps: QueuedFollowUp[] = [
        {
          id: 'queued-row-1',
          messageId: 'msg-queued-row-1',
          prompt: 'row queued prompt',
          timestamp: new Date(),
        },
      ];
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.queuedFollowUps.mockReturnValue(queuedFollowUps);
      mockChat.takeQueuedFollowUp.mockReturnValue(queuedFollowUps[0]);

      renderChat();

      const chatMessagesProps = mockChatMessagesProps.at(-1);
      expect(chatMessagesProps?.queuedFollowUps).toBe(queuedFollowUps);
      expect(chatMessagesProps?.queuedFollowUpsPaused).toBe(false);

      chatMessagesProps?.onEditQueuedFollowUp?.('queued-row-1');
      expect(mockChat.takeQueuedFollowUp).toHaveBeenCalledWith('queued-row-1');

      chatMessagesProps?.onCancelQueuedFollowUp?.('queued-row-1');
      expect(mockChat.cancelQueuedFollowUp).toHaveBeenCalledWith('queued-row-1');
    });

    it('removes an individual queued follow-up', () => {
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-1',
          messageId: 'msg-queued-1',
          prompt: 'remove this queued prompt',
          timestamp: new Date(),
        },
      ]);
      renderChat();

      fireEvent.click(
        screen.getByRole('button', {
          name: 'Remove queued follow-up: remove this queued prompt',
        }),
      );

      expect(mockChat.cancelQueuedFollowUp).toHaveBeenCalledWith('queued-1');
    });

    it('loads a focused queued follow-up row into the composer with Enter', () => {
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-1',
          messageId: 'msg-queued-1',
          prompt: 'keyboard edit queued prompt',
          timestamp: new Date(),
        },
      ]);
      mockChat.takeQueuedFollowUp.mockReturnValue({
        id: 'queued-1',
        messageId: 'msg-queued-1',
        prompt: 'keyboard edit queued prompt',
        timestamp: new Date(),
      });

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.keyDown(
        screen.getByRole('listitem', {
          name: 'Queued follow-up: keyboard edit queued prompt. Press Enter to edit or Delete to remove.',
        }),
        { key: 'Enter' },
      );

      expect(mockChat.takeQueuedFollowUp).toHaveBeenCalledWith('queued-1');
      expect(textarea.value).toBe('keyboard edit queued prompt');
    });

    it('removes a focused queued follow-up row with Delete', async () => {
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-1',
          messageId: 'msg-queued-1',
          prompt: 'keyboard remove queued prompt',
          timestamp: new Date(),
        },
      ]);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      fireEvent.keyDown(
        screen.getByRole('listitem', {
          name: 'Queued follow-up: keyboard remove queued prompt. Press Enter to edit or Delete to remove.',
        }),
        { key: 'Delete' },
      );

      expect(mockChat.cancelQueuedFollowUp).toHaveBeenCalledWith('queued-1');
      await waitFor(() => expect(document.activeElement).toBe(textarea));
    });

    it('loads an individual queued follow-up into the composer for editing', () => {
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.queuedFollowUps.mockReturnValue([
        {
          id: 'queued-1',
          messageId: 'msg-queued-1',
          prompt: 'edit this queued prompt',
          timestamp: new Date(),
        },
      ]);
      mockChat.takeQueuedFollowUp.mockReturnValue({
        id: 'queued-1',
        messageId: 'msg-queued-1',
        prompt: 'edit this queued prompt',
        mentions: [{ id: 'vm-1', name: 'web-1', type: 'vm', node: 'pve-1' }],
        findingId: 'finding-1',
        sendOptions: {
          autonomousMode: false,
          handoffContext: 'scoped context',
        },
        timestamp: new Date(),
      });

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      fireEvent.click(
        screen.getByRole('button', {
          name: 'Edit queued follow-up: edit this queued prompt',
        }),
      );

      expect(mockChat.takeQueuedFollowUp).toHaveBeenCalledWith('queued-1');
      expect(textarea.value).toBe('edit this queued prompt');

      fireEvent.input(textarea, { target: { value: 'edited queued prompt' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'edited queued prompt',
        [{ id: 'vm-1', name: 'web-1', type: 'vm', node: 'pve-1' }],
        'finding-1',
        {
          autonomousMode: false,
          handoffContext: 'scoped context',
        },
      );
    });
  });

  // ── Control level ────────────────────────────────────────────────────

  describe('control level', () => {
    it('displays Read-only as default control label', () => {
      renderChat();
      expect(screen.getByText('Read-only')).toBeInTheDocument();
    });

    it('opens control menu on click and focuses the current mode', async () => {
      renderChat();
      const controlButton = screen.getByRole('button', {
        name: 'Assistant control mode: Read-only',
      });
      expect(controlButton).toHaveAttribute('aria-expanded', 'false');

      fireEvent.click(controlButton);

      expect(controlButton).toHaveAttribute('aria-expanded', 'true');
      expect(
        screen.getByRole('menu', { name: 'Assistant control mode options' }),
      ).toBeInTheDocument();
      expect(screen.getByRole('menuitemradio', { name: /Read-only/ })).toHaveAttribute(
        'aria-checked',
        'true',
      );
      expect(screen.getByRole('menuitemradio', { name: /Approval/ })).toHaveAttribute(
        'aria-checked',
        'false',
      );
      expect(screen.getByText('Default control mode')).toBeInTheDocument();
      expect(screen.getByText('No commands or control actions')).toBeInTheDocument();
      expect(screen.getByText('Ask before running commands')).toBeInTheDocument();
      expect(screen.getByText('Executes without approval')).toBeInTheDocument();
      await waitFor(() => {
        expect(document.activeElement).toBe(
          screen.getByRole('menuitemradio', { name: /Read-only/ }),
        );
      });
    });

    it('opens the control menu from the keyboard', async () => {
      renderChat();
      const controlButton = screen.getByRole('button', {
        name: 'Assistant control mode: Read-only',
      });

      fireEvent.keyDown(controlButton, { key: 'ArrowDown' });

      expect(controlButton).toHaveAttribute('aria-expanded', 'true');
      expect(
        screen.getByRole('menu', { name: 'Assistant control mode options' }),
      ).toBeInTheDocument();
      await waitFor(() => {
        expect(document.activeElement).toBe(
          screen.getByRole('menuitemradio', { name: /Read-only/ }),
        );
      });
    });

    it('moves focus through control modes and closes on Escape without leaking key events', async () => {
      const onParentKeyDown = vi.fn();
      const onClose = vi.fn();
      render(() => (
        <div onKeyDown={onParentKeyDown}>
          <AIChat onClose={onClose} />
        </div>
      ));
      const controlButton = screen.getByRole('button', {
        name: 'Assistant control mode: Read-only',
      });

      fireEvent.click(controlButton);
      const readOnlyOption = screen.getByRole('menuitemradio', { name: /Read-only/ });
      const approvalOption = screen.getByRole('menuitemradio', { name: /Approval/ });
      const autonomousOption = screen.getByRole('menuitemradio', { name: /Autonomous/ });
      await waitFor(() => {
        expect(document.activeElement).toBe(readOnlyOption);
      });

      fireEvent.keyDown(readOnlyOption, { key: 'ArrowDown' });
      expect(document.activeElement).toBe(approvalOption);

      fireEvent.keyDown(approvalOption, { key: 'End' });
      expect(document.activeElement).toBe(autonomousOption);

      fireEvent.keyDown(autonomousOption, { key: 'Home' });
      expect(document.activeElement).toBe(readOnlyOption);

      fireEvent.keyDown(readOnlyOption, { key: 'ArrowUp' });
      expect(document.activeElement).toBe(autonomousOption);

      fireEvent.keyDown(autonomousOption, { key: 'Escape' });

      await waitFor(() => {
        expect(
          screen.queryByRole('menu', { name: 'Assistant control mode options' }),
        ).not.toBeInTheDocument();
        expect(document.activeElement).toBe(controlButton);
      });
      expect(onParentKeyDown).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
    });
  });

  // ── Session management ───────────────────────────────────────────────

  describe('session management', () => {
    it('starts a blank conversation on New button click', async () => {
      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Start new Assistant session' }));
      await waitFor(() => {
        expect(mockChat.newSession).toHaveBeenCalledTimes(1);
      });
    });

    it('clears scoped handoff context when starting a new session', async () => {
      mockAiChatStore.context = {
        findingId: 'finding-old',
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Old finding handoff',
        },
      };

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Start new Assistant session' }));

      await waitFor(() => {
        expect(mockAiChatStore.clearContext).toHaveBeenCalledTimes(1);
      });
    });

    it('returns focus to the composer after starting a blank conversation', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      textarea.blur();

      fireEvent.click(screen.getByRole('button', { name: 'Start new Assistant session' }));

      await waitFor(() => {
        expect(mockChat.newSession).toHaveBeenCalledTimes(1);
        expect(document.activeElement).toBe(textarea);
      });
    });

    it('keeps scoped handoff context when starting a new session fails', async () => {
      mockChat.newSession.mockResolvedValueOnce(null);
      mockAiChatStore.context = {
        findingId: 'finding-old',
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Old finding handoff',
        },
      };

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Start new Assistant session' }));

      await waitFor(() => {
        expect(mockChat.newSession).toHaveBeenCalledTimes(1);
      });
      expect(mockAiChatStore.clearContext).not.toHaveBeenCalled();
      expect(mockAiChatStore.context.findingId).toBe('finding-old');
    });

    it('forks the current saved Assistant session from the header', async () => {
      mockChat.sessionId.mockReturnValue('source-session');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'compare these tool outputs',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'The first node has stale discovery data.',
          timestamp: new Date('2026-06-06T12:35:01Z'),
        },
      ]);
      mockChat.model.mockReturnValue('openrouter:deepseek/deepseek-chat');
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 'source-session',
          title: 'Source session',
          created_at: '2026-06-06T12:30:00Z',
          updated_at: '2026-06-06T12:35:00Z',
          message_count: 2,
        },
      ]);
      mockAIChatAPI.forkSession.mockResolvedValue({
        id: 'forked-session',
        title: 'Forked session',
        created_at: '2026-06-06T12:45:00Z',
        updated_at: '2026-06-06T12:45:00Z',
        message_count: 2,
      });

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Fork current Assistant session' }));

      await waitFor(() => {
        expect(mockAIChatAPI.forkSession).toHaveBeenCalledWith('source-session');
        expect(mockChat.loadSession).toHaveBeenCalledWith('forked-session');
      });
      expect(mockAiChatStore.clearContext).toHaveBeenCalled();
      expect(mockNotificationStore.success).toHaveBeenCalledWith('Assistant session forked', 2000);
      expect(JSON.parse(localStorage.getItem('pulse:ai_chat_models_by_session') || '{}')).toEqual({
        'source-session': 'openrouter:deepseek/deepseek-chat',
        'forked-session': 'openrouter:deepseek/deepseek-chat',
      });
    });

    it('keeps the current session loaded when forking fails', async () => {
      mockChat.sessionId.mockReturnValue('source-session');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'branch from this point',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
      ]);
      mockAIChatAPI.forkSession.mockRejectedValueOnce(new Error('fork failed'));

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Fork current Assistant session' }));

      await waitFor(() => {
        expect(mockAIChatAPI.forkSession).toHaveBeenCalledWith('source-session');
      });
      expect(mockChat.loadSession).not.toHaveBeenCalled();
      expect(mockNotificationStore.error).toHaveBeenCalledWith('Failed to fork Assistant session');
    });

    it('compacts the current saved Assistant session from the header', async () => {
      mockChat.sessionId.mockReturnValue('source-session');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'inspect the noisy session',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'The session has enough context to compact.',
          timestamp: new Date('2026-06-06T12:35:01Z'),
        },
      ]);
      mockAIChatAPI.summarizeSession.mockResolvedValue({
        success: true,
        status: 'compacted',
        message: 'Compacted 8 older messages into a session summary.',
        session_id: 'source-session',
        compacted_messages: 8,
        kept_recent_messages: 4,
      });

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Compact session' }));

      await waitFor(() => {
        expect(mockAIChatAPI.summarizeSession).toHaveBeenCalledWith('source-session');
        expect(mockChat.loadSession).toHaveBeenCalledWith('source-session');
      });
      expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
      expect(mockNotificationStore.info).toHaveBeenCalledWith(
        'Compacting Assistant session...',
        2000,
      );
      await waitFor(() => {
        expect(mockNotificationStore.success).toHaveBeenCalledWith(
          'Compacted 8 older messages into a session summary.',
          2500,
        );
      });
    });

    it('undoes the latest Assistant turn and restores the prompt for editing', async () => {
      mockChat.sessionId.mockReturnValue('session-undo');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'inspect vm-101',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: 'vm-101 has stale guest tools.',
          timestamp: new Date('2026-06-06T12:35:01Z'),
        },
      ]);
      mockChat.undoLastTurn.mockResolvedValue({
        prompt: 'inspect vm-101',
        request: {
          mentions: [{ id: 'vm:pve:101', name: 'vm-101', type: 'vm', node: 'pve' }],
          findingId: 'finding-101',
          model: 'openrouter:deepseek/deepseek-chat',
          autonomousMode: false,
          handoffContext: '[Patrol Finding Context]\nVM 101 has stale guest tools',
        },
      });

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Undo last Assistant turn' }));

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      await waitFor(() => {
        expect(mockChat.undoLastTurn).toHaveBeenCalledTimes(1);
        expect(textarea).toHaveValue('inspect vm-101');
      });
      expect(mockNotificationStore.success).toHaveBeenCalledWith(
        'Last prompt restored for editing',
        2000,
      );

      fireEvent.click(screen.getByRole('button', { name: 'Send message' }));

      await waitFor(() => {
        expect(mockChat.sendMessage).toHaveBeenCalledWith(
          'inspect vm-101',
          [{ id: 'vm:pve:101', name: 'vm-101', type: 'vm', node: 'pve' }],
          'finding-101',
          expect.objectContaining({
            autonomousMode: false,
            handoffContext: '[Patrol Finding Context]\nVM 101 has stale guest tools',
            model: 'openrouter:deepseek/deepseek-chat',
          }),
        );
      });
    });

    it('redoes the latest undone Assistant turn from the header', async () => {
      mockChat.sessionId.mockReturnValue('session-redo');
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: 'inspect vm-101',
          timestamp: new Date('2026-06-06T12:34:56Z'),
        },
      ]);
      mockChat.undoLastTurn.mockResolvedValue({ prompt: 'inspect vm-101' });
      mockChat.redoLastTurn.mockResolvedValue({ success: true, canRedo: false });

      renderChat();
      fireEvent.click(screen.getByRole('button', { name: 'Undo last Assistant turn' }));

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Redo last Assistant turn' })).not.toBeDisabled();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Redo last Assistant turn' }));

      await waitFor(() => {
        expect(mockChat.redoLastTurn).toHaveBeenCalledTimes(1);
      });
      expect(screen.getByPlaceholderText('Ask about your infrastructure...')).toHaveValue('');
      expect(mockNotificationStore.success).toHaveBeenCalledWith('Assistant turn restored', 2000);
    });

    it('opens session picker on click', async () => {
      renderChat();
      const sessionButton = screen.getByRole('button', { name: 'Pulse Assistant sessions' });
      fireEvent.click(sessionButton);
      await waitFor(() => {
        expect(sessionButton).toHaveAttribute('aria-expanded', 'true');
        expect(
          screen.getByRole('dialog', { name: 'Pulse Assistant sessions' }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole('listbox', { name: 'Assistant session history' }),
        ).toBeInTheDocument();
        expect(screen.getByText('New session')).toBeInTheDocument();
        expect(document.activeElement).toBe(screen.getByPlaceholderText('Search sessions...'));
      });
    });

    it('shows "No previous assistant sessions" when empty', async () => {
      renderChat();
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(screen.getByText('No previous assistant sessions')).toBeInTheDocument();
      });
    });

    it('refreshes sessions before opening the picker', async () => {
      mockAIChatAPI.listSessions.mockResolvedValueOnce([]).mockResolvedValueOnce([
        {
          id: 's-fresh',
          title: 'Fresh session',
          created_at: '',
          updated_at: '',
          message_count: 1,
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledTimes(1);
        expect(mockAIChatAPI.listSessions).toHaveBeenNthCalledWith(1, { limit: 30 });
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledTimes(2);
        expect(mockAIChatAPI.listSessions).toHaveBeenNthCalledWith(2, { limit: 30 });
        expect(screen.getByText('Fresh session')).toBeInTheDocument();
      });
    });

    it('lists sessions in the dropdown', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
        { id: 's2', title: 'Session Two', created_at: '', updated_at: '', message_count: 3 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(
          screen.getByRole('option', { name: 'Resume Session One, 5 messages' }),
        ).toHaveAttribute('aria-selected', 'false');
        expect(
          screen.getByRole('option', { name: 'Resume Session Two, 3 messages' }),
        ).toHaveAttribute('aria-selected', 'false');
        expect(screen.getByText('5 messages')).toBeInTheDocument();
        expect(screen.getByText('3 messages')).toBeInTheDocument();
      });
    });

    it('restores redo availability from the active saved session summary', async () => {
      mockChat.sessionId.mockReturnValue('session-with-redo');
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 'session-with-redo',
          title: 'Needs redo',
          created_at: '2026-06-06T12:30:00Z',
          updated_at: '2026-06-06T12:35:00Z',
          message_count: 1,
          can_redo: true,
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Redo last Assistant turn' })).not.toBeDisabled();
      });
    });

    it('moves keyboard focus through session picker results', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
        { id: 's2', title: 'Session Two', created_at: '', updated_at: '', message_count: 3 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      const searchInput = await screen.findByPlaceholderText('Search sessions...');
      const firstOption = await screen.findByRole('option', {
        name: 'Resume Session One, 5 messages',
      });
      const secondOption = screen.getByRole('option', {
        name: 'Resume Session Two, 3 messages',
      });

      await waitFor(() => {
        expect(document.activeElement).toBe(searchInput);
      });

      fireEvent.keyDown(searchInput, { key: 'ArrowDown' });
      expect(document.activeElement).toBe(firstOption);

      fireEvent.keyDown(firstOption, { key: 'ArrowDown' });
      expect(document.activeElement).toBe(secondOption);

      fireEvent.keyDown(secondOption, { key: 'ArrowUp' });
      expect(document.activeElement).toBe(firstOption);

      fireEvent.keyDown(firstOption, { key: 'End' });
      expect(document.activeElement).toBe(secondOption);

      fireEvent.keyDown(secondOption, { key: 'Home' });
      expect(document.activeElement).toBe(firstOption);

      fireEvent.keyDown(firstOption, { key: 'ArrowUp' });
      expect(document.activeElement).toBe(secondOption);

      fireEvent.keyDown(secondOption, { key: 'ArrowDown' });
      expect(document.activeElement).toBe(firstOption);

      searchInput.focus();
      fireEvent.keyDown(searchInput, { key: 'ArrowUp' });
      expect(document.activeElement).toBe(secondOption);

      searchInput.focus();
      fireEvent.keyDown(searchInput, { key: 'Escape' });
      await waitFor(() => {
        expect(
          screen.queryByRole('dialog', { name: 'Pulse Assistant sessions' }),
        ).not.toBeInTheDocument();
      });
    });

    it('supports page, home, end, and Escape from session picker search', async () => {
      const onClose = vi.fn();
      const onParentKeyDown = vi.fn();
      const pageSessions = Array.from({ length: 12 }, (_, index) => ({
        id: `session-${index}`,
        title: `Page Session ${index}`,
        created_at: '',
        updated_at: '',
        message_count: index + 1,
      }));
      mockAIChatAPI.listSessions.mockResolvedValue(pageSessions);

      render(() => (
        <div onKeyDown={onParentKeyDown}>
          <AIChat onClose={onClose} />
        </div>
      ));
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      const sessionButton = screen.getByTitle('Pulse Assistant sessions');
      fireEvent.click(sessionButton);
      const searchInput = await screen.findByPlaceholderText('Search sessions...');
      const optionFor = (index: number) =>
        screen.getByRole('option', {
          name: `Resume Page Session ${index}, ${index + 1} ${index === 0 ? 'message' : 'messages'}`,
        });

      await waitFor(() => {
        expect(document.activeElement).toBe(searchInput);
      });

      fireEvent.keyDown(searchInput, { key: 'PageDown' });
      expect(document.activeElement).toBe(optionFor(10));

      searchInput.focus();
      fireEvent.keyDown(searchInput, { key: 'PageUp' });
      expect(document.activeElement).toBe(optionFor(11));

      searchInput.focus();
      fireEvent.keyDown(searchInput, { key: 'End' });
      expect(document.activeElement).toBe(optionFor(11));

      searchInput.focus();
      fireEvent.keyDown(searchInput, { key: 'Home' });
      expect(document.activeElement).toBe(optionFor(0));

      searchInput.focus();
      fireEvent.keyDown(searchInput, { key: 'Escape' });
      await waitFor(() => {
        expect(
          screen.queryByRole('dialog', { name: 'Pulse Assistant sessions' }),
        ).not.toBeInTheDocument();
        expect(document.activeElement).toBe(sessionButton);
      });
      expect(onParentKeyDown).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
    });

    it('consumes session picker search keys before later document handlers see them', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
        { id: 's2', title: 'Session Two', created_at: '', updated_at: '', message_count: 3 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      const searchInput = await screen.findByPlaceholderText('Search sessions...');
      await waitFor(() => {
        expect(document.activeElement).toBe(searchInput);
      });

      const laterDocumentHandler = vi.fn();
      document.addEventListener('keydown', laterDocumentHandler);
      const escapeEvent = new KeyboardEvent('keydown', {
        bubbles: true,
        cancelable: true,
        key: 'Escape',
      });

      searchInput.dispatchEvent(escapeEvent);

      document.removeEventListener('keydown', laterDocumentHandler);
      expect(escapeEvent.defaultPrevented).toBe(true);
      expect(laterDocumentHandler).not.toHaveBeenCalled();
      await waitFor(() => {
        expect(
          screen.queryByRole('dialog', { name: 'Pulse Assistant sessions' }),
        ).not.toBeInTheDocument();
      });
    });

    it('consumes session picker option keys before later document handlers see them', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
        { id: 's2', title: 'Session Two', created_at: '', updated_at: '', message_count: 3 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      const searchInput = await screen.findByPlaceholderText('Search sessions...');
      const firstOption = await screen.findByRole('option', {
        name: 'Resume Session One, 5 messages',
      });
      const secondOption = screen.getByRole('option', {
        name: 'Resume Session Two, 3 messages',
      });
      await waitFor(() => {
        expect(document.activeElement).toBe(searchInput);
      });
      fireEvent.keyDown(searchInput, { key: 'ArrowDown' });
      expect(document.activeElement).toBe(firstOption);

      const laterDocumentHandler = vi.fn();
      document.addEventListener('keydown', laterDocumentHandler);
      const arrowDownEvent = new KeyboardEvent('keydown', {
        bubbles: true,
        cancelable: true,
        key: 'ArrowDown',
      });

      firstOption.dispatchEvent(arrowDownEvent);

      document.removeEventListener('keydown', laterDocumentHandler);
      expect(arrowDownEvent.defaultPrevented).toBe(true);
      expect(laterDocumentHandler).not.toHaveBeenCalled();
      expect(document.activeElement).toBe(secondOption);
    });

    it('pins sessions above the recency groups and persists the choice', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's1',
          title: 'Recent session',
          created_at: '',
          updated_at: '2026-06-06T10:00:00Z',
          message_count: 5,
        },
        {
          id: 's2',
          title: 'Pinned session',
          created_at: '',
          updated_at: '2026-06-05T10:00:00Z',
          message_count: 3,
        },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await screen.findByText('Recent session');

      fireEvent.click(
        screen.getByRole('button', { name: 'Pin Assistant session: Pinned session' }),
      );

      await waitFor(() => {
        expect(localStorage.getItem('pulse:ai_chat_pinned_sessions')).toBe(JSON.stringify(['s2']));
      });

      const pinnedHeader = screen.getByText('Pinned');
      const pinnedSession = screen.getByText('Pinned session');
      const recentSession = screen.getByText('Recent session');
      expect(pinnedHeader.compareDocumentPosition(pinnedSession)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING,
      );
      expect(pinnedSession.compareDocumentPosition(recentSession)).toBe(
        Node.DOCUMENT_POSITION_FOLLOWING,
      );

      fireEvent.click(
        screen.getByRole('button', { name: 'Unpin Assistant session: Pinned session' }),
      );
      await waitFor(() => {
        expect(localStorage.getItem('pulse:ai_chat_pinned_sessions')).toBeNull();
      });
    });

    it('renames sessions inline from the picker', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);
      mockAIChatAPI.renameSession.mockResolvedValue({
        id: 's1',
        title: 'Renamed session',
        created_at: '',
        updated_at: '2026-06-06T12:35:00Z',
        message_count: 5,
      });
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await screen.findByText('Session One');

      fireEvent.click(
        screen.getByRole('button', { name: 'Rename Assistant session: Session One' }),
      );
      const titleInput = screen.getByLabelText('New title for Session One');
      expect(titleInput).toHaveValue('Session One');

      fireEvent.input(titleInput, { target: { value: 'Renamed session' } });
      fireEvent.click(screen.getByRole('button', { name: 'Save Assistant session title' }));

      await waitFor(() => {
        expect(mockAIChatAPI.renameSession).toHaveBeenCalledWith('s1', 'Renamed session');
      });
      expect(screen.getByText('Renamed session')).toBeInTheDocument();
      expect(screen.queryByText('Session One')).not.toBeInTheDocument();
    });

    it('keeps empty session rename drafts local', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await screen.findByText('Session One');

      fireEvent.click(
        screen.getByRole('button', { name: 'Rename Assistant session: Session One' }),
      );
      const titleInput = screen.getByLabelText('New title for Session One');
      fireEvent.input(titleInput, { target: { value: '   ' } });
      fireEvent.click(screen.getByRole('button', { name: 'Save Assistant session title' }));

      expect(mockAIChatAPI.renameSession).not.toHaveBeenCalled();
      expect(mockNotificationStore.error).toHaveBeenCalledWith('Session title cannot be empty');
      expect(titleInput).toBeInTheDocument();
    });

    it('marks the current working session in the picker', async () => {
      mockChat.sessionId.mockReturnValue('s1');
      mockChat.isLoading.mockReturnValue(true);
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      const option = await screen.findByRole('option', {
        name: 'Resume Session One, 5 messages, Current, Working',
      });
      expect(option).toHaveAttribute('aria-selected', 'true');
      expect(screen.getByText('Working')).toBeInTheDocument();
    });

    it('searches assistant sessions from the picker', async () => {
      mockAIChatAPI.listSessions
        .mockResolvedValueOnce([
          { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
          {
            id: 's2',
            title: 'Backup Patrol Follow-up',
            created_at: '',
            updated_at: '',
            message_count: 3,
          },
        ])
        .mockResolvedValueOnce([
          { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
          {
            id: 's2',
            title: 'Backup Patrol Follow-up',
            created_at: '',
            updated_at: '',
            message_count: 3,
          },
        ])
        .mockResolvedValueOnce([
          {
            id: 's2',
            title: 'Backup Patrol Follow-up',
            created_at: '',
            updated_at: '',
            message_count: 3,
          },
        ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledTimes(1);
      });

      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(screen.getByPlaceholderText('Search sessions...')).toBeInTheDocument();
      });

      fireEvent.input(screen.getByPlaceholderText('Search sessions...'), {
        target: { value: 'backup' },
      });

      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ search: 'backup', limit: 30 });
        expect(screen.getByText('Backup Patrol Follow-up')).toBeInTheDocument();
        expect(screen.queryByText('Session One')).not.toBeInTheDocument();
      });
    });

    it('loads a session when clicked in the dropdown', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(
          screen.getByRole('option', { name: 'Resume Session One, 5 messages' }),
        ).toBeInTheDocument();
      });
      fireEvent.click(screen.getByRole('option', { name: 'Resume Session One, 5 messages' }));
      expect(mockChat.loadSession).toHaveBeenCalledWith('s1');
    });

    it('returns focus to the composer after loading a session', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');

      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      textarea.blur();
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(screen.getByText('Session One')).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText('Session One'));

      await waitFor(() => {
        expect(mockChat.loadSession).toHaveBeenCalledWith('s1');
        expect(document.activeElement).toBe(textarea);
      });
    });

    it('restores safe Patrol handoff state from a loaded session summary', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol',
          title: 'High CPU follow-up',
          created_at: '',
          updated_at: '',
          message_count: 4,
          handoff_summary: {
            kind: 'patrol_finding',
            finding_id: 'finding-operator-briefing',
            has_model_context: true,
            resource_count: 1,
            primary_resource: {
              id: 'host:web-server',
              name: 'web-server',
              type: 'host',
              node: 'pve-1',
            },
            action_count: 1,
            requires_approval: true,
            last_known_approval_status: 'pending',
            last_known_action_state: 'awaiting_approval',
            last_known_action_risk: 'medium',
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
        expect(screen.getByText('Approval required')).toBeInTheDocument();
        expect(screen.getByText(/approval pending/)).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('High CPU follow-up'));

      await waitFor(() => {
        expect(mockChat.loadSession).toHaveBeenCalledWith('s-patrol');
        expect(mockAiChatStore.setContext).toHaveBeenCalledWith(
          expect.objectContaining({
            findingId: 'finding-operator-briefing',
            autonomousMode: false,
            briefing: expect.objectContaining({
              sourceLabel: 'Pulse Patrol',
              title: 'Patrol finding on web-server',
              subject: 'Finding finding-operator-briefing',
              actionLabel: 'Approval required',
            }),
          }),
        );
      });
      const restoredContext = mockAiChatStore.setContext.mock.calls[0]?.[0];
      expect(restoredContext.handoffContext).toBeUndefined();
      expect(restoredContext.handoffActions).toBeUndefined();
    });

    it('restores safe Patrol run handoff state from a loaded session summary', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol-run',
          title: 'Runtime failure follow-up',
          created_at: '',
          updated_at: '',
          message_count: 2,
          handoff_summary: {
            kind: 'patrol_run',
            run_id: 'run-runtime-error',
            run_type: 'Scoped run',
            run_status: 'error',
            runtime_failure: true,
            has_model_context: true,
            resource_count: 1,
            primary_resource: {
              id: 'vm-100',
              type: 'vm',
            },
            action_count: 0,
            requires_approval: false,
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
        expect(screen.getByText('Runtime issue')).toBeInTheDocument();
        expect(screen.getByText(/run error/)).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Runtime failure follow-up'));

      await waitFor(() => {
        expect(mockChat.loadSession).toHaveBeenCalledWith('s-patrol-run');
        expect(mockAiChatStore.setContext).toHaveBeenCalledWith(
          expect.objectContaining({
            targetType: 'patrol-run',
            targetId: 'run-runtime-error',
            findingId: undefined,
            autonomousMode: false,
            context: expect.objectContaining({
              kind: 'patrol_run',
              runId: 'run-runtime-error',
              runType: 'Scoped run',
              runStatus: 'error',
              runtimeFailure: true,
            }),
            briefing: expect.objectContaining({
              sourceLabel: 'Pulse Patrol',
              title: 'Patrol run run-runtime-error',
              subject: 'Run run-runtime-error',
              actionLabel: 'Review Patrol runtime issue',
              commandSummary: expect.stringContaining('runtime issue'),
            }),
          }),
        );
      });
      const restoredContext = mockAiChatStore.setContext.mock.calls[0]?.[0];
      expect(restoredContext.handoffContext).toBeUndefined();
      expect(restoredContext.handoffMetadata).toBeUndefined();

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'what failed?' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith('what failed?', undefined, undefined, {
        autonomousMode: false,
      });
    });

    it('does not restore handoff context when loading a session fails', async () => {
      mockChat.loadSession.mockResolvedValueOnce(false);
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol',
          title: 'High CPU follow-up',
          created_at: '',
          updated_at: '',
          message_count: 4,
          handoff_summary: {
            kind: 'patrol_finding',
            finding_id: 'finding-operator-briefing',
            has_model_context: true,
            resource_count: 1,
            primary_resource: {
              id: 'host:web-server',
              name: 'web-server',
              type: 'host',
              node: 'pve-1',
            },
            action_count: 1,
            requires_approval: true,
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(screen.getByText('High CPU follow-up')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('High CPU follow-up'));

      await waitFor(() => {
        expect(mockChat.loadSession).toHaveBeenCalledWith('s-patrol');
      });
      expect(mockAiChatStore.setContext).not.toHaveBeenCalled();
      expect(mockAiChatStore.clearContext).not.toHaveBeenCalled();
    });

    it('restores Patrol assessment handoff state without pinning to one finding', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol-assessment',
          title: 'Assessment follow-up',
          created_at: '',
          updated_at: '',
          message_count: 3,
          handoff_summary: {
            kind: 'patrol_assessment',
            has_model_context: true,
            resource_count: 2,
            action_count: 0,
            requires_approval: false,
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
        expect(screen.getByText('Assessment context')).toBeInTheDocument();
        expect(screen.queryByText('Recommended: Run Patrol')).not.toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Assessment follow-up'));

      await waitFor(() => {
        expect(mockAiChatStore.setContext).toHaveBeenCalledWith(
          expect.objectContaining({
            targetType: 'patrol-assessment',
            targetId: 'pulse-patrol-assessment',
            findingId: undefined,
            autonomousMode: false,
            context: expect.objectContaining({
              kind: 'patrol_assessment',
              findingId: undefined,
            }),
            briefing: expect.objectContaining({
              sourceLabel: 'Pulse Patrol',
              title: 'Patrol assessment handoff',
              subject: 'Current Patrol assessment',
              actionLabel: 'Review Patrol assessment',
              detailLines: ['2 linked resources'],
            }),
          }),
        );
      });
    });

    it('restores Patrol configuration failure handoff state as a runtime issue', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol-config',
          title: 'Configuration follow-up',
          created_at: '',
          updated_at: '',
          message_count: 2,
          handoff_summary: {
            kind: 'patrol_configuration_failure',
            runtime_failure: true,
            has_model_context: true,
            resource_count: 0,
            action_count: 0,
            requires_approval: false,
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
        expect(screen.getAllByText('Runtime issue').length).toBeGreaterThan(0);
      });

      fireEvent.click(screen.getByText('Configuration follow-up'));

      await waitFor(() => {
        expect(mockAiChatStore.setContext).toHaveBeenCalledWith(
          expect.objectContaining({
            targetType: 'patrol-configuration',
            targetId: 'pulse-patrol-configuration',
            autonomousMode: false,
            context: expect.objectContaining({
              kind: 'patrol_configuration_failure',
              runtimeFailure: true,
            }),
            briefing: expect.objectContaining({
              sourceLabel: 'Pulse Patrol',
              title: 'Patrol configuration failure',
              subject: 'Patrol configuration',
              actionLabel: 'Review Patrol configuration issue',
              statusLabel: 'runtime issue',
            }),
          }),
        );
      });
    });

    it('shows completed Patrol actions as action context instead of pending approval', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol-complete',
          title: 'Completed remediation follow-up',
          created_at: '',
          updated_at: '',
          message_count: 6,
          handoff_summary: {
            kind: 'patrol_finding',
            finding_id: 'finding-complete',
            has_model_context: true,
            resource_count: 1,
            primary_resource: {
              id: 'host:web-server',
              name: 'web-server',
              type: 'host',
              node: 'pve-1',
            },
            action_count: 1,
            requires_approval: false,
            last_known_approval_status: 'approved',
            last_known_action_state: 'completed',
            last_known_action_risk: 'medium',
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
        expect(screen.getByText('Action context')).toBeInTheDocument();
        expect(screen.getByText(/approval approved/)).toBeInTheDocument();
        expect(screen.queryByText('Approval required')).not.toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Completed remediation follow-up'));

      await waitFor(() => {
        expect(mockAiChatStore.setContext).toHaveBeenCalledWith(
          expect.objectContaining({
            findingId: 'finding-complete',
            autonomousMode: false,
            context: expect.objectContaining({
              requiresApproval: false,
              lastKnownActionState: 'completed',
            }),
            briefing: expect.objectContaining({
              actionLabel: 'Governed action context',
              commandSummary: expect.stringContaining('action completed'),
            }),
          }),
        );
      });
    });

    it('keeps restored Patrol handoffs approval-bound without queued actions', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockAIChatAPI.listSessions.mockResolvedValue([
        {
          id: 's-patrol-context',
          title: 'Context-only Patrol follow-up',
          created_at: '',
          updated_at: '',
          message_count: 2,
          handoff_summary: {
            kind: 'patrol_finding',
            finding_id: 'finding-context-only',
            has_model_context: true,
            resource_count: 1,
            primary_resource: {
              id: 'host:web-server',
              name: 'web-server',
              type: 'host',
              node: 'pve-1',
            },
            action_count: 0,
            requires_approval: false,
          },
        },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(screen.getByText('Pulse Patrol')).toBeInTheDocument();
        expect(screen.getByText('Context attached')).toBeInTheDocument();
        expect(screen.queryByText('Open Patrol provider settings')).not.toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Context-only Patrol follow-up'));

      await waitFor(() => {
        expect(mockChat.loadSession).toHaveBeenCalledWith('s-patrol-context');
        expect(mockAiChatStore.setContext).toHaveBeenCalledWith(
          expect.objectContaining({
            findingId: 'finding-context-only',
            autonomousMode: false,
            context: expect.objectContaining({
              actionCount: 0,
              requiresApproval: false,
            }),
            briefing: expect.objectContaining({
              sourceLabel: 'Pulse Patrol',
              title: 'Patrol finding on web-server',
              actionLabel: undefined,
            }),
          }),
        );
      });

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'what changed?' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'what changed?',
        undefined,
        'finding-context-only',
        { autonomousMode: false },
      );
    });

    it('clears stale handoff context when loading a plain session', async () => {
      mockAiChatStore.context = {
        findingId: 'finding-old',
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Old finding handoff',
        },
      };
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);

      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(screen.getByText('Session One')).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText('Session One'));

      await waitFor(() => {
        expect(mockChat.loadSession).toHaveBeenCalledWith('s1');
        expect(mockAiChatStore.clearContext).toHaveBeenCalledTimes(1);
      });
    });
  });

  // ── Initialization (onMount) ─────────────────────────────────────────

  describe('initialization', () => {
    it('loads status, sessions, settings, and models on mount', async () => {
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.getStatus).toHaveBeenCalledTimes(1);
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledTimes(1);
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledWith({ limit: 30 });
        expect(mockAIAPI.getSettings).toHaveBeenCalledTimes(1);
        expect(mockAIAPI.getModels).toHaveBeenCalledTimes(1);
      });
    });

    it('does not load sessions, settings, or models when AI is not running', async () => {
      mockAIChatAPI.getStatus.mockResolvedValue({ running: false });
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.getStatus).toHaveBeenCalledTimes(1);
      });
      expect(mockAIChatAPI.listSessions).not.toHaveBeenCalled();
      expect(mockAIAPI.getSettings).not.toHaveBeenCalled();
      expect(mockAIAPI.getModels).not.toHaveBeenCalled();
    });
  });

  // ── @ mention detection ──────────────────────────────────────────────

  describe('mention autocomplete', () => {
    it('activates mention autocomplete on @ input', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      Object.defineProperty(textarea, 'selectionStart', { value: 1, writable: true });
      fireEvent.input(textarea, { target: { value: '@' } });
      const autocomplete = screen.getByTestId('mention-autocomplete');
      expect(autocomplete).toHaveAttribute('data-visible', 'true');
    });

    it('passes the query after @ to mention autocomplete', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      Object.defineProperty(textarea, 'selectionStart', { value: 5, writable: true });
      fireEvent.input(textarea, { target: { value: '@node' } });
      const autocomplete = screen.getByTestId('mention-autocomplete');
      expect(autocomplete).toHaveAttribute('data-query', 'node');
    });

    it('deactivates mention when space is typed after query', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      Object.defineProperty(textarea, 'selectionStart', {
        value: 5,
        writable: true,
        configurable: true,
      });
      fireEvent.input(textarea, { target: { value: '@test' } });
      expect(screen.getByTestId('mention-autocomplete')).toHaveAttribute('data-visible', 'true');

      Object.defineProperty(textarea, 'selectionStart', {
        value: 6,
        writable: true,
        configurable: true,
      });
      fireEvent.input(textarea, { target: { value: '@test ' } });
      expect(screen.getByTestId('mention-autocomplete')).toHaveAttribute('data-visible', 'false');
    });

    it('does not activate mention when @ is in the middle of a word', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      Object.defineProperty(textarea, 'selectionStart', {
        value: 6,
        writable: true,
        configurable: true,
      });
      fireEvent.input(textarea, { target: { value: 'email@' } });
      const autocomplete = screen.getByTestId('mention-autocomplete');
      expect(autocomplete).toHaveAttribute('data-visible', 'false');
    });

    it('uses policy-aware labels and preserves distinct governed mention candidates', async () => {
      const governedPolicy = {
        sensitivity: 'restricted' as const,
        routing: { scope: 'local-only' as const, redact: ['hostname'] as const },
      };
      const governedResources = [
        {
          id: 'agent-1',
          name: 'secret-node-1',
          label: 'redacted by policy',
          type: 'agent' as const,
          status: 'online',
          policy: governedPolicy,
          aiSafeSummary: 'redacted by policy',
        },
        {
          id: 'agent-2',
          name: 'secret-node-2',
          label: 'redacted by policy',
          type: 'agent' as const,
          status: 'online',
          policy: governedPolicy,
          aiSafeSummary: 'redacted by policy',
        },
      ];
      mockByType.mockImplementation((type: string) => (type === 'agent' ? governedResources : []));
      mockResources.mockReturnValue(governedResources);

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      Object.defineProperty(textarea, 'selectionStart', { value: 1, writable: true });
      fireEvent.input(textarea, { target: { value: '@' } });

      const autocomplete = await screen.findByTestId('mention-autocomplete');
      expect(autocomplete).toHaveAttribute('data-resource-count', '2');
      expect(autocomplete).toHaveAttribute(
        'data-resource-labels',
        'redacted by policy|redacted by policy',
      );
    });

    it('surfaces TrueNAS app containers through canonical app-container mention IDs', async () => {
      const nextcloud = {
        id: 'app-container:truenas-main:nextcloud',
        type: 'app-container' as const,
        name: 'nextcloud',
        displayName: 'Nextcloud',
        status: 'running',
        parentId: 'agent:truenas-main',
        parentName: 'truenas-main',
        platformType: 'truenas',
        sourceType: 'truenas',
        tags: ['truenas', 'app'],
      };
      mockByType.mockImplementation((type: string) =>
        type === 'app-container' ? [nextcloud] : [],
      );
      mockResources.mockReturnValue([nextcloud]);

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      Object.defineProperty(textarea, 'selectionStart', { value: 5, writable: true });
      fireEvent.input(textarea, { target: { value: '@next' } });

      await waitFor(() => {
        expect(screen.getByTestId('mention-autocomplete')).toHaveAttribute(
          'data-resource-ids',
          'app-container:truenas-main:nextcloud',
        );
      });

      fireEvent.click(screen.getByTestId('mention-select-app-container:truenas-main:nextcloud'));
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        '@Nextcloud',
        [
          expect.objectContaining({
            id: 'app-container:truenas-main:nextcloud',
            name: 'Nextcloud',
            type: 'app-container',
            node: 'truenas-main',
          }),
        ],
        undefined,
      );
    });

    it('preserves canonical VMware agent, vm, and storage mention IDs in the shared payload', async () => {
      const vmwareResources = [
        {
          id: 'vmware-host-1',
          type: 'agent' as const,
          name: 'esxi-01.lab.local',
          displayName: 'ESXi 01',
          status: 'online',
          platformType: 'vmware-vsphere',
          sourceType: 'api',
          agent: {
            agentId: 'vc-1:host:host-101',
            hostname: 'esxi-01.lab.local',
            platform: 'VMware ESXi',
          },
          vmware: {
            managedObjectId: 'host-101',
            connectionName: 'Lab VC',
            entityType: 'host',
          },
        },
        {
          id: 'vmware-vm-1',
          type: 'vm' as const,
          name: 'app-01',
          displayName: 'App 01',
          status: 'running',
          parentId: 'vmware-host-1',
          parentName: 'esxi-01.lab.local',
          platformType: 'vmware-vsphere',
          sourceType: 'api',
          vmware: {
            managedObjectId: 'vm-201',
            runtimeHostName: 'esxi-01.lab.local',
            connectionName: 'Lab VC',
          },
        },
        {
          id: 'vmware-datastore-1',
          type: 'storage' as const,
          name: 'nvme-primary',
          displayName: 'NVMe Primary',
          status: 'online',
          parentName: 'Lab VC',
          platformType: 'vmware-vsphere',
          sourceType: 'api',
          vmware: {
            managedObjectId: 'datastore-11',
            connectionName: 'Lab VC',
          },
        },
      ];
      mockByType.mockImplementation((type: string) => {
        switch (type) {
          case 'agent':
            return [vmwareResources[0]];
          case 'vm':
            return [vmwareResources[1]];
          case 'storage':
            return [vmwareResources[2]];
          default:
            return [];
        }
      });
      mockResources.mockReturnValue(vmwareResources);

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;

      Object.defineProperty(textarea, 'selectionStart', { value: 5, writable: true });
      fireEvent.input(textarea, { target: { value: '@esxi' } });

      await waitFor(() => {
        expect(
          screen.getByTestId('mention-autocomplete').getAttribute('data-resource-ids'),
        ).toContain('agent:vc-1:host:host-101');
      });

      fireEvent.click(screen.getByTestId('mention-select-agent:vc-1:host:host-101'));
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockChat.sendMessage).toHaveBeenCalledWith(
          '@ESXi 01',
          [
            expect.objectContaining({
              id: 'agent:vc-1:host:host-101',
              name: 'ESXi 01',
              type: 'agent',
            }),
          ],
          undefined,
        );
      });

      mockChat.sendMessage.mockClear();
      await waitFor(() => {
        expect(textarea.value).toBe('');
      });
      Object.defineProperty(textarea, 'selectionStart', { value: 4, writable: true });
      fireEvent.input(textarea, { target: { value: '@app' } });

      await waitFor(() => {
        expect(
          screen.getByTestId('mention-autocomplete').getAttribute('data-resource-ids'),
        ).toContain('vmware-vm-1');
      });

      fireEvent.click(screen.getByTestId('mention-select-vmware-vm-1'));
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockChat.sendMessage).toHaveBeenCalledWith(
          '@App 01',
          [
            expect.objectContaining({
              id: 'vmware-vm-1',
              name: 'App 01',
              type: 'vm',
              node: 'esxi-01.lab.local',
            }),
          ],
          undefined,
        );
      });

      mockChat.sendMessage.mockClear();
      await waitFor(() => {
        expect(textarea.value).toBe('');
      });
      Object.defineProperty(textarea, 'selectionStart', { value: 5, writable: true });
      fireEvent.input(textarea, { target: { value: '@nvme' } });

      await waitFor(() => {
        expect(
          screen.getByTestId('mention-autocomplete').getAttribute('data-resource-ids'),
        ).toContain('vmware-datastore-1');
      });

      fireEvent.click(screen.getByTestId('mention-select-vmware-datastore-1'));
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockChat.sendMessage).toHaveBeenCalledWith(
          '@NVMe Primary',
          [
            expect.objectContaining({
              id: 'vmware-datastore-1',
              name: 'NVMe Primary',
              type: 'storage',
              node: 'Lab VC',
            }),
          ],
          undefined,
        );
      });
    });

    it('inserts the governed display label into the prompt when a mention is selected', async () => {
      const governedPolicy = {
        sensitivity: 'restricted' as const,
        routing: { scope: 'local-only' as const, redact: ['hostname'] as const },
      };
      const governedResources = [
        {
          id: 'agent-1',
          name: 'secret-node-1',
          label: 'redacted by policy',
          type: 'agent' as const,
          status: 'online',
          policy: governedPolicy,
          aiSafeSummary: 'redacted by policy',
        },
      ];
      mockByType.mockImplementation((type: string) => (type === 'agent' ? governedResources : []));
      mockResources.mockReturnValue(governedResources);

      renderChat();
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      Object.defineProperty(textarea, 'selectionStart', { value: 1, writable: true });
      fireEvent.input(textarea, { target: { value: '@' } });

      await waitFor(() => {
        expect(screen.getByTestId('mention-autocomplete')).toHaveAttribute(
          'data-resource-count',
          '1',
        );
      });

      fireEvent.click(screen.getByTestId('mention-select-first'));
      await waitFor(() => {
        expect(textarea.value).toBe('@redacted by policy ');
      });

      await waitForProviderCheckSettled();
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        '@redacted by policy',
        [
          expect.objectContaining({
            id: 'node::secret-node-1',
            name: 'redacted by policy',
            type: 'agent',
          }),
        ],
        undefined,
      );
    });
  });

  // ── Model persistence ────────────────────────────────────────────────

  describe('model persistence', () => {
    it('ignores malformed per-session model storage on mount', () => {
      localStorage.setItem('pulse:ai_chat_models_by_session', '{broken json');
      renderChat();
      expect(localStorage.getItem('pulse:ai_chat_models_by_session')).toBe('{broken json');
    });

    it('drops stale stored default model overrides on mount', () => {
      localStorage.setItem(
        'pulse:ai_chat_models_by_session',
        JSON.stringify({ __default__: 'claude-3' }),
      );
      renderChat();
      expect(mockChat.setModel).not.toHaveBeenCalledWith('claude-3');
      expect(localStorage.getItem('pulse:ai_chat_models_by_session')).toBeNull();
    });

    it('restores stored model choices for real sessions', async () => {
      localStorage.setItem(
        'pulse:ai_chat_models_by_session',
        JSON.stringify({ 'session-1': 'openrouter:deepseek/deepseek-v4-pro' }),
      );
      mockChat.sessionId.mockReturnValue('session-1');

      renderChat();

      await waitFor(() => {
        expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      });
    });

    it('lets active transcript route evidence override stale session model storage', async () => {
      const [selectedModel, setSelectedModel] = createSignal('openrouter:deepseek/deepseek-chat');
      localStorage.setItem(
        'pulse:ai_chat_models_by_session',
        JSON.stringify({ 'session-1': 'openrouter:deepseek/deepseek-chat' }),
      );
      mockChat.sessionId.mockReturnValue('session-1');
      mockChat.model.mockImplementation(() => selectedModel());
      mockChat.setModel.mockImplementation((modelId: string) => {
        setSelectedModel(modelId);
      });
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user',
          content: '/fixture provider-retry',
          timestamp: new Date('2026-06-07T21:58:00Z'),
          model: 'openrouter:qwen/qwen3.7-plus',
        },
        {
          id: 'assistant-1',
          role: 'assistant',
          content: '',
          timestamp: new Date('2026-06-07T21:58:01Z'),
          model: 'openrouter:qwen/qwen3.7-plus',
          isStreaming: true,
          workflowStatus: {
            phase: 'provider_retry',
            message: 'Provider connection failed before any output; retrying.',
            provider: 'openrouter',
            model: 'openrouter:qwen/qwen3.7-plus',
          },
          streamEvents: [
            {
              type: 'model_switch',
              model: 'openrouter:qwen/qwen3.7-plus',
              modelEvent: 'selected',
            },
            {
              type: 'workflow_status',
              workflowStatus: {
                phase: 'provider_retry',
                message: 'Provider connection failed before any output; retrying.',
                provider: 'openrouter',
                model: 'openrouter:qwen/qwen3.7-plus',
              },
            },
          ],
        },
      ]);

      renderChat();

      await waitFor(() => {
        expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:qwen/qwen3.7-plus');
      });
      expect(mockChat.setModel).not.toHaveBeenCalledWith('openrouter:deepseek/deepseek-chat');
      expect(localStorage.getItem('pulse:ai_chat_models_by_session')).toBe(
        JSON.stringify({ 'session-1': 'openrouter:qwen/qwen3.7-plus' }),
      );
      expect(screen.getByTestId('model-selector')).toHaveAttribute(
        'data-selected',
        'openrouter:qwen/qwen3.7-plus',
      );
    });

    it('passes stored recent model routes to the model selector', () => {
      localStorage.setItem(
        'pulse:ai_chat_recent_models',
        JSON.stringify([
          'openrouter:deepseek/deepseek-v4-pro',
          'plain-model-name',
          'openai:gpt-4o',
          'openrouter:deepseek/deepseek-v4-pro',
        ]),
      );

      renderChat();

      expect(screen.getByTestId('model-selector')).toHaveAttribute(
        'data-recent-models',
        'openrouter:deepseek/deepseek-v4-pro|openai:gpt-4o',
      );
    });

    it('records selected explicit model routes as recents', async () => {
      renderChat();

      mockModelSelectorProps[mockModelSelectorProps.length - 1].onModelSelect?.(
        'openrouter:deepseek/deepseek-v4-pro',
      );

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(localStorage.getItem('pulse:ai_chat_recent_models')).toBe(
        JSON.stringify(['openrouter:deepseek/deepseek-v4-pro']),
      );

      await waitFor(() => {
        expect(screen.getByTestId('model-selector')).toHaveAttribute(
          'data-recent-models',
          'openrouter:deepseek/deepseek-v4-pro',
        );
      });
    });

    it('does not adopt a completed route-switch row for the active session automatically', async () => {
      const [messages, setMessages] = createSignal<ChatMessage[]>([]);
      mockChat.messages.mockImplementation(() => messages());
      mockChat.sessionId.mockReturnValue('session-1');
      mockChat.model.mockReturnValue('deepseek:deepseek-v4-pro');
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });

      renderChat();

      setMessages([
        {
          id: 'assistant-route-switch',
          role: 'assistant',
          content: '',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          model: 'openrouter:deepseek/deepseek-v4-pro',
          isStreaming: true,
          streamEvents: [
            {
              type: 'model_switch',
              model: 'openrouter:deepseek/deepseek-v4-pro',
              failedModel: 'deepseek:deepseek-v4-pro',
            },
          ],
        },
      ]);

      expect(mockChat.setModel).not.toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');

      setMessages([
        {
          id: 'assistant-route-switch',
          role: 'assistant',
          content: 'The response completed on another route.',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          completedAt: new Date('2026-06-06T12:00:03Z'),
          model: 'openrouter:deepseek/deepseek-v4-pro',
          isStreaming: false,
          streamEvents: [
            {
              type: 'model_switch',
              model: 'openrouter:deepseek/deepseek-v4-pro',
              failedModel: 'deepseek:deepseek-v4-pro',
            },
            {
              type: 'content',
              content: 'The response completed on another route.',
            },
          ],
        },
      ]);

      expect(
        screen.queryByRole('status', { name: 'Assistant model route switch adopted' }),
      ).not.toBeInTheDocument();
      expect(mockChat.setModel).not.toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(mockNotificationStore.success).not.toHaveBeenCalledWith(
        expect.stringContaining('Assistant model route switched to'),
        expect.any(Number),
      );
      expect(localStorage.getItem('pulse:ai_chat_models_by_session')).toBe(
        JSON.stringify({ 'session-1': 'deepseek:deepseek-v4-pro' }),
      );
      expect(localStorage.getItem('pulse:ai_chat_recent_models')).toBeNull();
    });

    it('does not override a user-selected route when a route-switch row completes', async () => {
      const [messages, setMessages] = createSignal<ChatMessage[]>([]);
      const [selectedModel, setSelectedModel] = createSignal('deepseek:deepseek-v4-pro');
      mockChat.messages.mockImplementation(() => messages());
      mockChat.sessionId.mockReturnValue('session-1');
      mockChat.model.mockImplementation(() => selectedModel());
      mockChat.setModel.mockImplementation((modelId: string) => {
        setSelectedModel(modelId);
      });
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'deepseek:deepseek-v4-pro',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });

      renderChat();

      setMessages([
        {
          id: 'assistant-route-switch',
          role: 'assistant',
          content: '',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          model: 'openrouter:deepseek/deepseek-v4-pro',
          isStreaming: true,
          streamEvents: [
            {
              type: 'model_switch',
              model: 'openrouter:deepseek/deepseek-v4-pro',
              failedModel: 'deepseek:deepseek-v4-pro',
            },
          ],
        },
      ]);

      mockModelSelectorProps[mockModelSelectorProps.length - 1].onModelSelect?.('openai:gpt-4o');

      setMessages([
        {
          id: 'assistant-route-switch',
          role: 'assistant',
          content: 'Route switch answer.',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          completedAt: new Date('2026-06-06T12:00:03Z'),
          model: 'openrouter:deepseek/deepseek-v4-pro',
          isStreaming: false,
          streamEvents: [
            {
              type: 'model_switch',
              model: 'openrouter:deepseek/deepseek-v4-pro',
              failedModel: 'deepseek:deepseek-v4-pro',
            },
            {
              type: 'content',
              content: 'Route switch answer.',
            },
          ],
        },
      ]);

      await waitFor(() => {
        expect(localStorage.getItem('pulse:ai_chat_models_by_session')).toBe(
          JSON.stringify({ 'session-1': 'openai:gpt-4o' }),
        );
      });
      expect(mockChat.setModel).not.toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(localStorage.getItem('pulse:ai_chat_recent_models')).toBe(
        JSON.stringify(['openai:gpt-4o']),
      );
    });

    it('does not record non-routed model strings as recents', () => {
      renderChat();

      mockModelSelectorProps[mockModelSelectorProps.length - 1].onModelSelect?.('plain-model-name');

      expect(mockChat.setModel).toHaveBeenCalledWith('plain-model-name');
      expect(localStorage.getItem('pulse:ai_chat_recent_models')).toBeNull();
    });

    it('cycles to the next recent model route without reordering recents', () => {
      const recents = [
        'openrouter:qwen/qwen3.7-plus',
        'openrouter:deepseek/deepseek-v4-pro',
        'gemini:gemini-3.1-flash-lite',
      ];
      localStorage.setItem('pulse:ai_chat_recent_models', JSON.stringify(recents));
      mockChat.model.mockReturnValue('openrouter:qwen/qwen3.7-plus');

      renderChat();

      fireEvent.click(
        screen.getByRole('button', {
          name: /Cycle recent Assistant model: DeepSeek: DeepSeek V4 Pro via OpenRouter/,
        }),
      );

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(localStorage.getItem('pulse:ai_chat_recent_models')).toBe(JSON.stringify(recents));
    });

    it('cycles to the first recent model route when the current model is outside recents', () => {
      localStorage.setItem(
        'pulse:ai_chat_recent_models',
        JSON.stringify(['openrouter:qwen/qwen3.7-plus']),
      );
      mockChat.model.mockReturnValue('openai:gpt-4o');

      renderChat();

      fireEvent.click(
        screen.getByRole('button', {
          name: /Cycle recent Assistant model: Qwen: Qwen3.7 Plus via OpenRouter/,
        }),
      );

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:qwen/qwen3.7-plus');
    });

    it('labels direct provider recent routes with provider identity', () => {
      localStorage.setItem(
        'pulse:ai_chat_recent_models',
        JSON.stringify(['deepseek:deepseek-v4-pro']),
      );
      mockChat.model.mockReturnValue('openrouter:qwen/qwen3.7-plus');

      renderChat();

      fireEvent.click(
        screen.getByRole('button', {
          name: /Cycle recent Assistant model: DeepSeek: DeepSeek V4 Pro/,
        }),
      );

      expect(
        screen.queryByRole('button', {
          name: 'Cycle recent Assistant model: deepseek-v4-pro',
        }),
      ).not.toBeInTheDocument();
      expect(mockChat.setModel).toHaveBeenCalledWith('deepseek:deepseek-v4-pro');
    });

    it('disables recent model cycling when the active route is the only recent route', () => {
      localStorage.setItem(
        'pulse:ai_chat_recent_models',
        JSON.stringify(['openrouter:qwen/qwen3.7-plus']),
      );
      mockChat.model.mockReturnValue('openrouter:qwen/qwen3.7-plus');

      renderChat();

      expect(screen.getByRole('button', { name: 'Cycle recent Assistant model' })).toBeDisabled();
    });
  });

  // ── Autonomous warning ───────────────────────────────────────────────

  describe('autonomous warning', () => {
    it('shows autonomous warning in the activity dock when control level is autonomous', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      renderChat();
      await waitFor(() => {
        const warning = screen.getByRole('status', {
          name: 'Assistant autonomous control warning',
        });
        expect(screen.getByTestId('assistant-activity-dock')).toContainElement(warning);
        expect(warning).toHaveTextContent('Autonomous: commands execute without approval.');
      });
    });

    it('shows Switch to Approval button in autonomous warning row', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      renderChat();
      await waitFor(() => {
        expect(
          screen.getByRole('status', { name: 'Assistant autonomous control warning' }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole('button', { name: 'Switch Assistant control mode to Approval' }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole('button', { name: 'Dismiss autonomous control warning' }),
        ).toBeInTheDocument();
      });
    });

    it('keeps scoped dashboard handoffs approval-required without showing the autonomous warning', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockAiChatStore.context = {
        findingId: undefined,
        autonomousMode: false,
        briefing: undefined,
      };

      renderChat();

      await waitFor(() => {
        expect(screen.getByText('Approval')).toBeInTheDocument();
      });
      expect(
        screen.queryByText(/Approval required for this dashboard brief/),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('status', { name: 'Assistant autonomous control warning' }),
      ).not.toBeInTheDocument();

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'summarize this dashboard' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'summarize this dashboard',
        undefined,
        undefined,
        { autonomousMode: false },
      );
    });

    it('shows scoped Patrol handoffs as approval-bound without a banner', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockAiChatStore.context = {
        findingId: 'finding-1',
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding attached',
        },
      };

      renderChat();

      await waitFor(() => {
        expect(screen.getByText('Approval')).toBeInTheDocument();
      });
      expect(screen.getByText('Approval required before any action.')).toBeInTheDocument();
      expect(
        screen.queryByText(/Approval required for this Patrol handoff/),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Approval required for this dashboard brief/),
      ).not.toBeInTheDocument();
    });

    it('shows scoped alert handoffs as approval-bound without a banner', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockAiChatStore.context = {
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Alerts',
          title: 'Alert investigation attached',
        },
        context: {
          alertIdentifier: 'alert-1',
        },
      };

      renderChat();

      await waitFor(() => {
        expect(screen.getByText('Approval')).toBeInTheDocument();
      });
      expect(screen.getByText('Approval required before any action.')).toBeInTheDocument();
      expect(
        screen.queryByText(/Approval required for this alert investigation/),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Approval required for this dashboard brief/),
      ).not.toBeInTheDocument();
    });

    it('passes model-only handoff context and resources on submit', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockAiChatStore.context = {
        autonomousMode: false,
        handoffContext:
          '[Alert Incident Context]\nIncident ID: incident-1\nTimeline Event 1: Command event recorded',
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
        briefing: {
          sourceLabel: 'Pulse Alerts',
          title: 'Incident timeline attached',
        },
      };

      renderChat();

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'what happened here?' } });
      await waitForProviderCheckSettled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'what happened here?',
        undefined,
        undefined,
        {
          autonomousMode: false,
          handoffContext:
            '[Alert Incident Context]\nIncident ID: incident-1\nTimeline Event 1: Command event recorded',
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
        },
      );
    });

    it('clears request handoff payloads after the first successful turn', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockAiChatStore.context = {
        findingId: 'finding-1',
        autonomousMode: false,
        handoffContext: '[Patrol Finding Context]\nFinding ID: finding-1',
        handoffResources: [
          {
            id: 'host:web-server',
            name: 'web-server',
            type: 'host',
            node: 'pve-1',
          },
        ],
        handoffActions: [
          {
            findingId: 'finding-1',
            approvalId: 'approval-1',
            approvalStatus: 'pending',
          },
        ],
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding on web-server',
        },
      };

      renderChat();

      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'what happened here?' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockAiChatStore.clearFindingId).toHaveBeenCalledTimes(1);
        expect(mockAiChatStore.clearRequestHandoffPayload).toHaveBeenCalledTimes(1);
      });
      expect(mockAiChatStore.context.findingId).toBeUndefined();
      expect(mockAiChatStore.context.handoffContext).toBeUndefined();
      expect(mockAiChatStore.context.handoffResources).toBeUndefined();
      expect(mockAiChatStore.context.handoffActions).toBeUndefined();
      expect(mockAiChatStore.context.autonomousMode).toBe(false);
      expect(mockAiChatStore.context.briefing?.title).toBe('Patrol finding on web-server');

      fireEvent.input(textarea, { target: { value: 'what changed next?' } });
      await waitForProviderCheckSettled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      await waitFor(() => {
        expect(mockChat.sendMessage).toHaveBeenCalledTimes(2);
      });
      expect(mockChat.sendMessage).toHaveBeenLastCalledWith(
        'what changed next?',
        undefined,
        undefined,
        { autonomousMode: false },
      );
    });
  });

  // ── Discovery hint ───────────────────────────────────────────────────

  describe('discovery hint', () => {
    it('shows discovery hint when discovery is disabled', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: false,
      });
      renderChat();
      await waitFor(() => {
        expect(screen.getByText('Workload Discovery is off.')).toBeInTheDocument();
      });
      expect(
        screen.getByText(
          /Enable it in Settings so Pulse Assistant can reference real services, versions, and commands instead of generic guidance\./,
        ),
      ).toBeInTheDocument();
    });

    it('does not show discovery hint when discovery is enabled', async () => {
      renderChat();
      await waitFor(() => {
        expect(mockAIAPI.getSettings).toHaveBeenCalled();
      });
      expect(screen.queryByText('Workload Discovery is off.')).not.toBeInTheDocument();
    });
  });

  // ── Status indicator ─────────────────────────────────────────────────

  describe('status indicator', () => {
    it('shows active turn status when loading with no assistant message', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Sending prompt',
      );
    });

    it('shows startup elapsed time once the submitted prompt timestamp is known', () => {
      vi.useFakeTimers();
      vi.setSystemTime(5_000);
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'user-1',
          role: 'user' as const,
          content: 'Check current resource',
          timestamp: new Date(1_000),
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Sending prompt (4s)',
      );
    });

    it('keeps first-token status visible once an assistant turn exists', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          isStreaming: true,
          streamEvents: [],
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Sending prompt',
      );
      expect(screen.queryByText('Generating response...')).not.toBeInTheDocument();
    });

    it('keeps workflow progress visible in the active turn status', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          isStreaming: true,
          streamEvents: [],
          workflowStatus: {
            phase: 'plan',
            message: 'Planning governed action and safety checks before execution.',
            state: 'READING',
            tool: 'pulse_exec',
          },
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Planning governed action and safety checks before execution. · command',
      );
      expect(screen.getByLabelText('Assistant active turn status')).not.toHaveTextContent(
        'pulse_exec',
      );
      expect(screen.queryByText('Generating response...')).not.toBeInTheDocument();
    });

    it('shows the active model route beside active turn progress', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          isStreaming: true,
          model: 'openrouter:qwen/qwen3.7-plus',
          streamEvents: [],
          workflowStatus: {
            phase: 'context',
            message: 'Reading current Pulse inventory.',
            state: 'READING',
            tool: 'pulse_query',
          },
        },
      ]);
      renderChat();

      const status = screen.getByLabelText('Assistant active turn status');
      expect(status).toHaveTextContent('Reading current Pulse inventory.');
      expect(status).not.toHaveTextContent('Qwen');

      const route = screen.getByLabelText('Assistant active model route');
      expect(route).toHaveTextContent('Qwen: Qwen3.7 Plus via OpenRouter');
      expect(route).toHaveAttribute(
        'title',
        expect.stringContaining('Active Assistant model route'),
      );
    });

    it('shows live provider retry countdowns in the active turn status', () => {
      vi.useFakeTimers();
      vi.setSystemTime(2_300);
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(1_000),
          isStreaming: true,
          streamEvents: [],
          workflowStatus: {
            phase: 'provider_retry',
            message: 'Provider connection failed before any output; retrying.',
            attempt: 2,
            maxAttempts: 3,
            retryAfterMs: 3200,
            startedAt: 1_000,
          },
        },
      ]);
      renderChat();

      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Provider connection failed before any output; retrying. · attempt 2/3 · retrying in 1.9s',
      );
    });

    it('prefers workflow progress over selected model route evidence', () => {
      const startedAt = Date.now() - 1_000;
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(startedAt),
          isStreaming: true,
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
          workflowStatus: {
            phase: 'request_start',
            message: 'Preparing Pulse context.',
            startedAt,
          },
        },
      ]);
      renderChat();

      const status = screen.getByLabelText('Assistant active turn status');
      expect(status).toHaveTextContent('Preparing Pulse context.');
      expect(status).not.toHaveTextContent('Using Qwen: Qwen3.7 Plus via OpenRouter');
    });

    it('paces replacing workflow progress in the active turn status footer', async () => {
      vi.useFakeTimers();
      vi.setSystemTime(1_200);
      const [isLoading, setIsLoading] = createSignal(false);
      const [messages, setMessages] = createSignal<ChatMessage[]>([]);
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
          message: 'OpenRouter is starting the response.',
          startedAt: 1_200,
        },
      ];
      mockChat.isLoading.mockImplementation(() => isLoading());
      mockChat.messages.mockImplementation(() => messages());
      renderChat();

      setIsLoading(true);
      setMessages([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          isStreaming: true,
          workflowStatusHistory,
          workflowStatus: workflowStatusHistory[2],
          streamEvents: [
            {
              type: 'workflow_status',
              workflowStatus: workflowStatusHistory[2],
              startedAt: 1_200,
              updatedAt: 1_200,
            },
          ],
        },
      ]);

      const status = screen.getByLabelText('Assistant active turn status');
      await waitFor(() => expect(status).toHaveTextContent('Preparing Pulse context.'));
      expect(status).not.toHaveTextContent('OpenRouter is starting the response.');

      await vi.advanceTimersByTimeAsync(WORKFLOW_STATUS_PACE_MS);
      expect(status).toHaveTextContent('Reading current Pulse inventory.');

      await vi.advanceTimersByTimeAsync(WORKFLOW_STATUS_PACE_MS);
      expect(status).toHaveTextContent('OpenRouter is starting the response.');
      expect(status).not.toHaveTextContent('Preparing Pulse context.');
      expect(status).not.toHaveTextContent('Reading current Pulse inventory.');
    });

    it('paces replacing workflow progress without dropping queued follow-up pressure', async () => {
      vi.useFakeTimers();
      vi.setSystemTime(1_200);
      const [isLoading, setIsLoading] = createSignal(false);
      const [messages, setMessages] = createSignal<ChatMessage[]>([]);
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
          message: 'OpenRouter is starting the response.',
          startedAt: 1_200,
        },
      ];
      mockChat.isLoading.mockImplementation(() => isLoading());
      mockChat.messages.mockImplementation(() => messages());
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      renderChat();

      setIsLoading(true);
      setMessages([
        {
          id: 'assistant-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          isStreaming: true,
          workflowStatusHistory,
          workflowStatus: workflowStatusHistory[2],
          streamEvents: [
            {
              type: 'workflow_status',
              workflowStatus: workflowStatusHistory[2],
              startedAt: 1_200,
              updatedAt: 1_200,
            },
          ],
        },
        {
          id: 'queued-user-1',
          role: 'user' as const,
          content: 'follow up',
          timestamp: new Date(),
          delivery: 'queued',
        },
      ]);

      const status = screen.getByLabelText('Assistant active turn status');
      await waitFor(() => expect(status).toHaveTextContent('Preparing Pulse context.'));
      expect(status).not.toHaveTextContent('follow-up queued');
      expect(screen.getByLabelText('Queued follow-up messages')).toHaveTextContent(
        '1 follow-up queued',
      );
      expect(status).not.toHaveTextContent('OpenRouter is starting the response.');

      await vi.advanceTimersByTimeAsync(WORKFLOW_STATUS_PACE_MS);
      expect(status).toHaveTextContent('Reading current Pulse inventory.');
      expect(status).not.toHaveTextContent('follow-up queued');

      await vi.advanceTimersByTimeAsync(WORKFLOW_STATUS_PACE_MS);
      expect(status).toHaveTextContent('OpenRouter is starting the response.');
      expect(status).not.toHaveTextContent('follow-up queued');
      expect(status).not.toHaveTextContent('Preparing Pulse context.');
      expect(status).not.toHaveTextContent('Reading current Pulse inventory.');
    });

    it('keeps pending tool progress visible in the active turn status', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          pendingTools: [{ id: 't1', name: 'pulse_get_nodes', input: '{}' }],
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Running get nodes',
      );
    });

    it('shows completed tool activity instead of stale provider wait in the active turn status', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
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
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Completed $ ls /dev | wc -l',
      );
      expect(screen.getByLabelText('Assistant active turn status')).not.toHaveTextContent(
        'OpenRouter is starting the response.',
      );
    });

    it('shows generating status when assistant content is streaming', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: 'partial response',
          timestamp: new Date(),
          isStreaming: true,
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Generating response',
      );
    });

    it('keeps late workflow progress visible after assistant content has streamed', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: 'I checked the first source.',
          timestamp: new Date(),
          isStreaming: true,
          streamEvents: [{ type: 'content', content: 'I checked the first source.' }],
          workflowStatus: {
            phase: 'provider_start',
            message: 'OpenRouter is starting the response.',
            startedAt: Date.now() - 5_000,
          },
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'OpenRouter is starting the response.',
      );
    });

    it('does not fall back to waiting when loading outlives visible assistant content', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: 'UI_PARITY_OK',
          timestamp: new Date(),
          isStreaming: false,
          streamEvents: [{ type: 'content', content: 'UI_PARITY_OK' }],
        },
      ]);
      renderChat();
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Generating response',
      );
    });

    it('keeps queued follow-up status alongside active assistant streaming status', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.queuedFollowUpCount.mockReturnValue(1);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: 'partial response',
          timestamp: new Date(),
          isStreaming: true,
        },
        {
          id: 'msg-2',
          role: 'user' as const,
          content: 'follow up',
          timestamp: new Date(),
          delivery: 'queued',
        },
      ]);
      renderChat();
      const activityDock = screen.getByTestId('assistant-activity-dock');

      expect(activityDock).toContainElement(screen.getByLabelText('Assistant active turn status'));
      expect(screen.getByLabelText('Assistant active turn status')).toHaveTextContent(
        'Generating response',
      );
      expect(screen.getByLabelText('Assistant active turn status')).not.toHaveTextContent(
        'follow-up queued',
      );
      expect(activityDock).toContainElement(screen.getByLabelText('Queued follow-up messages'));
      expect(screen.getByText('1 follow-up queued')).toBeInTheDocument();
    });

    it('keeps autonomous warning alongside active assistant streaming status', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([
        {
          id: 'msg-1',
          role: 'assistant' as const,
          content: '',
          timestamp: new Date(),
          isStreaming: true,
        },
      ]);

      renderChat();

      await waitFor(() => {
        const activityDock = screen.getByTestId('assistant-activity-dock');
        expect(activityDock).toContainElement(
          screen.getByLabelText('Assistant active turn status'),
        );
        expect(activityDock).toContainElement(
          screen.getByRole('status', { name: 'Assistant autonomous control warning' }),
        );
      });
    });

    it('shows no status indicator when not loading', () => {
      mockChat.isLoading.mockReturnValue(false);
      renderChat();
      expect(screen.queryByLabelText('Assistant active turn status')).not.toBeInTheDocument();
    });
  });

  // ── Last-turn summary footer ───────────────────────────────────────────────

  describe('last-turn summary footer', () => {
    it('shows completed assistant route, duration, and token usage in the composer chrome', () => {
      mockChat.messages.mockReturnValue([
        {
          id: 'asst-1',
          role: 'assistant' as const,
          content: 'Done.',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          completedAt: new Date('2026-06-06T12:00:03Z'),
          isStreaming: false,
          model: 'openrouter:qwen/qwen3.7-plus',
          tokens: { input: 500, output: 200 },
        },
      ]);

      renderChat();

      const summary = screen.getByLabelText(/Last assistant turn summary/);
      expect(summary).toHaveTextContent(
        'Last turn: Qwen: Qwen3.7 Plus via OpenRouter · 3s · 700 tokens',
      );
      expect(summary).not.toHaveTextContent('500 in');
      expect(summary).not.toHaveTextContent('200 out');
      expect(summary).toHaveAttribute(
        'title',
        'Last assistant turn summary: Model: Qwen: Qwen3.7 Plus via OpenRouter. Duration: 3s. Usage: 700 total, 500 input, 200 output',
      );
    });

    it('uses the latest completed assistant turn and skips the active streaming turn', () => {
      mockChat.messages.mockReturnValue([
        {
          id: 'asst-1',
          role: 'assistant' as const,
          content: 'Earlier result.',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          completedAt: new Date('2026-06-06T12:00:02Z'),
          isStreaming: false,
          tokens: { input: 100, output: 50 },
        },
        {
          id: 'asst-2',
          role: 'assistant' as const,
          content: 'Streaming result.',
          timestamp: new Date(),
          isStreaming: true,
          tokens: { input: 900, output: 800 },
        },
      ]);

      renderChat();

      const summary = screen.getByLabelText(/Last assistant turn summary/);
      expect(summary).toHaveTextContent('Last turn: 2s · 150 tokens');
      expect(summary).not.toHaveTextContent('100 in');
      expect(summary).not.toHaveTextContent('50 out');
      expect(summary).not.toHaveTextContent('1,700 tokens');
    });

    it('still summarizes completed turns when token usage is missing', () => {
      mockChat.messages.mockReturnValue([
        {
          id: 'asst-1',
          role: 'assistant' as const,
          content: 'Done.',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          completedAt: new Date('2026-06-06T12:00:05Z'),
          isStreaming: false,
          model: 'deepseek:deepseek-chat',
          tokens: { input: 500, output: 0 },
        },
      ]);

      renderChat();

      const summary = screen.getByLabelText(/Last assistant turn summary/);
      expect(summary).toHaveTextContent('Last turn: DeepSeek: DeepSeek Chat · 5s');
      expect(summary).not.toHaveTextContent('tokens');
    });

    it('does not show the summary for an active streaming turn', () => {
      mockChat.messages.mockReturnValue([
        {
          id: 'asst-1',
          role: 'assistant' as const,
          content: 'Streaming.',
          timestamp: new Date('2026-06-06T12:00:00Z'),
          isStreaming: true,
          model: 'openrouter:qwen/qwen3.7-plus',
          tokens: { input: 500, output: 200 },
        },
      ]);

      renderChat();

      expect(screen.queryByLabelText(/Last assistant turn summary/)).not.toBeInTheDocument();
    });
  });

  // ── Finding ID passthrough ───────────────────────────────────────────

  describe('finding ID context', () => {
    it('passes findingId from store context on first message', () => {
      mockAiChatStore.context = {
        findingId: 'finding-123',
        autonomousMode: undefined,
        briefing: undefined,
      };
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'investigate this' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).toHaveBeenCalledWith(
        'investigate this',
        undefined,
        'finding-123',
      );
    });
  });

  // ── Delete session ───────────────────────────────────────────────────

  describe('delete session', () => {
    it('calls deleteSession and removes session from the dropdown', async () => {
      const confirmSpy = vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
      try {
        mockAIChatAPI.listSessions.mockResolvedValue([
          { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 2 },
        ]);
        renderChat();
        await waitFor(() => {
          expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
        });
        fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
        await waitFor(() => {
          expect(screen.getByText('Session One')).toBeInTheDocument();
        });

        fireEvent.click(
          screen.getByRole('button', { name: 'Delete Assistant session: Session One' }),
        );

        await waitFor(() => {
          expect(mockAIChatAPI.deleteSession).toHaveBeenCalledWith('s1');
        });
        // Verify the session was removed from the UI
        await waitFor(() => {
          expect(screen.queryByText('Session One')).not.toBeInTheDocument();
        });
      } finally {
        confirmSpy.mockRestore();
      }
    });
  });
});
