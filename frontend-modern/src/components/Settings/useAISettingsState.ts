import { createMemo, createSignal, onMount } from 'solid-js';
import { createStore } from 'solid-js/store';
import { AIAPI } from '@/api/ai';
import { AIChatAPI, type ChatSession } from '@/api/aiChat';
import { runDiscoveryRefresh } from '@/api/discovery';
import { runPatrolPreflight, type PatrolPreflightResponse } from '@/api/patrol';
import {
  AI_PROVIDERS,
  createInitialProviderHealth,
  isAIProviderConfigured,
  type ProviderHealthState,
  type ProviderTestResult,
} from '@/components/Settings/aiSettingsModel';
import { apiErrorDetails } from '@/api/responseUtils';
import {
  aiRuntimeModels,
  aiRuntimeModelsError,
  clearAIRuntimeModels,
  loadAIRuntimeModels,
  syncAIRuntimeSettings,
} from '@/stores/aiRuntimeState';
import { hasFeature, loadRuntimeCapabilities } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { aiChatStore } from '@/stores/aiChat';
import { notificationStore } from '@/stores/notifications';
import type { AISettings as AISettingsType, AIProvider, AuthMethod, ModelInfo } from '@/types/ai';
import { normalizeAIControlLevel, type AIControlLevel } from '@/utils/aiControlLevelPresentation';
import { getAIProviderDisplayName, getProviderFromModelId } from '@/utils/aiProviderPresentation';
import {
  getAICredentialsClearErrorMessage,
  getAIOAuthErrorMessage,
  getAIChatSessionsLoadErrorMessage,
  getAISessionSummarizeErrorMessage,
  getAIModelsLoadErrorMessage,
  getAISettingsReadinessPresentation,
  getAISettingsSaveErrorMessage,
  getAISettingsToggleErrorMessage,
} from '@/utils/aiSettingsPresentation';
import { logger } from '@/utils/logger';

type AISettingsSaveProviderFailure = {
  provider: string;
  model?: string;
  providerMessage?: string;
};

interface AISettingsStateOptions {
  saveErrorFallback?: string;
  savedLabel?: string;
}

const AI_SETTINGS_PROVIDER_PAYLOAD_FIELDS: Record<AIProvider, string[]> = {
  anthropic: ['anthropic_api_key'],
  openai: ['openai_api_key', 'openai_base_url'],
  openrouter: ['openrouter_api_key'],
  deepseek: ['deepseek_api_key'],
  zai: ['zai_api_key', 'zai_base_url'],
  groq: ['groq_api_key'],
  mistral: ['mistral_api_key'],
  cerebras: ['cerebras_api_key'],
  together: ['together_api_key'],
  fireworks: ['fireworks_api_key'],
  gemini: ['gemini_api_key'],
  ollama: ['ollama_base_url', 'ollama_keep_alive'],
  'codex-subscription': ['codex_subscription_enabled'],
  'claude-subscription': ['claude_subscription_enabled'],
};

const compactUnique = (values: Array<string | undefined>): string[] => {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const value of values) {
    const normalized = value?.trim();
    if (!normalized || seen.has(normalized)) continue;
    seen.add(normalized);
    result.push(normalized);
  }
  return result;
};

const getProviderTestDiagnosticMessage = (result: {
  message?: string;
  recommendation?: string;
}): string =>
  compactUnique([result.message, result.recommendation]).join(' · ') || 'Connection test failed';

const isKnownAIProvider = (provider: string): provider is AIProvider =>
  AI_PROVIDERS.includes(provider as AIProvider);

const providerPayloadEntries = (payload: Record<string, unknown>): string[] =>
  AI_PROVIDERS.filter((provider) =>
    AI_SETTINGS_PROVIDER_PAYLOAD_FIELDS[provider].some(
      (field) =>
        payload[field] === true ||
        (typeof payload[field] === 'string' && String(payload[field]).trim().length > 0),
    ),
  );

const modelProviderEntries = (models: string[]): Array<{ provider: string; model: string }> =>
  models
    .map((model) => model.trim())
    .filter(Boolean)
    .map((model) => ({
      provider: getProviderFromModelId(model),
      model,
    }));

const findModelForProvider = (
  provider: string,
  models: Array<{ provider: string; model: string }>,
): string | undefined => models.find((entry) => entry.provider === provider)?.model;

