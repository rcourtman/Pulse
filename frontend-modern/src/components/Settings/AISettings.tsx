import { Component, For, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { AIProviderConfigurationSection } from '@/components/Settings/AIProviderConfigurationSection';
import { AISettingsDialogs } from '@/components/Settings/AISettingsDialogs';
import {
  groupModelsByProvider,
  isAIProviderConfigured,
  isModelProviderConfigured,
} from '@/components/Settings/aiSettingsModel';
import { useAISettingsState } from '@/components/Settings/useAISettingsState';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { formField, labelClass, controlClass } from '@/components/shared/Form';
import {
  getAIControlLevelBadgeClass,
  getAIControlLevelDescription,
  getAIControlLevelPanelClass,
  type AIControlLevel,
} from '@/utils/aiControlLevelPresentation';
import { getAIProviderDisplayName, getProviderFromModelId } from '@/utils/aiProviderPresentation';
import {
  getAIChatSessionsEmptyState,
  getAIChatSessionsLoadingState,
  getAISettingsLoadingState,
  getAISettingsLoadErrorMessage,
  getAISettingsRetryLabel,
} from '@/utils/aiSettingsPresentation';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';

export const AISettings: Component = () => {
  const navigate = useNavigate();
  const {
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
    loadChatSessions,
    loadError,
    loading,
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
    upgradeAutofixUrl,
  } = useAISettingsState();

  return (
    <>
      <SettingsPanel
        title="AI Services"
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
        action={(() => {
          return (
            <Toggle
              checked={form.enabled}
              onChange={async (event) => {
                const newValue = event.currentTarget.checked;
                if (newValue && !hasConfiguredProvider()) {
                  event.currentTarget.checked = false;
                  setShowSetupModal(true);
                  return;
                }
                await handleEnabledToggle(newValue);
              }}
              disabled={loading() || saving() || loadError()}
              containerClass="items-center gap-2"
              label={
                <span class="text-xs font-medium text-muted">
                  {form.enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          );
        })()}
        noPadding
      >
        <form class="divide-y divide-border" onSubmit={handleSave}>
          <Show when={loading()}>
            <div class="flex items-center gap-3 text-sm text-muted p-4 sm:p-6">
              <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
              {getAISettingsLoadingState().text}
            </div>
          </Show>

          <Show when={!loading() && loadError()}>
            <div class="flex items-center justify-between gap-3 p-4 sm:p-6 bg-red-50 dark:bg-red-900/30 border-b border-red-200 dark:border-red-800">
              <div class="flex items-center gap-2 text-sm text-red-700 dark:text-red-300">
                <svg
                  class="h-4 w-4 flex-shrink-0"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
                  />
                </svg>
                <span>{getAISettingsLoadErrorMessage()}</span>
              </div>
              <button
                type="button"
                class="flex-shrink-0 px-3 py-1.5 text-sm font-medium text-red-700 dark:text-red-300 border border-red-300 dark:border-red-700 rounded-md hover:bg-red-100 dark:hover:bg-red-900/50"
                onClick={() => loadSettings()}
              >
                {getAISettingsRetryLabel()}
              </button>
            </div>
          </Show>

          <Show when={!loading() && !loadError()}>
            <Show when={form.enabled}>
              <div class="p-4 sm:p-6">
                <div class="flex items-start gap-2 text-xs text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                  <svg
                    class="w-4 h-4 mt-0.5 shrink-0"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
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
            <div class="space-y-6 p-4 sm:p-6">
              {/* Default Model Selection - Always visible */}
              <div class={formField}>
                <div class="flex items-center justify-between mb-1">
                  <label class={labelClass()}>
                    Default Model
                    {modelsLoading() && (
                      <span class="ml-2 text-xs text-slate-500">(loading...)</span>
                    )}
                  </label>
                  <button
                    type="button"
                    onClick={loadModels}
                    disabled={modelsLoading()}
                    class="inline-flex min-h-10 sm:min-h-9 items-center gap-1 rounded-md px-2 py-1.5 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
                    title="Refresh model list from all configured providers"
                  >
                    <svg
                      class={`w-3 h-3 ${modelsLoading() ? 'animate-spin' : ''}`}
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                      />
                    </svg>
                    Refresh
                  </button>
                </div>
                <Show
                  when={availableModels().length > 0}
                  fallback={
                    <input
                      type="text"
                      value={form.model}
                      onInput={(e) => setForm('model', e.currentTarget.value)}
                      placeholder="Configure a provider below to see available models"
                      class={controlClass()}
                      disabled={saving()}
                    />
                  }
                >
                  <select
                    value={form.model}
                    onChange={(e) => setForm('model', e.currentTarget.value)}
                    class={controlClass()}
                    disabled={saving()}
                  >
                    <Show when={!form.model || !availableModels().some((m) => m.id === form.model)}>
                      <option value={form.model}>{form.model || 'Select a model...'}</option>
                    </Show>
                    {/* Show configured providers first */}
                    <For
                      each={Array.from(groupModelsByProvider(availableModels()).entries()).filter(
                        ([p]) => isAIProviderConfigured(p, settings()),
                      )}
                    >
                      {([provider, models]) => (
                        <optgroup label={getAIProviderDisplayName(provider) || provider}>
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
                    <For
                      each={Array.from(groupModelsByProvider(availableModels()).entries()).filter(
                        ([p]) => !isAIProviderConfigured(p, settings()),
                      )}
                    >
                      {([provider, models]) => (
                        <optgroup
                          label={`${getAIProviderDisplayName(provider) || provider} (not configured)`}
                        >
                          <For each={models}>
                            {(model) => (
                              <option
                                value={model.id}
                                selected={model.id === form.model}
                                class="text-slate-400"
                              >
                                {model.name || model.id.split(':').pop()}
                              </option>
                            )}
                          </For>
                        </optgroup>
                      )}
                    </For>
                  </select>
                </Show>
                {/* Warning when model loading failed */}
                <Show when={modelsError()}>
                  <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 flex items-center gap-1">
                    <svg
                      class="w-3.5 h-3.5 flex-shrink-0"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    Failed to load models: {modelsError()}. Enter a model ID manually (format:
                    provider:model-name) or click Refresh to retry.
                  </p>
                </Show>
                {/* Warning if selected model's provider is not configured */}
                <Show when={form.model && !isModelProviderConfigured(form.model, settings())}>
                  <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 flex items-center gap-1">
                    <svg
                      class="w-3.5 h-3.5 flex-shrink-0"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                    This model requires{' '}
                    {getAIProviderDisplayName(getProviderFromModelId(form.model)) ||
                      getProviderFromModelId(form.model)}{' '}
                    to be configured. Add an API key below or select a different model.
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
                    <svg
                      class="w-4 h-4 text-slate-500"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
                      />
                    </svg>
                    <span class="text-sm font-medium text-base-content">
                      Advanced Model Selection
                    </span>
                    <Show when={form.chatModel || form.patrolModel}>
                      <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                        Customized
                      </span>
                    </Show>
                  </div>
                  <svg
                    class={`w-4 h-4 text-slate-500 transition-transform ${showAdvancedModels() ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </button>
                <Show when={showAdvancedModels()}>
                  <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
                    <p class="text-xs text-muted">
                      Override the default model for specific tasks. Leave empty to use the default.
                    </p>
                    {/* Chat Model */}
                    <div>
                      <label class="block text-xs font-medium text-muted mb-0.5">
                        Chat Model (Interactive)
                      </label>
                      <p class="text-[11px] text-muted mb-1">
                        Used for chat and fix execution — a more capable model is recommended.
                      </p>
                      <Show
                        when={availableModels().length > 0}
                        fallback={
                          <input
                            type="text"
                            value={form.chatModel}
                            onInput={(e) => setForm('chatModel', e.currentTarget.value)}
                            placeholder="Use default model"
                            class={controlClass()}
                            disabled={saving()}
                          />
                        }
                      >
                        <select
                          value={form.chatModel}
                          onChange={(e) => setForm('chatModel', e.currentTarget.value)}
                          class={controlClass()}
                          disabled={saving()}
                        >
                          <option value="">
                            Use default ({form.model?.split(':').pop() || 'not set'})
                          </option>
                          <For
                            each={Array.from(groupModelsByProvider(availableModels()).entries())}
                          >
                            {([provider, models]) => (
                              <optgroup label={getAIProviderDisplayName(provider) || provider}>
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
                      <label class="block text-xs font-medium text-muted mb-0.5">
                        Patrol Model (Background)
                      </label>
                      <p class="text-[11px] text-muted mb-1">
                        Runs frequently for detection — a smaller, cheaper model keeps costs low.
                      </p>
                      <Show
                        when={availableModels().length > 0}
                        fallback={
                          <input
                            type="text"
                            value={form.patrolModel}
                            onInput={(e) => setForm('patrolModel', e.currentTarget.value)}
                            placeholder="Use default model"
                            class={controlClass()}
                            disabled={saving()}
                          />
                        }
                      >
                        <select
                          value={form.patrolModel}
                          onChange={(e) => setForm('patrolModel', e.currentTarget.value)}
                          class={controlClass()}
                          disabled={saving()}
                        >
                          <option value="">
                            Use default ({form.model?.split(':').pop() || 'not set'})
                          </option>
                          <For
                            each={Array.from(groupModelsByProvider(availableModels()).entries())}
                          >
                            {([provider, models]) => (
                              <optgroup label={getAIProviderDisplayName(provider) || provider}>
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

              <div class={formField}>
                <AIProviderConfigurationSection
                  settings={settings}
                  form={form}
                  setForm={setForm}
                  expandedProviders={expandedProviders}
                  setExpandedProviders={setExpandedProviders}
                  providerHealth={providerHealth}
                  preflightRunning={preflightRunning}
                  preflightLastCheckedAt={preflightLastCheckedAt}
                  providerIssueCount={providerIssueCount}
                  testingProvider={testingProvider}
                  providerTestResult={providerTestResult}
                  saving={saving}
                  runProviderPreflight={() => runProviderPreflight()}
                  handleTestProvider={handleTestProvider}
                  handleClearProvider={handleClearProvider}
                />
              </div>

              {/* Discovery Settings - Collapsible */}
              <div class="rounded-md border border-blue-200 dark:border-blue-800 overflow-hidden">
                <button
                  type="button"
                  class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-blue-50 dark:bg-blue-900 hover:bg-blue-100 dark:hover:bg-blue-900 transition-colors text-left"
                  onClick={() => setShowDiscoverySettings(!showDiscoverySettings())}
                >
                  <div class="flex items-center gap-2">
                    <svg
                      class="w-4 h-4 text-blue-600 dark:text-blue-400"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                      />
                    </svg>
                    <span class="text-sm font-medium text-base-content">Discovery Settings</span>
                    {/* Summary badges */}
                    <Show when={form.discoveryEnabled}>
                      <span class="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">
                        {form.discoveryIntervalHours > 0
                          ? `${form.discoveryIntervalHours}h`
                          : 'Manual'}
                      </span>
                    </Show>
                    <Show when={!form.discoveryEnabled}>
                      <span class="px-1.5 py-0.5 text-[10px] font-medium bg-surface-hover text-muted rounded">
                        Off
                      </span>
                    </Show>
                  </div>
                  <svg
                    class={`w-4 h-4 transition-transform ${showDiscoverySettings() ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </button>
                <Show when={showDiscoverySettings()}>
                  <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
                    {/* Discovery Enabled Toggle */}
                    <div class="flex items-center justify-between gap-2">
                      <label class="text-xs font-medium text-muted flex items-center gap-1.5">
                        Enable Discovery
                        <HelpIcon
                          inline={{
                            title: 'What is Discovery?',
                            description:
                              'Discovery scans your VMs, containers, and container runtimes to identify what services are running (databases, web servers, etc.), their versions, and how to access them. This information helps Pulse AI give you accurate troubleshooting commands and understand your infrastructure.',
                          }}
                          size="xs"
                        />
                      </label>
                      <Toggle
                        checked={form.discoveryEnabled}
                        onChange={(event) =>
                          setForm('discoveryEnabled', event.currentTarget.checked)
                        }
                        disabled={saving()}
                      />
                    </div>

                    {/* Discovery Interval - Only when enabled */}
                    <Show when={form.discoveryEnabled}>
                      <div class="flex flex-col gap-1">
                        <div class="flex items-center gap-3">
                          <label class="text-xs font-medium text-muted w-32 flex-shrink-0">
                            Scan Interval
                          </label>
                          <select
                            class="flex-1 px-2 py-1 text-sm border border-border rounded bg-surface"
                            value={form.discoveryIntervalHours}
                            onChange={(e) =>
                              setForm('discoveryIntervalHours', parseInt(e.currentTarget.value, 10))
                            }
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
                      Discovery gives Pulse AI workload context, so responses can reference concrete
                      services and commands instead of generic advice.
                    </p>
                  </div>
                </Show>
              </div>

              {/* Usage Cost Controls - Compact */}
              <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
                <svg
                  class="w-4 h-4 text-muted flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
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
                  <span class="text-xs">
                    ≈ ${(parseFloat(form.costBudgetUSD30d) / 30).toFixed(2)}/day
                  </span>
                </Show>
                <Show when={!form.costBudgetUSD30d || parseFloat(form.costBudgetUSD30d) === 0}>
                  <span class="text-[10px] text-muted">Set a budget to receive usage alerts</span>
                </Show>
              </div>

              {/* Request Timeout - For slow Ollama hardware */}
              <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
                <svg
                  class="w-4 h-4 text-muted flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
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
                <span class="text-xs">seconds</span>
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
              <div
                class={`space-y-3 p-4 rounded-md border ${getAIControlLevelPanelClass(form.controlLevel)}`}
              >
                <div class="flex items-center gap-2">
                  <svg
                    class="w-4 h-4 text-blue-600 dark:text-blue-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                    />
                  </svg>
                  <span class="text-sm font-medium text-base-content">Pulse Permission Level</span>
                  <Show when={form.controlLevel !== 'read_only'}>
                    <span
                      class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${getAIControlLevelBadgeClass(form.controlLevel)}`}
                    >
                      {form.controlLevel}
                    </span>
                  </Show>
                </div>

                {/* Permission Level */}
                <div class="flex items-center gap-3">
                  <label class="text-xs font-medium text-muted w-28 flex-shrink-0">
                    Permission
                  </label>
                  <select
                    value={form.controlLevel}
                    onChange={(e) =>
                      setForm('controlLevel', e.currentTarget.value as AIControlLevel)
                    }
                    class="flex-1 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded bg-surface"
                    disabled={saving()}
                  >
                    <option value="read_only">Read Only - Pulse Assistant can only observe</option>
                    <option value="controlled">
                      Controlled - Pulse Assistant executes with your approval
                    </option>
                    <option value="autonomous">
                      Autonomous - Pulse Assistant executes without approval (Pro)
                    </option>
                  </select>
                </div>
                <p class="text-[10px] text-muted ml-[7.5rem]">
                  {getAIControlLevelDescription(form.controlLevel)}
                </p>
                <Show when={form.controlLevel === 'autonomous'}>
                  <div class="p-2 bg-amber-100 dark:bg-amber-900 rounded border border-amber-200 dark:border-amber-800 text-[10px] text-amber-800 dark:text-amber-200">
                    <strong>Legal Disclaimer:</strong> Model-driven systems can hallucinate. You are
                    responsible for any damage caused by autonomous actions. See{' '}
                    <a
                      href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 underline"
                    >
                      Terms of Service
                    </a>
                    .
                  </div>
                </Show>
                <Show when={form.controlLevel === 'autonomous' && autoFixLocked()}>
                  <p class="text-xs text-muted">
                    <a
                      class="text-blue-600 dark:text-blue-400 font-medium hover:underline"
                      href={upgradeAutofixUrl()}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={() =>
                        trackUpgradeClicked('settings_ai_patrol_autofix', 'ai_autofix')
                      }
                    >
                      Upgrade to Pro
                    </a>{' '}
                    to enable autonomous mode.
                    <Show when={canStartTrial()}>
                      {' '}
                      <button
                        type="button"
                        onClick={handleStartTrial}
                        disabled={startingTrial()}
                        class="text-indigo-500 hover:underline disabled:opacity-50"
                      >
                        Start free trial
                      </button>
                    </Show>
                  </p>
                </Show>

                {/* Protected Guests - Only show if control is enabled */}
                <Show when={form.controlLevel !== 'read_only'}>
                  <div class="flex items-start gap-3 pt-2 border-t border-blue-200 dark:border-blue-700">
                    <label class="text-xs font-medium text-muted w-28 flex-shrink-0 pt-1">
                      Protected
                    </label>
                    <div class="flex-1">
                      <input
                        type="text"
                        value={form.protectedGuests}
                        onInput={(e) => setForm('protectedGuests', e.currentTarget.value)}
                        placeholder="e.g., 100, 101, prod-db"
                        class="w-full min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded"
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
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
                      />
                    </svg>
                    <span class="text-sm font-medium text-base-content">
                      Chat Session Maintenance
                    </span>
                  </div>
                  <svg
                    class={`w-4 h-4 transition-transform ${showChatMaintenance() ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </button>
                <Show when={showChatMaintenance()}>
                  <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
                    <p class="text-xs text-muted">
                      Use this panel to summarize, inspect, or revert a specific chat session. It
                      does not change your default Pulse Assistant settings.
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
                      <div class="text-xs text-muted">{getAIChatSessionsLoadingState().text}</div>
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
                              {getAIChatSessionsEmptyState().text}
                            </div>
                          }
                        >
                          <select
                            value={selectedSessionId()}
                            onChange={(e) => setSelectedSessionId(e.currentTarget.value)}
                            class="w-full min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded"
                            disabled={saving()}
                          >
                            <For each={chatSessions()}>
                              {(session) => (
                                <option value={session.id}>{formatSessionLabel(session)}</option>
                              )}
                            </For>
                          </select>
                          <Show when={selectedChatSession()}>
                            <p class="text-[10px] text-muted mt-1">
                              Last updated{' '}
                              {new Date(selectedChatSession()!.updated_at).toLocaleString()}
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
                        {sessionActionLoading() === 'summarize'
                          ? 'Summarizing...'
                          : 'Summarize context'}
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
              <div class="p-4 sm:p-6">
                <div
                  class={`flex items-center gap-2 p-3 rounded-md ${settingsReadiness().containerClassName}`}
                >
                  <div class={`w-2 h-2 rounded-full ${settingsReadiness().dotClassName}`} />
                  <div class="flex-1 min-w-0">
                    <span class="text-xs font-medium">{settingsReadiness().summary}</span>
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
              <Show when={settings()?.configured}>
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

      <AISettingsDialogs
        showDiffModal={showDiffModal}
        setShowDiffModal={setShowDiffModal}
        diffFiles={diffFiles}
        diffSummary={diffSummary}
        diffSessionLabel={diffSessionLabel}
        formatDiffStats={formatDiffStats}
        showSetupModal={showSetupModal}
        setupProvider={setupProvider}
        setSetupProvider={setSetupProvider}
        setupApiKey={setupApiKey}
        setSetupApiKey={setSetupApiKey}
        setupOllamaUrl={setupOllamaUrl}
        setSetupOllamaUrl={setSetupOllamaUrl}
        setupSaving={setupSaving}
        handleCloseSetupModal={handleCloseSetupModal}
        handleSetupSubmit={handleSetupSubmit}
      />
    </>
  );
};

export default AISettings;
