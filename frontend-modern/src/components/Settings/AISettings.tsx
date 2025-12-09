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
import { PROVIDER_NAMES, PROVIDER_DESCRIPTIONS, DEFAULT_MODELS } from '@/types/ai';

const PROVIDERS: AIProvider[] = ['anthropic', 'openai', 'ollama', 'deepseek'];

export const AISettings: Component = () => {
  const [settings, setSettings] = createSignal<AISettingsType | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [startingOAuth, setStartingOAuth] = createSignal(false);
  const [disconnectingOAuth, setDisconnectingOAuth] = createSignal(false);
  const [exchangingCode, setExchangingCode] = createSignal(false);

  // OAuth flow state
  const [oauthAuthUrl, setOAuthAuthUrl] = createSignal<string | null>(null);
  const [oauthState, setOAuthState] = createSignal<string | null>(null);
  const [oauthCode, setOAuthCode] = createSignal('');

  const [form, setForm] = createStore({
    enabled: false,
    provider: 'anthropic' as AIProvider,
    apiKey: '',
    model: '',
    baseUrl: '',
    clearApiKey: false,
    autonomousMode: false,
    authMethod: 'api_key' as AuthMethod,
  });

  const resetForm = (data: AISettingsType | null) => {
    if (!data) {
      setForm({
        enabled: false,
        provider: 'anthropic',
        apiKey: '',
        model: DEFAULT_MODELS.anthropic,
        baseUrl: '',
        clearApiKey: false,
        autonomousMode: false,
        authMethod: 'api_key',
      });
      return;
    }

    setForm({
      enabled: data.enabled,
      provider: data.provider,
      apiKey: '',
      model: data.model || DEFAULT_MODELS[data.provider],
      baseUrl: data.base_url || '',
      clearApiKey: false,
      autonomousMode: data.autonomous_mode || false,
      authMethod: data.auth_method || 'api_key',
    });
  };

  const loadSettings = async () => {
    setLoading(true);
    try {
      const data = await AIAPI.getSettings();
      setSettings(data);
      resetForm(data);
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

  const handleProviderChange = (provider: AIProvider) => {
    setForm('provider', provider);
    // Update model to default for new provider if current model doesn't look like it belongs
    const currentModel = form.model;
    if (!currentModel || currentModel === DEFAULT_MODELS[settings()?.provider || 'anthropic']) {
      setForm('model', DEFAULT_MODELS[provider]);
    }
  };

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

  const handleStartOAuth = async () => {
    setStartingOAuth(true);
    try {
      const result = await AIAPI.startOAuth();
      // Store the auth URL and state for the user to visit manually
      setOAuthAuthUrl(result.auth_url);
      setOAuthState(result.state);
      setOAuthCode('');
      notificationStore.info('Click the link below to sign in, then paste the code back here', 5000);
    } catch (error) {
      logger.error('[AISettings] OAuth start failed:', error);
      const message = error instanceof Error ? error.message : 'Failed to start OAuth flow';
      notificationStore.error(message);
    } finally {
      setStartingOAuth(false);
    }
  };

  const handleExchangeCode = async () => {
    const code = oauthCode().trim();
    const state = oauthState();

    if (!code || !state) {
      notificationStore.error('Please enter the authorization code');
      return;
    }

    setExchangingCode(true);
    try {
      await AIAPI.exchangeOAuthCode(code, state);
      notificationStore.success('Successfully connected to Claude with your subscription!');
      // Clear OAuth flow state
      setOAuthAuthUrl(null);
      setOAuthState(null);
      setOAuthCode('');
      // Reload settings
      await loadSettings();
    } catch (error) {
      logger.error('[AISettings] OAuth code exchange failed:', error);
      const message = error instanceof Error ? error.message : 'Failed to exchange authorization code';
      notificationStore.error(message);
    } finally {
      setExchangingCode(false);
    }
  };

  const handleCancelOAuth = () => {
    setOAuthAuthUrl(null);
    setOAuthState(null);
    setOAuthCode('');
  };

  const handleDisconnectOAuth = async () => {
    if (!confirm('Are you sure you want to disconnect your Claude subscription? You will need to provide an API key to continue using AI features.')) {
      return;
    }

    setDisconnectingOAuth(true);
    try {
      await AIAPI.disconnectOAuth();
      notificationStore.success('Claude subscription disconnected');
      // Reload settings
      await loadSettings();
    } catch (error) {
      logger.error('[AISettings] OAuth disconnect failed:', error);
      const message = error instanceof Error ? error.message : 'Failed to disconnect OAuth';
      notificationStore.error(message);
    } finally {
      setDisconnectingOAuth(false);
    }
  };

  const needsApiKey = () => form.provider !== 'ollama' && (form.provider !== 'anthropic' || form.authMethod !== 'oauth');
  const showBaseUrl = () => form.provider === 'ollama' || form.provider === 'openai' || form.provider === 'deepseek';
  const isAnthropicWithOAuth = () => form.provider === 'anthropic' && form.authMethod === 'oauth';
  const showAuthMethodSelector = () => form.provider === 'anthropic';

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
            {/* Provider Selection */}
            <div class={formField}>
              <label class={labelClass()}>AI Provider</label>
              <div class="grid grid-cols-3 gap-3">
                <For each={PROVIDERS}>
                  {(provider) => (
                    <button
                      type="button"
                      class={`p-3 rounded-lg border-2 text-left transition-all ${form.provider === provider
                        ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                        : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                        }`}
                      onClick={() => handleProviderChange(provider)}
                      disabled={saving()}
                    >
                      <div class="font-medium text-sm text-gray-900 dark:text-gray-100">
                        {PROVIDER_NAMES[provider]}
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                        {PROVIDER_DESCRIPTIONS[provider]}
                      </div>
                    </button>
                  )}
                </For>
              </div>
            </div>

            {/* Authentication Method - only for Anthropic */}
            <Show when={showAuthMethodSelector()}>
              <div class={formField}>
                <label class={labelClass()}>Authentication Method</label>
                <div class="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    class={`p-3 rounded-lg border-2 text-left transition-all ${form.authMethod === 'api_key'
                      ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                      : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                      }`}
                    onClick={() => setForm('authMethod', 'api_key')}
                    disabled={saving()}
                  >
                    <div class="font-medium text-sm text-gray-900 dark:text-gray-100">
                      API Key
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      Pay per token usage
                    </div>
                  </button>
                  <button
                    type="button"
                    class={`p-3 rounded-lg border-2 text-left transition-all opacity-60 cursor-not-allowed ${form.authMethod === 'oauth'
                      ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/30'
                      : 'border-gray-200 dark:border-gray-700'
                      }`}
                    disabled={true}
                    title="OAuth with Claude Pro/Max subscription is not yet available. Anthropic currently restricts third-party OAuth access."
                  >
                    <div class="font-medium text-sm text-gray-900 dark:text-gray-100 flex items-center gap-1.5">
                      Claude Pro/Max
                      <span class="inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded">
                        Unavailable
                      </span>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      Use your subscription
                    </div>
                  </button>
                </div>
                <p class={formHelpText}>
                  {form.authMethod === 'api_key'
                    ? 'Pay-per-use API billing from console.anthropic.com'
                    : 'Use your Claude Pro ($20/mo) or Max ($100+/mo) subscription'}
                </p>
              </div>
            </Show>

            {/* OAuth Login/Status - shown when Anthropic + OAuth selected */}
            <Show when={isAnthropicWithOAuth()}>
              <div class={`${formField} p-4 rounded-lg border ${settings()?.oauth_connected
                ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800'
                : oauthAuthUrl()
                  ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800'
                  : 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800'
                }`}>
                {/* Connected state */}
                <Show when={settings()?.oauth_connected}>
                  <div class="flex items-center justify-between">
                    <div>
                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                        </svg>
                        Connected to Claude with your subscription
                      </div>
                      <p class="text-xs text-green-700 dark:text-green-300 mt-1">
                        AI requests use your Claude Pro/Max subscription limits instead of API billing.
                      </p>
                    </div>
                    <button
                      type="button"
                      class="px-3 py-1.5 text-xs border border-red-300 dark:border-red-700 text-red-700 dark:text-red-300 rounded hover:bg-red-50 dark:hover:bg-red-900/30 disabled:opacity-50"
                      onClick={handleDisconnectOAuth}
                      disabled={disconnectingOAuth() || saving()}
                    >
                      {disconnectingOAuth() ? 'Disconnecting...' : 'Disconnect'}
                    </button>
                  </div>
                </Show>

                {/* OAuth flow in progress - show URL and code input */}
                <Show when={!settings()?.oauth_connected && oauthAuthUrl()}>
                  <div class="space-y-4">
                    <div class="text-sm text-amber-800 dark:text-amber-200">
                      <strong>Step 1:</strong> Click the link below to sign in with your Anthropic account:
                    </div>
                    <a
                      href={oauthAuthUrl()!}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="block p-2 bg-white dark:bg-gray-800 rounded border border-amber-300 dark:border-amber-700 text-xs text-blue-600 dark:text-blue-400 hover:underline break-all"
                    >
                      {oauthAuthUrl()}
                    </a>
                    <div class="text-sm text-amber-800 dark:text-amber-200">
                      <strong>Step 2:</strong> After signing in, you'll see a code. Paste it here:
                    </div>
                    <div class="flex gap-2">
                      <input
                        type="text"
                        value={oauthCode()}
                        onInput={(e) => setOAuthCode(e.currentTarget.value)}
                        placeholder="Paste authorization code here..."
                        class="flex-1 px-3 py-2 text-sm border border-amber-300 dark:border-amber-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400"
                      />
                      <button
                        type="button"
                        class="px-4 py-2 bg-gradient-to-r from-purple-600 to-pink-600 text-white text-sm rounded-md hover:from-purple-700 hover:to-pink-700 disabled:opacity-50 disabled:cursor-not-allowed"
                        onClick={handleExchangeCode}
                        disabled={exchangingCode() || !oauthCode().trim()}
                      >
                        {exchangingCode() ? 'Connecting...' : 'Connect'}
                      </button>
                    </div>
                    <button
                      type="button"
                      class="text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                      onClick={handleCancelOAuth}
                    >
                      Cancel
                    </button>
                  </div>
                </Show>

                {/* Initial state - show sign in button */}
                <Show when={!settings()?.oauth_connected && !oauthAuthUrl()}>
                  <div class="text-center">
                    <div class="text-sm text-blue-800 dark:text-blue-200 mb-3">
                      <strong>Use your Claude subscription</strong>
                      <p class="text-xs mt-1 text-blue-700 dark:text-blue-300">
                        Sign in with your Anthropic account to use your Pro/Max subscription for AI features instead of API billing.
                      </p>
                    </div>
                    <button
                      type="button"
                      class="px-4 py-2 bg-gradient-to-r from-purple-600 to-pink-600 text-white rounded-md hover:from-purple-700 hover:to-pink-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 mx-auto"
                      onClick={handleStartOAuth}
                      disabled={startingOAuth() || saving()}
                    >
                      <Show when={startingOAuth()}>
                        <span class="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                      </Show>
                      <Show when={!startingOAuth()}>
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1" />
                        </svg>
                      </Show>
                      {startingOAuth() ? 'Starting...' : 'Sign in with Claude'}
                    </button>
                  </div>
                </Show>
              </div>
            </Show>

            {/* API Key - not shown for Ollama or when using OAuth */}
            <Show when={needsApiKey()}>
              <div class={formField}>
                <div class="flex items-center justify-between">
                  <label class={labelClass('mb-0')}>API Key</label>
                  <Show when={settings()?.api_key_set}>
                    <button
                      type="button"
                      class="text-xs text-purple-600 hover:underline dark:text-purple-300"
                      onClick={() => {
                        if (!saving()) {
                          setForm('apiKey', '');
                          setForm('clearApiKey', true);
                          notificationStore.info('API key will be cleared on save', 2500);
                        }
                      }}
                      disabled={saving()}
                    >
                      Clear stored key
                    </button>
                  </Show>
                </div>
                <input
                  type="password"
                  value={form.apiKey}
                  onInput={(event) => {
                    setForm('apiKey', event.currentTarget.value);
                    if (event.currentTarget.value.trim() !== '') {
                      setForm('clearApiKey', false);
                    }
                  }}
                  placeholder={
                    settings()?.api_key_set
                      ? '•••••••• (leave blank to keep existing)'
                      : `Enter ${PROVIDER_NAMES[form.provider]} API key`
                  }
                  class={controlClass()}
                  disabled={saving()}
                />
                <p class={formHelpText}>
                  {form.provider === 'anthropic'
                    ? 'Get your API key from console.anthropic.com'
                    : form.provider === 'deepseek'
                      ? 'Get your API key from platform.deepseek.com'
                      : 'Get your API key from platform.openai.com'}
                </p>
              </div>
            </Show>

            {/* Model */}
            <div class={formField}>
              <label class={labelClass()}>Model</label>
              <input
                type="text"
                value={form.model}
                onInput={(event) => setForm('model', event.currentTarget.value)}
                placeholder={DEFAULT_MODELS[form.provider]}
                class={controlClass()}
                disabled={saving()}
              />
              <p class={formHelpText}>
                {form.provider === 'anthropic'
                  ? 'e.g., claude-opus-4-5-20251101, claude-sonnet-4-20250514'
                  : form.provider === 'openai'
                    ? 'e.g., gpt-4o, gpt-4-turbo'
                    : form.provider === 'deepseek'
                      ? 'e.g., deepseek-chat, deepseek-coder'
                      : 'e.g., llama3, mixtral, codellama'}
              </p>
            </div>

            {/* Base URL - shown for Ollama (required) and OpenAI (optional) */}
            <Show when={showBaseUrl()}>
              <div class={formField}>
                <label class={labelClass()}>
                  {form.provider === 'ollama' ? 'Ollama Server URL' : 'API Base URL (optional)'}
                </label>
                <input
                  type="url"
                  value={form.baseUrl}
                  onInput={(event) => setForm('baseUrl', event.currentTarget.value)}
                  placeholder={
                    form.provider === 'ollama'
                      ? 'http://localhost:11434'
                      : form.provider === 'deepseek'
                        ? 'https://api.deepseek.com/chat/completions'
                        : 'https://api.openai.com/v1'
                  }
                  class={controlClass()}
                  disabled={saving()}
                />
                <p class={formHelpText}>
                  {form.provider === 'ollama'
                    ? 'URL where your Ollama server is running'
                    : form.provider === 'deepseek'
                      ? 'Custom endpoint (leave blank for default DeepSeek API)'
                      : 'Custom endpoint for Azure OpenAI or compatible APIs'}
                </p>
              </div>
            </Show>

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
              <span class="text-xs font-medium">
                {settings()?.configured
                  ? settings()?.oauth_connected
                    ? `Ready to use with ${settings()?.model} (via Claude subscription)`
                    : `Ready to use with ${settings()?.model}`
                  : form.authMethod === 'oauth' && form.provider === 'anthropic'
                    ? 'Sign in with your Claude subscription to enable AI features'
                    : needsApiKey()
                      ? 'API key required to enable AI features'
                      : 'Configure Ollama server URL to enable AI features'}
              </span>
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