const isGenericAISettingsSaveFailure = (error: unknown): boolean => {
  const message = error instanceof Error ? error.message.trim().toLowerCase() : '';
  if (!message) return true;
  return (
    message === 'failed to save pulse intelligence settings' ||
    message === 'failed to save provider & models settings' ||
    message === 'unable to save provider & models settings.' ||
    message === 'unable to save provider & models settings' ||
    message.includes('failed to save pulse intelligence settings') ||
    message.includes('failed to save provider & models settings') ||
    message.startsWith('request failed with status')
  );
};

export function getAISettingsSaveProviderFailureMessage(
  message: string | undefined,
  failure?: AISettingsSaveProviderFailure | null,
  fallback?: string,
): string {
  const baseMessage = getAISettingsSaveErrorMessage(message, fallback);
  if (!failure?.provider) return baseMessage;

  const providerLabel = getAIProviderDisplayName(failure.provider);
  const lowerBase = baseMessage.toLowerCase();
  const context = [
    lowerBase.includes(providerLabel.toLowerCase()) ? undefined : `${providerLabel} provider`,
    failure.model && !baseMessage.includes(failure.model) ? `model ${failure.model}` : undefined,
    failure.providerMessage && !baseMessage.includes(failure.providerMessage)
      ? failure.providerMessage
      : undefined,
  ].filter((value): value is string => Boolean(value));

  return context.length > 0 ? `${context.join(' · ')}: ${baseMessage}` : baseMessage;
}

export function getAISettingsPatrolReadinessSaveMessage(
  readiness: AISettingsType['patrol_readiness'] | null | undefined,
  savedLabel = 'Provider & Models settings saved',
): string | null {
  if (!readiness || readiness.status === 'ready') return null;

  const provider = readiness.provider ? getAIProviderDisplayName(readiness.provider) : '';
  const model = readiness.model?.trim();
  const context = [
    readiness.summary?.trim(),
    provider ? `Provider: ${provider}` : undefined,
    model ? `Model: ${model}` : undefined,
  ].filter((value): value is string => Boolean(value));
  const readinessLabel = readiness.status === 'not_ready' ? 'not ready' : 'degraded';

  return context.length > 0
    ? `${savedLabel}, but Patrol is ${readinessLabel}: ${context.join(' · ')}`
    : `${savedLabel}, but Patrol is ${readinessLabel}.`;
}

export function resolveAISettingsSaveProviderFailure(input: {
  error: unknown;
  payload: Record<string, unknown>;
  providerHealth: Record<AIProvider, ProviderHealthState>;
  models: string[];
}): AISettingsSaveProviderFailure | null {
  const details = apiErrorDetails(input.error);
  const detailProvider = details?.provider?.trim();
  const detailModel = details?.model?.trim();
  if (detailProvider) {
    return {
      provider: detailProvider,
      model: detailModel,
      providerMessage: details?.cause || details?.reason || details?.summary,
    };
  }

  const modelEntries = modelProviderEntries(input.models);
  const payloadProviders = providerPayloadEntries(input.payload);
  const canUseLocalProviderContext =
    payloadProviders.length > 0 || isGenericAISettingsSaveFailure(input.error);
  if (!canUseLocalProviderContext) {
    return null;
  }
  const candidateProviders = compactUnique([
    ...payloadProviders,
    ...modelEntries.map((entry) => entry.provider),
  ]);

  const erroredCandidate = candidateProviders.find(
    (provider) => isKnownAIProvider(provider) && input.providerHealth[provider].status === 'error',
  );
  if (erroredCandidate && isKnownAIProvider(erroredCandidate)) {
    return {
      provider: erroredCandidate,
      model: findModelForProvider(erroredCandidate, modelEntries),
      providerMessage: input.providerHealth[erroredCandidate].message,
    };
  }

  const anyErroredProvider = AI_PROVIDERS.find(
    (provider) => input.providerHealth[provider].status === 'error',
  );
  if (anyErroredProvider) {
    return {
      provider: anyErroredProvider,
      model: findModelForProvider(anyErroredProvider, modelEntries),
      providerMessage: input.providerHealth[anyErroredProvider].message,
    };
  }

  if (candidateProviders.length === 1) {
    const provider = candidateProviders[0];
    return {
      provider,
      model: findModelForProvider(provider, modelEntries),
    };
  }

  return null;
}

