import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { AIProvider, AISettings as AISettingsType } from '@/types/ai';
import { createInitialProviderHealth } from '../aiSettingsModel';
import { AIModelSelectionSection, PatrolModelReadinessControl } from '../AIModelSelectionSection';
import type { AISettingsState } from '../useAISettingsState';

function createMockAISettingsState(options: { configured: boolean }): AISettingsState {
  const settingsData: AISettingsType = {
    enabled: options.configured,
    model: options.configured ? 'anthropic:claude-sonnet-4-6' : '',
    configured: options.configured,
    custom_context: '',
    auth_method: 'api_key',
    oauth_connected: false,
    anthropic_configured: options.configured,
    openai_configured: false,
    openrouter_configured: false,
    deepseek_configured: false,
    gemini_configured: false,
    ollama_configured: false,
    ollama_base_url: 'http://localhost:11434',
    ollama_keep_alive: '',
    configured_providers: options.configured ? ['anthropic'] : [],
  };

  const [settings] = createSignal<AISettingsType | null>(settingsData);
  const [expandedProviders, setExpandedProviders] = createSignal<Set<AIProvider>>(
    options.configured ? new Set() : new Set(['anthropic']),
  );
  const [setupApiKey, setSetupApiKey] = createSignal('');
  const [setupOllamaUrl, setSetupOllamaUrl] = createSignal('http://localhost:11434');
  const [setupOpenAIBaseUrl, setSetupOpenAIBaseUrl] = createSignal('');
  const [setupProvider, setSetupProvider] = createSignal<AIProvider>('anthropic');
  const [showSetupModal, setShowSetupModal] = createSignal(false);
  const [providerHealth] = createStore(createInitialProviderHealth());
  const [form, setForm] = createStore({
    enabled: options.configured,
    model: settingsData.model,
    patrolModel: '',
    patrolIntervalMinutes: 360,
    alertTriggeredAnalysis: true,
    patrolAlertTriggers: true,
    patrolAnomalyTriggers: true,
    patrolAlertTriggerMinSeverity: 'critical' as 'warning' | 'critical',
    patrolFindingNotifications: true,
    patrolFindingNotifyMinSeverity: 'warning' as 'warning' | 'critical',
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
    ollamaKeepAlive: '',
    ollamaUsername: '',
    ollamaPassword: '',
    clearOllamaPassword: false,
    openaiBaseUrl: '',
    zaiBaseUrl: '',
    codexSubscriptionEnabled: false,
    claudeSubscriptionEnabled: false,
    costBudgetUSD30d: '',
    requestTimeoutSeconds: 300,
  });

  return {
    alertAnalysisLocked: () => false,
    availableModels: () =>
      options.configured
        ? [{ id: 'anthropic:claude-sonnet-4-6', name: 'Claude Sonnet 4.6', notable: true }]
        : [],
    cancelRemoveProvider: vi.fn(),
    confirmRemoveProvider: vi.fn(async () => {}),
    effectivePatrolModel: () => form.patrolModel || form.model,
    expandedProviders,
    form,
    handleModelSelection: vi.fn(),
    handleClearProvider: vi.fn(async () => {}),
    handleCloseSetupModal: vi.fn(),
    handleEnableRequest: vi.fn(async () => {}),
    handleEnabledToggle: vi.fn(async () => {}),
    handleSave: vi.fn(async () => {}),
    handleSetupSubmit: vi.fn(async () => {}),
    handleTest: vi.fn(async () => {}),
    handleTestProvider: vi.fn(async () => {}),
    hasConfiguredProvider: () => options.configured,
    loading: () => false,
    loadError: () => false,
    loadModels: vi.fn(async () => {}),
    loadSettings: vi.fn(async () => {}),
    modelsError: () => '',
    modelsLoading: () => false,
    patrolAutonomyLevel: () => null,
    patrolCostPreview: () => null,
    patrolCostPreviewLoading: () => false,
    patrolIntervalAutoAdjust: () => null,
    patrolModelGuidance: () => null,
    patrolModelReadinessResult: () => null,
    patrolModelReadinessRunning: () => false,
    pendingRemoveProvider: () => null,
    preflightLastCheckedAt: () => null,
    preflightRunning: () => false,
    providerHealth,
    providerIssueCount: () => 0,
    providerTestResult: () => null,
    resetForm: vi.fn(),
    runProviderPreflight: vi.fn(async () => {}),
    runPatrolModelReadinessAdvisor: vi.fn(async () => {}),
    cancelPatrolModelReadinessAdvisor: vi.fn(),
    saving: () => false,
    setExpandedProviders,
    setForm,
    setSetupApiKey,
    setSetupOllamaUrl,
    setSetupOpenAIBaseUrl,
    setSetupProvider,
    setShowSetupModal,
    settings,
    settingsReadiness: () => ({
      containerClassName: '',
      dotClassName: '',
      summary: '',
    }),
    setupApiKey,
    setupOllamaUrl,
    setupOpenAIBaseUrl,
    setupProvider,
    setupSaving: () => false,
    showSetupModal,
    testing: () => false,
    testingProvider: () => null,
  };
}

describe('AIModelSelectionSection', () => {
  afterEach(() => {
    cleanup();
  });

  it('places Provider Configuration before Shared Default Model and disables preflight when 0 providers are configured', () => {
    const state = createMockAISettingsState({ configured: false });
    render(() => (
      <Router>
        <Route path="/" component={() => <AIModelSelectionSection state={state} />} />
      </Router>
    ));

    const providerHeading = screen.getByText('Provider Configuration');
    const modelLabel = screen.getByText('Shared Default Model');
    expect(
      providerHeading.compareDocumentPosition(modelLabel) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    expect(screen.getByRole('button', { name: 'Run Preflight' })).toBeDisabled();
    expect(
      screen.getByText('Save at least one provider below to load models and run preflight checks.'),
    ).toBeInTheDocument();
  });

  it('places Shared Default Model first and enables preflight once a provider is configured', () => {
    const state = createMockAISettingsState({ configured: true });
    render(() => (
      <Router>
        <Route path="/" component={() => <AIModelSelectionSection state={state} />} />
      </Router>
    ));

    const providerHeading = screen.getByText('Provider Configuration');
    const modelLabel = screen.getByText('Shared Default Model');
    expect(
      modelLabel.compareDocumentPosition(providerHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    expect(screen.getByRole('button', { name: 'Run Preflight' })).toBeEnabled();
  });

  it('disables Check Patrol model with prerequisite guidance when no provider is configured', () => {
    const state = createMockAISettingsState({ configured: false });
    render(() => <PatrolModelReadinessControl state={state} />);

    expect(screen.getByRole('button', { name: 'Check Patrol model' })).toBeDisabled();
    expect(
      screen.getByText('Configure a provider on Provider & Models before checking a Patrol model.'),
    ).toBeInTheDocument();
  });
});
