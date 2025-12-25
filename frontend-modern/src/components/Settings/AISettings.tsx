import { Component, Show, createSignal, onMount, For, createMemo } from 'solid-js';
import { createStore } from 'solid-js/store';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { AIAPI } from '@/api/ai';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import type { AISettings as AISettingsType, AIProvider, AuthMethod } from '@/types/ai';

// Providers are now configured via accordion sections, not a single-provider selector

// Provider display names for optgroup labels
const PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  deepseek: 'DeepSeek',
  gemini: 'Google Gemini',
  ollama: 'Ollama',
};

// Parse provider from model ID (format: "provider:model-name")
function getProviderFromModelId(modelId: string): string {
  const colonIndex = modelId.indexOf(':');
  if (colonIndex > 0) {
    return modelId.substring(0, colonIndex);
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
  const [settings, setSettings] = createSignal<AISettingsType | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);

  // Dynamic model list from provider API
  const [availableModels, setAvailableModels] = createSignal<{ id: string; name: string; description?: string }[]>([]);
  const [modelsLoading, setModelsLoading] = createSignal(false);

  // Accordion state for provider configuration sections
  const [expandedProviders, setExpandedProviders] = createSignal<Set<AIProvider>>(new Set(['anthropic']));

  // Per-provider test state
  const [testingProvider, setTestingProvider] = createSignal<string | null>(null);
  const [providerTestResult, setProviderTestResult] = createSignal<{ provider: string; success: boolean; message: string } | null>(null);
  const [licenseStatus, setLicenseStatus] = createSignal<LicenseStatus | null>(null);
  const hasAlertAnalysisFeature = createMemo(() => {
    const status = licenseStatus();
    if (!status) return true;
    return Boolean(status.valid && status.features?.includes('ai_alerts'));
  });
  const hasAutoFixFeature = createMemo(() => {
    const status = licenseStatus();
    if (!status) return true;
    return Boolean(status.valid && status.features?.includes('ai_autofix'));
  });
  const alertAnalysisLocked = createMemo(() => !hasAlertAnalysisFeature());
  const autoFixLocked = createMemo(() => !hasAutoFixFeature());

  // Auto-fix acknowledgement state (not persisted - must acknowledge each session)
  const [autoFixAcknowledged, setAutoFixAcknowledged] = createSignal(false);

  // First-time setup modal state
  const [showSetupModal, setShowSetupModal] = createSignal(false);
  const [setupProvider, setSetupProvider] = createSignal<'anthropic' | 'openai' | 'deepseek' | 'gemini' | 'ollama'>('anthropic');
  const [setupApiKey, setSetupApiKey] = createSignal('');
  const [setupOllamaUrl, setSetupOllamaUrl] = createSignal('http://localhost:11434');
  const [setupSaving, setSetupSaving] = createSignal(false);

  // UI state for collapsible sections - START COLLAPSED for compact view
  const [showAdvancedModels, setShowAdvancedModels] = createSignal(false);
  const [showPatrolSettings, setShowPatrolSettings] = createSignal(false);

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
    autonomousMode: false,
    authMethod: 'api_key' as AuthMethod,
    patrolIntervalMinutes: 360, // 6 hours default
    alertTriggeredAnalysis: true,
    patrolAutoFix: false,
    // Multi-provider credentials
    anthropicApiKey: '',
    openaiApiKey: '',
    deepseekApiKey: '',
    geminiApiKey: '',
    ollamaBaseUrl: 'http://localhost:11434',
    openaiBaseUrl: '',
    // Cost controls
    costBudgetUSD30d: '',
    // Request timeout (seconds) - for slow Ollama hardware
    requestTimeoutSeconds: 300,
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
        autonomousMode: false,
        authMethod: 'api_key',
        patrolIntervalMinutes: 360, // 6 hours default
        alertTriggeredAnalysis: true,
        patrolAutoFix: false,
        // Multi-provider - empty by default
        anthropicApiKey: '',
        openaiApiKey: '',
        deepseekApiKey: '',
        geminiApiKey: '',
        ollamaBaseUrl: 'http://localhost:11434',
        openaiBaseUrl: '',
        costBudgetUSD30d: '',
        requestTimeoutSeconds: 300,
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
      autonomousMode: data.autonomous_mode || false,
      authMethod: data.auth_method || 'api_key',
      patrolIntervalMinutes: data.patrol_interval_minutes ?? 360, // Use minutes, default to 6hr
      alertTriggeredAnalysis: data.alert_triggered_analysis !== false, // default to true
      patrolAutoFix: data.patrol_auto_fix || false, // default to false (observe only)
      // Multi-provider - never load actual keys from server (security), just track if configured
      anthropicApiKey: '', // Always empty - we only show if configured
      openaiApiKey: '',
      deepseekApiKey: '',
      geminiApiKey: '',
      ollamaBaseUrl: data.ollama_base_url || 'http://localhost:11434',
      openaiBaseUrl: data.openai_base_url || '',
      costBudgetUSD30d:
        typeof data.cost_budget_usd_30d === 'number' && data.cost_budget_usd_30d > 0
          ? String(data.cost_budget_usd_30d)
          : '',
      requestTimeoutSeconds: data.request_timeout_seconds ?? 300,
    });

    // Auto-expand providers that are configured
    const configured = new Set<AIProvider>();
    if (data.anthropic_configured) configured.add('anthropic');
    if (data.openai_configured) configured.add('openai');
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
    } catch (error) {
      logger.error('[AISettings] Failed to load settings:', error);
      notificationStore.error('Failed to load AI settings');
      setSettings(null);
      resetForm(null);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadSettings();
    void (async () => {
      try {
        const status = await LicenseAPI.getStatus();
        setLicenseStatus(status);
      } catch (err) {
        logger.debug('Failed to load license status for AI gating', err);
        setLicenseStatus(null);
      }
    })();

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
  });

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

      // Include autonomous mode if changed
      if (form.autonomousMode !== settings()?.autonomous_mode) {
        payload.autonomous_mode = form.autonomousMode;
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

      const updated = await AIAPI.updateSettings(payload);
      setSettings(updated);
      resetForm(updated);
      notificationStore.success('AI settings saved');
    } catch (error) {
      logger.error('[AISettings] Failed to save settings:', error);
      const message = error instanceof Error ? error.message : 'Failed to save AI settings';
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
    setTestingProvider(provider);
    setProviderTestResult(null);
    try {
      const result = await AIAPI.testProvider(provider);
      setProviderTestResult(result);
      if (result.success) {
        notificationStore.success(`${provider}: ${result.message}`);
      } else {
        notificationStore.error(`${provider}: ${result.message}`);
      }
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
    const configuredCount = [s?.anthropic_configured, s?.openai_configured, s?.deepseek_configured, s?.gemini_configured, s?.ollama_configured].filter(Boolean).length;
    const isLastProvider = configuredCount === 1 && isProviderConfigured(provider, s);

    // Check if current model uses this provider
    const currentModel = form.model.trim();
    const modelUsesProvider = currentModel && getProviderFromModelId(currentModel) === provider;

    let confirmMessage = `Clear ${PROVIDER_DISPLAY_NAMES[provider] || provider} credentials?`;
    if (isLastProvider) {
      confirmMessage = `⚠️ This is your only configured provider! Clearing it will disable AI until you configure another provider. Continue?`;
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
      if (provider === 'deepseek') clearPayload.clear_deepseek_key = true;
      if (provider === 'gemini') clearPayload.clear_gemini_key = true;
      if (provider === 'ollama') clearPayload.clear_ollama_url = true;

      await AIAPI.updateSettings(clearPayload);

      // Reload settings to reflect the change
      const newSettings = await AIAPI.getSettings();
      setSettings(newSettings);

      // Clear the local form field
      if (provider === 'anthropic') setForm('anthropicApiKey', '');
      if (provider === 'openai') setForm('openaiApiKey', '');
      if (provider === 'deepseek') setForm('deepseekApiKey', '');
      if (provider === 'gemini') setForm('geminiApiKey', '');
      if (provider === 'ollama') setForm('ollamaBaseUrl', '');

      notificationStore.success(`${provider} credentials cleared`);
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
      <Card
        padding="none"
        class="overflow-hidden border border-gray-200 dark:border-gray-700"
        border={false}
      >
        <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
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
            </div>
            <SectionHeader
              title="AI Assistant"
              description="Configure AI-powered infrastructure analysis"
              size="sm"
              class="flex-1"
            />
            {/* Toggle with first-time setup flow */}
            {(() => {
              const s = settings();
              const hasConfiguredProvider = s && (s.anthropic_configured || s.openai_configured || s.deepseek_configured || s.ollama_configured);

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
                      notificationStore.success(newValue ? 'AI Assistant enabled' : 'AI Assistant disabled');
                    } catch (error) {
                      // Revert on failure
                      setForm('enabled', !newValue);
                      logger.error('[AISettings] Failed to toggle AI:', error);
                      const message = error instanceof Error ? error.message : 'Failed to update AI setting';
                      notificationStore.error(message);
                    }
                  }}
                  disabled={loading() || saving()}
                  containerClass="items-center gap-2"
                  label={
                    <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
                      {form.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  }
                />
              );
            })()}
          </div>
        </div>

        <form class="p-6 space-y-6" onSubmit={handleSave}>
          <Show when={loading()}>
            <div class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-300">
              <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
              Loading AI settings...
            </div>
          </Show>

          <Show when={!loading()}>
            <div class="space-y-6">
              {/* Default Model Selection - Always visible */}
              <div class={formField}>
                <div class="flex items-center justify-between mb-1">
                  <label class={labelClass()}>
                    Default Model
                    {modelsLoading() && <span class="ml-2 text-xs text-gray-500">(loading...)</span>}
                  </label>
                  <button
                    type="button"
                    onClick={loadModels}
                    disabled={modelsLoading()}
                    class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50 flex items-center gap-1"
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
                        <optgroup label={`⚠️ ${PROVIDER_DISPLAY_NAMES[provider] || provider} (not configured)`}>
                          <For each={models}>
                            {(model) => (
                              <option value={model.id} selected={model.id === form.model} class="text-gray-400">
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
              <div class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <button
                  type="button"
                  class="w-full px-3 py-2 flex items-center justify-between bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors text-left"
                  onClick={() => setShowAdvancedModels(!showAdvancedModels())}
                >
                  <div class="flex items-center gap-2">
                    <svg class="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                    </svg>
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">Advanced Model Selection</span>
                    <Show when={form.chatModel || form.patrolModel}>
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Customized</span>
                    </Show>
                  </div>
                  <svg class={`w-4 h-4 text-gray-500 transition-transform ${showAdvancedModels() ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showAdvancedModels()}>
                  <div class="px-3 py-3 bg-white dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-3">
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      Override the default model for specific tasks. Leave empty to use the default.
                    </p>
                    {/* Chat Model */}
                    <div>
                      <label class="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Chat Model (Interactive)</label>
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
                      <label class="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">Patrol Model (Background)</label>
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

              {/* AI Provider Configuration - Configure API keys for all providers */}
              <div class={`${formField} p-5 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40`}>
                <div class="mb-3">
                  <h4 class="font-medium text-gray-900 dark:text-white flex items-center gap-2">
                    <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                    </svg>
                    AI Provider Configuration
                  </h4>
                  <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                    Configure API keys for each AI provider you want to use. Models from all configured providers will appear in the model selectors.
                  </p>
                </div>

                {/* Provider Accordions */}
                <div class="space-y-2">
                  {/* Anthropic */}
                  <div class={`border rounded-lg overflow-hidden ${settings()?.anthropic_configured ? 'border-green-300 dark:border-green-700' : 'border-gray-200 dark:border-gray-700'}`}>
                    <button
                      type="button"
                      class="w-full px-3 py-2 flex items-center justify-between bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('anthropic') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('anthropic')}>
                      <div class="px-3 py-3 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-2">
                        <input
                          type="password"
                          value={form.anthropicApiKey}
                          onInput={(e) => setForm('anthropicApiKey', e.currentTarget.value)}
                          placeholder={settings()?.anthropic_configured ? '••••••••••• (configured)' : 'sk-ant-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-gray-500">
                            <a href="https://console.anthropic.com/settings/keys" target="_blank" rel="noopener" class="text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.anthropic_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('anthropic')}
                                disabled={testingProvider() === 'anthropic' || saving()}
                                class="px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'anthropic' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('anthropic')}
                                disabled={saving()}
                                class="px-2 py-1 text-xs bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 rounded hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
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
                  <div class={`border rounded-lg overflow-hidden ${settings()?.openai_configured ? 'border-green-300 dark:border-green-700' : 'border-gray-200 dark:border-gray-700'}`}>
                    <button
                      type="button"
                      class="w-full px-3 py-2 flex items-center justify-between bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('openai') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('openai')}>
                      <div class="px-3 py-3 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-2">
                        <input
                          type="password"
                          value={form.openaiApiKey}
                          onInput={(e) => setForm('openaiApiKey', e.currentTarget.value)}
                          placeholder={settings()?.openai_configured ? '••••••••••• (configured)' : 'sk-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <input
                          type="url"
                          value={form.openaiBaseUrl}
                          onInput={(e) => setForm('openaiBaseUrl', e.currentTarget.value)}
                          placeholder="Custom base URL (optional, for Azure OpenAI)"
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-gray-500">
                            <a href="https://platform.openai.com/api-keys" target="_blank" rel="noopener" class="text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.openai_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('openai')}
                                disabled={testingProvider() === 'openai' || saving()}
                                class="px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'openai' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('openai')}
                                disabled={saving()}
                                class="px-2 py-1 text-xs bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 rounded hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
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

                  {/* DeepSeek */}
                  <div class={`border rounded-lg overflow-hidden ${settings()?.deepseek_configured ? 'border-green-300 dark:border-green-700' : 'border-gray-200 dark:border-gray-700'}`}>
                    <button
                      type="button"
                      class="w-full px-3 py-2 flex items-center justify-between bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('deepseek') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('deepseek')}>
                      <div class="px-3 py-3 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-2">
                        <input
                          type="password"
                          value={form.deepseekApiKey}
                          onInput={(e) => setForm('deepseekApiKey', e.currentTarget.value)}
                          placeholder={settings()?.deepseek_configured ? '••••••••••• (configured)' : 'sk-...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-gray-500">
                            <a href="https://platform.deepseek.com/api_keys" target="_blank" rel="noopener" class="text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.deepseek_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('deepseek')}
                                disabled={testingProvider() === 'deepseek' || saving()}
                                class="px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'deepseek' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('deepseek')}
                                disabled={saving()}
                                class="px-2 py-1 text-xs bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 rounded hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
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
                  <div class={`border rounded-lg overflow-hidden ${settings()?.gemini_configured ? 'border-green-300 dark:border-green-700' : 'border-gray-200 dark:border-gray-700'}`}>
                    <button
                      type="button"
                      class="w-full px-3 py-2 flex items-center justify-between bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('gemini') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('gemini')}>
                      <div class="px-3 py-3 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-2">
                        <input
                          type="password"
                          value={form.geminiApiKey}
                          onInput={(e) => setForm('geminiApiKey', e.currentTarget.value)}
                          placeholder={settings()?.gemini_configured ? '••••••••••• (configured)' : 'AIza...'}
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-gray-500">
                            <a href="https://aistudio.google.com/app/apikey" target="_blank" rel="noopener" class="text-blue-600 dark:text-blue-400 hover:underline">Get API key →</a>
                          </p>
                          <Show when={settings()?.gemini_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('gemini')}
                                disabled={testingProvider() === 'gemini' || saving()}
                                class="px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'gemini' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('gemini')}
                                disabled={saving()}
                                class="px-2 py-1 text-xs bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 rounded hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
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
                  <div class={`border rounded-lg overflow-hidden ${settings()?.ollama_configured ? 'border-green-300 dark:border-green-700' : 'border-gray-200 dark:border-gray-700'}`}>
                    <button
                      type="button"
                      class="w-full px-3 py-2 flex items-center justify-between bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
                      </div>
                      <svg class={`w-4 h-4 transition-transform ${expandedProviders().has('ollama') ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>
                    <Show when={expandedProviders().has('ollama')}>
                      <div class="px-3 py-3 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-2">
                        <input
                          type="url"
                          value={form.ollamaBaseUrl}
                          onInput={(e) => setForm('ollamaBaseUrl', e.currentTarget.value)}
                          placeholder="http://localhost:11434"
                          class={controlClass()}
                          disabled={saving()}
                        />
                        <div class="flex items-center justify-between">
                          <p class="text-xs text-gray-500">
                            <a href="https://ollama.ai" target="_blank" rel="noopener" class="text-blue-600 dark:text-blue-400 hover:underline">Learn about Ollama →</a>
                            <span class="text-gray-400"> · Free & local</span>
                          </p>
                          <Show when={settings()?.ollama_configured}>
                            <div class="flex gap-1">
                              <button
                                type="button"
                                onClick={() => handleTestProvider('ollama')}
                                disabled={testingProvider() === 'ollama' || saving()}
                                class="px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
                              >
                                {testingProvider() === 'ollama' ? 'Testing...' : 'Test'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleClearProvider('ollama')}
                                disabled={saving()}
                                class="px-2 py-1 text-xs bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 rounded hover:bg-red-200 dark:hover:bg-red-800 disabled:opacity-50"
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

              {/* Autonomous Mode */}
              <div class={`${formField} p-4 rounded-lg border ${form.autonomousMode ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800' : 'bg-gray-50 dark:bg-gray-800/50 border-gray-200 dark:border-gray-700'}`}>
                <div class="flex items-start justify-between gap-4">
                  <div class="flex-1">
                    <label class={`${labelClass()} flex items-center gap-2`}>
                      Autonomous Mode
                      <Show when={form.autonomousMode}>
                        <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-amber-200 dark:bg-amber-800 text-amber-800 dark:text-amber-200 rounded">
                          ENABLED
                        </span>
                      </Show>
                      <Show when={autoFixLocked()}>
                        <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded">Pro</span>
                      </Show>
                    </label>
                    <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                      {form.autonomousMode
                        ? 'AI will execute all commands without asking for approval. Only enable if you trust your configured model.'
                        : 'AI will ask for approval before running commands that modify your system. Read-only commands (like df, ps, docker stats) run automatically.'}
                    </p>
                    <Show when={autoFixLocked()}>
                      <p class="text-[10px] text-amber-600 dark:text-amber-400 mt-1">
                        Pulse Pro required for autonomous mode.{' '}
                        <a
                          class="underline decoration-dotted"
                          href="https://pulserelay.pro"
                          target="_blank"
                          rel="noreferrer"
                        >
                          Upgrade
                        </a>
                      </p>
                    </Show>
                  </div>
                  <Toggle
                    checked={form.autonomousMode}
                    onChange={(event) => setForm('autonomousMode', event.currentTarget.checked)}
                    disabled={saving() || autoFixLocked()}
                  />
                </div>
              </div>

              {/* AI Patrol & Efficiency Settings - Collapsible */}
              <div class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <button
                  type="button"
                  class="w-full px-3 py-2 flex items-center justify-between bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 transition-colors text-left"
                  onClick={() => setShowPatrolSettings(!showPatrolSettings())}
                >
                  <div class="flex items-center gap-2">
                    <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                    </svg>
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">AI Patrol Settings</span>
                    {/* Summary badges */}
                    <Show when={form.patrolIntervalMinutes > 0}>
                      <span class="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">
                        {form.patrolIntervalMinutes >= 60 ? `${Math.floor(form.patrolIntervalMinutes / 60)}h` : `${form.patrolIntervalMinutes}m`}
                      </span>
                    </Show>
                    <Show when={form.patrolAutoFix}>
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-amber-100 dark:bg-amber-800 text-amber-700 dark:text-amber-300 rounded">Auto-Fix</span>
                    </Show>
                  </div>
                  <svg class={`w-4 h-4 text-gray-500 transition-transform ${showPatrolSettings() ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <Show when={showPatrolSettings()}>
                  <div class="px-3 py-3 bg-white dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 space-y-3">
                    {/* Patrol Interval - Compact */}
                    <div class="flex flex-col gap-1">
                      <div class="flex items-center gap-3">
                        <label class="text-xs font-medium text-gray-600 dark:text-gray-400 w-32 flex-shrink-0">Patrol Interval</label>
                        <input
                          type="number"
                          class={`w-20 px-2 py-1 text-sm border rounded bg-white dark:bg-gray-700 ${form.patrolIntervalMinutes > 0 && form.patrolIntervalMinutes < 10
                            ? 'border-red-300 dark:border-red-600'
                            : 'border-gray-300 dark:border-gray-600'
                            }`}
                          value={form.patrolIntervalMinutes}
                          onInput={(e) => {
                            const value = parseInt(e.currentTarget.value, 10);
                            if (!isNaN(value)) setForm('patrolIntervalMinutes', Math.max(0, value));
                          }}
                          min={0}
                          max={10080}
                          step={15}
                          disabled={saving()}
                        />
                        <span class="text-xs text-gray-500">min (0=off, 10+ to enable)</span>
                      </div>
                      <Show when={form.patrolIntervalMinutes > 0 && form.patrolIntervalMinutes < 10}>
                        <p class="text-xs text-red-500 ml-32 pl-3">Minimum interval is 10 minutes</p>
                      </Show>
                    </div>

                    {/* Alert Analysis Toggle - Compact */}
                    <div class="flex items-center justify-between gap-2">
                      <label class="text-xs font-medium text-gray-600 dark:text-gray-400 flex items-center gap-1.5">
                        Alert-Triggered Analysis
                        <span class="px-1 py-0.5 text-[9px] font-medium bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">Efficient</span>
                        <Show when={alertAnalysisLocked()}>
                          <span class="px-1 py-0.5 text-[9px] font-semibold bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded">Pro</span>
                        </Show>
                      </label>
                      <Toggle
                        checked={form.alertTriggeredAnalysis}
                        onChange={(event) => setForm('alertTriggeredAnalysis', event.currentTarget.checked)}
                        disabled={saving() || alertAnalysisLocked()}
                      />
                    </div>
                    <Show when={alertAnalysisLocked()}>
                      <p class="text-[10px] text-amber-600 dark:text-amber-400 mt-1">
                        Pulse Pro required for alert-triggered analysis.{' '}
                        <a
                          class="underline decoration-dotted"
                          href="https://pulserelay.pro"
                          target="_blank"
                          rel="noreferrer"
                        >
                          Upgrade
                        </a>
                      </p>
                    </Show>

                    {/* Auto-Fix Toggle - Compact with inline warning */}
                    <div class="pt-2 border-t border-gray-200 dark:border-gray-700">
                      <div class="flex items-center justify-between gap-2">
                        <label class="text-xs font-medium text-gray-600 dark:text-gray-400 flex items-center gap-1.5">
                          Auto-Fix Mode
                          <span class="px-1 py-0.5 text-[9px] font-medium bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded">Advanced</span>
                          <Show when={autoFixLocked()}>
                            <span class="px-1 py-0.5 text-[9px] font-semibold bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded">Pro</span>
                          </Show>
                        </label>
                        <Show when={autoFixAcknowledged() || form.patrolAutoFix}>
                          <Toggle
                            checked={form.patrolAutoFix}
                            onChange={(event) => setForm('patrolAutoFix', event.currentTarget.checked)}
                            disabled={saving() || autoFixLocked()}
                          />
                        </Show>
                        <Show when={!autoFixAcknowledged() && !form.patrolAutoFix}>
                          <button
                            type="button"
                            onClick={() => {
                              setAutoFixAcknowledged(true);
                              setForm('patrolAutoFix', true);
                            }}
                            disabled={saving() || autoFixLocked()}
                            class="px-2 py-1 text-xs bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded hover:bg-amber-200 dark:hover:bg-amber-800 disabled:opacity-60 disabled:cursor-not-allowed"
                          >
                            Enable
                          </button>
                        </Show>
                      </div>
                      <Show when={autoFixLocked()}>
                        <p class="text-[10px] text-amber-600 dark:text-amber-400 mt-1">
                          Pulse Pro required for auto-fix.{' '}
                          <a
                            class="underline decoration-dotted"
                            href="https://pulserelay.pro"
                            target="_blank"
                            rel="noreferrer"
                          >
                            Upgrade
                          </a>
                        </p>
                      </Show>
                      <Show when={!autoFixLocked() && !form.patrolAutoFix && !autoFixAcknowledged()}>
                        <p class="text-[10px] text-amber-600 dark:text-amber-400 mt-1">
                          ⚠️ AI will execute fixes without approval. Enable with caution.
                        </p>
                      </Show>
                      <Show when={!autoFixLocked() && form.patrolAutoFix}>
                        <p class="text-[10px] text-red-600 dark:text-red-400 mt-1 flex items-center gap-1">
                          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                          </svg>
                          Auto-Fix is ON. AI will attempt automatic remediation.
                        </p>
                      </Show>
                    </div>

                    {/* Auto-Fix Model - Only when enabled */}
                    <Show when={form.patrolAutoFix && !autoFixLocked()}>
                      <div class="flex items-center gap-3 pt-2 border-t border-gray-200 dark:border-gray-700">
                        <label class="text-xs font-medium text-gray-600 dark:text-gray-400 w-32 flex-shrink-0">Fix Model</label>
                        <Show when={availableModels().length > 0} fallback={
                          <input
                            type="text"
                            value={form.autoFixModel}
                            onInput={(e) => setForm('autoFixModel', e.currentTarget.value)}
                            placeholder="Use patrol model"
                            class="flex-1 px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700"
                            disabled={saving()}
                          />
                        }>
                          <select
                            value={form.autoFixModel}
                            onChange={(e) => setForm('autoFixModel', e.currentTarget.value)}
                            class="flex-1 px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700"
                            disabled={saving()}
                          >
                            <option value="">Use patrol model</option>
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
                    </Show>
                  </div>
                </Show>
              </div>

              {/* AI Cost Controls - Compact */}
              <div class="flex items-center gap-3 p-3 rounded-lg border border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-900/20">
                <svg class="w-4 h-4 text-emerald-600 dark:text-emerald-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <label class="text-xs font-medium text-gray-700 dark:text-gray-300">30-day Budget</label>
                <div class="relative flex-shrink-0">
                  <span class="absolute left-2 top-1/2 -translate-y-1/2 text-gray-500 dark:text-gray-400 text-xs">$</span>
                  <input
                    type="number"
                    class="w-24 pl-5 pr-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800"
                    value={form.costBudgetUSD30d}
                    onInput={(e) => setForm('costBudgetUSD30d', e.currentTarget.value)}
                    min={0}
                    step={1}
                    placeholder="0"
                    disabled={saving()}
                  />
                </div>
                <Show when={parseFloat(form.costBudgetUSD30d) > 0}>
                  <span class="text-xs text-gray-500">≈ ${(parseFloat(form.costBudgetUSD30d) / 30).toFixed(2)}/day</span>
                </Show>
                <Show when={!form.costBudgetUSD30d || parseFloat(form.costBudgetUSD30d) === 0}>
                  <span class="text-[10px] text-amber-600 dark:text-amber-400">💡 Set budget for alerts</span>
                </Show>
              </div>

              {/* Request Timeout - For slow Ollama hardware */}
              <div class="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40">
                <svg class="w-4 h-4 text-gray-500 dark:text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <label class="text-xs font-medium text-gray-700 dark:text-gray-300">Request Timeout</label>
                <input
                  type="number"
                  class="w-20 px-2 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800"
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
                <span class="text-xs text-gray-500">seconds</span>
                <Show when={form.requestTimeoutSeconds !== 300}>
                  <span class="text-[10px] text-blue-600 dark:text-blue-400">Custom</span>
                </Show>
                <Show when={form.requestTimeoutSeconds === 300}>
                  <span class="text-[10px] text-gray-400 dark:text-gray-500">default</span>
                </Show>
              </div>
              <p class="text-[10px] text-gray-500 dark:text-gray-400 -mt-4 ml-1">
                💡 Increase for slow Ollama hardware (default: 300s / 5 min)
              </p>


            </div>

            {/* Status indicator */}
            <Show when={settings()}>
              <div
                class={`flex items-center gap-2 p-3 rounded-lg ${settings()?.configured
                  ? 'bg-green-50 dark:bg-green-900/30 text-green-800 dark:text-green-200'
                  : 'bg-amber-50 dark:bg-amber-900/30 text-amber-800 dark:text-amber-200'
                  }`}
              >
                <div
                  class={`w-2 h-2 rounded-full ${settings()?.configured ? 'bg-green-500' : 'bg-amber-500'
                    }`}
                />
                <div class="flex-1">
                  <span class="text-xs font-medium">
                    {settings()?.configured
                      ? `Ready • ${settings()?.configured_providers?.length || 0} provider${(settings()?.configured_providers?.length || 0) !== 1 ? 's' : ''} • ${availableModels().length} models`
                      : 'Configure at least one AI provider above to enable AI features'}
                  </span>
                  <Show when={settings()?.configured && settings()?.model}>
                    <span class="text-xs opacity-75 ml-2">
                      • Default: {settings()?.model?.split(':').pop() || settings()?.model}
                    </span>
                  </Show>
                </div>
              </div>
            </Show>

            {/* Actions - sticky at bottom for easy access */}
            <div class="sticky bottom-0 bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700 -mx-6 px-6 py-4 mt-6 flex flex-wrap items-center justify-between gap-3">
              <Show when={settings()?.api_key_set || settings()?.oauth_connected}>
                <button
                  type="button"
                  class="px-4 py-2 text-sm border border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900/30 disabled:opacity-50 disabled:cursor-not-allowed"
                  onClick={handleTest}
                  disabled={testing() || saving() || loading()}
                >
                  {testing() ? 'Testing...' : 'Test Connection'}
                </button>
              </Show>
              <div class="flex gap-3 ml-auto">
                <button
                  type="button"
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                  onClick={() => resetForm(settings())}
                  disabled={saving() || loading()}
                >
                  Reset
                </button>
                <button
                  type="submit"
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={saving() || loading()}
                >
                  {saving() ? 'Saving...' : 'Save changes'}
                </button>
              </div>
            </div>
          </Show>
        </form>
      </Card>

      {/* First-time Setup Modal */}
      <Show when={showSetupModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-md w-full mx-4 overflow-hidden">
            {/* Header */}
            <div class="bg-gradient-to-r from-purple-600 to-pink-600 px-6 py-4">
              <h3 class="text-lg font-semibold text-white">Set Up AI Assistant</h3>
              <p class="text-purple-100 text-sm mt-1">Choose a provider to get started</p>
            </div>

            {/* Provider Selection */}
            <div class="p-6 space-y-4">
              <div class="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setSetupProvider('anthropic')}
                  class={`p-3 rounded-lg border-2 transition-all text-center ${setupProvider() === 'anthropic'
                    ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                    : 'border-gray-200 dark:border-gray-700 hover:border-purple-300'
                    }`}
                >
                  <div class="text-sm font-medium">Anthropic</div>
                  <div class="text-xs text-gray-500 mt-0.5">Claude</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('openai')}
                  class={`p-3 rounded-lg border-2 transition-all text-center ${setupProvider() === 'openai'
                    ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                    : 'border-gray-200 dark:border-gray-700 hover:border-purple-300'
                    }`}
                >
                  <div class="text-sm font-medium">OpenAI</div>
                  <div class="text-xs text-gray-500 mt-0.5">ChatGPT</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('deepseek')}
                  class={`p-3 rounded-lg border-2 transition-all text-center ${setupProvider() === 'deepseek'
                    ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                    : 'border-gray-200 dark:border-gray-700 hover:border-purple-300'
                    }`}
                >
                  <div class="text-sm font-medium">DeepSeek</div>
                  <div class="text-xs text-gray-500 mt-0.5">V3</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('gemini')}
                  class={`p-3 rounded-lg border-2 transition-all text-center ${setupProvider() === 'gemini'
                    ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                    : 'border-gray-200 dark:border-gray-700 hover:border-purple-300'
                    }`}
                >
                  <div class="text-sm font-medium">Gemini</div>
                  <div class="text-xs text-gray-500 mt-0.5">Google</div>
                </button>
                <button
                  type="button"
                  onClick={() => setSetupProvider('ollama')}
                  class={`p-3 rounded-lg border-2 transition-all text-center ${setupProvider() === 'ollama'
                    ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                    : 'border-gray-200 dark:border-gray-700 hover:border-purple-300'
                    }`}
                >
                  <div class="text-sm font-medium">Ollama</div>
                  <div class="text-xs text-gray-500 mt-0.5">Local</div>
                </button>
              </div>

              {/* API Key / URL Input */}
              <Show when={setupProvider() === 'ollama'} fallback={
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                    {setupProvider() === 'anthropic' ? 'Anthropic' : setupProvider() === 'openai' ? 'OpenAI' : setupProvider() === 'gemini' ? 'Google Gemini' : 'DeepSeek'} API Key
                  </label>
                  <input
                    type="password"
                    value={setupApiKey()}
                    onInput={(e) => setSetupApiKey(e.currentTarget.value)}
                    placeholder={setupProvider() === 'anthropic' ? 'sk-ant-...' : setupProvider() === 'gemini' ? 'AIza...' : 'sk-...'}
                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                  />
                  <p class="text-xs text-gray-500 mt-1.5">
                    <a
                      href={setupProvider() === 'anthropic'
                        ? 'https://console.anthropic.com/settings/keys'
                        : setupProvider() === 'openai'
                          ? 'https://platform.openai.com/api-keys'
                          : setupProvider() === 'gemini'
                            ? 'https://aistudio.google.com/app/apikey'
                            : 'https://platform.deepseek.com/api_keys'
                      }
                      target="_blank"
                      rel="noopener"
                      class="text-purple-600 hover:underline"
                    >
                      Get your API key →
                    </a>
                  </p>
                </div>
              }>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                    Ollama Server URL
                  </label>
                  <input
                    type="url"
                    value={setupOllamaUrl()}
                    onInput={(e) => setSetupOllamaUrl(e.currentTarget.value)}
                    placeholder="http://localhost:11434"
                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                  />
                  <p class="text-xs text-gray-500 mt-1.5">
                    Ollama runs locally - no API key needed
                  </p>
                </div>
              </Show>
            </div>

            {/* Footer */}
            <div class="px-6 py-4 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => {
                  setShowSetupModal(false);
                  setSetupApiKey('');
                }}
                class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
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
                    setShowSetupModal(false);
                    setSetupApiKey('');
                    notificationStore.success('AI Assistant enabled! You can customize settings below.');
                    // Load models after setup
                    loadModels();
                  } catch (error) {
                    logger.error('[AISettings] Setup failed:', error);
                    const message = error instanceof Error ? error.message : 'Setup failed';
                    notificationStore.error(message);
                  } finally {
                    setSetupSaving(false);
                  }
                }}
                class="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 flex items-center gap-2"
                disabled={setupSaving() || (setupProvider() !== 'ollama' && !setupApiKey().trim()) || (setupProvider() === 'ollama' && !setupOllamaUrl().trim())}
              >
                {setupSaving() && <span class="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />}
                Enable AI
              </button>
            </div>
          </div>
        </div>
      </Show>
    </>
  );
};

export default AISettings;