export const useAISettingsState = (options: AISettingsStateOptions = {}) => {
  const [settings, setSettings] = createSignal<AISettingsType | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);

  const [availableModels, setAvailableModels] = createSignal<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);
  const [modelsError, setModelsError] = createSignal('');

  const [chatSessions, setChatSessions] = createSignal<ChatSession[]>([]);
  const [chatSessionsLoading, setChatSessionsLoading] = createSignal(false);
  const [chatSessionsError, setChatSessionsError] = createSignal('');
  const [selectedSessionId, setSelectedSessionId] = createSignal('');
  const [sessionActionLoading, setSessionActionLoading] = createSignal<string | null>(null);

  const [expandedProviders, setExpandedProviders] = createSignal<Set<AIProvider>>(
    new Set(['anthropic']),
  );
  // Tracks whether the initial expand policy has been applied, so later
  // resetForm calls (after every save) preserve the operator's own
  // expand/collapse choices instead of clobbering them.
  const [providersExpandedInitialized, setProvidersExpandedInitialized] = createSignal(false);

  const [testingProvider, setTestingProvider] = createSignal<AIProvider | null>(null);
  const [providerTestResult, setProviderTestResult] = createSignal<ProviderTestResult | null>(null);
  const [providerHealth, setProviderHealth] = createStore<Record<AIProvider, ProviderHealthState>>(
    createInitialProviderHealth(),
  );
  const [preflightRunning, setPreflightRunning] = createSignal(false);
  const [preflightLastCheckedAt, setPreflightLastCheckedAt] = createSignal<number | null>(null);
  const [patrolPreflightRunning, setPatrolPreflightRunning] = createSignal(false);
  const [patrolPreflightResult, setPatrolPreflightResult] =
    createSignal<PatrolPreflightResponse | null>(null);
  const [discoveryRunRunning, setDiscoveryRunRunning] = createSignal(false);

  // hydratePatrolPreflightFromSettings projects the cached preflight
  // snapshot from /api/settings/ai into the same response shape the
  // manual Check Patrol model button writes, so the inline result panel can
  // render the most-recent outcome on page load without forcing a
  // re-click. Returns null when preflight has never run on the
  // current Pulse instance.
  const hydratePatrolPreflightFromSettings = (data: AISettingsType | null) => {
    const snapshot = data?.patrol_preflight;
    if (!snapshot) {
      setPatrolPreflightResult(null);
      return;
    }
    setPatrolPreflightResult({
      success: snapshot.success,
      provider: snapshot.provider,
      model: snapshot.model,
      tool_call_observed: snapshot.tool_call_observed,
      duration_ms: snapshot.duration_ms,
      message: snapshot.title || snapshot.summary || '',
      cause: snapshot.cause,
      summary: snapshot.summary,
      recommendation: snapshot.recommendation,
      recorded_at: snapshot.recorded_at,
      recorded_at_unix: snapshot.recorded_at_unix,
    });
  };

  const [showSetupModal, setShowSetupModal] = createSignal(false);
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
    discoveryModel: '',
    autoFixModel: '',
    authMethod: 'api_key' as AuthMethod,
    patrolIntervalMinutes: 360,
    alertTriggeredAnalysis: true,
    patrolAlertTriggers: true,
    patrolAnomalyTriggers: true,
    patrolAlertTriggerMinSeverity: 'critical' as 'warning' | 'critical',
    patrolFindingNotifications: true,
    patrolFindingNotifyMinSeverity: 'warning' as 'warning' | 'critical',
    patrolAutoFix: false,
    anthropicApiKey: '',
    openaiApiKey: '',
    openrouterApiKey: '',
    deepseekApiKey: '',
    zaiApiKey: '',
    groqApiKey: '',
    mistralApiKey: '',
    cerebrasApiKey: '',
    togetherApiKey: '',
    fireworksApiKey: '',
    geminiApiKey: '',
    ollamaBaseUrl: 'http://localhost:11434',
    ollamaKeepAlive: '30s',
    openaiBaseUrl: '',
    zaiBaseUrl: '',
    codexSubscriptionEnabled: false,
    claudeSubscriptionEnabled: false,
    costBudgetUSD30d: '',
    requestTimeoutSeconds: 300,
    controlLevel: 'read_only' as AIControlLevel,
    protectedGuests: '' as string,
    discoveryEnabled: false,
    discoveryIntervalHours: 0,
  });

  const showUpgradePrompts = () => !presentationPolicyHidesUpgradePrompts();

  const settingsReadiness = createMemo(() =>
    getAISettingsReadinessPresentation({
      configured: Boolean(settings()?.configured),
      providerCount: settings()?.configured_providers?.length || 0,
      modelCount: availableModels().length,
    }),
  );
  const autoFixLocked = createMemo(() => !hasFeature('ai_autofix'));
  const alertAnalysisLocked = createMemo(() => !hasFeature('ai_alerts'));
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
        current.zai_configured ||
        current.groq_configured ||
        current.mistral_configured ||
        current.cerebras_configured ||
        current.together_configured ||
        current.fireworks_configured ||
        current.gemini_configured ||
        current.codex_subscription_enabled ||
        current.claude_subscription_enabled ||
        current.ollama_configured),
    );
  });
  const upgradeAutofixDestination = () => getUpgradeActionDestination('ai_autofix');

  const hasProviderBackedModels = (data: AISettingsType | null | undefined) =>
    (data?.configured_providers?.length ?? 0) > 0;

  const syncModelCatalogForSettings = (data: AISettingsType | null | undefined) => {
    syncAIRuntimeSettings(data ?? null);
    if (hasProviderBackedModels(data)) {
      void loadModels();
      return;
    }
    setAvailableModels([]);
    setModelsError('');
    clearAIRuntimeModels();
  };

  const resetForm = (data: AISettingsType | null) => {
    if (!data) {
      setForm({
        enabled: false,
        model: '',
        chatModel: '',
        patrolModel: '',
        discoveryModel: '',
        autoFixModel: '',
        authMethod: 'api_key',
        patrolIntervalMinutes: 360,
        alertTriggeredAnalysis: true,
        patrolAlertTriggers: true,
        patrolAnomalyTriggers: true,
        patrolAlertTriggerMinSeverity: 'critical',
        patrolFindingNotifications: true,
        patrolFindingNotifyMinSeverity: 'warning',
        patrolAutoFix: false,
        anthropicApiKey: '',
        openaiApiKey: '',
        openrouterApiKey: '',
        deepseekApiKey: '',
        zaiApiKey: '',
        groqApiKey: '',
        mistralApiKey: '',
        cerebrasApiKey: '',
        togetherApiKey: '',
        fireworksApiKey: '',
        geminiApiKey: '',
        ollamaBaseUrl: 'http://localhost:11434',
        ollamaKeepAlive: '30s',
        openaiBaseUrl: '',
        zaiBaseUrl: '',
        codexSubscriptionEnabled: false,
        claudeSubscriptionEnabled: false,
        costBudgetUSD30d: '',
        requestTimeoutSeconds: 300,
        controlLevel: 'read_only',
        protectedGuests: '',
        discoveryEnabled: false,
        discoveryIntervalHours: 0,
      });
      return;
    }

    const legacyEventTriggersEnabled =
      data.patrol_event_triggers_enabled ?? data.alert_triggered_analysis !== false;

    setForm({
      enabled: data.enabled,
      model: data.model || '',
      chatModel: data.chat_model || '',
      patrolModel: data.patrol_model || '',
      discoveryModel: data.discovery_model || '',
      autoFixModel: data.auto_fix_model || '',
      authMethod: data.auth_method || 'api_key',
      patrolIntervalMinutes: data.patrol_interval_minutes ?? 360,
      alertTriggeredAnalysis: data.alert_triggered_analysis !== false,
      patrolAlertTriggers: data.patrol_alert_triggers_enabled ?? legacyEventTriggersEnabled,
      patrolAnomalyTriggers: data.patrol_anomaly_triggers_enabled ?? legacyEventTriggersEnabled,
      patrolAlertTriggerMinSeverity:
        data.patrol_alert_trigger_min_severity === 'warning' ? 'warning' : 'critical',
      patrolFindingNotifications: data.patrol_finding_notifications_enabled !== false,
      patrolFindingNotifyMinSeverity:
        data.patrol_finding_notify_min_severity === 'critical' ? 'critical' : 'warning',
      patrolAutoFix: data.patrol_auto_fix || false,
      anthropicApiKey: '',
      openaiApiKey: '',
      openrouterApiKey: '',
      deepseekApiKey: '',
      zaiApiKey: '',
      groqApiKey: '',
      mistralApiKey: '',
      cerebrasApiKey: '',
      togetherApiKey: '',
      fireworksApiKey: '',
      geminiApiKey: '',
      ollamaBaseUrl: data.ollama_base_url || 'http://localhost:11434',
      ollamaKeepAlive: data.ollama_keep_alive ?? '30s',
      openaiBaseUrl: data.openai_base_url || '',
      zaiBaseUrl: data.zai_base_url || '',
      codexSubscriptionEnabled: Boolean(data.codex_subscription_enabled),
      claudeSubscriptionEnabled: Boolean(data.claude_subscription_enabled),
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
    if (data.zai_configured) configured.add('zai');
    if (data.groq_configured) configured.add('groq');
    if (data.mistral_configured) configured.add('mistral');
    if (data.cerebras_configured) configured.add('cerebras');
    if (data.together_configured) configured.add('together');
    if (data.fireworks_configured) configured.add('fireworks');
    if (data.codex_subscription_enabled) configured.add('codex-subscription');
    if (data.claude_subscription_enabled) configured.add('claude-subscription');
    if (data.ollama_configured) configured.add('ollama');
    // Apply the expand policy only on the first load. Guide first-time
    // setup by expanding the default provider when nothing is configured
    // yet; once at least one provider is configured, keep the list
    // collapsed — the per-provider badges and the health callout already
    // convey status, and expanding every configured provider just produces
    // a wall of API-key inputs. Later saves preserve the operator's own
    // expand/collapse selections.
    if (!providersExpandedInitialized()) {
      setProvidersExpandedInitialized(true);
      setExpandedProviders(
        configured.size === 0 ? new Set<AIProvider>(['anthropic']) : new Set<AIProvider>(),
      );
    }
  };

  const loadModels = async () => {
    setModelsLoading(true);
    setModelsError('');
    try {
      await loadAIRuntimeModels(true);
      const nextError = aiRuntimeModelsError();
      if (nextError) {
        setModelsError(nextError);
        logger.debug('[AISettings] API returned error for models:', nextError);
      }
      setAvailableModels(aiRuntimeModels());
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

  const selectedModelForProviderTest = (provider: AIProvider): string | undefined =>
    compactUnique([
      form.patrolModel,
      form.chatModel,
      form.discoveryModel,
      form.autoFixModel,
      form.model,
    ]).find((model) => getProviderFromModelId(model) === provider);

  const handleCloseSetupModal = () => {
    setShowSetupModal(false);
    setSetupApiKey('');
  };

  const openEnableSetupModal = () => {
    setShowSetupModal(true);
  };

  const checkProviderHealth = async (
    provider: AIProvider,
    opts: { notify?: boolean; storeManualResult?: boolean } = {},
  ): Promise<ProviderTestResult> => {
    try {
      const result = await AIAPI.testProvider(provider, selectedModelForProviderTest(provider));
      const message = getProviderTestDiagnosticMessage(result);
      const normalizedResult: ProviderTestResult = {
        provider,
        success: result.success,
        message,
        model: result.model,
        cause: result.cause,
        summary: result.summary,
        recommendation: result.recommendation,
        action: result.action,
      };
      setProviderHealth(provider, {
        status: result.success ? 'ok' : 'error',
        message,
        model: result.model,
        cause: result.cause,
        summary: result.summary,
        recommendation: result.recommendation,
        action: result.action,
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
      setProviderHealth(provider, {
        status: 'error',
        message,
        model: undefined,
        cause: undefined,
        summary: undefined,
        recommendation: undefined,
        action: undefined,
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

    const configuredProviders = AI_PROVIDERS.filter((provider) =>
      isAIProviderConfigured(provider, current),
    );
    for (const provider of AI_PROVIDERS) {
      if (!configuredProviders.includes(provider)) {
        setProviderHealth(provider, {
          status: 'not_configured',
          message: '',
          model: undefined,
          cause: undefined,
          summary: undefined,
          recommendation: undefined,
          action: undefined,
        });
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
          setProviderHealth(provider, {
            status: 'checking',
            message: '',
            model: undefined,
            cause: undefined,
            summary: undefined,
            recommendation: undefined,
            action: undefined,
          });
          await checkProviderHealth(provider, { notify: false, storeManualResult: false });
        }),
      );
      setPreflightLastCheckedAt(Date.now());
    } finally {
      setPreflightRunning(false);
    }
  };

  // runPatrolToolPreflight verifies the configured Patrol provider+model
  // can actually call tools end-to-end. Distinct from runProviderPreflight,
  // which only confirms each provider's model catalog is reachable.
  //
  // Passes the form's pending patrolModel as a model override so clicking
  // Check Patrol model after changing the dropdown actually tests the operator's
  // pending selection, not whatever was previously saved. Empty form value
  // means "use the shared default" — the backend handles the fallback.
  const runPatrolToolPreflight = async () => {
    setPatrolPreflightRunning(true);
    try {
      const pendingModel = form.patrolModel.trim();
      const result = await runPatrolPreflight(pendingModel ? { model: pendingModel } : {});
      setPatrolPreflightResult(result);
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? error.message
          : 'Pulse could not run the Patrol tool-call preflight.';
      setPatrolPreflightResult({
        success: false,
        tool_call_observed: false,
        duration_ms: 0,
        message,
        summary:
          'Pulse could not reach the preflight endpoint. Check that the backend is running and you have settings-write permission.',
      });
    } finally {
      setPatrolPreflightRunning(false);
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
      hydratePatrolPreflightFromSettings(data);
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
      setPatrolPreflightResult(null);
    } finally {
      setLoading(false);
    }
  };

  const handleSetupSubmit = async () => {
    setSetupSaving(true);
    let payload: Record<string, unknown> = {};
    try {
      payload = { enabled: true };

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
      } else if (setupProvider() === 'zai') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Z.ai API key');
          return;
        }
        payload.zai_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'groq') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Groq API key');
          return;
        }
        payload.groq_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'mistral') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Mistral API key');
          return;
        }
        payload.mistral_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'cerebras') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Cerebras API key');
          return;
        }
        payload.cerebras_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'together') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Together AI API key');
          return;
        }
        payload.together_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'fireworks') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Fireworks AI API key');
          return;
        }
        payload.fireworks_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'gemini') {
        if (!setupApiKey().trim()) {
          notificationStore.error('Please enter your Google Gemini API key');
          return;
        }
        payload.gemini_api_key = setupApiKey().trim();
      } else if (setupProvider() === 'codex-subscription') {
        payload.codex_subscription_enabled = true;
      } else if (setupProvider() === 'claude-subscription') {
        payload.claude_subscription_enabled = true;
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
      hydratePatrolPreflightFromSettings(updated);
      void runProviderPreflight(updated);
      handleCloseSetupModal();
      // First-time configuration is the moment of maximum intent:
      // reveal the assistant entry points (the bootstrap only reads
      // the capability on page load) and open the Assistant so the
      // user meets the feature they just enabled.
      void aiChatStore.refreshEnabledFromServer();
      aiChatStore.open();
      const patrolReadinessMessage = getAISettingsPatrolReadinessSaveMessage(
        updated.patrol_readiness,
        'Pulse Intelligence enabled',
      );
      if (patrolReadinessMessage) {
        notificationStore.warning(patrolReadinessMessage);
      } else {
        notificationStore.success(
          'Pulse Intelligence enabled. This is the Assistant — ask it anything about your infrastructure.',
        );
      }
    } catch (error) {
      logger.error('[AISettings] Setup failed:', error);
      const providerFailure = resolveAISettingsSaveProviderFailure({
        error,
        payload,
        providerHealth,
        models: [
          form.model,
          form.chatModel,
          form.patrolModel,
          form.discoveryModel,
          form.autoFixModel,
        ],
      });
      notificationStore.error(
        getAISettingsSaveProviderFailureMessage(
          error instanceof Error ? error.message : 'Setup failed',
          providerFailure,
        ),
      );
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
          (modelProvider === 'zai' && form.zaiApiKey.trim()) ||
          (modelProvider === 'groq' && form.groqApiKey.trim()) ||
          (modelProvider === 'mistral' && form.mistralApiKey.trim()) ||
          (modelProvider === 'cerebras' && form.cerebrasApiKey.trim()) ||
          (modelProvider === 'together' && form.togetherApiKey.trim()) ||
          (modelProvider === 'fireworks' && form.fireworksApiKey.trim()) ||
          (modelProvider === 'gemini' && form.geminiApiKey.trim()) ||
          (modelProvider === 'codex-subscription' && form.codexSubscriptionEnabled) ||
          (modelProvider === 'claude-subscription' && form.claudeSubscriptionEnabled) ||
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
    let payload: Record<string, unknown> = {};
    try {
      payload = {
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
      if (form.patrolAlertTriggers !== (settings()?.patrol_alert_triggers_enabled ?? true)) {
        payload.patrol_alert_triggers_enabled = form.patrolAlertTriggers;
      }
      if (form.patrolAnomalyTriggers !== (settings()?.patrol_anomaly_triggers_enabled ?? true)) {
        payload.patrol_anomaly_triggers_enabled = form.patrolAnomalyTriggers;
      }
      if (
        form.patrolAlertTriggerMinSeverity !==
        (settings()?.patrol_alert_trigger_min_severity ?? 'critical')
      ) {
        payload.patrol_alert_trigger_min_severity = form.patrolAlertTriggerMinSeverity;
      }
      if (
        form.patrolFindingNotifications !==
        (settings()?.patrol_finding_notifications_enabled ?? true)
      ) {
        payload.patrol_finding_notifications_enabled = form.patrolFindingNotifications;
      }
      if (
        form.patrolFindingNotifyMinSeverity !==
        (settings()?.patrol_finding_notify_min_severity ?? 'warning')
      ) {
        payload.patrol_finding_notify_min_severity = form.patrolFindingNotifyMinSeverity;
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
      if (form.discoveryModel !== (settings()?.discovery_model || '')) {
        payload.discovery_model = form.discoveryModel;
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
      if (form.zaiApiKey.trim()) {
        payload.zai_api_key = form.zaiApiKey.trim();
      }
      if (form.groqApiKey.trim()) {
        payload.groq_api_key = form.groqApiKey.trim();
      }
      if (form.mistralApiKey.trim()) {
        payload.mistral_api_key = form.mistralApiKey.trim();
      }
      if (form.cerebrasApiKey.trim()) {
        payload.cerebras_api_key = form.cerebrasApiKey.trim();
      }
      if (form.togetherApiKey.trim()) {
        payload.together_api_key = form.togetherApiKey.trim();
      }
      if (form.fireworksApiKey.trim()) {
        payload.fireworks_api_key = form.fireworksApiKey.trim();
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
      if (form.ollamaKeepAlive.trim() !== (settings()?.ollama_keep_alive ?? '30s')) {
        payload.ollama_keep_alive = form.ollamaKeepAlive.trim();
      }
      if (form.openaiBaseUrl !== (settings()?.openai_base_url || '')) {
        payload.openai_base_url = form.openaiBaseUrl.trim();
      }
      if (form.zaiBaseUrl !== (settings()?.zai_base_url || '')) {
        payload.zai_base_url = form.zaiBaseUrl.trim();
      }
      if (form.codexSubscriptionEnabled !== Boolean(settings()?.codex_subscription_enabled)) {
        payload.codex_subscription_enabled = form.codexSubscriptionEnabled;
      }
      if (form.claudeSubscriptionEnabled !== Boolean(settings()?.claude_subscription_enabled)) {
        payload.claude_subscription_enabled = form.claudeSubscriptionEnabled;
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

      payload.discovery_enabled = form.discoveryEnabled;
      payload.discovery_interval_hours = form.discoveryIntervalHours;

      const updated = await AIAPI.updateSettings(payload);
      setSettings(updated);
      resetForm(updated);
      syncModelCatalogForSettings(updated);
      hydratePatrolPreflightFromSettings(updated);
      void runProviderPreflight(updated);
      void aiChatStore.refreshEnabledFromServer();
      const savedLabel = options.savedLabel ?? 'Provider & Models settings saved';
      const patrolReadinessMessage = getAISettingsPatrolReadinessSaveMessage(
        updated.patrol_readiness,
        savedLabel,
      );
      if (patrolReadinessMessage) {
        notificationStore.warning(patrolReadinessMessage);
      } else {
        notificationStore.success(savedLabel);
      }
    } catch (error) {
      logger.error('[AISettings] Failed to save settings:', error);
      const providerFailure = resolveAISettingsSaveProviderFailure({
        error,
        payload,
        providerHealth,
        models: [
          selectedModel,
          form.chatModel,
          form.patrolModel,
          form.discoveryModel,
          form.autoFixModel,
        ],
      });
      notificationStore.error(
        getAISettingsSaveProviderFailureMessage(
          error instanceof Error ? error.message : '',
          providerFailure,
          options.saveErrorFallback,
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    try {
      const result = await AIAPI.testConnection();
      const message = getProviderTestDiagnosticMessage(result);
      if (result.success) {
        notificationStore.success(message);
      } else {
        notificationStore.error(message);
      }
    } catch (error) {
      logger.error('[AISettings] Test failed:', error);
      notificationStore.error(error instanceof Error ? error.message : 'Connection test failed');
    } finally {
      setTesting(false);
    }
  };

  const handleRunDiscoveryRefresh = async () => {
    setDiscoveryRunRunning(true);
    try {
      const result = await runDiscoveryRefresh();
      if (result.failed_count > 0) {
        notificationStore.warning(
          `Discovery refresh finished: ${result.discovered_count} refreshed, ${result.failed_count} failed.`,
        );
      } else if (result.candidate_count === 0) {
        notificationStore.info(
          'Discovery refresh finished: no new, changed, stale, or repairable workloads.',
        );
      } else {
        notificationStore.success(
          `Discovery refresh finished: ${result.discovered_count} workload${result.discovered_count === 1 ? '' : 's'} refreshed.`,
        );
      }
    } catch (error) {
      logger.error('[AISettings] Discovery refresh failed:', error);
      notificationStore.error(
        error instanceof Error ? error.message : 'Unable to run discovery refresh.',
      );
    } finally {
      setDiscoveryRunRunning(false);
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
      current?.zai_configured,
      current?.groq_configured,
      current?.mistral_configured,
      current?.cerebras_configured,
      current?.together_configured,
      current?.fireworks_configured,
      current?.gemini_configured,
      current?.codex_subscription_enabled,
      current?.claude_subscription_enabled,
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
      if (provider === 'zai') clearPayload.clear_zai_key = true;
      if (provider === 'groq') clearPayload.clear_groq_key = true;
      if (provider === 'mistral') clearPayload.clear_mistral_key = true;
      if (provider === 'cerebras') clearPayload.clear_cerebras_key = true;
      if (provider === 'together') clearPayload.clear_together_key = true;
      if (provider === 'fireworks') clearPayload.clear_fireworks_key = true;
      if (provider === 'gemini') clearPayload.clear_gemini_key = true;
      if (provider === 'ollama') clearPayload.clear_ollama_url = true;
      if (provider === 'codex-subscription') clearPayload.codex_subscription_enabled = false;
      if (provider === 'claude-subscription') clearPayload.claude_subscription_enabled = false;

      await AIAPI.updateSettings(clearPayload);
      const newSettings = await AIAPI.getSettings();
      setSettings(newSettings);
      syncModelCatalogForSettings(newSettings);
      void runProviderPreflight(newSettings);

      if (provider === 'anthropic') setForm('anthropicApiKey', '');
      if (provider === 'openai') setForm('openaiApiKey', '');
      if (provider === 'openrouter') setForm('openrouterApiKey', '');
      if (provider === 'deepseek') setForm('deepseekApiKey', '');
      if (provider === 'zai') setForm('zaiApiKey', '');
      if (provider === 'groq') setForm('groqApiKey', '');
      if (provider === 'mistral') setForm('mistralApiKey', '');
      if (provider === 'cerebras') setForm('cerebrasApiKey', '');
      if (provider === 'together') setForm('togetherApiKey', '');
      if (provider === 'fireworks') setForm('fireworksApiKey', '');
      if (provider === 'gemini') setForm('geminiApiKey', '');
      if (provider === 'ollama') setForm('ollamaBaseUrl', '');
      if (provider === 'codex-subscription') setForm('codexSubscriptionEnabled', false);
      if (provider === 'claude-subscription') setForm('claudeSubscriptionEnabled', false);

      notificationStore.success(`${provider} credentials cleared`);
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
      void aiChatStore.refreshEnabledFromServer();
      notificationStore.success(
        newValue
          ? 'Pulse Intelligence enabled. Ask the Assistant anything from the sparkles button on the right edge.'
          : 'Pulse Intelligence disabled',
      );
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
    if (hasConfiguredProvider()) {
      await handleEnabledToggle(true);
      return;
    }
    openEnableSetupModal();
  };

  onMount(() => {
    loadRuntimeCapabilities();

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
    alertAnalysisLocked,
    autoFixLocked,
    availableModels,
    chatSessions,
    chatSessionsError,
    chatSessionsLoading,
    discoveryRunRunning,
    expandedProviders,
    form,
    formatSessionLabel,
    handleClearProvider,
    handleCloseSetupModal,
    handleEnableRequest,
    handleEnabledToggle,
    handleRunDiscoveryRefresh,
    handleSave,
    handleSessionSummarize,
    handleSetupSubmit,
    handleTest,
    handleTestProvider,
    hasConfiguredProvider,
    loadChatSessions,
    loading,
    loadError,
    loadModels,
    loadSettings,
    modelsError,
    modelsLoading,
    patrolPreflightResult,
    patrolPreflightRunning,
    preflightLastCheckedAt,
    preflightRunning,
    providerHealth,
    providerIssueCount,
    providerTestResult,
    resetForm,
    runProviderPreflight,
    runPatrolToolPreflight,
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
    setShowDiscoverySettings,
    setShowSetupModal,
    settings,
    settingsReadiness,
    setupApiKey,
    setupOllamaUrl,
    setupProvider,
    setupSaving,
    showAdvancedModels,
    showChatMaintenance,
    showDiscoverySettings,
    showSetupModal,
    showUpgradePrompts,
    testing,
    testingProvider,
    upgradeAutofixDestination,
  };
};

export type AISettingsState = ReturnType<typeof useAISettingsState>;
