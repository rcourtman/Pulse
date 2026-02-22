import { Component, Show, createSignal, onMount, For, createMemo, createEffect } from 'solid-js';
import { createStore } from 'solid-js/store';
import { useNavigate } from '@solidjs/router';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { formField, labelClass, controlClass } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { logger } from '@/utils/logger';
import { AIAPI } from '@/api/ai';
import { AIChatAPI, type ChatSession, type FileChange } from '@/api/aiChat';
import { getUpgradeActionUrlOrFallback, hasFeature, loadLicenseStatus } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import type { AISettings as AISettingsType, AIProvider, AuthMethod } from '@/types/ai';

// Providers are now configured via accordion sections, not a single-provider selector

// Provider display names for optgroup labels
const PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  openrouter: 'OpenRouter',
  deepseek: 'DeepSeek',
  gemini: 'Google Gemini',
  ollama: 'Ollama',
};

const AI_PROVIDERS: AIProvider[] = ['anthropic', 'openai', 'openrouter', 'deepseek', 'gemini', 'ollama'];

type ProviderHealthStatus = 'not_configured' | 'checking' | 'ok' | 'error';

type ProviderHealthState = {
  status: ProviderHealthStatus;
  message: string;
};

const createInitialProviderHealth = (): Record<AIProvider, ProviderHealthState> => ({
  anthropic: { status: 'not_configured', message: '' },
  openai: { status: 'not_configured', message: '' },
  openrouter: { status: 'not_configured', message: '' },
  deepseek: { status: 'not_configured', message: '' },
  gemini: { status: 'not_configured', message: '' },
  ollama: { status: 'not_configured', message: '' },
});

type ControlLevel = 'read_only' | 'controlled' | 'autonomous';

const normalizeControlLevel = (value?: string): ControlLevel => {
  if (value === 'controlled' || value === 'autonomous' || value === 'read_only') {
    return value;
  }
  if (value === 'suggest') {
    return 'controlled';
  }
  return 'read_only';
};

// Parse provider from model ID (format: "provider:model-name")
function getProviderFromModelId(modelId: string): string {
  const colonIndex = modelId.indexOf(':');
  if (colonIndex > 0) {
    return modelId.substring(0, colonIndex);
  }
  if (/^(openai|anthropic|google|deepseek|meta-llama|mistralai|x-ai|xai|cohere|qwen)\//.test(modelId)) {
    return 'openrouter';
  }
  // Default detection for models without prefix
  if (modelId.includes('claude') || modelId.includes('opus') || modelId.includes('sonnet') || modelId.includes('haiku')) {
    return 'anthropic';
  }
  if (modelId.includes('gpt') || modelId.includes('o1') || modelId.includes('o3')) {
    return 'openai';
  }
  if (modelId.includes('deepseek')) {
    return 'deepseek';
  }
  if (modelId.includes('gemini')) {
    return 'gemini';
  }
  return 'ollama';
}

// Check if a provider is configured based on settings
function isProviderConfigured(provider: string, settings: AISettingsType | null): boolean {
  if (!settings) return false;
  switch (provider) {
    case 'anthropic': return settings.anthropic_configured;
    case 'openai': return settings.openai_configured;
    case 'openrouter': return settings.openrouter_configured;
    case 'deepseek': return settings.deepseek_configured;
    case 'gemini': return settings.gemini_configured;
    case 'ollama': return settings.ollama_configured;
    default: return false;
  }
}

// Check if a model's provider is configured
function isModelProviderConfigured(modelId: string, settings: AISettingsType | null): boolean {
  const provider = getProviderFromModelId(modelId);
  return isProviderConfigured(provider, settings);
}

// Group models by provider for optgroup rendering
function groupModelsByProvider(models: { id: string; name: string; description?: string }[]): Map<string, { id: string; name: string; description?: string }[]> {
  const grouped = new Map<string, { id: string; name: string; description?: string }[]>();

  for (const model of models) {
    const provider = getProviderFromModelId(model.id);
    const existing = grouped.get(provider) || [];
    existing.push(model);
    grouped.set(provider, existing);
  }

  return grouped;
}

