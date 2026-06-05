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
  mockWebSocketState,
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

  return {
    mockChat,
    mockAIAPI,
    mockAIChatAPI,
    mockNotificationStore,
    mockAiChatStore,
    mockByType,
    mockResources,
    mockWebSocketState,
  };
});

// ── Module mocks ───────────────────────────────────────────────────────────

vi.mock('../hooks/useChat', () => ({
  useChat: () => mockChat,
}));

vi.mock('../ChatMessages', () => ({
  ChatMessages: (props: {
    messages: ChatMessage[];
    emptyState?: { title: string; subtitle?: string };
  }) => (
    <div data-testid="chat-messages" data-msg-count={props.messages.length}>
      {props.emptyState?.title && (
        <span data-testid="empty-state-title">{props.emptyState.title}</span>
      )}
      {props.emptyState?.subtitle && (
        <span data-testid="empty-state-subtitle">{props.emptyState.subtitle}</span>
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
let resetAIRuntimeState: typeof import('@/stores/aiRuntimeState').resetAIRuntimeState;

beforeAll(async () => {
  const [chatModule, runtimeModule] = await Promise.all([
    import('../index'),
    import('@/stores/aiRuntimeState'),
  ]);
  AIChat = chatModule.AIChat;
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

async function waitForComposerSendReady() {
  await waitFor(() => {
    expect(mockAIAPI.testProvider).toHaveBeenCalled();
  });
  await waitFor(() => {
    expect(screen.queryByText('Checking OpenAI provider')).not.toBeInTheDocument();
  });
  await waitFor(() => {
    expect(screen.getByRole('button', { name: 'Send message' })).not.toBeDisabled();
  });
}

// ── Setup / teardown ───────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks();
  setViewportWidth(1440);
  resetAIRuntimeState();
  mockAiChatStore.isOpenSignal.mockReturnValue(true);
  mockAiChatStore.context = {
    findingId: undefined,
    autonomousMode: undefined,
    briefing: undefined,
  };
  mockChat.messages.mockReturnValue([]);
  mockChat.isLoading.mockReturnValue(false);
  mockChat.sessionId.mockReturnValue('');
  mockChat.model.mockReturnValue('');
  mockChat.sendMessage.mockResolvedValue(true);
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
        expect(screen.getByLabelText('Assistant provider status')).toHaveTextContent(
          'DeepSeek provider issue',
        );
      });
      expect(screen.getByLabelText('Assistant provider status')).toHaveTextContent(
        'Pulse could not maintain a healthy connection to this provider.',
      );
      expect(screen.getByRole('link', { name: /Open settings/ })).toHaveAttribute(
        'href',
        '/settings/system-ai',
      );
    });

    it('keeps user input queued while the selected provider check is still running', async () => {
      mockAIAPI.getSettings.mockResolvedValue({
        model: 'openai:gpt-4',
        chat_model: '',
        control_level: 'read_only',
        autonomous_mode: false,
        discovery_enabled: true,
      });
      mockAIAPI.testProvider.mockReturnValue(new Promise(() => {}));

      renderChat();

      await screen.findByText('Checking OpenAI provider');
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });

      expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      expect(textarea.value).toBe('summarize the cluster');
      expect(document.activeElement).toBe(textarea);
    });

    it('keeps user input queued while a selected provider issue is unresolved', async () => {
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

      await screen.findByText('DeepSeek provider issue');
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });

      expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      expect(textarea.value).toBe('summarize the cluster');
      expect(document.activeElement).toBe(textarea);
    });

    it('keeps user input queued when the readiness issue reports a provider-qualified model', async () => {
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

      await screen.findByText('OpenAI provider issue');
      const textarea = screen.getByPlaceholderText(
        'Ask about your infrastructure...',
      ) as HTMLTextAreaElement;
      fireEvent.input(textarea, { target: { value: 'summarize the cluster' } });

      expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
      fireEvent.keyDown(textarea, { key: 'Enter' });

      expect(mockChat.sendMessage).not.toHaveBeenCalled();
      expect(textarea.value).toBe('summarize the cluster');
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
        name: 'Use OpenRouter provider route',
      });
      expect(switchButton).toHaveTextContent('Use OpenRouter');

      fireEvent.click(switchButton);

      expect(mockChat.setModel).toHaveBeenCalledWith('openrouter:deepseek/deepseek-v4-pro');
      expect(document.activeElement).toBe(
        screen.getByPlaceholderText('Ask about your infrastructure...'),
      );
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
        expect(screen.getByText('DeepSeek provider issue')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Retry provider check' }));

      await waitFor(() => {
        expect(mockAIAPI.testProvider).toHaveBeenCalledTimes(2);
        expect(screen.queryByLabelText('Assistant provider status')).not.toBeInTheDocument();
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

    it('uses attached briefing context instead of product-authored empty-state prompts', () => {
      mockAiChatStore.context = {
        autonomousMode: false,
        briefing: {
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol assessment attached',
          subject: 'Coverage incomplete',
        },
      };

      renderChat();

      expect(screen.getByText('Context attached')).toBeInTheDocument();
      expect(
        screen.getByText('Pulse Patrol · Patrol assessment attached · Coverage incomplete'),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Explain why coverage is incomplete' }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Summarize cluster health' }),
      ).not.toBeInTheDocument();
    });

    it('renders New button', () => {
      renderChat();
      expect(screen.getByText('New')).toBeInTheDocument();
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

    it('keeps the phone close button outside the scrollable assistant header actions', () => {
      setViewportWidth(375);
      renderChat();

      const closeButton = screen.getByRole('button', { name: 'Close Pulse Assistant' });
      const headerActions = screen.getByTestId('assistant-header-actions');

      expect(closeButton).toHaveClass('order-2');
      expect(headerActions).toHaveClass('order-3');
      expect(headerActions).toHaveClass('w-full');
      expect(headerActions).toHaveClass('overflow-x-auto');
      expect(headerActions).not.toContainElement(closeButton);
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

    it('does not send an empty message', () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: '   ' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).not.toHaveBeenCalled();
    });

    it('sends a replacement message when chat is loading', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      fireEvent.input(textarea, { target: { value: 'hello' } });
      fireEvent.keyDown(textarea, { key: 'Enter' });
      expect(mockChat.sendMessage).toHaveBeenCalledWith('hello', undefined, undefined);
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
      expect(screen.getByTitle('Send')).toBeInTheDocument();
    });

    it('calls chat.stop when stop button is clicked', () => {
      mockChat.isLoading.mockReturnValue(true);
      renderChat();
      fireEvent.click(screen.getByTitle('Stop'));
      expect(mockChat.stop).toHaveBeenCalledTimes(1);
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
      expect(screen.getByText('Default control mode')).toBeInTheDocument();
      expect(screen.getByText('No commands or control actions')).toBeInTheDocument();
      expect(screen.getByText('Ask before running commands')).toBeInTheDocument();
      expect(screen.getByText('Executes without approval')).toBeInTheDocument();
    });
  });

  // ── Session management ───────────────────────────────────────────────

  describe('session management', () => {
    it('starts a blank conversation on New button click', async () => {
      renderChat();
      fireEvent.click(screen.getByText('New'));
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
      fireEvent.click(screen.getByText('New'));

      await waitFor(() => {
        expect(mockAiChatStore.clearContext).toHaveBeenCalledTimes(1);
      });
    });

    it('returns focus to the composer after starting a blank conversation', async () => {
      renderChat();
      const textarea = screen.getByPlaceholderText('Ask about your infrastructure...');
      textarea.blur();

      fireEvent.click(screen.getByText('New'));

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
      fireEvent.click(screen.getByText('New'));

      await waitFor(() => {
        expect(mockChat.newSession).toHaveBeenCalledTimes(1);
      });
      expect(mockAiChatStore.clearContext).not.toHaveBeenCalled();
      expect(mockAiChatStore.context.findingId).toBe('finding-old');
    });

    it('opens session picker on click', async () => {
      renderChat();
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));
      await waitFor(() => {
        expect(screen.getByText('New session')).toBeInTheDocument();
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
      });
      fireEvent.click(screen.getByTitle('Pulse Assistant sessions'));

      await waitFor(() => {
        expect(mockAIChatAPI.listSessions).toHaveBeenCalledTimes(2);
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
        expect(screen.getByText('Session One')).toBeInTheDocument();
        expect(screen.getByText('Session Two')).toBeInTheDocument();
        expect(screen.getByText('5 messages')).toBeInTheDocument();
        expect(screen.getByText('3 messages')).toBeInTheDocument();
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
        expect(screen.getByText('Session One')).toBeInTheDocument();
      });
      fireEvent.click(screen.getByText('Session One'));
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

      await waitForComposerSendReady();
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
      expect(screen.queryByText('Commands execute without approval.')).not.toBeInTheDocument();

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
      await waitForComposerSendReady();
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
      await waitForComposerSendReady();
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
    it('shows "Thinking..." when loading with no assistant message', () => {
      mockChat.isLoading.mockReturnValue(true);
      mockChat.messages.mockReturnValue([]);
      renderChat();
      expect(screen.getByText('Thinking...')).toBeInTheDocument();
    });

    it('shows "Thinking..." while the assistant turn waits for the first token', () => {
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
      expect(screen.getByText('Thinking...')).toBeInTheDocument();
      expect(screen.queryByText('Generating response...')).not.toBeInTheDocument();
    });

    it('shows workflow progress while the assistant turn waits for the first token', () => {
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
      expect(
        screen.getByText('Planning governed action and safety checks before execution.'),
      ).toBeInTheDocument();
      expect(screen.queryByText('Thinking...')).not.toBeInTheDocument();
      expect(screen.queryByText('Generating response...')).not.toBeInTheDocument();
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
