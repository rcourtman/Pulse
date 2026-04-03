import { createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { createStore } from 'solid-js/store';
import { AIAPI } from '@/api/ai';
import { AIChatAPI, type ChatSession, type FileChange } from '@/api/aiChat';
import {
  AI_PROVIDERS,
  createInitialProviderHealth,
  isAIProviderConfigured,
  type ProviderHealthState,
  type ProviderTestResult,
} from '@/components/Settings/aiSettingsModel';
import { aiChatStore } from '@/stores/aiChat';
import {
  entitlements,
  getUpgradeActionUrlOrFallback,
  hasFeature,
  loadLicenseStatus,
} from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import type { AISettings as AISettingsType, AIProvider, AuthMethod } from '@/types/ai';
import {
  normalizeAIControlLevel,
  type AIControlLevel,
} from '@/utils/aiControlLevelPresentation';
import { getAIProviderDisplayName, getProviderFromModelId } from '@/utils/aiProviderPresentation';
import {
  getAICredentialsClearErrorMessage,
  getAIOAuthErrorMessage,
  getAIChatSessionsLoadErrorMessage,
  getAISessionDiffErrorMessage,
  getAISessionRevertErrorMessage,
  getAISessionSummarizeErrorMessage,
  getAIModelsLoadErrorMessage,
  getAISettingsReadinessPresentation,
  getAISettingsSaveErrorMessage,
  getAISettingsToggleErrorMessage,
} from '@/utils/aiSettingsPresentation';
import {
  AI_QUICKSTART_ACTIVATION_REQUIRED_REASON,
  normalizeQuickstartReason,
} from '@/utils/aiQuickstartContract';
import { logger } from '@/utils/logger';
import { showSuccess, showWarning } from '@/utils/toast';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';

type AISetupMode = 'provider' | 'activation-or-provider' | 'provider-required';

export const useAISettingsState = () => {
  const [settings, setSettings] = createSignal<AISettingsType | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);

  const [availableModels, setAvailableModels] = createSignal<
    { id: string; name: string; description?: string }[]
  >([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);
  const [modelsError, setModelsError] = createSignal('');

  const [chatSessions, setChatSessions] = createSignal<ChatSession[]>([]);
  const [chatSessionsLoading, setChatSessionsLoading] = createSignal(false);
  const [chatSessionsError, setChatSessionsError] = createSignal('');
  const [selectedSessionId, setSelectedSessionId] = createSignal('');
  const [sessionActionLoading, setSessionActionLoading] = createSignal<string | null>(null);

  const [showDiffModal, setShowDiffModal] = createSignal(false);
  const [diffFiles, setDiffFiles] = createSignal<FileChange[]>([]);
  const [diffSummary, setDiffSummary] = createSignal('');
  const [diffSessionLabel, setDiffSessionLabel] = createSignal('');

  const [expandedProviders, setExpandedProviders] = createSignal<Set<AIProvider>>(
    new Set(['anthropic']),
  );

  const [testingProvider, setTestingProvider] = createSignal<AIProvider | null>(null);
  const [providerTestResult, setProviderTestResult] = createSignal<ProviderTestResult | null>(null);
  const [providerHealth, setProviderHealth] = createStore<Record<AIProvider, ProviderHealthState>>(
    createInitialProviderHealth(),
  );
  const [preflightRunning, setPreflightRunning] = createSignal(false);
  const [preflightLastCheckedAt, setPreflightLastCheckedAt] = createSignal<number | null>(null);

  const [startingTrial, setStartingTrial] = createSignal(false);
  const [showSetupModal, setShowSetupModal] = createSignal(false);
  const [setupMode, setSetupMode] = createSignal<AISetupMode>('provider');
  const [setupProvider, setSetupProvider] = createSignal<AIProvider>('anthropic');
  const [setupApiKey, setSetupApiKey] = createSignal('');
  const [setupOllamaUrl, setSetupOllamaUrl] = createSignal('http://localhost:11434');
  const [setupSaving, setSetupSaving] = createSignal(false);
  const [showAdvancedModels, setShowAdvancedModels] = createSignal(false);
  const [showDiscoverySettings, setShowDiscoverySettings] = createSignal(false);
  const [showChatMaintenance, setShowChatMaintenance] = createSignal(false);

  const [form, setForm] = createStore({
    enabled: false,
    model: '',
    chatModel: '',
    patrolModel: '',
    autoFixModel: '',
    authMethod: 'api_key' as AuthMethod,
    patrolIntervalMinutes: 360,
    alertTriggeredAnalysis: true,
    patrolAutoFix: false,
    anthropicApiKey: '',
    openaiApiKey: '',
    openrouterApiKey: '',
    deepseekApiKey: '',
    geminiApiKey: '',
    ollamaBaseUrl: 'http://localhost:11434',
    openaiBaseUrl: '',
    costBudgetUSD30d: '',
    requestTimeoutSeconds: 300,
    controlLevel: 'read_only' as AIControlLevel,
    protectedGuests: '' as string,
    discoveryEnabled: false,
    discoveryIntervalHours: 0,
  });

  const settingsReadiness = createMemo(() =>
    getAISettingsReadinessPresentation({
      configured: Boolean(settings()?.configured),
      providerCount: settings()?.configured_providers?.length || 0,
      modelCount: availableModels().length,
      quickstartCreditsAvailable: Boolean(settings()?.quickstart_credits_available),
      quickstartCreditsRemaining: settings()?.quickstart_credits_remaining ?? 0,
      quickstartCreditsTotal: settings()?.quickstart_credits_total ?? 0,
      quickstartBlockedReason: normalizeQuickstartReason(settings()?.quickstart_blocked_reason),
    }),
  );
  const autoFixLocked = createMemo(() => !hasFeature('ai_autofix'));
  const providerIssueCount = createMemo(
    () => AI_PROVIDERS.filter((provider) => providerHealth[provider].status === 'error').length,
  );
  const selectedChatSession = createMemo(() => {
    const id = selectedSessionId();
    if (!id) return null;
    return chatSessions().find((session) => session.id === id) || null;
  });
  const hasConfiguredProvider = createMemo(() => {
    const current = settings();
    return Boolean(
      current &&
        (current.anthropic_configured ||
          current.openai_configured ||
          current.openrouter_configured ||
          current.deepseek_configured ||
          current.gemini_configured ||
          current.ollama_configured),
    );
  });
  const hasQuickstartAvailable = createMemo(() => Boolean(settings()?.quickstart_credits_available));
  const quickstartBlockedReason = createMemo(() =>
    normalizeQuickstartReason(settings()?.quickstart_blocked_reason),
  );
  const canStartTrial = () => entitlements()?.trial_eligible !== false;
  const upgradeAutofixUrl = () => getUpgradeActionUrlOrFallback('ai_autofix');

  const hasProviderBackedModels = (data: AISettingsType | null | undefined) =>
    (data?.configured_providers?.length ?? 0) > 0;

  const syncModelCatalogForSettings = (data: AISettingsType | null | undefined) => {
    if (hasProviderBackedModels(data)) {
      void loadModels();
      return;
    }
    setAvailableModels([]);
    setModelsError('');
  };

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible = form.controlLevel === 'autonomous' && autoFixLocked();
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('ai_autofix', 'settings_ai_patrol_autofix');
    }
    return isPaywallVisible;
  }, false);

  const resetForm = (data: AISettingsType | null) => {
    if (!data) {
      setForm({
        enabled: false,
        model: '',
        chatModel: '',
        patrolModel: '',
        autoFixModel: '',
        authMethod: 'api_key',
        patrolIntervalMinutes: 360,
        alertTriggeredAnalysis: true,
        patrolAutoFix: false,
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
      model: data.model || '',
      chatModel: data.chat_model || '',
      patrolModel: data.patrol_model || '',
      autoFixModel: data.auto_fix_model || '',
      authMethod: data.auth_method || 'api_key',
      patrolIntervalMinutes: data.patrol_interval_minutes ?? 360,
      alertTriggeredAnalysis: data.alert_triggered_analysis !== false,
      patrolAutoFix: data.patrol_auto_fix || false,
      anthropicApiKey: '',
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
      controlLevel: normalizeAIControlLevel(data.control_level),
      protectedGuests: Array.isArray(data.protected_guests) ? data.protected_guests.join(', ') : '',
      discoveryEnabled: data.discovery_enabled ?? false,
      discoveryIntervalHours: data.discovery_interval_hours ?? 0,
    });

    const configured = new Set<AIProvider>();
    if (data.anthropic_configured) configured.add('anthropic');
    if (data.openai_configured) configured.add('openai');
    if (data.openrouter_configured) configured.add('openrouter');
    if (data.deepseek_configured) configured.add('deepseek');
    if (data.gemini_configured) configured.add('gemini');
    if (data.ollama_configured) configured.add('ollama');
    if (configured.size === 0) configured.add('anthropic');
    setExpandedProviders(configured);
  };

  const loadModels = async () => {
    setModelsLoading(true);
    setModelsError('');
    try {
      const result = await AIAPI.getModels();
      if (result.error) {
        setModelsError(result.error);
        logger.debug('[AISettings] API returned error for models:', result.error);
      }
      setAvailableModels(result.models ?? []);
    } catch (error) {
      const message = getAIModelsLoadErrorMessage(error instanceof Error ? error.message : '');
      setModelsError(message);
      setAvailableModels([]);
      logger.debug('[AISettings] Failed to load models from API:', error);
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
      setChatSessionsError(
        getAIChatSessionsLoadErrorMessage(error instanceof Error ? error.message : ''),
      );
    } finally {
      setChatSessionsLoading(false);
    }
  };

  const formatSessionLabel = (session: ChatSession) => {
    const updatedAt = new Date(session.updated_at);
    const dateLabel = updatedAt.toLocaleDateString([], { month: 'short', day: 'numeric' });
    const timeLabel = updatedAt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    return `${session.title || 'Untitled'} - ${session.message_count} msgs - ${dateLabel} ${timeLabel}`;
  };

  const formatDiffStats = (change: FileChange) => `+${change.added} -${change.removed}`;

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const outcome = await runStartProTrialAction({
        showSuccess,
        showError: showWarning,
      });
      if (outcome === 'activated') {
        setShowSetupModal(false);
        await loadSettings();
      }
    } finally {
      setStartingTrial(false);
    }
  };

  const handleCloseSetupModal = () => {
    setShowSetupModal(false);
    setSetupMode('provider');
    setSetupApiKey('');
  };

  const openEnableSetupModal = () => {
    const blockedReason = quickstartBlockedReason();
    if (blockedReason === AI_QUICKSTART_ACTIVATION_REQUIRED_REASON) {
      setSetupMode('activation-or-provider');
    } else if (blockedReason) {
      setSetupMode('provider-required');
    } else {
      setSetupMode('provider');
    }
    setShowSetupModal(true);
  };

  const checkProviderHealth = async (
    provider: AIProvider,
    opts: { notify?: boolean; storeManualResult?: boolean } = {},
  ): Promise<ProviderTestResult> => {
    try {
      const result = await AIAPI.testProvider(provider);
      const normalizedResult: ProviderTestResult = {
        provider,
        success: result.success,
        message: result.message,
      };
      setProviderHealth(provider, {
        status: result.success ? 'ok' : 'error',
        message: result.message || '',
      });
      if (opts.storeManualResult) {
        setProviderTestResult(normalizedResult);
      }
      if (opts.notify) {
        if (result.success) {
          notificationStore.success(`${provider}: ${result.message}`);
        } else {
          notificationStore.error(`${provider}: ${result.message}`);
        }
      }
      return normalizedResult;
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Connection test failed';
      const result: ProviderTestResult = { provider, success: false, message };
      setProviderHealth(provider, { status: 'error', message });
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

    const configuredProviders = AI_PROVIDERS.filter((provider) =>
      isAIProviderConfigured(provider, current),
    );
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
        }),
      );
      setPreflightLastCheckedAt(Date.now());
    } finally {
      setPreflightRunning(false);
    }
  };

  const loadSettings = async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const data = await AIAPI.getSettings();
      setSettings(data);
      resetForm(data);
      syncModelCatalogForSettings(data);
      void runProviderPreflight(data);
    } catch (error) {
      logger.error('[AISettings] Failed to load settings:', error);
      setLoadError(true);
      setSettings(null);
      resetForm(null);
      setAvailableModels([]);
      setModelsError('');
      setProviderHealth(createInitialProviderHealth());
      setPreflightLastCheckedAt(null);
    } finally {
      setLoading(false);
    }
  };

  const handleSetupSubmit = async () => {
    setSetupSaving(true);
    try {
      const payload: Record<string, unknown> = { enabled: true };

      if (setupProvider() === 'anthropic') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Anthropic API key');
          return;
        }
        payload.anthropic_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'openai') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your OpenAI API key');
          return;
        }
        payload.openai_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'openrouter') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your OpenRouter API key');
          return;
        }
        payload.openrouter_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'deepseek') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your DeepSeek API key');
          return;
        }
        payload.deepseek_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'gemini') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Google Gemini API key');
          return;
        }
        payload.gemini_api_key = setupApiKey().trim();
      } else {
        if (!setupOllamaUrl().trim()) {
          notificationStore.error('Please enter your Ollama server URL');
          return;
        }
        payload.ollama_base_url = setupOllamaUrl().trim();
      }

      const updated = await AIAPI.updateSettings(payload);
      setSettings(updated);
      setForm('enabled', true);
      resetForm(updated);
      syncModelCatalogForSettings(updated);
      void runProviderPreflight(updated);
      handleCloseSetupModal();
      notificationStore.success('Pulse Assistant enabled! You can customize settings below.');
      aiChatStore.notifySettingsChanged();
    } catch (error) {
      logger.error('[AISettings] Setup failed:', error);
      notificationStore.error(error instanceof Error ? error.message : 'Setup failed');
    } finally {
      setSetupSaving(false);
    }
  };

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
      notificationStore.error(
        getAISessionSummarizeErrorMessage(error instanceof Error ? error.message : ''),
      );
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
      setDiffSessionLabel(session ? session.title || 'Untitled session' : 'Selected session');
      setShowDiffModal(true);
    } catch (error) {
      logger.error('[AISettings] Failed to get session diff:', error);
      notificationStore.error(getAISessionDiffErrorMessage(error instanceof Error ? error.message : ''));
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
      notificationStore.error(
        getAISessionRevertErrorMessage(error instanceof Error ? error.message : ''),
      );
    } finally {
      setSessionActionLoading(null);
    }
  };

  const handleSave = async (event?: Event) => {
    event?.preventDefault();

    if (settings() === null) {
      notificationStore.error('Cannot save: settings failed to load. Please retry loading first.');
      return;
    }

    const selectedModel = form.model.trim();
    if (selectedModel && form.enabled) {
      const modelProvider = getProviderFromModelId(selectedModel);
      if (!isAIProviderConfigured(modelProvider, settings())) {
        const isAddingCredential =
          (modelProvider === 'anthropic' && form.anthropicApiKey.trim()) ||
          (modelProvider === 'openai' && form.openaiApiKey.trim()) ||
          (modelProvider === 'openrouter' && form.openrouterApiKey.trim()) ||
          (modelProvider === 'deepseek' && form.deepseekApiKey.trim()) ||
          (modelProvider === 'gemini' && form.geminiApiKey.trim()) ||
          (modelProvider === 'ollama' && form.ollamaBaseUrl.trim());

        if (!isAddingCredential) {
          notificationStore.error(
            `Cannot save: Model "${selectedModel}" requires ${getAIProviderDisplayName(modelProvider) || modelProvider} to be configured. ` +
              `Please add an API key for ${getAIProviderDisplayName(modelProvider) || modelProvider} or select a different model.`,
          );
          return;
        }
      }
    }

    if (form.patrolIntervalMinutes > 0 && form.patrolIntervalMinutes < 10) {
      notificationStore.error('Patrol interval must be at least 10 minutes (or 0 to disable)');
      return;
    }

    setSaving(true);
    try {
      const payload: Record<string, unknown> = {
        model: selectedModel,
      };

      if (form.enabled !== settings()?.enabled) {
        payload.enabled = form.enabled;
      }
      if (form.patrolIntervalMinutes !== (settings()?.patrol_interval_minutes ?? 360)) {
        payload.patrol_interval_minutes = form.patrolIntervalMinutes;
      }
      if (form.alertTriggeredAnalysis !== settings()?.alert_triggered_analysis) {
        payload.alert_triggered_analysis = form.alertTriggeredAnalysis;
      }
      if (form.patrolAutoFix !== settings()?.patrol_auto_fix) {
        payload.patrol_auto_fix = form.patrolAutoFix;
      }
      if (form.chatModel !== (settings()?.chat_model || '')) {
        payload.chat_model = form.chatModel;
      }
      if (form.patrolModel !== (settings()?.patrol_model || '')) {
        payload.patrol_model = form.patrolModel;
      }
      if (form.autoFixModel !== (settings()?.auto_fix_model || '')) {
        payload.auto_fix_model = form.autoFixModel;
      }
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
      if (
        form.ollamaBaseUrl.trim() &&
        form.ollamaBaseUrl.trim() !== (settings()?.ollama_base_url || '')
      ) {
        payload.ollama_base_url = form.ollamaBaseUrl.trim();
      }
      if (form.openaiBaseUrl !== (settings()?.openai_base_url || '')) {
        payload.openai_base_url = form.openaiBaseUrl.trim();
      }

      const rawBudget = form.costBudgetUSD30d.trim();
      const parsedBudget = rawBudget === '' ? 0 : Number(rawBudget);
      if (!Number.isFinite(parsedBudget) || parsedBudget < 0) {
        notificationStore.error('Cost budget must be a non-negative number');
        return;
      }
      const currentBudget = settings()?.cost_budget_usd_30d ?? 0;
      if (Math.abs(parsedBudget - currentBudget) > 0.0001) {
        payload.cost_budget_usd_30d = parsedBudget;
      }

      if (form.requestTimeoutSeconds !== (settings()?.request_timeout_seconds ?? 300)) {
        payload.request_timeout_seconds = form.requestTimeoutSeconds;
      }
      if (form.controlLevel !== (settings()?.control_level || 'read_only')) {
        payload.control_level = form.controlLevel;
      }

      const currentProtected = settings()?.protected_guests || [];
      const newProtected = form.protectedGuests
        .split(',')
        .map((value: string) => value.trim())
        .filter((value: string) => value.length > 0);
      const protectedChanged =
        newProtected.length !== currentProtected.length ||
        newProtected.some((guest: string, index: number) => guest !== currentProtected[index]);
      if (protectedChanged) {
        payload.protected_guests = newProtected;
      }

      if (form.discoveryEnabled !== (settings()?.discovery_enabled ?? false)) {
        payload.discovery_enabled = form.discoveryEnabled;
      }
      if (form.discoveryIntervalHours !== (settings()?.discovery_interval_hours ?? 0)) {
        payload.discovery_interval_hours = form.discoveryIntervalHours;
      }

      const updated = await AIAPI.updateSettings(payload);
      setSettings(updated);
      resetForm(updated);
      syncModelCatalogForSettings(updated);
      void runProviderPreflight(updated);
      notificationStore.success('Pulse Assistant settings saved');
      aiChatStore.notifySettingsChanged();
    } catch (error) {
      logger.error('[AISettings] Failed to save settings:', error);
      notificationStore.error(getAISettingsSaveErrorMessage(error instanceof Error ? error.message : ''));
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
      notificationStore.error(error instanceof Error ? error.message : 'Connection test failed');
    } finally {
      setTesting(false);
    }
  };

  const handleTestProvider = async (provider: AIProvider) => {
    setTestingProvider(provider);
    setProviderTestResult(null);
    try {
      await checkProviderHealth(provider, { notify: true, storeManualResult: true });
    } catch (error) {
      logger.error(`[AISettings] Test ${provider} failed:`, error);
      const message = error instanceof Error ? error.message : 'Connection test failed';
      setProviderTestResult({ provider, success: false, message });
      notificationStore.error(`${provider}: ${message}`);
    } finally {
      setTestingProvider(null);
    }
  };

  const handleClearProvider = async (provider: AIProvider) => {
    const current = settings();
    const configuredCount = [
      current?.anthropic_configured,
      current?.openai_configured,
      current?.openrouter_configured,
      current?.deepseek_configured,
      current?.gemini_configured,
      current?.ollama_configured,
    ].filter(Boolean).length;
    const isLastProvider = configuredCount === 1 && isAIProviderConfigured(provider, current);
    const currentModel = form.model.trim();
    const modelUsesProvider = currentModel && getProviderFromModelId(currentModel) === provider;

    let confirmMessage = `Clear ${getAIProviderDisplayName(provider) || provider} credentials?`;
    if (isLastProvider) {
      confirmMessage =
        'Warning: this is your only configured provider. Clearing it will disable Pulse Assistant until you configure another provider. Continue?';
    } else if (modelUsesProvider) {
      confirmMessage = `Your current model uses ${getAIProviderDisplayName(provider) || provider}. Clearing this will require selecting a different model. Continue?`;
    } else {
      confirmMessage += " You'll need to re-enter credentials to use this provider.";
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
      const newSettings = await AIAPI.getSettings();
      setSettings(newSettings);
      syncModelCatalogForSettings(newSettings);
      void runProviderPreflight(newSettings);

      if (provider === 'anthropic') setForm('anthropicApiKey', '');
      if (provider === 'openai') setForm('openaiApiKey', '');
      if (provider === 'openrouter') setForm('openrouterApiKey', '');
      if (provider === 'deepseek') setForm('deepseekApiKey', '');
      if (provider === 'gemini') setForm('geminiApiKey', '');
      if (provider === 'ollama') setForm('ollamaBaseUrl', '');

      notificationStore.success(`${provider} credentials cleared`);
      aiChatStore.notifySettingsChanged();
    } catch (error) {
      logger.error(`[AISettings] Clear ${provider} failed:`, error);
      notificationStore.error(
        getAICredentialsClearErrorMessage(error instanceof Error ? error.message : ''),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleEnabledToggle = async (newValue: boolean) => {
    setForm('enabled', newValue);
    try {
      const updated = await AIAPI.updateSettings({ enabled: newValue });
      setSettings(updated);
      syncModelCatalogForSettings(updated);
      void runProviderPreflight(updated);
      notificationStore.success(newValue ? 'Pulse Assistant enabled' : 'Pulse Assistant disabled');
      aiChatStore.notifySettingsChanged();
    } catch (error) {
      setForm('enabled', !newValue);
      logger.error('[AISettings] Failed to toggle AI:', error);
      notificationStore.error(
        getAISettingsToggleErrorMessage(error instanceof Error ? error.message : ''),
      );
    }
  };

  const handleEnableRequest = async (newValue: boolean) => {
    if (!newValue) {
      await handleEnabledToggle(false);
      return;
    }
    if (hasConfiguredProvider() || hasQuickstartAvailable()) {
      await handleEnabledToggle(true);
      return;
    }
    openEnableSetupModal();
  };

  onMount(() => {
    loadLicenseStatus();

    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      const oauthSuccess = params.get('ai_oauth_success');
      const oauthError = params.get('ai_oauth_error');

      if (oauthSuccess === 'true') {
        notificationStore.success('Successfully connected to Claude with your subscription!');
        window.history.replaceState({}, '', window.location.pathname);
      } else if (oauthError) {
        notificationStore.error(getAIOAuthErrorMessage(oauthError));
        window.history.replaceState({}, '', window.location.pathname);
      }
    }

    void loadSettings();
  });

  return {
    autoFixLocked,
    availableModels,
    canStartTrial,
    chatSessions,
    chatSessionsError,
    chatSessionsLoading,
    diffFiles,
    diffSessionLabel,
    diffSummary,
    expandedProviders,
    form,
    formatDiffStats,
    formatSessionLabel,
    handleClearProvider,
    handleCloseSetupModal,
    handleEnableRequest,
    handleEnabledToggle,
    handleSave,
    handleSessionDiff,
    handleSessionRevert,
    handleSessionSummarize,
    handleSetupSubmit,
    handleStartTrial,
    handleTest,
    handleTestProvider,
    hasConfiguredProvider,
    hasQuickstartAvailable,
    loadChatSessions,
    loading,
    loadError,
    loadModels,
    loadSettings,
    modelsError,
    modelsLoading,
    preflightLastCheckedAt,
    preflightRunning,
    providerHealth,
    providerIssueCount,
    providerTestResult,
    resetForm,
    runProviderPreflight,
    saving,
    selectedChatSession,
    selectedSessionId,
    sessionActionLoading,
    setExpandedProviders,
    setForm,
    setSelectedSessionId,
    setSetupApiKey,
    setSetupOllamaUrl,
    setSetupProvider,
    setShowAdvancedModels,
    setShowChatMaintenance,
    setShowDiffModal,
    setShowDiscoverySettings,
    setShowSetupModal,
    setupMode,
    settings,
    settingsReadiness,
    setupApiKey,
    setupOllamaUrl,
    setupProvider,
    setupSaving,
    showAdvancedModels,
    showChatMaintenance,
    showDiffModal,
    showDiscoverySettings,
    showSetupModal,
    startingTrial,
    testing,
    testingProvider,
    quickstartBlockedReason,
    upgradeAutofixUrl,
  };
};

export type AISettingsState = ReturnType<typeof useAISettingsState>;
