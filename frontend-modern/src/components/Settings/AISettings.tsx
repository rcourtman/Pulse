import { Component, Show, createSignal, onMount, For } from 'solid-js';
import { createStore } from 'solid-js/store';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { AIAPI } from '@/api/ai';
import type { AISettings as AISettingsType, AIProvider, AuthMethod } from '@/types/ai';

// Providers are now configured via accordion sections, not a single-provider selector

// Provider display names for optgroup labels
const PROVIDER_DISPLAY_NAMES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  deepseek: 'DeepSeek',
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
  return 'ollama';
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

  // Auto-fix acknowledgement state (not persisted - must acknowledge each session)
  const [autoFixAcknowledged, setAutoFixAcknowledged] = createSignal(false);

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
    ollamaBaseUrl: 'http://localhost:11434',
    openaiBaseUrl: '',
    // Cost controls
    costBudgetUSD30d: '',
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
        ollamaBaseUrl: 'http://localhost:11434',
        openaiBaseUrl: '',
        costBudgetUSD30d: '',
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
      ollamaBaseUrl: data.ollama_base_url || 'http://localhost:11434',
      openaiBaseUrl: data.openai_base_url || '',
      costBudgetUSD30d:
        typeof data.cost_budget_usd_30d === 'number' && data.cost_budget_usd_30d > 0
          ? String(data.cost_budget_usd_30d)
          : '',
    });

    // Auto-expand providers that are configured
    const configured = new Set<AIProvider>();
    if (data.anthropic_configured) configured.add('anthropic');
    if (data.openai_configured) configured.add('openai');
    if (data.deepseek_configured) configured.add('deepseek');
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

    setSaving(true);
    try {
      const payload: Record<string, unknown> = {
        provider: form.provider,
        model: form.model.trim(),
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
      // Always include Ollama URL changes
      if (form.ollamaBaseUrl !== (settings()?.ollama_base_url || 'http://localhost:11434')) {
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
    if (!confirm(`Clear ${provider} credentials? You'll need to re-enter them to use this provider.`)) {
      return;
    }

    setSaving(true);
    try {
      const clearPayload: Record<string, boolean> = {};
      if (provider === 'anthropic') clearPayload.clear_anthropic_key = true;
      if (provider === 'openai') clearPayload.clear_openai_key = true;
      if (provider === 'deepseek') clearPayload.clear_deepseek_key = true;
      if (provider === 'ollama') clearPayload.clear_ollama_url = true;

      await AIAPI.updateSettings(clearPayload);

      // Reload settings to reflect the change
      const newSettings = await AIAPI.getSettings();
      setSettings(newSettings);

      // Clear the local form field
      if (provider === 'anthropic') setForm('anthropicApiKey', '');
      if (provider === 'openai') setForm('openaiApiKey', '');
      if (provider === 'deepseek') setForm('deepseekApiKey', '');
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
    <Card
      padding="none"
      class="overflow-hidden border border-gray-200 dark:border-gray-700"
      border={false}
    >
      <div class="bg-gradient-to-r from-purple-50 to-pink-50 dark:from-purple-900/20 dark:to-pink-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-purple-100 dark:bg-purple-900/40 rounded-lg">
            <svg
              class="w-5 h-5 text-purple-600 dark:text-purple-300"
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
          <Toggle
            checked={form.enabled}
            onChange={(event) => {
              setForm('enabled', event.currentTarget.checked);
            }}
            disabled={loading() || saving()}
            containerClass="items-center gap-2"
            label={
              <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
                {form.enabled ? 'Enabled' : 'Disabled'}
              </span>
            }
          />
        </div>
      </div>

      <form class="p-6 space-y-5" onSubmit={handleSave}>
        <div class="bg-purple-50 dark:bg-purple-900/30 border border-purple-200 dark:border-purple-800 rounded-lg p-3 text-xs text-purple-800 dark:text-purple-200">
          <p class="font-medium mb-1">AI Assistant helps you:</p>
          <ul class="space-y-0.5 list-disc pl-4">
            <li>Diagnose infrastructure issues with context-aware analysis</li>
            <li>Get remediation suggestions based on your specific environment</li>
            <li>Understand alerts and metrics with plain-language explanations</li>
          </ul>
        </div>

        <Show when={loading()}>
          <div class="flex items-center gap-3 text-sm text-gray-600 dark:text-gray-300">
            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
            Loading AI settings...
          </div>
        </Show>

        <Show when={!loading()}>
          <div class="space-y-4">
            {/* Model Selection */}
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
                  class="text-xs text-purple-600 dark:text-purple-400 hover:text-purple-800 dark:hover:text-purple-300 disabled:opacity-50 flex items-center gap-1"
                  title="Refresh model list from all configured providers"
                >
                  <svg class={`w-3 h-3 ${modelsLoading() ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                  Refresh Models
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
              <p class={formHelpText}>
                Main model used when no specific override is set. {availableModels().length === 0 && 'Save API key and refresh to see available models.'}
              </p>
            </div>

            {/* Chat Model Override */}
            <div class={formField}>
              <label class={labelClass()}>Chat Model (Interactive)</label>
              <Show when={availableModels().length > 0} fallback={
                <input
                  type="text"
                  value={form.chatModel}
                  onInput={(e) => setForm('chatModel', e.currentTarget.value)}
                  placeholder="Leave empty to use default model"
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
                  <option value="">Use default model ({form.model || 'not set'})</option>
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
              <p class={formHelpText}>
                Model for interactive AI chat. Use a more capable model for complex reasoning.
              </p>
            </div>

            {/* Patrol Model Override */}
            <div class={formField}>
              <label class={labelClass()}>Patrol Model (Background)</label>
              <Show when={availableModels().length > 0} fallback={
                <input
                  type="text"
                  value={form.patrolModel}
                  onInput={(e) => setForm('patrolModel', e.currentTarget.value)}
                  placeholder="Leave empty to use default model"
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
                  <option value="">Use default model ({form.model || 'not set'})</option>
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
              <p class={formHelpText}>
                Model for background patrol analysis. Use a cheaper/faster model to save tokens.
              </p>
            </div>

            {/* AI Provider Configuration - Configure API keys for all providers */}
            <div class={`${formField} p-4 rounded-lg border bg-gradient-to-br from-purple-50 to-indigo-50 dark:from-purple-900/20 dark:to-indigo-900/20 border-purple-200 dark:border-purple-800`}>
              <div class="mb-3">
                <h4 class="font-medium text-gray-900 dark:text-white flex items-center gap-2">
                  <svg class="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
                          <a href="https://console.anthropic.com/settings/keys" target="_blank" rel="noopener" class="text-purple-600 hover:underline">Get API key →</a>
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
                          <a href="https://platform.openai.com/api-keys" target="_blank" rel="noopener" class="text-purple-600 hover:underline">Get API key →</a>
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
                          <a href="https://platform.deepseek.com/api_keys" target="_blank" rel="noopener" class="text-purple-600 hover:underline">Get API key →</a>
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
                          <a href="https://ollama.ai" target="_blank" rel="noopener" class="text-purple-600 hover:underline">Learn about Ollama →</a>
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
                  </label>
                  <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                    {form.autonomousMode
                      ? 'AI will execute all commands without asking for approval. Only enable if you trust your configured model.'
                      : 'AI will ask for approval before running commands that modify your system. Read-only commands (like df, ps, docker stats) run automatically.'}
                  </p>
                </div>
                <Toggle
                  checked={form.autonomousMode}
                  onChange={(event) => setForm('autonomousMode', event.currentTarget.checked)}
                  disabled={saving()}
                />
              </div>
            </div>

            {/* AI Patrol & Efficiency Settings */}
            <div class={`${formField} p-4 rounded-lg border bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800`}>
              <div class="mb-3">
                <label class={`${labelClass()} flex items-center gap-2`}>
                  <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                  </svg>
                  AI Patrol & Token Efficiency
                </label>
                <p class="text-xs text-blue-700 dark:text-blue-300 mt-1">
                  Configure how AI monitors your infrastructure. Balance between coverage and token usage.
                </p>
              </div>

              {/* Patrol Interval */}
              <div class="space-y-3">
                <div>
                  <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                    Background Patrol Interval (minutes)
                  </label>
                  <div class="flex items-center gap-2">
                    <input
                      type="number"
                      class={controlClass()}
                      value={form.patrolIntervalMinutes}
                      onInput={(e) => {
                        const value = parseInt(e.currentTarget.value, 10);
                        if (!isNaN(value)) {
                          setForm('patrolIntervalMinutes', Math.max(0, value));
                        }
                      }}
                      min={0}
                      max={10080}
                      step={15}
                      disabled={saving()}
                      style={{ width: '120px' }}
                    />
                    <span class="text-xs text-gray-500 dark:text-gray-400">
                      {form.patrolIntervalMinutes === 0
                        ? 'Disabled'
                        : form.patrolIntervalMinutes >= 60
                          ? `${Math.floor(form.patrolIntervalMinutes / 60)}h ${form.patrolIntervalMinutes % 60 > 0 ? `${form.patrolIntervalMinutes % 60}m` : ''}`
                          : `${form.patrolIntervalMinutes}m`}
                    </span>
                  </div>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    Set to 0 to disable scheduled patrol. Minimum 10 minutes when enabled.
                  </p>
                </div>

                {/* Alert-Triggered Analysis Toggle */}
                <div class="flex items-start justify-between gap-4 pt-2">
                  <div class="flex-1">
                    <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 flex items-center gap-2">
                      Alert-Triggered Analysis
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                        TOKEN EFFICIENT
                      </span>
                    </label>
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      When enabled, AI automatically analyzes specific resources when alerts fire.
                      Uses minimal tokens since it only analyzes affected resources.
                    </p>
                  </div>
                  <Toggle
                    checked={form.alertTriggeredAnalysis}
                    onChange={(event) => setForm('alertTriggeredAnalysis', event.currentTarget.checked)}
                    disabled={saving()}
                  />
                </div>

                {/* Auto-Fix Mode Toggle */}
                <div class="flex items-start justify-between gap-4 pt-3 mt-3 border-t border-blue-200 dark:border-blue-800">
                  <div class="flex-1">
                    <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 flex items-center gap-2">
                      Auto-Fix Mode
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 rounded">
                        ADVANCED
                      </span>
                    </label>
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      When enabled, patrol can attempt automatic remediation of issues.
                      When disabled (default), patrol only observes and reports - it won't make changes.
                    </p>
                  </div>
                  <Toggle
                    checked={form.patrolAutoFix}
                    onChange={(event) => {
                      // Can only enable if acknowledged
                      if (event.currentTarget.checked && !autoFixAcknowledged()) {
                        return; // Prevent enabling without acknowledgement
                      }
                      setForm('patrolAutoFix', event.currentTarget.checked);
                    }}
                    disabled={saving() || (!form.patrolAutoFix && !autoFixAcknowledged())}
                  />
                </div>

                {/* Auto-Fix Warning & Acknowledgement */}
                <Show when={!form.patrolAutoFix}>
                  <div class="mt-3 p-3 bg-amber-50 dark:bg-amber-900/30 border border-amber-200 dark:border-amber-800 rounded-lg">
                    <div class="flex gap-2">
                      <svg class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                      </svg>
                      <div class="text-xs text-amber-800 dark:text-amber-200">
                        <p class="font-semibold mb-1">Before enabling Auto-Fix Mode:</p>
                        <ul class="list-disc pl-4 space-y-0.5 mb-2">
                          <li>AI will execute commands <strong>without asking for approval</strong></li>
                          <li>Actions may be <strong>irreversible</strong> (e.g., restarting services, clearing caches)</li>
                          <li>Recommended to test in non-production environments first</li>
                        </ul>
                        <label class="flex items-center gap-2 cursor-pointer mt-2 pt-2 border-t border-amber-200 dark:border-amber-700">
                          <input
                            type="checkbox"
                            checked={autoFixAcknowledged()}
                            onChange={(e) => setAutoFixAcknowledged(e.currentTarget.checked)}
                            class="w-4 h-4 rounded border-amber-400 text-amber-600 focus:ring-amber-500"
                          />
                          <span class="font-medium">I understand the risks and want to enable Auto-Fix</span>
                        </label>
                      </div>
                    </div>
                  </div>
                </Show>

                {/* Warning when enabled */}
                <Show when={form.patrolAutoFix}>
                  <div class="mt-3 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg">
                    <div class="flex gap-2">
                      <svg class="w-4 h-4 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                      </svg>
                      <div class="text-xs text-red-800 dark:text-red-200">
                        <p class="font-semibold">⚠️ Auto-Fix is ENABLED</p>
                        <p class="mt-1">AI patrol will automatically attempt to fix issues without asking for approval. Review findings regularly.</p>
                      </div>
                    </div>
                  </div>

                  {/* Auto-Fix Model Selector - shown when auto-fix is enabled */}
                  <div class="mt-3">
                    <label class={labelClass()}>Auto-Fix Model (Remediation)</label>
                    <Show when={availableModels().length > 0} fallback={
                      <input
                        type="text"
                        value={form.autoFixModel}
                        onInput={(e) => setForm('autoFixModel', e.currentTarget.value)}
                        placeholder="Leave empty to use patrol model"
                        class={controlClass()}
                        disabled={saving()}
                      />
                    }>
                      <select
                        value={form.autoFixModel}
                        onChange={(e) => setForm('autoFixModel', e.currentTarget.value)}
                        class={controlClass()}
                        disabled={saving()}
                      >
                        <option value="">Use patrol model ({form.patrolModel || form.model || 'not set'})</option>
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
                    <p class={formHelpText}>
                      Model for automatic remediation. Use a more capable model for better fix accuracy.
                    </p>
                  </div>
                </Show>
              </div>
            </div>

            {/* AI Cost Controls */}
            <div class={`${formField} p-4 rounded-lg border bg-emerald-50 dark:bg-emerald-900/20 border-emerald-200 dark:border-emerald-800`}>
              <div class="mb-2">
                <label class={`${labelClass()} flex items-center gap-2`}>
                  AI Cost Controls
                </label>
                <p class="text-xs text-emerald-700 dark:text-emerald-300 mt-1">
                  This budget is a cross-provider estimate for Pulse usage. Provider dashboards remain the source of truth for billing.
                </p>
              </div>

              <div>
                <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                  Budget alert (USD per 30 days)
                </label>
                <input
                  type="number"
                  class={controlClass()}
                  value={form.costBudgetUSD30d}
                  onInput={(e) => setForm('costBudgetUSD30d', e.currentTarget.value)}
                  min={0}
                  step={1}
                  placeholder="0 (disabled)"
                  disabled={saving()}
                  style={{ width: '180px' }}
                />
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Set to 0 to disable. Cost dashboard pro-rates for shorter ranges.
                </p>
              </div>
            </div>


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

          {/* Actions */}
          <div class="flex flex-wrap items-center justify-between gap-3 pt-4">
            <Show when={settings()?.api_key_set || settings()?.oauth_connected}>
              <button
                type="button"
                class="px-4 py-2 text-sm border border-purple-300 dark:border-purple-600 text-purple-700 dark:text-purple-300 rounded-md hover:bg-purple-50 dark:hover:bg-purple-900/30 disabled:opacity-50 disabled:cursor-not-allowed"
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
                class="px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={saving() || loading()}
              >
                {saving() ? 'Saving...' : 'Save changes'}
              </button>
            </div>
          </div>
        </Show>
      </form>
    </Card>
  );
};

export default AISettings;
