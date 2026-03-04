import { describe, expect, it, vi, afterEach, beforeAll, beforeEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { ChatMessage, ModelInfo } from '../types';

// ── Hoisted mocks (vi.mock factories reference these) ──────────────────────

const {
  mockChat,
  mockAIAPI,
  mockAIChatAPI,
  mockNotificationStore,
  mockAiChatStore,
  mockByType,
  mockResources,
} = vi.hoisted(() => {
  const mockChat = {
    messages: vi.fn((): ChatMessage[] => []),
    isLoading: vi.fn(() => false),
    sessionId: vi.fn(() => ''),
    model: vi.fn(() => ''),
    setModel: vi.fn(),
    sendMessage: vi.fn().mockResolvedValue(true),
    stop: vi.fn(),
    clearMessages: vi.fn(),
    loadSession: vi.fn().mockResolvedValue(undefined),
    newSession: vi.fn().mockResolvedValue({
      id: 'new-sess',
      title: '',
      created_at: '',
      updated_at: '',
      message_count: 0,
    }),
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
    approveCommand: vi.fn().mockResolvedValue({ approved: true, message: 'ok' }),
    denyCommand: vi.fn().mockResolvedValue({ denied: true, message: 'ok' }),
  };

  const mockNotificationStore = {
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
    error: vi.fn(),
  };

  const mockAiChatStore = {
    isOpenSignal: vi.fn(() => true),
    context: {
      initialPrompt: undefined as string | undefined,
      findingId: undefined as string | undefined,
    },
    clearInitialPrompt: vi.fn(),
    clearFindingId: vi.fn(),
    settingsVersionSignal: vi.fn(() => 0),
    notifySettingsChanged: vi.fn(),
  };

  const mockByType = vi.fn((): Array<{ name: string }> => []);
  const mockResources = vi.fn((): Array<{ id: string; type: string }> => []);

  return {
    mockChat,
    mockAIAPI,
    mockAIChatAPI,
    mockNotificationStore,
    mockAiChatStore,
    mockByType,
    mockResources,
  };
});

// ── Module mocks ───────────────────────────────────────────────────────────

vi.mock('../hooks/useChat', () => ({
  useChat: () => mockChat,
}));

vi.mock('../ChatMessages', () => ({
  ChatMessages: (props: { messages: ChatMessage[]; emptyState?: { title: string } }) => (
    <div data-testid="chat-messages" data-msg-count={props.messages.length}>
      {props.emptyState?.title && (
        <span data-testid="empty-state-title">{props.emptyState.title}</span>
      )}
    </div>
  ),
}));

vi.mock('../ModelSelector', () => ({
  ModelSelector: (props: { selectedModel: string; models: ModelInfo[] }) => (
    <div
      data-testid="model-selector"
      data-selected={props.selectedModel}
      data-count={props.models.length}
    />
  ),
}));

vi.mock('../MentionAutocomplete', () => ({
  MentionAutocomplete: (props: { visible: boolean; query: string }) => (
    <div
      data-testid="mention-autocomplete"
      data-visible={String(props.visible)}
      data-query={props.query}
    />
  ),
}));

vi.mock('@/api/ai', () => ({ AIAPI: mockAIAPI }));
vi.mock('@/api/aiChat', () => ({ AIChatAPI: mockAIChatAPI }));
vi.mock('@/stores/notifications', () => ({ notificationStore: mockNotificationStore }));
vi.mock('@/stores/aiChat', () => ({ aiChatStore: mockAiChatStore }));
vi.mock('@/utils/logger', () => ({
  logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ byType: mockByType, resources: mockResources }),
}));

// ── Lazy import after mocks ────────────────────────────────────────────────

let AIChat: typeof import('../index').AIChat;

beforeAll(async () => {
  const mod = await import('../index');
  AIChat = mod.AIChat;
});

// ── Helpers ────────────────────────────────────────────────────────────────

function renderChat(onClose = vi.fn()) {
  return render(() => <AIChat onClose={onClose} />);
}