export const AISettings: Component = () => {
  const navigate = useNavigate();
  const [settings, setSettings] = createSignal<AISettingsType | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);

  // Dynamic model list from provider API
  const [availableModels, setAvailableModels] = createSignal<{ id: string; name: string; description?: string }[]>([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);

  const [chatSessions, setChatSessions] = createSignal<ChatSession[]>([]);
  const [chatSessionsLoading, setChatSessionsLoading] = createSignal(false);
  const [chatSessionsError, setChatSessionsError] = createSignal('');
  const [selectedSessionId, setSelectedSessionId] = createSignal('');
  const [sessionActionLoading, setSessionActionLoading] = createSignal<string | null>(null);

  const [showDiffModal, setShowDiffModal] = createSignal(false);
  const [diffFiles, setDiffFiles] = createSignal<FileChange[]>([]);
  const [diffSummary, setDiffSummary] = createSignal('');
  const [diffSessionLabel, setDiffSessionLabel] = createSignal('');

  // Accordion state for provider configuration sections
  const [expandedProviders, setExpandedProviders] = createSignal<Set<AIProvider>>(new Set(['anthropic']));

  // Per-provider test state
  const [testingProvider, setTestingProvider] = createSignal<string | null>(null);
  const [providerTestResult, setProviderTestResult] = createSignal<{ provider: string; success: boolean; message: string } | null>(null);
  const [providerHealth, setProviderHealth] = createStore<Record<AIProvider, ProviderHealthState>>(createInitialProviderHealth());
  const [preflightRunning, setPreflightRunning] = createSignal(false);
  const [preflightLastCheckedAt, setPreflightLastCheckedAt] = createSignal<number | null>(null);
  const hasAutoFixFeature = createMemo(() => hasFeature('ai_autofix'));
  const autoFixLocked = createMemo(() => !hasAutoFixFeature());
  const providerIssueCount = createMemo(() => AI_PROVIDERS.filter((provider) => providerHealth[provider].status === 'error').length);

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible = form.controlLevel === 'autonomous' && autoFixLocked();
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('ai_autofix', 'settings_ai_autonomous_mode');
    }
    return isPaywallVisible;
  }, false);

  // Auto-fix acknowledgement state (not persisted - must acknowledge each session)
  // Note: autoFixAcknowledged removed — auto-fix UI moved to Patrol page

  // First-time setup modal state
  const [showSetupModal, setShowSetupModal] = createSignal(false);
  const [setupProvider, setSetupProvider] = createSignal<'anthropic' | 'openai' | 'openrouter' | 'deepseek' | 'gemini' | 'ollama'>('anthropic');
  const [setupApiKey, setSetupApiKey] = createSignal('');
  const [setupOllamaUrl, setSetupOllamaUrl] = createSignal('http://localhost:11434');
  const [setupSaving, setSetupSaving] = createSignal(false);

  // UI state for collapsible sections - START COLLAPSED for compact view
  const [showAdvancedModels, setShowAdvancedModels] = createSignal(false);
  // Note: showPatrolSettings removed — patrol settings section moved to Patrol page
  const [showDiscoverySettings, setShowDiscoverySettings] = createSignal(false);

  const [showChatMaintenance, setShowChatMaintenance] = createSignal(false);

  const [form, setForm] = createStore({
    enabled: false,
    provider: 'anthropic' as AIProvider, // Legacy - kept for compatibility
    apiKey: '', // Legacy - kept for compatibility
    model: '',
    chatModel: '', // Empty means use default model
    patrolModel: '', // Empty means use default model
    autoFixModel: '', // Empty means use patrol model
    baseUrl: '', // Legacy - kept for compatibility
    clearApiKey: false,
    authMethod: 'api_key' as AuthMethod,
    patrolIntervalMinutes: 360, // 6 hours default
    alertTriggeredAnalysis: true,
    patrolAutoFix: false,
    // Multi-provider credentials
    anthropicApiKey: '',
    openaiApiKey: '',
    openrouterApiKey: '',
    deepseekApiKey: '',
    geminiApiKey: '',
    ollamaBaseUrl: 'http://localhost:11434',
    openaiBaseUrl: '',
    // Cost controls
    costBudgetUSD30d: '',
    // Request timeout (seconds) - for slow Ollama hardware
    requestTimeoutSeconds: 300,
    // Infrastructure control settings
    controlLevel: 'read_only' as ControlLevel,
    protectedGuests: '' as string, // Comma-separated VMIDs/names
    // Discovery settings
    discoveryEnabled: false,
    discoveryIntervalHours: 0, // 0 = manual only
  });

  const resetForm = (data: AISettingsType | null) => {
    if (!data) {
      setForm({
        enabled: false,
        provider: 'anthropic',
        apiKey: '',
        model: '', // Will be set when provider is configured
        chatModel: '',
        patrolModel: '',
        autoFixModel: '',
        baseUrl: '',
        clearApiKey: false,
        authMethod: 'api_key',
        patrolIntervalMinutes: 360, // 6 hours default
        alertTriggeredAnalysis: true,
        patrolAutoFix: false,
        // Multi-provider - empty by default
        anthropicApiKey: '',
        openaiApiKey: '',
        openrouterApiKey: '',
        deepseekApiKey: '',
        geminiApiKey: '',
        ollamaBaseUrl: 'http://localhost:11434',
        openaiBaseUrl: '',
        costBudgetUSD30d: '',
        requestTimeoutSeconds: 300,
        controlLevel: 'read_only',
        protectedGuests: '',
        discoveryEnabled: false,
        discoveryIntervalHours: 0,
      });
      return;
    }

    setForm({
      enabled: data.enabled,
      provider: data.provider,
      apiKey: '',
      model: data.model || '', // User must select a model
      chatModel: data.chat_model || '',
      patrolModel: data.patrol_model || '',
      autoFixModel: data.auto_fix_model || '',
      baseUrl: data.base_url || '',
      clearApiKey: false,
      authMethod: data.auth_method || 'api_key',
      patrolIntervalMinutes: data.patrol_interval_minutes ?? 360, // Use minutes, default to 6hr
      alertTriggeredAnalysis: data.alert_triggered_analysis !== false, // default to true
      patrolAutoFix: data.patrol_auto_fix || false, // default to false (observe only)
      // Multi-provider - never load actual keys from server (security), just track if configured
      anthropicApiKey: '', // Always empty - we only show if configured
      openaiApiKey: '',
      openrouterApiKey: '',
      deepseekApiKey: '',
      geminiApiKey: '',
      ollamaBaseUrl: data.ollama_base_url || 'http://localhost:11434',
      openaiBaseUrl: data.openai_base_url || '',
      costBudgetUSD30d:
        typeof data.cost_budget_usd_30d === 'number' && data.cost_budget_usd_30d > 0
          ? String(data.cost_budget_usd_30d)
          : '',
      requestTimeoutSeconds: data.request_timeout_seconds ?? 300,
      controlLevel: normalizeControlLevel(data.control_level),
      protectedGuests: Array.isArray(data.protected_guests) ? data.protected_guests.join(', ') : '',
      discoveryEnabled: data.discovery_enabled ?? false,
      discoveryIntervalHours: data.discovery_interval_hours ?? 0,
    });

    // Auto-expand providers that are configured
    const configured = new Set<AIProvider>();
    if (data.anthropic_configured) configured.add('anthropic');
    if (data.openai_configured) configured.add('openai');
    if (data.openrouter_configured) configured.add('openrouter');
    if (data.deepseek_configured) configured.add('deepseek');
    if (data.gemini_configured) configured.add('gemini');
    if (data.ollama_configured) configured.add('ollama');
    // Default to anthropic if nothing is configured
    if (configured.size === 0) configured.add('anthropic');
    setExpandedProviders(configured);
  };

  // Load available models from the provider's API
  const loadModels = async () => {
    setModelsLoading(true);
    try {
      const result = await AIAPI.getModels();
      if (result.models && result.models.length > 0) {
        setAvailableModels(result.models);
      }
    } catch (e) {
      // Silently fail - user can still type model names manually
      logger.debug('[AISettings] Failed to load models from API:', e);
    } finally {
      setModelsLoading(false);
    }
  };

  const loadChatSessions = async () => {
    setChatSessionsLoading(true);
    setChatSessionsError('');
    try {
      const sessions = await AIChatAPI.listSessions();
      setChatSessions(sessions);
      const current = selectedSessionId();
      if (!Array.isArray(sessions) || sessions.length === 0) {
        setSelectedSessionId('');
      } else if (!current || !sessions.some((session) => session.id === current)) {
        setSelectedSessionId(sessions[0].id);
      }
    } catch (error) {
      logger.error('[AISettings] Failed to load chat sessions:', error);
      setChatSessions([]);
      const message = error instanceof Error ? error.message : 'Failed to load chat sessions.';
      setChatSessionsError(message);
    } finally {
      setChatSessionsLoading(false);
    }
  };

  const selectedChatSession = createMemo(() => {
    const id = selectedSessionId();
    if (!id) return null;
    return chatSessions().find((session) => session.id === id) || null;
  });

  const formatSessionLabel = (session: ChatSession) => {
    const updatedAt = new Date(session.updated_at);
    const dateLabel = updatedAt.toLocaleDateString([], { month: 'short', day: 'numeric' });
    const timeLabel = updatedAt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    return `${session.title || 'Untitled'} - ${session.message_count} msgs - ${dateLabel} ${timeLabel}`;
  };

  const formatDiffStatus = (status: FileChange['status']) => {
    switch (status) {
      case 'added':
        return 'Added';
      case 'modified':
        return 'Modified';
      case 'deleted':
        return 'Deleted';
      default:
        return 'Changed';
    }
  };

  const diffStatusClasses = (status: FileChange['status']) => {
    switch (status) {
      case 'added':
        return 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-200';
      case 'modified':
        return 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200';
      case 'deleted':
        return 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200';
      default:
        return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200';
    }
  };

  const formatDiffStats = (change: FileChange) => `+${change.added} -${change.removed}`;

  const handleSessionSummarize = async () => {
    const sessionId = selectedSessionId();
    if (!sessionId) {
      notificationStore.info('Select a chat session first.');
      return;
    }

    setSessionActionLoading('summarize');
    try {
      await AIChatAPI.summarizeSession(sessionId);
      notificationStore.success('Session summarized.');
    } catch (error) {
      logger.error('[AISettings] Failed to summarize session:', error);
      const message = error instanceof Error ? error.message : 'Failed to summarize session.';
      notificationStore.error(message);
    } finally {
      setSessionActionLoading(null);
    }
  };

  const handleSessionDiff = async () => {
    const sessionId = selectedSessionId();
    if (!sessionId) {
      notificationStore.info('Select a chat session first.');
      return;
    }

    setSessionActionLoading('diff');
    try {
      const diff = await AIChatAPI.getSessionDiff(sessionId);
      const files = diff.files || [];
      if (files.length === 0) {
        setDiffFiles([]);
        setDiffSummary('');
        setShowDiffModal(false);
        notificationStore.info('No file changes in this session.');
        return;
      }
      setDiffFiles(files);
      setDiffSummary(diff.summary || '');
      const session = selectedChatSession();
      setDiffSessionLabel(session ? (session.title || 'Untitled session') : 'Selected session');
      setShowDiffModal(true);
    } catch (error) {
      logger.error('[AISettings] Failed to get session diff:', error);
      const message = error instanceof Error ? error.message : 'Failed to get session diff.';
      notificationStore.error(message);
    } finally {
      setSessionActionLoading(null);
    }
  };

  const handleSessionRevert = async () => {
    const sessionId = selectedSessionId();
    if (!sessionId) {
      notificationStore.info('Select a chat session first.');
      return;
    }
    if (!confirm('Revert all changes from this session? This cannot be undone.')) return;

    setSessionActionLoading('revert');
    try {
      await AIChatAPI.revertSession(sessionId);
      notificationStore.success('Session changes reverted.');
    } catch (error) {
      logger.error('[AISettings] Failed to revert session:', error);
      const message = error instanceof Error ? error.message : 'Failed to revert session.';
      notificationStore.error(message);
    } finally {
      setSessionActionLoading(null);
    }
  };

  const loadSettings = async () => {
    setLoading(true);
    try {
      const data = await AIAPI.getSettings();
      setSettings(data);
      resetForm(data);
      // Load available models after settings (needs API key to be configured)
      if (data.configured) {
        loadModels();
      }
      void runProviderPreflight(data);
    } catch (error) {
      logger.error('[AISettings] Failed to load settings:', error);
      notificationStore.error('Failed to load Pulse Assistant settings');
      setSettings(null);
      resetForm(null);
      setProviderHealth(createInitialProviderHealth());
      setPreflightLastCheckedAt(null);
    } finally {
      setLoading(false);
    }
  };

  const getProviderHealthBadgeClass = (provider: AIProvider): string => {
    switch (providerHealth[provider].status) {
      case 'ok':
        return 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300';
      case 'error':
        return 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300';
      case 'checking':
        return 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300';
      default:
        return 'bg-surface-hover text-muted';
    }
  };

  const getProviderHealthLabel = (provider: AIProvider): string => {
    switch (providerHealth[provider].status) {
      case 'ok':
        return 'Healthy';
      case 'error':
        return 'Issue';
      case 'checking':
        return 'Checking...';
      default:
        return 'Not checked';
    }
  };

  const checkProviderHealth = async (
    provider: AIProvider,
    opts: { notify?: boolean; storeManualResult?: boolean } = {}
  ): Promise<{ success: boolean; message: string; provider: string }> => {
    try {
      const result = await AIAPI.testProvider(provider);
      setProviderHealth(provider, {
        status: result.success ? 'ok' : 'error',
        message: result.message || '',
      });
      if (opts.storeManualResult) {
        setProviderTestResult(result);
      }
      if (opts.notify) {
        if (result.success) {
          notificationStore.success(`${provider}: ${result.message}`);
        } else {
          notificationStore.error(`${provider}: ${result.message}`);
        }
      }
      return result;
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Connection test failed';
      const result = { provider, success: false, message };
      setProviderHealth(provider, {
        status: 'error',
        message,
      });
      if (opts.storeManualResult) {
        setProviderTestResult(result);
      }
      if (opts.notify) {
        notificationStore.error(`${provider}: ${message}`);
      }
      return result;
    }
  };

  const runProviderPreflight = async (settingsSnapshot?: AISettingsType | null) => {
    const current = settingsSnapshot ?? settings();
    if (!current) {
      return;
    }

    const configuredProviders = AI_PROVIDERS.filter((provider) => isProviderConfigured(provider, current));
    for (const provider of AI_PROVIDERS) {
      if (!configuredProviders.includes(provider)) {
        setProviderHealth(provider, { status: 'not_configured', message: '' });
      }
    }
    if (configuredProviders.length === 0) {
      setPreflightLastCheckedAt(Date.now());
      return;
    }

    setPreflightRunning(true);
    try {
      await Promise.all(
        configuredProviders.map(async (provider) => {
          setProviderHealth(provider, { status: 'checking', message: '' });
          await checkProviderHealth(provider, { notify: false, storeManualResult: false });
        })
      );
      setPreflightLastCheckedAt(Date.now());
    } finally {
      setPreflightRunning(false);
    }
  };

  onMount(() => {
    loadLicenseStatus();
    loadSettings();
  });

  // Check for OAuth callback parameters in URL
  const params = new URLSearchParams(window.location.search);
  const oauthSuccess = params.get('ai_oauth_success');
  const oauthError = params.get('ai_oauth_error');

  if (oauthSuccess === 'true') {
    notificationStore.success('Successfully connected to Claude with your subscription!');
    // Clean up URL
    window.history.replaceState({}, '', window.location.pathname);
    // Reload settings to get updated OAuth status
    loadSettings();
  } else if (oauthError) {
    const errorMessages: Record<string, string> = {
      'missing_params': 'OAuth callback missing required parameters',
      'invalid_state': 'Invalid OAuth state - please try again',
      'token_exchange_failed': 'Failed to complete authentication with Claude',
      'save_failed': 'Failed to save OAuth credentials',
    };
    notificationStore.error(errorMessages[oauthError] || `OAuth error: ${oauthError}`);
    // Clean up URL
    window.history.replaceState({}, '', window.location.pathname);
  }

  // Note: handleProviderChange is no longer used as we now use multi-provider accordions
  // The provider is implicitly determined by the selected model (e.g., "anthropic:claude-opus")

  const handleSave = async (event?: Event) => {
    event?.preventDefault();

    // Frontend validation: warn if model's provider isn't configured
    const selectedModel = form.model.trim();
    if (selectedModel && form.enabled) {
      const modelProvider = getProviderFromModelId(selectedModel);
      if (!isProviderConfigured(modelProvider, settings())) {
        // Check if any API key is being added in this save for this provider
        const isAddingCredential =
          (modelProvider === 'anthropic' && form.anthropicApiKey.trim()) ||
          (modelProvider === 'openai' && form.openaiApiKey.trim()) ||
          (modelProvider === 'openrouter' && form.openrouterApiKey.trim()) ||
          (modelProvider === 'deepseek' && form.deepseekApiKey.trim()) ||
          (modelProvider === 'gemini' && form.geminiApiKey.trim()) ||
          (modelProvider === 'ollama' && form.ollamaBaseUrl.trim());

        if (!isAddingCredential) {
          notificationStore.error(
            `Cannot save: Model "${selectedModel}" requires ${PROVIDER_DISPLAY_NAMES[modelProvider] || modelProvider} to be configured. ` +
            `Please add an API key for ${PROVIDER_DISPLAY_NAMES[modelProvider] || modelProvider} or select a different model.`
          );
          return;
        }
      }
    }

    // Validate patrol interval (must be 0 or >= 10)
    if (form.patrolIntervalMinutes > 0 && form.patrolIntervalMinutes < 10) {
      notificationStore.error('Patrol interval must be at least 10 minutes (or 0 to disable)');
      return;
    }

    setSaving(true);
    try {
      const payload: Record<string, unknown> = {
        provider: form.provider,
        model: selectedModel,
      };

      // Only include base_url if it's set or if provider is ollama
      if (form.baseUrl.trim() || form.provider === 'ollama') {
        payload.base_url = form.baseUrl.trim();
      }

      // Handle API key
      if (form.apiKey.trim() !== '') {
        payload.api_key = form.apiKey.trim();
      } else if (form.clearApiKey) {
        payload.api_key = '';
      }

      // Only include enabled if we're toggling it
      if (form.enabled !== settings()?.enabled) {
        payload.enabled = form.enabled;
      }

      // Include patrol settings if changed
      if (form.patrolIntervalMinutes !== (settings()?.patrol_interval_minutes ?? 360)) {
        payload.patrol_interval_minutes = form.patrolIntervalMinutes;
      }

      if (form.alertTriggeredAnalysis !== settings()?.alert_triggered_analysis) {
        payload.alert_triggered_analysis = form.alertTriggeredAnalysis;
      }

      if (form.patrolAutoFix !== settings()?.patrol_auto_fix) {
        payload.patrol_auto_fix = form.patrolAutoFix;
      }

      // Include model overrides if changed
      if (form.chatModel !== (settings()?.chat_model || '')) {
        payload.chat_model = form.chatModel;
      }

      if (form.patrolModel !== (settings()?.patrol_model || '')) {
        payload.patrol_model = form.patrolModel;
      }

      if (form.autoFixModel !== (settings()?.auto_fix_model || '')) {
        payload.auto_fix_model = form.autoFixModel;
      }

      // Include multi-provider credentials if set (non-empty)
      if (form.anthropicApiKey.trim()) {
        payload.anthropic_api_key = form.anthropicApiKey.trim();
      }
      if (form.openaiApiKey.trim()) {
        payload.openai_api_key = form.openaiApiKey.trim();
      }
      if (form.openrouterApiKey.trim()) {
        payload.openrouter_api_key = form.openrouterApiKey.trim();
      }
      if (form.deepseekApiKey.trim()) {
        payload.deepseek_api_key = form.deepseekApiKey.trim();
      }
      if (form.geminiApiKey.trim()) {
        payload.gemini_api_key = form.geminiApiKey.trim();
      }
      // Always include Ollama URL if it has a value and differs from what's saved
      // Compare against actual saved value (empty string if not set), not a prefilled default
      if (form.ollamaBaseUrl.trim() && form.ollamaBaseUrl.trim() !== (settings()?.ollama_base_url || '')) {
        payload.ollama_base_url = form.ollamaBaseUrl.trim();
      }
      if (form.openaiBaseUrl !== (settings()?.openai_base_url || '')) {
        payload.openai_base_url = form.openaiBaseUrl.trim();
      }

      // Cost controls (server-side budget, cross-provider estimate)
      {
        const raw = form.costBudgetUSD30d.trim();
        const parsed = raw === '' ? 0 : Number(raw);
        if (!Number.isFinite(parsed) || parsed < 0) {
          notificationStore.error('Cost budget must be a non-negative number');
          return;
        }
        const current = settings()?.cost_budget_usd_30d ?? 0;
        if (Math.abs(parsed - current) > 0.0001) {
          payload.cost_budget_usd_30d = parsed;
        }
      }

      // Request timeout (for slow Ollama hardware)
      if (form.requestTimeoutSeconds !== (settings()?.request_timeout_seconds ?? 300)) {
        payload.request_timeout_seconds = form.requestTimeoutSeconds;
      }

      // Infrastructure control settings
      if (form.controlLevel !== (settings()?.control_level || 'read_only')) {
        payload.control_level = form.controlLevel;
      }

      // Protected guests (comma-separated string to array)
      const currentProtected = settings()?.protected_guests || [];
      const newProtected = form.protectedGuests
        .split(',')
        .map((s: string) => s.trim())
        .filter((s: string) => s.length > 0);
      const protectedChanged =
        newProtected.length !== currentProtected.length ||
        newProtected.some((g: string, i: number) => g !== currentProtected[i]);
      if (protectedChanged) {
        payload.protected_guests = newProtected;
      }

      // Discovery settings
      if (form.discoveryEnabled !== (settings()?.discovery_enabled ?? false)) {
        payload.discovery_enabled = form.discoveryEnabled;
      }
      if (form.discoveryIntervalHours !== (settings()?.discovery_interval_hours ?? 0)) {
        payload.discovery_interval_hours = form.discoveryIntervalHours;
      }

      const updated = await AIAPI.updateSettings(payload);
      setSettings(updated);
      resetForm(updated);
      void runProviderPreflight(updated);
      notificationStore.success('Pulse Assistant settings saved');
      // Notify other components (like AIChat) that settings changed so they can refresh models
      aiChatStore.notifySettingsChanged();
    } catch (error) {
      logger.error('[AISettings] Failed to save settings:', error);
      const message = error instanceof Error ? error.message : 'Failed to save Pulse Assistant settings';
      notificationStore.error(message);
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    try {
      const result = await AIAPI.testConnection();
      if (result.success) {
        notificationStore.success(result.message);
      } else {
        notificationStore.error(result.message);
      }
    } catch (error) {
      logger.error('[AISettings] Test failed:', error);
      const message = error instanceof Error ? error.message : 'Connection test failed';
      notificationStore.error(message);
    } finally {
      setTesting(false);
    }
  };

  const handleTestProvider = async (provider: string) => {
    const typedProvider = provider as AIProvider;
    setTestingProvider(typedProvider);
    setProviderTestResult(null);
    try {
      await checkProviderHealth(typedProvider, { notify: true, storeManualResult: true });
    } catch (error) {
      logger.error(`[AISettings] Test ${provider} failed:`, error);
      const message = error instanceof Error ? error.message : 'Connection test failed';
      setProviderTestResult({ provider, success: false, message });
      notificationStore.error(`${provider}: ${message}`);
    } finally {
      setTestingProvider(null);
    }
  };

  const handleClearProvider = async (provider: string) => {
    // Check if this is the last configured provider
    const s = settings();
    const configuredCount = [s?.anthropic_configured, s?.openai_configured, s?.openrouter_configured, s?.deepseek_configured, s?.gemini_configured, s?.ollama_configured].filter(Boolean).length;
    const isLastProvider = configuredCount === 1 && isProviderConfigured(provider, s);

    // Check if current model uses this provider
    const currentModel = form.model.trim();
    const modelUsesProvider = currentModel && getProviderFromModelId(currentModel) === provider;

    let confirmMessage = `Clear ${PROVIDER_DISPLAY_NAMES[provider] || provider} credentials?`;
    if (isLastProvider) {
      confirmMessage = `Warning: this is your only configured provider. Clearing it will disable Pulse Assistant until you configure another provider. Continue?`;
    } else if (modelUsesProvider) {
      confirmMessage = `Your current model uses ${PROVIDER_DISPLAY_NAMES[provider] || provider}. Clearing this will require selecting a different model. Continue?`;
    } else {
      confirmMessage += ` You'll need to re-enter credentials to use this provider.`;
    }

    if (!confirm(confirmMessage)) {
      return;
    }

    setSaving(true);
    try {
      const clearPayload: Record<string, boolean> = {};
      if (provider === 'anthropic') clearPayload.clear_anthropic_key = true;
      if (provider === 'openai') clearPayload.clear_openai_key = true;
      if (provider === 'openrouter') clearPayload.clear_openrouter_key = true;
      if (provider === 'deepseek') clearPayload.clear_deepseek_key = true;
      if (provider === 'gemini') clearPayload.clear_gemini_key = true;
      if (provider === 'ollama') clearPayload.clear_ollama_url = true;

      await AIAPI.updateSettings(clearPayload);

      // Reload settings to reflect the change
      const newSettings = await AIAPI.getSettings();
      setSettings(newSettings);
      void runProviderPreflight(newSettings);

      // Clear the local form field
      if (provider === 'anthropic') setForm('anthropicApiKey', '');
      if (provider === 'openai') setForm('openaiApiKey', '');
      if (provider === 'openrouter') setForm('openrouterApiKey', '');
      if (provider === 'deepseek') setForm('deepseekApiKey', '');
      if (provider === 'gemini') setForm('geminiApiKey', '');
      if (provider === 'ollama') setForm('ollamaBaseUrl', '');

      notificationStore.success(`${provider} credentials cleared`);
      // Notify other components (like AIChat) that settings changed
      aiChatStore.notifySettingsChanged();
    } catch (error) {
      logger.error(`[AISettings] Clear ${provider} failed:`, error);
      const message = error instanceof Error ? error.message : 'Failed to clear credentials';
      notificationStore.error(message);
    } finally {
      setSaving(false);
    }
  };

  // OAuth handlers removed - OAuth is currently unavailable from Anthropic for third-party apps
  // When OAuth becomes available, handlers can be added back to the Anthropic accordion section

  // Legacy helper functions removed - multi-provider accordions handle all provider-specific UI

  return (
    <>
      <SettingsPanel
        title="AI"
        description="Configure AI providers, models, Pulse Assistant, and Patrol."
        icon={
          <svg
            class="w-5 h-5 text-blue-600 dark:text-blue-300"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
              d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5"
            />
          </svg>
        }
        action={
          (() => {
            const s = settings();
            const hasConfiguredProvider = s && (s.anthropic_configured || s.openai_configured || s.openrouter_configured || s.deepseek_configured || s.gemini_configured || s.ollama_configured);

            return (
              <Toggle
                checked={form.enabled}
                onChange={async (event) => {
                  const newValue = event.currentTarget.checked;
                  // Show setup modal if trying to enable without a configured provider
                  if (newValue && !hasConfiguredProvider) {
                    event.currentTarget.checked = false;
                    setShowSetupModal(true);
                    return;
                  }
                  setForm('enabled', newValue);
                  // Auto-save the enabled toggle immediately
                  try {
                    const updated = await AIAPI.updateSettings({ enabled: newValue });
                    setSettings(updated);
                    void runProviderPreflight(updated);
                    notificationStore.success(newValue ? 'Pulse Assistant enabled' : 'Pulse Assistant disabled');
                    aiChatStore.notifySettingsChanged();
                  } catch (error) {
                    // Revert on failure
                    setForm('enabled', !newValue);
                    logger.error('[AISettings] Failed to toggle AI:', error);
                    const message = error instanceof Error ? error.message : 'Failed to update Pulse Assistant setting';
                    notificationStore.error(message);
                  }
                }}
                disabled={loading() || saving()}
                containerClass="items-center gap-2"
                label={
                  <span class="text-xs font-medium text-muted">
                    {form.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                }
              />
            );
          })()
        }
        noPadding
      >
        <form class="divide-y divide-border" onSubmit={handleSave}>
          <Show when={loading()}>
            <div class="flex items-center gap-3 text-sm text-muted p-4 sm:p-6">
              <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
              Loading Pulse Assistant settings...
            </div>
          </Show>

          <Show when={!loading()}>
            <Show when={form.enabled}>
              <div class="p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                <div class="flex items-start gap-2 text-xs text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                  <svg class="w-4 h-4 mt-0.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <span>
                    Patrol runs automatically every{' '}
                    {form.patrolIntervalMinutes >= 60
                      ? `${Math.round(form.patrolIntervalMinutes / 60)} hour${Math.round(form.patrolIntervalMinutes / 60) === 1 ? '' : 's'}`
                      : `${form.patrolIntervalMinutes} minute${form.patrolIntervalMinutes === 1 ? '' : 's'}`}{' '}
                    to monitor your infrastructure.{' '}
                    <button
                      type="button"
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm underline hover:text-blue-800 dark:hover:text-blue-300"
                      onClick={() => navigate('/ai')}
                    >
                      Configure schedule & autonomy
                    </button>
                  </span>
                </div>
              </div>
            </Show>
            <div class="space-y-6 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
              {/* Default Model Selection - Always visible */}
              <div class={formField}>
                <div class="flex items-center justify-between mb-1">
                  <label class={labelClass()}>
                    Default Model
                    {modelsLoading() && <span class="ml-2 text-xs text-slate-500">(loading...)</span>}
                  </label>
                  <button
                    type="button"
                    onClick={loadModels}
                    disabled={modelsLoading()}
                    class="inline-flex min-h-10 sm:min-h-9 items-center gap-1 rounded-md px-2 py-1.5 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
                    title="Refresh model list from all configured providers"
                  >
                    <svg class={`w-3 h-3 ${modelsLoading() ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                    Refresh
                  </button>
                </div>
                <Show when={availableModels().length > 0} fallback={
                  <input
                    type="text"
                    value={form.model}
                    onInput={(e) => setForm('model', e.currentTarget.value)}
                    placeholder="Configure a provider below to see available models"
                    class={controlClass()}
                    disabled={saving()}
                  />
                }>
                  <select
                    value={form.model}
                    onChange={(e) => setForm('model', e.currentTarget.value)}
                    class={controlClass()}
                    disabled={saving()}
                  >
                    <Show when={!form.model || !availableModels().some(m => m.id === form.model)}>
                      <option value={form.model}>{form.model || 'Select a model...'}</option>
                    </Show>
                    {/* Show configured providers first */}
                    <For each={Array.from(groupModelsByProvider(availableModels()).entries()).filter(([p]) => isProviderConfigured(p, settings()))}>
                      {([provider, models]) => (
                        <optgroup label={PROVIDER_DISPLAY_NAMES[provider] || provider}>
                          <For each={models}>
                            {(model) => (
                              <option value={model.id} selected={model.id === form.model}>
                                {model.name || model.id.split(':').pop()}
                              </option>
                            )}
                          </For>
                        </optgroup>
                      )}
                    </For>
                    {/* Show unconfigured providers in a separate section with warning */}
                    <For each={Array.from(groupModelsByProvider(availableModels()).entries()).filter(([p]) => !isProviderConfigured(p, settings()))}>
                      {([provider, models]) => (
                        <optgroup label={`${PROVIDER_DISPLAY_NAMES[provider] || provider} (not configured)`}>
                          <For each={models}>
                            {(model) => (
                              <option value={model.id} selected={model.id === form.model} class="text-slate-400">
                                {model.name || model.id.split(':').pop()}
                              </option>
                            )}
                          </For>
                        </optgroup>
                      )}
                    </For>
                  </select>
                </Show>
                {/* Warning if selected model's provider is not configured */}
                <Show when={form.model && !isModelProviderConfigured(form.model, settings())}>
                  <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 flex items-center gap-1">
                    <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                    This model requires {PROVIDER_DISPLAY_NAMES[getProviderFromModelId(form.model)] || getProviderFromModelId(form.model)} to be configured.
                    Add an API key below or select a different model.
                  </p>
                </Show>
              </div>

              {/* Advanced Model Selection - Collapsible */}
              <div class="border border-border rounded-md overflow-hidden">
                <button
                  type="button"
                  class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface-alt hover:bg-surface-hover transition-colors text-left"
                  onClick={() => setShowAdvancedModels(!showAdvancedModels())}
                >
                  <div class="flex items-center gap-2">
                    <svg class="w-4 h-4 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                    </svg>
                    <span class="text-sm font-medium text-base-content">Advanced Model Selection</span>
                    <Show when={form.chatModel || form.patrolModel}>
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Customized</span>
                    </Show>
                  </div>
                  <svg class={`w-4 h-4 text-slate-500 transition-transform ${showAdvancedModels() ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showAdvancedModels()}>
                  <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
                    <p class="text-xs text-muted">
                      Override the default model for specific tasks. Leave empty to use the default.
                    </p>
                    {/* Chat Model */}
                    <div>
                      <label class="block text-xs font-medium text-muted mb-0.5">Chat Model (Interactive)</label>
                      <p class="text-[11px] text-muted mb-1">Used for chat and fix execution — a more capable model is recommended.</p>
                      <Show when={availableModels().length > 0} fallback={
                        <input
                          type="text"
                          value={form.chatModel}
                          onInput={(e) => setForm('chatModel', e.currentTarget.value)}
                          placeholder="Use default model"
                          class={controlClass()}
                          disabled={saving()}
                        />
                      }>
                        <select
                          value={form.chatModel}
                          onChange={(e) => setForm('chatModel', e.currentTarget.value)}
                          class={controlClass()}
                          disabled={saving()}
                        >
                          <option value="">Use default ({form.model?.split(':').pop() || 'not set'})</option>
                          <For each={Array.from(groupModelsByProvider(availableModels()).entries())}>
                            {([provider, models]) => (
                              <optgroup label={PROVIDER_DISPLAY_NAMES[provider] || provider}>
                                <For each={models}>
                                  {(model) => (
                                    <option value={model.id}>
                                      {model.name || model.id.split(':').pop()}
                                    </option>
                                  )}
                                </For>
                              </optgroup>
                            )}
                          </For>
                        </select>
                      </Show>
                    </div>
                    {/* Patrol Model */}
                    <div>
                      <label class="block text-xs font-medium text-muted mb-0.5">Patrol Model (Background)</label>
                      <p class="text-[11px] text-muted mb-1">Runs frequently for detection — a smaller, cheaper model keeps costs low.</p>
                      <Show when={availableModels().length > 0} fallback={
                        <input
                          type="text"
                          value={form.patrolModel}
                          onInput={(e) => setForm('patrolModel', e.currentTarget.value)}
                          placeholder="Use default model"
                          class={controlClass()}
                          disabled={saving()}
                        />
                      }>
                        <select
                          value={form.patrolModel}
                          onChange={(e) => setForm('patrolModel', e.currentTarget.value)}
                          class={controlClass()}
                          disabled={saving()}
                        >
                          <option value="">Use default ({form.model?.split(':').pop() || 'not set'})</option>
                          <For each={Array.from(groupModelsByProvider(availableModels()).entries())}>
                            {([provider, models]) => (
                              <optgroup label={PROVIDER_DISPLAY_NAMES[provider] || provider}>
                                <For each={models}>
                                  {(model) => (
                                    <option value={model.id}>
                                      {model.name || model.id.split(':').pop()}
                                    </option>
                                  )}
                                </For>
                              </optgroup>
                            )}
                          </For>
                        </select>
                      </Show>
                    </div>
                  </div>
                </Show>
              </div>

              {/* Provider Configuration - Configure API keys for all providers */}
              <div class={`${formField} p-5 rounded-md border border-border bg-surface-alt`}>
                <div class="mb-3 space-y-1.5">
                  <div class="flex items-center justify-between gap-2">
                    <h4 class="font-medium text-base-content flex items-center gap-2">
                      <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                      </svg>
                      Provider Configuration
                    </h4>
                    <button
                      type="button"
                      onClick={() => void runProviderPreflight()}
                      disabled={preflightRunning() || saving()}
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                    >
                      {preflightRunning() ? 'Checking...' : 'Run Preflight'}
                    </button>
                  </div>
                  <p class="text-xs text-muted mt-1">
                    Configure API keys for each provider you want to use. Models from all configured providers will appear in the model selectors.
                  </p>
                  <Show when={preflightLastCheckedAt()}>
                    <p class="text-[11px] text-muted">
                      Last checked: {new Date(preflightLastCheckedAt()!).toLocaleTimeString()}
                    </p>
                  </Show>
                  <Show when={providerIssueCount() > 0}>
                    <div class="rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900 px-2 py-1.5">
                      <p class="text-xs text-red-700 dark:text-red-300">
                        {providerIssueCount()} provider{providerIssueCount() === 1 ? '' : 's'} configured but currently not usable.
                      </p>
                      <For each={AI_PROVIDERS.filter((provider) => providerHealth[provider].status === 'error')}>
                        {(provider) => (
                          <p class="text-[11px] text-red-600 dark:text-red-300">
                            <span class="font-medium">{PROVIDER_DISPLAY_NAMES[provider] || provider}:</span> {providerHealth[provider].message}
                          </p>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>

                {/* Provider Accordions */}
                <div class="space-y-2">
                  {/* Anthropic */}
                  <div class={`border rounded-md overflow-hidden ${settings()?.anthropic_configured ? 'border-green-300 dark:border-green-700' : 'border-border'}`}>
                    <button
                      type="button"
                      class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                      onClick={() => {
                        const current = expandedProviders();
                        const next = new Set(current);
                        if (next.has('anthropic')) next.delete('anthropic');
                        else next.add('anthropic');
                        setExpandedProviders(next);
                      }}
                    >
                      <div class="flex items-center gap-2">
                        <span class="font-medium text-sm">Anthropic</span>
                        <Show when={settings()?.anthropic_configured}>
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Configured</span>
                        </Show>
                        <Show when={settings()?.anthropic_configured}>
                          <span class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getProviderHealthBadgeClass('anthropic')}`}>
                            {getProviderHealthLabel('anthropic')}
                          </span>
                        </Show>
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('anthropic') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('anthropic')}>
                      <div class="px-3 py-3 bg-surface-alt border-t border-border space-y-2">
                        <input
                          type="password"
                          value={form.anthropicApiKey}
                          onInput={(e) => setForm('anthropicApiKey', e.currentTarget.value)}
                          placeholder={settings()?.anthropic_configured ? '••••••••••• (configured)' : 'sk-ant-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-slate-500">
                            <a href="https://console.anthropic.com/settings/keys" target="_blank" rel="noopener" class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.anthropic_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('anthropic')}
                                disabled={testingProvider() === 'anthropic' || saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'anthropic' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('anthropic')}
                                disabled={saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                                title="Clear API key"
                              >
                                Clear
                              </button>
                            </div>
                          </Show>
                        </div>
                        <Show when={providerTestResult()?.provider === 'anthropic'}>
                          <p class={`text-xs ${providerTestResult()?.success ? 'text-green-600' : 'text-red-600'}`}>
                            {providerTestResult()?.message}
                          </p>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* OpenAI */}
                  <div class={`border rounded-md overflow-hidden ${settings()?.openai_configured ? 'border-green-300 dark:border-green-700' : 'border-border'}`}>
                    <button
                      type="button"
                      class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                      onClick={() => {
                        const current = expandedProviders();
                        const next = new Set(current);
                        if (next.has('openai')) next.delete('openai');
                        else next.add('openai');
                        setExpandedProviders(next);
                      }}
                    >
                      <div class="flex items-center gap-2">
                        <span class="font-medium text-sm">OpenAI</span>
                        <Show when={settings()?.openai_configured}>
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Configured</span>
                        </Show>
                        <Show when={settings()?.openai_configured}>
                          <span class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getProviderHealthBadgeClass('openai')}`}>
                            {getProviderHealthLabel('openai')}
                          </span>
                        </Show>
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('openai') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('openai')}>
                      <div class="px-3 py-3 bg-surface-alt border-t border-border space-y-2">
                        <input
                          type="password"
                          value={form.openaiApiKey}
                          onInput={(e) => setForm('openaiApiKey', e.currentTarget.value)}
                          placeholder={settings()?.openai_configured ? '••••••••••• (configured)' : 'sk-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="space-y-1">
                          <label class="text-xs text-muted inline-flex items-center gap-1">
                            Custom Base URL
                            <HelpIcon contentId="ai.openai.baseUrl" size="xs" />
                          </label>
                          <input
                            type="url"
                            value={form.openaiBaseUrl}
                            onInput={(e) => setForm('openaiBaseUrl', e.currentTarget.value)}
                            placeholder="https://api.together.xyz/v1 (optional)"
                            class={controlClass()}
                            disabled={saving()}
                          />
                        </div>
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-slate-500">
                            <a href="https://platform.openai.com/api-keys" target="_blank" rel="noopener" class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.openai_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('openai')}
                                disabled={testingProvider() === 'openai' || saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'openai' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('openai')}
                                disabled={saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                                title="Clear API key"
                              >
                                Clear
                              </button>
                            </div>
                          </Show>
                        </div>
                        <Show when={providerTestResult()?.provider === 'openai'}>
                          <p class={`text-xs ${providerTestResult()?.success ? 'text-green-600' : 'text-red-600'}`}>
                            {providerTestResult()?.message}
                          </p>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* OpenRouter */}
                  <div class={`border rounded-md overflow-hidden ${settings()?.openrouter_configured ? 'border-green-300 dark:border-green-700' : 'border-border'}`}>
                    <button
                      type="button"
                      class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                      onClick={() => {
                        const current = expandedProviders();
                        const next = new Set(current);
                        if (next.has('openrouter')) next.delete('openrouter');
                        else next.add('openrouter');
                        setExpandedProviders(next);
                      }}
                    >
                      <div class="flex items-center gap-2">
                        <span class="font-medium text-sm">OpenRouter</span>
                        <Show when={settings()?.openrouter_configured}>
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Configured</span>
                        </Show>
                        <Show when={settings()?.openrouter_configured}>
                          <span class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getProviderHealthBadgeClass('openrouter')}`}>
                            {getProviderHealthLabel('openrouter')}
                          </span>
                        </Show>
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('openrouter') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('openrouter')}>
                      <div class="px-3 py-3 bg-surface-alt border-t border-border space-y-2">
                        <input
                          type="password"
                          value={form.openrouterApiKey}
                          onInput={(e) => setForm('openrouterApiKey', e.currentTarget.value)}
                          placeholder={settings()?.openrouter_configured ? '••••••••••• (configured)' : 'sk-or-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <p class="text-xs text-slate-500">
                          Uses <code>https://openrouter.ai/api/v1</code> automatically.
                        </p>
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-slate-500">
                            <a href="https://openrouter.ai/keys" target="_blank" rel="noopener" class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.openrouter_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('openrouter')}
                                disabled={testingProvider() === 'openrouter' || saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'openrouter' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('openrouter')}
                                disabled={saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                                title="Clear API key"
                              >
                                Clear
                              </button>
                            </div>
                          </Show>
                        </div>
                        <Show when={providerTestResult()?.provider === 'openrouter'}>
                          <p class={`text-xs ${providerTestResult()?.success ? 'text-green-600' : 'text-red-600'}`}>
                            {providerTestResult()?.message}
                          </p>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* DeepSeek */}
                  <div class={`border rounded-md overflow-hidden ${settings()?.deepseek_configured ? 'border-green-300 dark:border-green-700' : 'border-border'}`}>
                    <button
                      type="button"
                      class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                      onClick={() => {
                        const current = expandedProviders();
                        const next = new Set(current);
                        if (next.has('deepseek')) next.delete('deepseek');
                        else next.add('deepseek');
                        setExpandedProviders(next);
                      }}
                    >
                      <div class="flex items-center gap-2">
                        <span class="font-medium text-sm">DeepSeek</span>
                        <Show when={settings()?.deepseek_configured}>
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Configured</span>
                        </Show>
                        <Show when={settings()?.deepseek_configured}>
                          <span class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getProviderHealthBadgeClass('deepseek')}`}>
                            {getProviderHealthLabel('deepseek')}
                          </span>
                        </Show>
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('deepseek') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('deepseek')}>
                      <div class="px-3 py-3 bg-surface-alt border-t border-border space-y-2">
                        <input
                          type="password"
                          value={form.deepseekApiKey}
                          onInput={(e) => setForm('deepseekApiKey', e.currentTarget.value)}
                          placeholder={settings()?.deepseek_configured ? '••••••••••• (configured)' : 'sk-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-slate-500">
                            <a href="https://platform.deepseek.com/api_keys" target="_blank" rel="noopener" class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.deepseek_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('deepseek')}
                                disabled={testingProvider() === 'deepseek' || saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'deepseek' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('deepseek')}
                                disabled={saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                                title="Clear API key"
                              >
                                Clear
                              </button>
                            </div>
                          </Show>
                        </div>
                        <Show when={providerTestResult()?.provider === 'deepseek'}>
                          <p class={`text-xs ${providerTestResult()?.success ? 'text-green-600' : 'text-red-600'}`}>
                            {providerTestResult()?.message}
                          </p>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* Google Gemini */}
                  <div class={`border rounded-md overflow-hidden ${settings()?.gemini_configured ? 'border-green-300 dark:border-green-700' : 'border-border'}`}>
                    <button
                      type="button"
                      class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                      onClick={() => {
                        const current = expandedProviders();
                        const next = new Set(current);
                        if (next.has('gemini')) next.delete('gemini');
                        else next.add('gemini');
                        setExpandedProviders(next);
                      }}
                    >
                      <div class="flex items-center gap-2">
                        <span class="font-medium text-sm">Google Gemini</span>
                        <Show when={settings()?.gemini_configured}>
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Configured</span>
                        </Show>
                        <Show when={settings()?.gemini_configured}>
                          <span class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getProviderHealthBadgeClass('gemini')}`}>
                            {getProviderHealthLabel('gemini')}
                          </span>
                        </Show>
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('gemini') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('gemini')}>
                      <div class="px-3 py-3 bg-surface-alt border-t border-border space-y-2">
                        <input
                          type="password"
                          value={form.geminiApiKey}
                          onInput={(e) => setForm('geminiApiKey', e.currentTarget.value)}
                          placeholder={settings()?.gemini_configured ? '••••••••••• (configured)' : 'AIza...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-slate-500">
                            <a href="https://aistudio.google.com/app/apikey" target="_blank" rel="noopener" class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.gemini_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('gemini')}
                                disabled={testingProvider() === 'gemini' || saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'gemini' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('gemini')}
                                disabled={saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                                title="Clear API key"
                              >
                                Clear
                              </button>
                            </div>
                          </Show>
                        </div>
                        <Show when={providerTestResult()?.provider === 'gemini'}>
                          <p class={`text-xs ${providerTestResult()?.success ? 'text-green-600' : 'text-red-600'}`}>
                            {providerTestResult()?.message}
                          </p>
                        </Show>
                      </div>
                    </Show>
                  </div>

                  {/* Ollama */}
                  <div class={`border rounded-md overflow-hidden ${settings()?.ollama_configured ? 'border-green-300 dark:border-green-700' : 'border-border'}`}>
                    <button
                      type="button"
                      class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface hover:bg-surface-hover transition-colors"
                      onClick={() => {
                        const current = expandedProviders();
                        const next = new Set(current);
                        if (next.has('ollama')) next.delete('ollama');
                        else next.add('ollama');
                        setExpandedProviders(next);
                      }}
                    >
                      <div class="flex items-center gap-2">
                        <span class="font-medium text-sm">Ollama</span>
                        <Show when={settings()?.ollama_configured}>
                          <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Available</span>
                        </Show>
                        <Show when={settings()?.ollama_configured}>
                          <span class={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${getProviderHealthBadgeClass('ollama')}`}>
                            {getProviderHealthLabel('ollama')}
                          </span>
                        </Show>
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('ollama') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('ollama')}>
                      <div class="px-3 py-3 bg-surface-alt border-t border-border space-y-2">
                        <div class="space-y-1">
                          <label class="text-xs text-muted inline-flex items-center gap-1">
                            Server URL
                            <HelpIcon contentId="ai.ollama.baseUrl" size="xs" />
                          </label>
                          <input
                            type="url"
                            value={form.ollamaBaseUrl}
                            onInput={(e) => setForm('ollamaBaseUrl', e.currentTarget.value)}
                            placeholder="http://localhost:11434"
                            class={controlClass()}
                            disabled={saving()}
                          />
                        </div>
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-slate-500">
                            <a href="https://ollama.ai" target="_blank" rel="noopener" class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm text-blue-600 dark:text-blue-400 hover:underline">Learn about Ollama →</a>
                            <span class="text-slate-400"> · Free & local</span>
                          </p>
                          <Show when={settings()?.ollama_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('ollama')}
                                disabled={testingProvider() === 'ollama' || saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'ollama' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('ollama')}
                                disabled={saving()}
                                class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-3 py-2 text-sm bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
                                title="Clear Ollama URL"
                              >
                                Clear
                              </button>
                            </div>
                          </Show>
                        </div>
                        <Show when={providerTestResult()?.provider === 'ollama'}>
                          <p class={`text-xs ${providerTestResult()?.success ? 'text-green-600' : 'text-red-600'}`}>
                            {providerTestResult()?.message}
                          </p>
                        </Show>
                      </div>
                    </Show>
                  </div>
                </div>
              </div>

              {/* Discovery Settings - Collapsible */}
              <div class="rounded-md border border-blue-200 dark:border-blue-800 overflow-hidden">
                <button
                  type="button"
                  class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-blue-50 dark:bg-blue-900 hover:bg-blue-100 dark:hover:bg-blue-900 transition-colors text-left"
                  onClick={() => setShowDiscoverySettings(!showDiscoverySettings())}
                >
                  <div class="flex items-center gap-2">
                    <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                    <span class="text-sm font-medium text-base-content">Discovery Settings</span>
                    {/* Summary badges */}
                    <Show when={form.discoveryEnabled}>
                      <span class="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">
                        {form.discoveryIntervalHours > 0 ? `${form.discoveryIntervalHours}h` : 'Manual'}
 </span>
 </Show>
 <Show when={!form.discoveryEnabled}>
 <span class="px-1.5 py-0.5 text-[10px] font-medium bg-surface-hover text-muted rounded">Off</span>
 </Show>
 </div>
 <svg class={`w-4 h-4 transition-transform ${showDiscoverySettings() ?'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showDiscoverySettings()}>
                  <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
                    {/* Discovery Enabled Toggle */}
                    <div class="flex items-center justify-between gap-2">
                      <label class="text-xs font-medium text-muted flex items-center gap-1.5">
                        Enable Discovery
                        <HelpIcon inline={{ title: "What is Discovery?", description: "Discovery scans your VMs, containers, and Docker hosts to identify what services are running (databases, web servers, etc.), their versions, and how to access them. This information helps Pulse AI give you accurate troubleshooting commands and understand your infrastructure." }} size="xs" />
                      </label>
                      <Toggle
                        checked={form.discoveryEnabled}
                        onChange={(event) => setForm('discoveryEnabled', event.currentTarget.checked)}
                        disabled={saving()}
                      />
                    </div>

                    {/* Discovery Interval - Only when enabled */}
                    <Show when={form.discoveryEnabled}>
                      <div class="flex flex-col gap-1">
                        <div class="flex items-center gap-3">
                          <label class="text-xs font-medium text-muted w-32 flex-shrink-0">Scan Interval</label>
                          <select
                            class="flex-1 px-2 py-1 text-sm border border-border rounded bg-white dark:bg-slate-700"
                            value={form.discoveryIntervalHours}
                            onChange={(e) => setForm('discoveryIntervalHours', parseInt(e.currentTarget.value, 10))}
                            disabled={saving()}
                          >
                            <option value={0}>Manual only</option>
                            <option value={6}>Every 6 hours</option>
                            <option value={12}>Every 12 hours</option>
                            <option value={24}>Every 24 hours</option>
                            <option value={48}>Every 2 days</option>
                            <option value={168}>Every 7 days</option>
                          </select>
                        </div>
                        <p class="text-[10px] text-muted ml-32 pl-3">
                          {form.discoveryIntervalHours === 0
                            ? 'Discovery runs only when you click "Update Discovery" on a resource'
                            : 'Discovery will automatically re-scan resources at this interval'}
                        </p>
                      </div>
                    </Show>

                    <p class="text-[10px] text-muted">
                      Discovery gives Pulse AI workload context, so responses can reference concrete services and commands instead of generic advice.
                    </p>
                  </div>
                </Show>
              </div>

              {/* Usage Cost Controls - Compact */}
              <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
                <svg class="w-4 h-4 text-muted flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <label class="text-xs font-medium text-base-content">30-day Budget</label>
                <div class="relative flex-shrink-0">
                  <span class="absolute left-2 top-1/2 -translate-y-1/2 text-muted text-xs">$</span>
                  <input
                    type="number"
                    class="w-24 min-h-10 sm:min-h-9 pl-5 pr-2 py-2 text-sm border border-border rounded bg-surface"
                    value={form.costBudgetUSD30d}
                    onInput={(e) => setForm('costBudgetUSD30d', e.currentTarget.value)}
 min={0}
 step={1}
 placeholder="0"
 disabled={saving()}
 />
 </div>
 <Show when={parseFloat(form.costBudgetUSD30d) > 0}>
 <span class="text-xs ">≈ ${(parseFloat(form.costBudgetUSD30d) / 30).toFixed(2)}/day</span>
 </Show>
 <Show when={!form.costBudgetUSD30d || parseFloat(form.costBudgetUSD30d) === 0}>
 <span class="text-[10px] text-muted">Set a budget to receive usage alerts</span>
 </Show>
 </div>

 {/* Request Timeout - For slow Ollama hardware */}
 <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
 <svg class="w-4 h-4 text-muted flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
 </svg>
 <label class="text-xs font-medium text-base-content">Request Timeout</label>
 <input
 type="number"
 class="w-20 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded bg-surface"
 value={form.requestTimeoutSeconds}
 onInput={(e) => {
 const value = parseInt(e.currentTarget.value, 10);
 if (!isNaN(value) && value > 0) setForm('requestTimeoutSeconds', value);
 }}
 min={30}
 max={3600}
 step={30}
 disabled={saving()}
 />
 <span class="text-xs ">seconds</span>
 <Show when={form.requestTimeoutSeconds !== 300}>
 <span class="text-[10px] text-blue-600 dark:text-blue-400">Custom</span>
 </Show>
 <Show when={form.requestTimeoutSeconds === 300}>
 <span class="text-[10px] text-muted">default</span>
 </Show>
 </div>
 <p class="text-[10px] text-muted -mt-4 ml-1">
 Increase for slower Ollama hardware (default: 300s / 5 min)
 </p>

 {/* Pulse Permission Level */}
 <div class={`space-y-3 p-4 rounded-md border ${form.controlLevel ==='autonomous' ? 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900' : 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900'}`}>
                <div class="flex items-center gap-2">
                  <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                  </svg>
                  <span class="text-sm font-medium text-base-content">Pulse Permission Level</span>
                  <Show when={form.controlLevel !== 'read_only'}>
                    <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${form.controlLevel === 'autonomous'
 ? 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300'
 : form.controlLevel === 'controlled'
 ? 'bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300'
 : 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300'
 }`}>
                      {form.controlLevel}
                    </span>
                  </Show>
                </div>

                {/* Permission Level */}
                <div class="flex items-center gap-3">
                  <label class="text-xs font-medium text-muted w-28 flex-shrink-0">Permission</label>
                  <select
                    value={form.controlLevel}
                    onChange={(e) => setForm('controlLevel', e.currentTarget.value as ControlLevel)}
                    class="flex-1 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded bg-white dark:bg-slate-700"
                    disabled={saving()}
                  >
                    <option value="read_only">Read Only - Pulse Assistant can only observe</option>
                    <option value="controlled">Controlled - Pulse Assistant executes with your approval</option>
                    <option value="autonomous">Autonomous - Pulse Assistant executes without approval (Pro)</option>
                  </select>
                </div>
                <p class="text-[10px] text-muted ml-[7.5rem]">
                  {form.controlLevel === 'read_only' && 'Read-only mode: Pulse Assistant can query and observe only.'}
                  {form.controlLevel === 'controlled' && 'Controlled mode: Pulse Assistant can execute commands and control VMs/containers with approval.'}
                  {form.controlLevel === 'autonomous' && 'Autonomous mode: Pulse Assistant executes commands and control actions without confirmation.'}
                </p>
                <Show when={form.controlLevel === 'autonomous'}>
                  <div class="p-2 bg-amber-100 dark:bg-amber-900 rounded border border-amber-200 dark:border-amber-800 text-[10px] text-amber-800 dark:text-amber-200">
                    <strong>Legal Disclaimer:</strong> Model-driven systems can hallucinate. You are responsible for any damage caused by autonomous actions. See <a href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 underline">Terms of Service</a>.
                  </div>
                </Show>
                <Show when={form.controlLevel === 'autonomous' && autoFixLocked()}>
                  <p class="text-xs text-muted">
                    <a
                      class="text-blue-600 dark:text-blue-400 font-medium hover:underline"
                      href={getUpgradeActionUrlOrFallback('ai_autofix')}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={() => trackUpgradeClicked('settings_ai_autonomous_mode', 'ai_autofix')}
                    >
                      Upgrade to Pro
                    </a>{' '}
                    to enable autonomous mode.
                  </p>
                </Show>

                {/* Protected Guests - Only show if control is enabled */}
                <Show when={form.controlLevel !== 'read_only'}>
                  <div class="flex items-start gap-3 pt-2 border-t border-blue-200 dark:border-blue-700">
                    <label class="text-xs font-medium text-muted w-28 flex-shrink-0 pt-1">Protected</label>
                    <div class="flex-1">
                      <input
                        type="text"
                        value={form.protectedGuests}
                        onInput={(e) => setForm('protectedGuests', e.currentTarget.value)}
 placeholder="e.g., 100, 101, prod-db"
 class="w-full min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded "
 disabled={saving()}
 />
 <p class="text-[10px] text-muted mt-1">
 Comma-separated VMIDs or names that Pulse Assistant cannot control
 </p>
 </div>
 </div>
 </Show>
 </div>

 {/* Chat session maintenance */}
 <div class="border border-border rounded-md overflow-hidden">
 <button
 type="button"
 class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface-alt hover:bg-surface-hover transition-colors text-left"
 onClick={() => {
 const next = !showChatMaintenance();
 setShowChatMaintenance(next);
 if (next) {
 loadChatSessions();
 }
 }}
 >
 <div class="flex items-center gap-2">
 <svg class="w-4 h-4 " fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
 </svg>
 <span class="text-sm font-medium text-base-content">Chat Session Maintenance</span>
 </div>
 <svg class={`w-4 h-4 transition-transform ${showChatMaintenance() ?'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showChatMaintenance()}>
                  <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
                    <p class="text-xs text-muted">
                      Use this panel to summarize, inspect, or revert a specific chat session. It does not change your default Pulse Assistant settings.
                    </p>
                    <div class="flex items-center justify-between">
                      <label class="text-xs font-medium text-muted">Session</label>
                      <button
                        type="button"
                        onClick={loadChatSessions}
                        disabled={chatSessionsLoading()}
                        class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2 py-1.5 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
                      >
                        {chatSessionsLoading() ? 'Refreshing...' : 'Refresh'}
 </button>
 </div>

 <Show when={chatSessionsLoading()}>
 <div class="text-xs text-muted">Loading chat sessions...</div>
 </Show>
 <Show when={!chatSessionsLoading()}>
 <Show when={chatSessionsError()}>
 <div class="text-xs text-red-500">{chatSessionsError()}</div>
 </Show>
 <Show when={!chatSessionsError()}>
 <Show
 when={chatSessions().length > 0}
 fallback={
 <div class="text-xs text-muted">
 No chat sessions yet. Start a chat to create one.
 </div>
 }
 >
 <select
 value={selectedSessionId()}
 onChange={(e) => setSelectedSessionId(e.currentTarget.value)}
 class="w-full min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded "
 disabled={saving()}
 >
 <For each={chatSessions()}>
 {(session) => (
 <option value={session.id}>
 {formatSessionLabel(session)}
 </option>
 )}
 </For>
 </select>
 <Show when={selectedChatSession()}>
 <p class="text-[10px] text-muted mt-1">
 Last updated {new Date(selectedChatSession()!.updated_at).toLocaleString()}
 </p>
 </Show>
 </Show>
 </Show>
 </Show>

 <div class="flex flex-wrap gap-2 pt-1">
 <button
 type="button"
 onClick={handleSessionSummarize}
 disabled={!selectedSessionId() || sessionActionLoading() !== null}
 class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded border border-border bg-surface text-base-content hover:bg-surface-hover disabled:opacity-50"
 >
 {sessionActionLoading() ==='summarize' ? 'Summarizing...' : 'Summarize context'}
                      </button>
                      <button
                        type="button"
                        onClick={handleSessionDiff}
                        disabled={!selectedSessionId() || sessionActionLoading() !== null}
                        class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded border border-border bg-surface text-base-content hover:bg-surface-hover disabled:opacity-50"
                      >
                        {sessionActionLoading() === 'diff' ? 'Loading...' : 'View file changes'}
                      </button>
                      <button
                        type="button"
                        onClick={handleSessionRevert}
                        disabled={!selectedSessionId() || sessionActionLoading() !== null}
                        class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded border border-red-200 dark:border-red-700 bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-100 dark:hover:bg-red-900 disabled:opacity-50"
                      >
                        {sessionActionLoading() === 'revert' ? 'Reverting...' : 'Revert changes'}
                      </button>
                    </div>
                    <p class="text-[10px] text-muted">
                      These actions apply to the selected chat session only.
                    </p>
                  </div>
                </Show>
              </div>

            </div>

            {/* Status indicator */}
            <Show when={settings()}>
              <div class="p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                <div
                  class={`flex items-center gap-2 p-3 rounded-md ${settings()?.configured
 ? 'bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200'
 : 'bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200'
 }`}
                >
                  <div
                    class={`w-2 h-2 rounded-full ${settings()?.configured ? 'bg-emerald-400' : 'bg-amber-400'
 }`}
                  />
                  <div class="flex-1 min-w-0">
                    <span class="text-xs font-medium">
                      {settings()?.configured
                        ? `Ready • ${settings()?.configured_providers?.length || 0} provider${(settings()?.configured_providers?.length || 0) !== 1 ? 's' : ''} • ${availableModels().length} models`
                        : 'Configure at least one provider above to enable Pulse Assistant features'}
                    </span>
                    <Show when={settings()?.configured && settings()?.model}>
                      <span class="block sm:inline text-xs opacity-75 sm:ml-2">
                        • Default: {settings()?.model?.split(':').pop() || settings()?.model}
                      </span>
                    </Show>
                  </div>
                </div>
              </div>
            </Show>

            {/* Actions - sticky at bottom for easy access */}
            <div class="sticky bottom-0 bg-surface px-4 sm:px-6 py-4 flex flex-col sm:flex-row sm:flex-wrap sm:items-center sm:justify-between gap-3">
              <Show when={settings()?.api_key_set || settings()?.oauth_connected}>
                <button
                  type="button"
                  class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 text-sm border border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900 disabled:opacity-50 disabled:cursor-not-allowed"
                  onClick={handleTest}
                  disabled={testing() || saving() || loading()}
                >
                  {testing() ? 'Testing...' : 'Test Connection'}
                </button>
              </Show>
              <div class="grid grid-cols-1 sm:flex gap-3 w-full sm:w-auto sm:ml-auto">
                <button
                  type="button"
                  class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 border border-border text-base-content rounded-md hover:bg-surface-hover"
                  onClick={() => resetForm(settings())}
                  disabled={saving() || loading()}
                >
                  Reset
                </button>
                <button
                  type="submit"
                  class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={saving() || loading()}
                >
                  {saving() ? 'Saving...' : 'Save changes'}
                </button>
              </div>
            </div>
          </Show>
        </form>
      </SettingsPanel>

      {/* Session Diff Modal */}
      <Show when={showDiffModal()}>
        <div
          class="fixed inset-0 z-50 flex items-center justify-center bg-black p-4"
          onClick={() => setShowDiffModal(false)}
        >
          <div
            class="bg-surface rounded-md shadow-sm max-w-2xl w-full overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div class="flex items-start justify-between gap-4 px-6 py-4 border-b border-border">
              <div>
                <h3 class="text-lg font-semibold text-base-content">Session File Changes</h3>
                <p class="text-xs text-muted">
                  {diffSessionLabel() || 'Selected session'}
                </p>
              </div>
              <button
                type="button"
                class="text-sm text-muted hover:text-base-content"
                onClick={() => setShowDiffModal(false)}
              >
                Close
              </button>
            </div>
            <div class="p-6 space-y-4 max-h-[70vh] overflow-y-auto">
              <Show when={diffSummary()}>
                <div class="rounded-md border border-border bg-surface-alt p-3">
                  <p class="text-xs font-semibold text-base-content">Summary</p>
                  <p class="text-xs text-muted mt-1 whitespace-pre-wrap">{diffSummary()}</p>
                </div>
              </Show>
              <div class="space-y-2">
                <For each={diffFiles()}>
                  {(file) => (
                    <div class="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between rounded-md border border-border px-3 py-2 text-xs">
                      <div class="flex items-center gap-2 min-w-0">
                        <span
                          class={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase ${diffStatusClasses(file.status)}`}
                        >
                          {formatDiffStatus(file.status)}
                        </span>
                        <span class="text-base-content truncate">{file.path}</span>
                      </div>
                      <span class="text-muted sm:flex-shrink-0">{formatDiffStats(file)}</span>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>
        </div>
      </Show>

      {/* First-time Setup Modal */}
      <Show when={showSetupModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black">
          <div class="bg-surface rounded-md shadow-sm max-w-md w-full mx-4 overflow-hidden">
            {/* Header */}
            <div class="bg-blue-600 px-6 py-4">
              <h3 class="text-lg font-semibold text-white">Set Up Pulse Assistant</h3>
              <p class="text-blue-100 text-sm mt-1">Choose a provider to get started</p>
            </div>

            {/* Provider Selection */}
            <div class="p-6 space-y-4">
              <div class="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setSetupProvider('anthropic')}
                  class={`p-3 rounded-md border-2 transition-all text-center ${setupProvider() === 'anthropic'
 ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
 : 'border-border hover:border-blue-300'
 }`}
                >
                  <div class="text-sm font-medium">Anthropic</div>
                  <div class="text-xs text-slate-500 mt-0.5">Claude</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('openai')}
                  class={`p-3 rounded-md border-2 transition-all text-center ${setupProvider() === 'openai'
 ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
 : 'border-border hover:border-blue-300'
 }`}
                >
                  <div class="text-sm font-medium">OpenAI</div>
                  <div class="text-xs text-slate-500 mt-0.5">ChatGPT</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('openrouter')}
                  class={`p-3 rounded-md border-2 transition-all text-center ${setupProvider() === 'openrouter'
 ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
 : 'border-border hover:border-blue-300'
 }`}
                >
                  <div class="text-sm font-medium">OpenRouter</div>
                  <div class="text-xs text-slate-500 mt-0.5">Gateway</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('deepseek')}
                  class={`p-3 rounded-md border-2 transition-all text-center ${setupProvider() === 'deepseek'
 ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
 : 'border-border hover:border-blue-300'
 }`}
                >
                  <div class="text-sm font-medium">DeepSeek</div>
                  <div class="text-xs text-slate-500 mt-0.5">V3</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('gemini')}
                  class={`p-3 rounded-md border-2 transition-all text-center ${setupProvider() === 'gemini'
 ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
 : 'border-border hover:border-blue-300'
 }`}
                >
                  <div class="text-sm font-medium">Gemini</div>
                  <div class="text-xs text-slate-500 mt-0.5">Google</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('ollama')}
                  class={`p-3 rounded-md border-2 transition-all text-center ${setupProvider() === 'ollama'
 ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
 : 'border-border hover:border-blue-300'
 }`}
                >
                  <div class="text-sm font-medium">Ollama</div>
                  <div class="text-xs text-slate-500 mt-0.5">Local</div>
                </button>
              </div>

              {/* API Key / URL Input */}
              <Show when={setupProvider() === 'ollama'} fallback={
                <div>
                  <label class="block text-sm font-medium text-base-content mb-1.5">
                    {setupProvider() === 'anthropic'
                      ? 'Anthropic'
                      : setupProvider() === 'openai'
                        ? 'OpenAI'
                        : setupProvider() === 'openrouter'
                          ? 'OpenRouter'
                          : setupProvider() === 'gemini'
                            ? 'Google Gemini'
                            : 'DeepSeek'} API Key
                  </label>
                  <input
                    type="password"
                    value={setupApiKey()}
                    onInput={(e) => setSetupApiKey(e.currentTarget.value)}
                    placeholder={setupProvider() === 'anthropic' ? 'sk-ant-...' : setupProvider() === 'gemini' ? 'AIza...' : setupProvider() === 'openrouter' ? 'sk-or-...' : 'sk-...'}
                    class="w-full px-3 py-2 border border-border rounded-md bg-white dark:bg-slate-700 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  />
                  <p class="text-xs text-slate-500 mt-1.5">
                    <a
                      href={setupProvider() === 'anthropic'
                        ? 'https://console.anthropic.com/settings/keys'
                        : setupProvider() === 'openai'
                          ? 'https://platform.openai.com/api-keys'
                          : setupProvider() === 'openrouter'
                            ? 'https://openrouter.ai/keys'
                            : setupProvider() === 'gemini'
                              ? 'https://aistudio.google.com/app/apikey'
                              : 'https://platform.deepseek.com/api_keys'}
 target="_blank"
 rel="noopener"
 class="text-blue-600 hover:underline"
 >
 Get your API key →
 </a>
 </p>
 </div>
 }>
 <div>
 <label class="block text-sm font-medium text-base-content mb-1.5">
 Ollama Server URL
 </label>
 <input
 type="url"
 value={setupOllamaUrl()}
 onInput={(e) => setSetupOllamaUrl(e.currentTarget.value)}
 placeholder="http://localhost:11434"
 class="w-full px-3 py-2 border border-border rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
 />
 <p class="text-xs text-slate-500 mt-1.5">
 Ollama runs locally - no API key needed
 </p>
 </div>
 </Show>
 </div>

 {/* Footer */}
 <div class="px-6 py-4 bg-surface-alt border-t border-border flex justify-end gap-3">
 <button
 type="button"
 onClick={() => {
 setShowSetupModal(false);
 setSetupApiKey('');
                }}
                class="px-4 py-2 text-base-content hover:bg-surface-hover rounded-md"
                disabled={setupSaving()}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={async () => {
                  setSetupSaving(true);
                  try {
                    const payload: Record<string, unknown> = { enabled: true };

                    if (setupProvider() === 'anthropic') {
                      if (!setupApiKey().trim()) {
                        notificationStore.error('Please enter your Anthropic API key');
                        return;
                      }
                      payload.anthropic_api_key = setupApiKey().trim();
                      payload.model = 'anthropic:claude-sonnet-4-20250514';
                    } else if (setupProvider() === 'openai') {
                      if (!setupApiKey().trim()) {
                        notificationStore.error('Please enter your OpenAI API key');
                        return;
                      }
                      payload.openai_api_key = setupApiKey().trim();
                      payload.model = 'openai:gpt-4o';
                    } else if (setupProvider() === 'openrouter') {
                      if (!setupApiKey().trim()) {
                        notificationStore.error('Please enter your OpenRouter API key');
                        return;
                      }
                      payload.openrouter_api_key = setupApiKey().trim();
                      payload.model = 'openrouter:openai/gpt-4o-mini';
                    } else if (setupProvider() === 'deepseek') {
                      if (!setupApiKey().trim()) {
                        notificationStore.error('Please enter your DeepSeek API key');
                        return;
                      }
                      payload.deepseek_api_key = setupApiKey().trim();
                      payload.model = 'deepseek:deepseek-chat';
                    } else if (setupProvider() === 'gemini') {
                      if (!setupApiKey().trim()) {
                        notificationStore.error('Please enter your Google Gemini API key');
                        return;
                      }
                      payload.gemini_api_key = setupApiKey().trim();
                      payload.model = 'gemini:gemini-2.5-flash';
                    } else {
                      if (!setupOllamaUrl().trim()) {
                        notificationStore.error('Please enter your Ollama server URL');
                        return;
                      }
                      payload.ollama_base_url = setupOllamaUrl().trim();
                      payload.model = 'ollama:llama3.2:latest';
                    }

                    const updated = await AIAPI.updateSettings(payload);
                    setSettings(updated);
                    setForm('enabled', true);
                    resetForm(updated);
                    void runProviderPreflight(updated);
                    setShowSetupModal(false);
                    setSetupApiKey('');
                    notificationStore.success('Pulse Assistant enabled! You can customize settings below.');
                    // Load models after setup
                    loadModels();
                    // Notify other components (like AIChat) that settings changed
                    aiChatStore.notifySettingsChanged();
                  } catch (error) {
                    logger.error('[AISettings] Setup failed:', error);
                    const message = error instanceof Error ? error.message : 'Setup failed';
                    notificationStore.error(message);
                  } finally {
                    setSetupSaving(false);
                  }
                }}
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2"
                disabled={setupSaving() || (setupProvider() !== 'ollama' && !setupApiKey().trim()) || (setupProvider() === 'ollama' && !setupOllamaUrl().trim())}
              >
                {setupSaving() && <span class="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />}
                Enable Pulse Assistant
              </button>
            </div>
          </div>
        </div>
      </Show >
    </>
  );
};

export default AISettings;