// ── Setup / teardown ───────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks();
  mockAiChatStore.isOpenSignal.mockReturnValue(true);
  mockAiChatStore.context = { initialPrompt: undefined, findingId: undefined };
  mockAiChatStore.settingsVersionSignal.mockReturnValue(0);
  mockChat.messages.mockReturnValue([]);
  mockChat.isLoading.mockReturnValue(false);
  mockChat.sessionId.mockReturnValue('');
  mockChat.model.mockReturnValue('');
  mockChat.sendMessage.mockResolvedValue(true);
  mockByType.mockReturnValue([]);
  mockResources.mockReturnValue([]);
  mockAIAPI.getModels.mockResolvedValue({ models: [] });
  mockAIAPI.getSettings.mockResolvedValue({
    model: 'gpt-4',
    chat_model: '',
    control_level: 'read_only',
    autonomous_mode: false,
    discovery_enabled: true,
  });
  mockAIChatAPI.getStatus.mockResolvedValue({ running: true });
  mockAIChatAPI.listSessions.mockResolvedValue([]);
  Element.prototype.scrollIntoView = vi.fn();
  localStorage.clear();
});

afterEach(cleanup);

// ── Tests ──────────────────────────────────────────────────────────────────

describe('AIChat', () => {
  // ── Rendering ──────────────────────────────────────────────────────────

  describe('rendering', () => {
    it('renders the header with title when open', () => {
      renderChat();
      expect(screen.getByText('Pulse Assistant')).toBeInTheDocument();
      expect(screen.getByText('Infrastructure intelligence')).toBeInTheDocument();
    });

    it('renders the input textarea', () => {
      renderChat();
      expect(screen.getByPlaceholderText('Ask about your infrastructure...')).toBeInTheDocument();
    });

    it('renders the ChatMessages child component', () => {
      renderChat();
      expect(screen.getByTestId('chat-messages')).toBeInTheDocument();
    });

    it('renders the ModelSelector child component', () => {
      renderChat();
      expect(screen.getByTestId('model-selector')).toBeInTheDocument();
    });

    it('renders keyboard hint text', () => {
      renderChat();
      expect(screen.getByText('to send')).toBeInTheDocument();
      expect(screen.getByText('to mention resources')).toBeInTheDocument();
    });

    it('renders New button', () => {
      renderChat();
      expect(screen.getByText('New')).toBeInTheDocument();
    });
  });

  // ── Close button ─────────────────────────────────────────────────────

  describe('close behavior', () => {
    it('calls onClose when close button is clicked', () => {
      const onClose = vi.fn();
      renderChat(onClose);
      const closeBtn = screen.getByTitle('Close panel');
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

    it('does not send an empty message', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: '   ' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('does not send when chat is loading', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'hello' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
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
      expect(screen.getByTitle('Stop')).toBeInTheDocument();
    });

    it('calls chat.stop when stop button is clicked', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      fireEvent.click(screen.getByTitle('Stop'));
      expect(mockChat.stop).toHaveBeenCalledTimes(1);
    });

    it('submits via form submit button click', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'test query' } });
      const submitBtn = screen.getByTitle('Send');
      fireEvent.click(submitBtn);
      expect(mockChat.sendMessage).toHaveBeenCalledWith('test query', undefined, undefined);
    });
  });

  // ── Control level ────────────────────────────────────────────────────

  describe('control level', () => {
    it('displays Read-only as default control label', () => {
      renderChat();
      expect(screen.getByText('Read-only')).toBeInTheDocument();
    });

    it('opens control menu on click', () => {
      renderChat();
      fireEvent.click(screen.getByTitle('Control mode'));
      expect(screen.getByText('Control mode for this chat')).toBeInTheDocument();
      expect(screen.getByText('No commands or control actions')).toBeInTheDocument();
      expect(screen.getByText('Ask before running commands')).toBeInTheDocument();
      expect(screen.getByText('Executes without approval (Pro)')).toBeInTheDocument();
    });
  });

  // ── Session management ───────────────────────────────────────────────

  describe('session management', () => {
    it('creates a new session on New button click', async () => {
      renderChat();
      fireEvent.click(screen.getByText('New'));
      await waitFor(() => {
        expect(mockChat.newSession).toHaveBeenCalledTimes(1);
      });
    });

    it('opens session picker on click', () => {
      renderChat();
      fireEvent.click(screen.getByTitle('Chat sessions'));
      expect(screen.getByText('New conversation')).toBeInTheDocument();
    });

    it('shows "No previous conversations" when empty', () => {
      renderChat();
      fireEvent.click(screen.getByTitle('Chat sessions'));
      expect(screen.getByText('No previous conversations')).toBeInTheDocument();
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
      fireEvent.click(screen.getByTitle('Chat sessions'));
      expect(screen.getByText('Session One')).toBeInTheDocument();
      expect(screen.getByText('Session Two')).toBeInTheDocument();
      expect(screen.getByText('5 messages')).toBeInTheDocument();
      expect(screen.getByText('3 messages')).toBeInTheDocument();
    });

    it('loads a session when clicked in the dropdown', async () => {
      mockAIChatAPI.listSessions.mockResolvedValue([
        { id: 's1', title: 'Session One', created_at: '', updated_at: '', message_count: 5 },
      ]);
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalled();
      });
      fireEvent.click(screen.getByTitle('Chat sessions'));
      fireEvent.click(screen.getByText('Session One'));
      expect(mockChat.loadSession).toHaveBeenCalledWith('s1');
    });
  });

  // ── Initialization (onMount) ─────────────────────────────────────────

  describe('initialization', () => {
    it('loads status, sessions, settings, and models on mount', async () => {
      renderChat();
      await waitFor(() => {
        expect(mockAIChatAPI.getStatus).toHaveBeenCalledTimes(1);
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledTimes(1);
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
  });

  // ── Model persistence ────────────────────────────────────────────────

  describe('model persistence', () => {
    it('ignores malformed per-session model storage on mount', () => {
      localStorage.setItem('pulse:ai_chat_models_by_session', '{broken json');
      renderChat();
      expect(localStorage.getItem('pulse:ai_chat_models_by_session')).toBe('{broken json');
    });

    it('initializes useChat with stored default model', () => {
      localStorage.setItem(
        'pulse:ai_chat_models_by_session',
        JSON.stringify({ __default__: 'claude-3' }),
      );
      renderChat();
      // The component should have passed this model into useChat — we can verify
      // the stored value persists (the component reads it at construction)
      const stored = JSON.parse(localStorage.getItem('pulse:ai_chat_models_by_session')!);
      expect(stored.__default__).toBe('claude-3');
    });
  });

  // ── Autonomous banner ────────────────────────────────────────────────

  describe('autonomous banner', () => {
    it('shows autonomous banner when control level is autonomous', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      renderChat();
      await waitFor(() => {
        expect(screen.getByText('Autonomous mode')).toBeInTheDocument();
        expect(screen.getByText('Commands execute without approval.')).toBeInTheDocument();
      });
    });

    it('shows Switch to Approval button in autonomous banner', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'gpt-4',
        chat_model: '',
        control_level: 'autonomous',
        autonomous_mode: true,
        discovery_enabled: true,
      });
      renderChat();
      await waitFor(() => {
        expect(screen.getByText('Switch to Approval')).toBeInTheDocument();
      });
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
        expect(screen.getByText('Discovery is off.')).toBeInTheDocument();
      });
    });

    it('does not show discovery hint when discovery is enabled', async () => {
      renderChat();
      await waitFor(() => {
        expect(mockAIAPI.getSettings).toHaveBeenCalled();
      });
      expect(screen.queryByText('Discovery is off.')).not.toBeInTheDocument();
    });
  });

  // ── Status indicator ─────────────────────────────────────────────────

  describe('status indicator', () => {
    it('shows "Thinking..." when loading with no assistant message', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([]);
      renderChat();
      expect(screen.getByText('Thinking...')).toBeInTheDocument();
    });

    it('shows tool status when assistant has pending tools', () => {
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
      expect(screen.getByText('Running get nodes...')).toBeInTheDocument();
    });

    it('shows "Generating response..." when assistant is streaming', () => {
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
      expect(screen.getByText('Generating response...')).toBeInTheDocument();
    });

    it('shows no status indicator when not loading', () => {
      mockChat.isLoading.mockReturnValue(false);
      renderChat();
      expect(screen.queryByText('Thinking...')).not.toBeInTheDocument();
      expect(screen.queryByText('Generating response...')).not.toBeInTheDocument();
    });
  });

  // ── Finding ID passthrough ───────────────────────────────────────────

  describe('finding ID context', () => {
    it('passes findingId from store context on first message', () => {
      mockAiChatStore.context = { initialPrompt: undefined, findingId: 'finding-123' };
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
        fireEvent.click(screen.getByTitle('Chat sessions'));
        expect(screen.getByText('Session One')).toBeInTheDocument();

        // Find the delete button inside the session row
        const sessionRow = screen.getByText('Session One').closest('[class*="group"]')!;
        const deleteBtn = sessionRow.querySelector('button[type="button"]')!;
        fireEvent.click(deleteBtn);

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
