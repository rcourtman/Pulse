import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import { PULSE_MCP_TOKEN_SETUP_PATH } from '@/routing/resourceLinks';
import { aiChatStore } from '@/stores/aiChat';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import type { AISettings as AISettingsType } from '@/types/ai';
import {
  AIAssistantSettings,
  AIDiscoverySettings,
  AISettings,
  AIPatrolSettings,
} from '../AISettings';

const getSettingsMock = vi.fn();
const updateSettingsMock = vi.fn();
const getModelsMock = vi.fn();
const testProviderMock = vi.fn();
const testConnectionMock = vi.fn();
const runDiscoveryRefreshMock = vi.fn();
const runPatrolModelReadinessMock = vi.fn();
const getPatrolCostPreviewMock = vi.fn();
const getPatrolModelGuidanceMock = vi.fn();
const fetchAgentCapabilitiesManifestMock = vi.fn();
const listSessionsMock = vi.fn();
const summarizeSessionMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const notificationInfoMock = vi.fn();
const notificationWarningMock = vi.fn();
const loggerDebugMock = vi.fn();
const loggerErrorMock = vi.fn();
const hasFeatureMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const loadCommercialPostureMock = vi.fn();
const commercialPostureMock = vi.fn();
const entitlementsMock = vi.fn();
const presentationPolicyHidesUpgradePromptsMock = vi.fn();

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getSettings: (...args: unknown[]) => getSettingsMock(...args),
    updateSettings: (...args: unknown[]) => updateSettingsMock(...args),
    getModels: (...args: unknown[]) => getModelsMock(...args),
    testProvider: (...args: unknown[]) => testProviderMock(...args),
    testConnection: (...args: unknown[]) => testConnectionMock(...args),
  },
}));

vi.mock('@/api/aiChat', () => ({
  AIChatAPI: {
    listSessions: (...args: unknown[]) => listSessionsMock(...args),
    summarizeSession: (...args: unknown[]) => summarizeSessionMock(...args),
  },
}));

vi.mock('@/api/discovery', () => ({
  runDiscoveryRefresh: (...args: unknown[]) => runDiscoveryRefreshMock(...args),
}));

vi.mock('@/api/patrol', () => ({
  runPatrolModelReadiness: (...args: unknown[]) => runPatrolModelReadinessMock(...args),
}));

vi.mock('@/api/aiPatrolCost', () => ({
  getPatrolCostPreview: (...args: unknown[]) => getPatrolCostPreviewMock(...args),
  getPatrolModelGuidance: (...args: unknown[]) => getPatrolModelGuidanceMock(...args),
}));

vi.mock('@/api/agentCapabilities', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/agentCapabilities')>();
  return {
    ...actual,
    fetchAgentCapabilitiesManifest: (...args: unknown[]) =>
      fetchAgentCapabilitiesManifestMock(...args),
  };
});

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
    info: (...args: unknown[]) => notificationInfoMock(...args),
    warning: (...args: unknown[]) => notificationWarningMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: (...args: unknown[]) => loggerDebugMock(...args),
    error: (...args: unknown[]) => loggerErrorMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  getRuntimeCapabilityBlock: () => undefined,
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
  runtimeCapabilities: () => ({ capabilities: [], runtime: undefined }),
}));

vi.mock('@/stores/licenseCommercial', () => ({
  canOfferCommercialTrial: () => commercialPostureMock()?.trial_eligible !== false,
  commercialPosture: (...args: unknown[]) => commercialPostureMock(...args),
  entitlements: (...args: unknown[]) => entitlementsMock(...args),
  getUpgradeActionDestination: () => ({ href: 'https://example.com/upgrade', external: true }),
  loadCommercialPosture: (...args: unknown[]) => loadCommercialPostureMock(...args),
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: () => false,
  presentationPolicyHidesUpgradePrompts: () => presentationPolicyHidesUpgradePromptsMock(),
}));

const baseSettings = (): AISettingsType => ({
  enabled: false,
  model: '',
  configured: false,
  custom_context: '',
  auth_method: 'api_key',
  oauth_connected: false,
  anthropic_configured: false,
  openai_configured: false,
  openrouter_configured: false,
  deepseek_configured: false,
  gemini_configured: false,
  ollama_configured: false,
  ollama_base_url: 'http://localhost:11434',
  ollama_keep_alive: '',
  configured_providers: [],
});

type AISettingsTestPage = 'provider' | 'patrol' | 'assistant' | 'discovery';

const getTestComponent = (page: AISettingsTestPage) => {
  switch (page) {
    case 'patrol':
      return AIPatrolSettings;
    case 'assistant':
      return AIAssistantSettings;
    case 'discovery':
      return AIDiscoverySettings;
    default:
      return AISettings;
  }
};

const renderComponent = (page: AISettingsTestPage = 'provider') => {
  const Component = getTestComponent(page);
  return render(() => (
    <Router>
      <Route path="/" component={() => <Component />} />
    </Router>
  ));
};

const resetAllMocks = () => {
  getSettingsMock.mockReset();
  updateSettingsMock.mockReset();
  getModelsMock.mockReset();
  testProviderMock.mockReset();
  testConnectionMock.mockReset();
  runDiscoveryRefreshMock.mockReset();
  runPatrolModelReadinessMock.mockReset();
  getPatrolCostPreviewMock.mockReset();
  getPatrolModelGuidanceMock.mockReset();
  fetchAgentCapabilitiesManifestMock.mockReset();
  listSessionsMock.mockReset();
  summarizeSessionMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  notificationInfoMock.mockReset();
  notificationWarningMock.mockReset();
  loggerDebugMock.mockReset();
  loggerErrorMock.mockReset();
  hasFeatureMock.mockReset();
  loadLicenseStatusMock.mockReset();
  loadCommercialPostureMock.mockReset();
  commercialPostureMock.mockReset();
  entitlementsMock.mockReset();
  presentationPolicyHidesUpgradePromptsMock.mockReset();
};

const setupDefaultMocks = () => {
  hasFeatureMock.mockReturnValue(true);
  loadCommercialPostureMock.mockResolvedValue(undefined);
  commercialPostureMock.mockReturnValue({ trial_eligible: true });
  entitlementsMock.mockReturnValue({ trial_eligible: true });
  getSettingsMock.mockResolvedValue(baseSettings());
  getModelsMock.mockResolvedValue({ models: [] });
  getPatrolCostPreviewMock.mockResolvedValue(null);
  getPatrolModelGuidanceMock.mockResolvedValue({ rules: [] });
  testConnectionMock.mockResolvedValue({ success: true, message: 'ok' });
  testProviderMock.mockResolvedValue({
    success: true,
    message: 'OpenRouter reachable',
    provider: 'openrouter',
  });
  runDiscoveryRefreshMock.mockResolvedValue({
    mode: 'manual',
    fingerprint_count: 1,
    changed_count: 1,
    stale_count: 0,
    candidate_count: 1,
    discovered_count: 1,
    failed_count: 0,
    last_run: '2026-05-15T12:00:00Z',
  });
  fetchAgentCapabilitiesManifestMock.mockResolvedValue(null);
  listSessionsMock.mockResolvedValue([]);
  summarizeSessionMock.mockResolvedValue(undefined);
  presentationPolicyHidesUpgradePromptsMock.mockReturnValue(false);
};

describe('AISettings model loading error states', () => {
  beforeEach(() => {
    resetAIRuntimeState();
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('separates Patrol mode from Assistant chat actions on the settings page', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      patrol_interval_minutes: 180,
      control_level: 'controlled',
    });

    renderComponent('assistant');

    expect(await screen.findByText('Assistant chat actions')).toBeInTheDocument();
    expect(
      screen.getByText('This controls actions started from Assistant chat only', { exact: false }),
    ).toBeInTheDocument();
    const chatActionMode = screen.getByLabelText('Chat action mode');
    expect(chatActionMode).toBeInTheDocument();
    expect(chatActionMode).toHaveClass('w-full', 'min-w-0', 'sm:flex-1');
    expect(chatActionMode.parentElement).toHaveClass('flex-col', 'sm:flex-row');
    expect(
      screen.getByText('Assistant asks before chat-only actions.', { exact: false }),
    ).toHaveClass('sm:ml-[7.5rem]');
    expect(screen.getByRole('option', { name: /Ask first/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Save Assistant settings/i })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'External agents' })).toBeInTheDocument();
    expect(
      screen.getByText('Connect external tools to read Pulse context and request Patrol work.', {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show connector setup' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Connector setup' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Create token' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show connector setup' }));

    expect(screen.getByRole('link', { name: 'Create token' })).toHaveAttribute(
      'href',
      PULSE_MCP_TOKEN_SETUP_PATH,
    );
  });

  it('keeps Provider & Models focused on provider setup and runtime cost controls', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
    });

    renderComponent();

    expect(await screen.findByText('Provider Configuration')).toBeInTheDocument();
    expect(screen.getByText('30-day Budget')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Enable Pulse Intelligence' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Test Connection/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Save provider settings/i })).toBeInTheDocument();
    expect(screen.queryByText('Assistant chat actions')).not.toBeInTheDocument();
    expect(screen.queryByText('External agents')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Service Context/i })).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol mode')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Open Patrol/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Set how much Patrol can do/i)).not.toBeInTheDocument();
  });

  it('keeps model identifiers and provider secrets out of login credential autofill', async () => {
    renderComponent();

    const modelInput = await screen.findByLabelText('Default model identifier');
    expect(modelInput).toHaveAttribute('autocomplete', 'off');
    expect(modelInput.closest('form')).toHaveAttribute('autocomplete', 'off');

    expect(await screen.findByLabelText('Anthropic API key')).toHaveAttribute(
      'autocomplete',
      'new-password',
    );
  });

  it('shows Patrol scheduling and readiness without rendering the operator loop', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      patrol_interval_minutes: 180,
      alert_triggered_analysis: true,
      patrol_alert_triggers_enabled: true,
      patrol_anomaly_triggers_enabled: false,
      patrol_alert_trigger_min_severity: 'warning',
    });

    renderComponent('patrol');

    expect(await screen.findByText('Patrol mode')).toBeInTheDocument();
    expect(screen.getByLabelText('Schedule')).toHaveValue('180');
    expect(screen.getByLabelText('Enable alert-triggered Patrols')).toBeInTheDocument();
    expect(screen.getByLabelText('Enable anomaly-triggered Patrols')).toBeInTheDocument();
    expect(screen.getByLabelText('Enable container update risk analysis')).toBeInTheDocument();
    expect(screen.getByLabelText('Investigate alerts at or above')).toHaveValue('warning');
    expect(screen.getByText('Model readiness')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Open Patrol/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Save Patrol settings/i })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Enable Pulse Intelligence' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Test Connection/i })).not.toBeInTheDocument();
    expect(screen.queryByText('Provider Configuration')).not.toBeInTheDocument();
    expect(screen.queryByText('External agents')).not.toBeInTheDocument();
  });

  it('describes Watch only for plan-locked installs instead of telling the user to choose a mode', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'ai_autofix');
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      patrol_interval_minutes: 180,
    });

    renderComponent('patrol');

    expect(await screen.findByText('Patrol mode')).toBeInTheDocument();
    expect(screen.getByText(/This install runs Watch only/i)).toBeInTheDocument();
    expect(screen.queryByText(/Choose a Patrol mode on the Patrol page/i)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Open Patrol/i })).toBeInTheDocument();
  });

  it('saves Patrol trigger settings from Pulse Intelligence Patrol settings', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      patrol_alert_triggers_enabled: true,
      patrol_anomaly_triggers_enabled: false,
      patrol_alert_trigger_min_severity: 'critical',
      alert_triggered_analysis: true,
    });
    updateSettingsMock.mockImplementation(async (payload: Record<string, unknown>) => ({
      ...baseSettings(),
      enabled: true,
      configured: true,
      ...payload,
    }));

    renderComponent('patrol');

    await screen.findByLabelText('Enable alert-triggered Patrols');
    fireEvent.change(screen.getByLabelText('Investigate alerts at or above'), {
      target: { value: 'warning' },
    });
    fireEvent.click(screen.getByLabelText('Enable alert-triggered Patrols'));
    fireEvent.click(screen.getByLabelText('Enable anomaly-triggered Patrols'));
    fireEvent.click(screen.getByLabelText('Enable container update risk analysis'));
    fireEvent.click(screen.getByRole('button', { name: /Save Patrol settings/i }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          patrol_alert_triggers_enabled: false,
          patrol_anomaly_triggers_enabled: true,
          patrol_alert_trigger_min_severity: 'warning',
          alert_triggered_analysis: false,
        }),
      );
    });
    await waitFor(() => {
      expect(notificationSuccessMock).toHaveBeenCalledWith('Patrol settings saved');
    });
  });

  it('shows inline warning when getModels throws a network error', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
    });
    getModelsMock.mockRejectedValue(new Error('Network request failed'));

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Failed to load models: Network request failed/)).toBeInTheDocument();
    });
  });

  it('shows inline warning when API returns an error field', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
    });
    getModelsMock.mockResolvedValue({ models: [], error: 'Invalid API key' });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Failed to load models: Invalid API key/)).toBeInTheDocument();
    });
  });

  it('clears error and retries when Refresh is clicked', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
    });
    getModelsMock.mockRejectedValueOnce(new Error('Temporary failure'));

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Failed to load models: Temporary failure/)).toBeInTheDocument();
    });

    // Now mock a successful response for retry
    getModelsMock.mockResolvedValueOnce({
      models: [{ id: 'anthropic:claude-sonnet-4-20250514', name: 'Claude Sonnet' }],
    });

    fireEvent.click(screen.getByRole('button', { name: /refresh/i }));

    await waitFor(() => {
      expect(screen.queryByText(/Failed to load models/)).not.toBeInTheDocument();
    });

    // Verify retry actually completed and models loaded
    expect(getModelsMock).toHaveBeenCalledTimes(2);
  });

  it('does not show warning when models load successfully', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
    });
    getModelsMock.mockResolvedValue({
      models: [{ id: 'anthropic:claude-sonnet-4-20250514', name: 'Claude Sonnet' }],
    });

    renderComponent();

    await waitFor(() => {
      expect(getModelsMock).toHaveBeenCalled();
    });

    expect(screen.queryByText(/Failed to load models/)).not.toBeInTheDocument();
  });

  it('clears stale models when API returns error with empty list', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
    });
    // First call succeeds with models, second returns error with empty list
    getModelsMock
      .mockResolvedValueOnce({
        models: [{ id: 'anthropic:claude-sonnet-4-20250514', name: 'Claude Sonnet' }],
      })
      .mockResolvedValueOnce({ models: [], error: 'API key revoked' });

    renderComponent();

    await waitFor(() => {
      expect(getModelsMock).toHaveBeenCalledTimes(1);
      expect(screen.getByTitle('Select shared default model')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /refresh/i })).not.toBeDisabled();
    });

    // Trigger a refresh that returns an error
    fireEvent.click(screen.getByRole('button', { name: /refresh/i }));

    await waitFor(() => {
      expect(getModelsMock).toHaveBeenCalledTimes(2);
      expect(screen.getByText(/Failed to load models: API key revoked/)).toBeInTheDocument();
    });

    // Stale model options should be cleared — fallback text input should be shown instead of select
    expect(
      screen.getByPlaceholderText('Configure a provider below to see available models'),
    ).toBeInTheDocument();
  });

  it('keeps large provider catalogs searchable without dumping older models into settings', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      model: 'openrouter:minimax/minimax-m2.5',
      openrouter_configured: true,
      configured_providers: ['openrouter'],
    });
    getModelsMock.mockResolvedValue({
      models: [
        {
          id: 'openrouter:minimax/minimax-m2.5',
          name: 'MiniMax: MiniMax M2.5',
          notable: true,
        },
        {
          id: 'openrouter:legacy/model-v1',
          name: 'Legacy Model V1',
          notable: false,
        },
        {
          id: 'anthropic:claude-sonnet-4-20250514',
          name: 'Claude Sonnet 4',
          notable: true,
        },
      ],
    });

    renderComponent();

    const pickerButton = await screen.findByTitle('Select shared default model');
    expect(screen.getByText('MiniMax: MiniMax M2.5 via OpenRouter')).toBeInTheDocument();

    fireEvent.click(pickerButton);

    expect(screen.queryByText('Legacy Model V1 via OpenRouter')).not.toBeInTheDocument();
    expect(screen.queryByText('Claude Sonnet 4')).not.toBeInTheDocument();
    expect(screen.getByText('Show 1 older models')).toBeInTheDocument();

    fireEvent.input(screen.getByPlaceholderText('Search configured provider models'), {
      target: { value: 'legacy' },
    });

    expect(screen.getByText('Legacy Model V1 via OpenRouter')).toBeInTheDocument();
  });

  it('shows the OpenRouter route for a gateway-hosted DeepSeek default model', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: 'openrouter:deepseek/deepseek-v4-pro',
      openrouter_configured: true,
      deepseek_configured: true,
      configured_providers: ['openrouter', 'deepseek'],
    });
    getModelsMock.mockResolvedValue({
      models: [
        {
          id: 'openrouter:deepseek/deepseek-v4-pro',
          name: 'DeepSeek: DeepSeek V4 Pro',
          provider: 'openrouter',
          notable: true,
        },
        {
          id: 'deepseek:deepseek-v4-pro',
          name: 'DeepSeek: DeepSeek V4 Pro',
          provider: 'deepseek',
          notable: true,
        },
      ],
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByTitle('Select shared default model')).toBeInTheDocument();
    });

    expect(screen.getByText('DeepSeek: DeepSeek V4 Pro via OpenRouter')).toBeInTheDocument();
    expect(
      screen.getByText(/Default: DeepSeek: DeepSeek V4 Pro via OpenRouter/),
    ).toBeInTheDocument();
  });

  it('hides autonomous controls when auto-fix is locked and upgrade prompts are hidden', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'ai_autofix');
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);

    renderComponent('assistant');

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    expect(
      screen.queryByRole('option', { name: /Allow chat-only actions/i }),
    ).not.toBeInTheDocument();
  });

  it('keeps an existing autonomous setting visible even when upgrade prompts are hidden', async () => {
    hasFeatureMock.mockImplementation((feature: string) => feature !== 'ai_autofix');
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      control_level: 'autonomous',
    });

    renderComponent('assistant');

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    expect(screen.getByRole('option', { name: /Allow chat-only actions/i })).toBeInTheDocument();
    expect(screen.queryByText('(Pro)')).not.toBeInTheDocument();
  });
});

describe('AISettings load failure error state', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('shows persistent error banner and hides form when settings fail to load', async () => {
    getSettingsMock.mockRejectedValue(new Error('Network error'));

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Unable to load Provider & Models settings/)).toBeInTheDocument();
    });

    // Retry button should be present
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();

    // Save button should NOT be present (form is hidden)
    expect(screen.queryByRole('button', { name: /Save .* settings/i })).not.toBeInTheDocument();
  });

  it('clears error and shows form after successful retry', async () => {
    getSettingsMock.mockRejectedValueOnce(new Error('Network error'));

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Unable to load Provider & Models settings/)).toBeInTheDocument();
    });

    // Now mock a successful response for retry
    getSettingsMock.mockResolvedValueOnce({
      ...baseSettings(),
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
    });
    getModelsMock.mockResolvedValueOnce({ models: [] });

    fireEvent.click(screen.getByRole('button', { name: /retry/i }));

    // Wait for the form to fully render after successful retry (not just banner disappearing during loading)
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Save provider settings/i })).toBeInTheDocument();
    });

    // Error banner should be gone
    expect(screen.queryByText(/Unable to load Provider & Models settings/)).not.toBeInTheDocument();

    // Verify retry actually called getSettings again
    expect(getSettingsMock).toHaveBeenCalledTimes(2);
  });
});

describe('AISettings service context persistence', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('saves service context enablement and scan interval as an explicit pair', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      discovery_enabled: true,
      discovery_interval_hours: 24,
    });
    updateSettingsMock.mockImplementation(async (payload: Record<string, unknown>) => ({
      ...baseSettings(),
      discovery_enabled: payload.discovery_enabled as boolean,
      discovery_interval_hours: payload.discovery_interval_hours as number,
    }));

    renderComponent('discovery');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Service Context Auto 24h/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Service Context Auto 24h/i }));
    const intervalSelect = screen.getByLabelText('Scan Interval');
    expect(intervalSelect).toHaveValue('24');

    fireEvent.change(intervalSelect, {
      target: { value: '6' },
    });
    expect(intervalSelect).toHaveValue('6');

    fireEvent.click(screen.getByRole('button', { name: /Save service context settings/i }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          discovery_enabled: true,
          discovery_interval_hours: 6,
        }),
      );
    });
  });

  it('runs a manual service context refresh from settings', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      discovery_enabled: true,
      discovery_interval_hours: 0,
    });

    renderComponent('discovery');

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /Service Context Manual only/i }),
      ).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Service Context Manual only/i }));
    fireEvent.click(screen.getByRole('button', { name: /Run context scan/i }));

    await waitFor(() => {
      expect(runDiscoveryRefreshMock).toHaveBeenCalledTimes(1);
      expect(notificationSuccessMock).toHaveBeenCalledWith(
        'Discovery refresh finished: 1 workload refreshed.',
      );
    });
  });

  it('keeps the manual discovery refresh visible when recurring discovery is off', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      discovery_enabled: false,
      discovery_interval_hours: 0,
    });

    renderComponent('discovery');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Service Context Off/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Service Context Off/i }));

    expect(screen.queryByLabelText('Scan Interval')).not.toBeInTheDocument();
    expect(
      screen.getByText('Runs one service context scan without changing the schedule.'),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Run context scan/i }));

    await waitFor(() => {
      expect(runDiscoveryRefreshMock).toHaveBeenCalledTimes(1);
      expect(notificationSuccessMock).toHaveBeenCalledWith(
        'Discovery refresh finished: 1 workload refreshed.',
      );
    });
  });

  it('reports when a manual discovery refresh has no pending work', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      discovery_enabled: true,
      discovery_interval_hours: 0,
    });
    runDiscoveryRefreshMock.mockResolvedValue({
      mode: 'manual',
      fingerprint_count: 1,
      changed_count: 0,
      stale_count: 0,
      candidate_count: 0,
      discovered_count: 0,
      failed_count: 0,
      last_run: '2026-05-15T12:00:00Z',
    });

    renderComponent('discovery');

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /Service Context Manual only/i }),
      ).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Service Context Manual only/i }));
    fireEvent.click(screen.getByRole('button', { name: /Run context scan/i }));

    await waitFor(() => {
      expect(notificationInfoMock).toHaveBeenCalledWith(
        'Discovery refresh finished: no new, changed, stale, or repairable workloads.',
      );
    });
  });
});

describe('AISettings OpenRouter flow', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();

    updateSettingsMock.mockImplementation(async (payload: Record<string, unknown>) => {
      if (typeof payload.openrouter_api_key === 'string') {
        return {
          ...baseSettings(),
          model: 'openrouter:openai/gpt-4o-mini',
          configured: true,
          openrouter_configured: true,
          configured_providers: ['openrouter'],
        } satisfies AISettingsType;
      }
      return baseSettings();
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('configures OpenRouter and runs provider test from the OpenRouter panel', async () => {
    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /openrouter/i }));
    fireEvent.input(await screen.findByPlaceholderText('sk-or-...'), {
      target: { value: 'sk-or-configured' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Save provider settings/i }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          model: '',
          openrouter_api_key: 'sk-or-configured',
        }),
      );
    });

    const payload = updateSettingsMock.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      model: '',
      openrouter_api_key: 'sk-or-configured',
    });
    expect(payload).not.toMatchObject({
      model: 'openrouter:openai/gpt-4o-mini',
    });

    // Ignore preflight call triggered after save; validate explicit test button action.
    testProviderMock.mockClear();
    fireEvent.click(await screen.findByRole('button', { name: /^Test$/ }));

    await waitFor(() => {
      expect(testProviderMock).toHaveBeenCalledTimes(1);
      expect(testProviderMock).toHaveBeenCalledWith('openrouter', 'openrouter:openai/gpt-4o-mini');
    });
  });

  it('tests the pending model selection instead of the previously saved provider model', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: true,
      model: 'openrouter:openai/gpt-4o-mini',
      openrouter_configured: true,
      configured_providers: ['openrouter'],
    });
    getModelsMock.mockResolvedValue({ models: [] });

    renderComponent();

    await waitFor(() => {
      expect(testProviderMock).toHaveBeenCalled();
    });

    testProviderMock.mockClear();
    fireEvent.input(screen.getByLabelText('Default model identifier'), {
      target: { value: 'openrouter:anthropic/claude-sonnet-4' },
    });
    fireEvent.click(screen.getByRole('button', { name: /openrouter/i }));
    fireEvent.click(await screen.findByRole('button', { name: /^Test$/ }));

    await waitFor(() => {
      expect(testProviderMock).toHaveBeenCalledTimes(1);
      expect(testProviderMock).toHaveBeenCalledWith(
        'openrouter',
        'openrouter:anthropic/claude-sonnet-4',
      );
    });
  });
});

describe('AISettings Ollama provider options', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('saves Ollama keep alive through the provider settings panel', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      model: 'ollama:llama3',
      ollama_configured: true,
      configured_providers: ['ollama'],
      ollama_keep_alive: '30s',
    });
    updateSettingsMock.mockImplementation(async (payload: Record<string, unknown>) => ({
      ...baseSettings(),
      configured: true,
      model: 'ollama:llama3',
      ollama_configured: true,
      configured_providers: ['ollama'],
      ollama_base_url: 'http://localhost:11434',
      ollama_keep_alive: (payload.ollama_keep_alive as string) ?? '30s',
    }));

    renderComponent();

    // Configured providers start collapsed; open the Ollama accordion to edit
    // its advanced options, the same way an operator would.
    fireEvent.click(await screen.findByRole('button', { name: /ollama/i }));
    fireEvent.input(await screen.findByLabelText('Ollama Keep Alive'), {
      target: { value: '24h' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Save provider settings/i }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          model: 'ollama:llama3',
          ollama_keep_alive: '24h',
        }),
      );
    });
  });
});

describe('AISettings OpenAI-compatible provider lifecycle', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('saves a keyless custom endpoint as provider configuration', async () => {
    updateSettingsMock.mockImplementation(async (payload: Record<string, unknown>) => ({
      ...baseSettings(),
      openai_configured: true,
      configured: false,
      openai_base_url: payload.openai_base_url as string,
      configured_providers: ['openai'],
    }));

    renderComponent();
    fireEvent.click(await screen.findByRole('button', { name: /openai/i }));
    fireEvent.input(await screen.findByLabelText('OpenAI Custom Base URL'), {
      target: { value: 'http://127.0.0.1:8080/v1' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Save provider settings/i }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          openai_base_url: 'http://127.0.0.1:8080/v1',
        }),
      );
    });
    expect(updateSettingsMock.mock.calls[0]?.[0]).not.toHaveProperty('openai_api_key');
  });

  it('removes endpoint, credentials, and selected models through the lifecycle API', async () => {
    const configured = {
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: 'openai:opaque-local-model',
      chat_model: 'openai:opaque-local-model',
      patrol_model: 'openai:opaque-local-model',
      openai_configured: true,
      openai_base_url: 'http://127.0.0.1:8080/v1',
      configured_providers: ['openai' as const],
    };
    getSettingsMock.mockResolvedValue(configured);
    updateSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: false,
      model: '',
      openai_base_url: '',
    });
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderComponent();
    fireEvent.click(await screen.findByRole('button', { name: /openai/i }));
    fireEvent.click(await screen.findByRole('button', { name: 'Remove' }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith({
        remove_providers: ['openai'],
      });
    });
    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('only configured provider'));
    confirmSpy.mockRestore();
  });
});

describe('AISettings provider save failure context', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('names the provider and selected model when a settings save fails after preflight', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: true,
      model: 'openrouter:deepseek/deepseek-r1',
      patrol_model: 'openrouter:deepseek/deepseek-r1',
      openrouter_configured: true,
      deepseek_configured: true,
      configured_providers: ['openrouter', 'deepseek'],
    });
    getModelsMock.mockResolvedValue({
      models: [
        {
          id: 'openrouter:deepseek/deepseek-r1',
          name: 'DeepSeek R1 via OpenRouter',
        },
      ],
    });
    testProviderMock.mockImplementation(async (provider: string) => ({
      success: provider !== 'openrouter',
      message:
        provider === 'openrouter' ? 'Provider authentication issue' : `${provider} reachable`,
      recommendation:
        provider === 'openrouter'
          ? 'Check the API key or provider authentication in Provider & Models settings, then retry.'
          : undefined,
      cause: provider === 'openrouter' ? 'provider_auth' : undefined,
      provider,
    }));
    updateSettingsMock.mockRejectedValue(new Error('Unable to save Provider & Models settings.'));

    renderComponent();

    await waitFor(() => {
      expect(testProviderMock).toHaveBeenCalledWith(
        'openrouter',
        'openrouter:deepseek/deepseek-r1',
      );
    });

    fireEvent.click(screen.getByRole('button', { name: /Save provider settings/i }));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        expect.stringContaining('OpenRouter provider'),
      );
    });
    const message = String(notificationErrorMock.mock.calls.at(-1)?.[0] ?? '');
    expect(message).toContain('model openrouter:deepseek/deepseek-r1');
    expect(message).toContain('Provider authentication issue');
    expect(message).toContain('Check the API key or provider authentication');
    expect(message).toContain('Unable to save Provider & Models settings.');
  });

  it('warns with Patrol provider and model when settings save but Patrol is not ready', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: true,
      model: 'openrouter:deepseek/deepseek-r1',
      patrol_model: 'openrouter:deepseek/deepseek-r1',
      openrouter_configured: true,
      configured_providers: ['openrouter'],
    });
    updateSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: true,
      model: 'openrouter:deepseek/deepseek-r1',
      patrol_model: 'openrouter:deepseek/deepseek-r1',
      openrouter_configured: true,
      configured_providers: ['openrouter'],
      patrol_readiness: {
        status: 'not_ready',
        ready: false,
        cause: 'model_unsupported_tools',
        summary:
          'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
        provider: 'openrouter',
        model: 'openrouter:deepseek/deepseek-r1',
        checks: [],
      },
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Save provider settings/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Save provider settings/i }));

    await waitFor(() => {
      expect(notificationWarningMock).toHaveBeenCalledWith(
        expect.stringContaining('Provider & Models settings saved, but Patrol is not ready'),
      );
    });
    const message = String(notificationWarningMock.mock.calls.at(-1)?.[0] ?? '');
    expect(message).toContain('Provider: OpenRouter');
    expect(message).toContain('Model: openrouter:deepseek/deepseek-r1');
    expect(message).toContain('reasoning-only model family');
    expect(notificationSuccessMock).not.toHaveBeenCalledWith('Provider & Models settings saved');
  });

  it('does not attach provider context to specific backend validation errors', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: true,
      model: 'openrouter:deepseek/deepseek-r1',
      openrouter_configured: true,
      configured_providers: ['openrouter'],
    });
    testProviderMock.mockResolvedValue({
      success: false,
      message: 'OpenRouter returned 401 during provider preflight',
      provider: 'openrouter',
    });
    updateSettingsMock.mockRejectedValue(new Error('Patrol interval must be at least 10 minutes'));

    renderComponent();

    await waitFor(() => {
      expect(testProviderMock).toHaveBeenCalledWith(
        'openrouter',
        'openrouter:deepseek/deepseek-r1',
      );
    });

    fireEvent.click(screen.getByRole('button', { name: /Save provider settings/i }));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Patrol interval must be at least 10 minutes',
      );
    });
    const message = String(notificationErrorMock.mock.calls.at(-1)?.[0] ?? '');
    expect(message).not.toContain('OpenRouter provider');
    expect(message).not.toContain('openrouter:deepseek/deepseek-r1');
  });

  it('renders dimensioned model readiness without certifying higher autonomy', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: 'ollama:qwen3:8b',
      patrol_model: 'ollama:qwen3:8b',
      ollama_configured: true,
      configured_providers: ['ollama'],
    });
    runPatrolModelReadinessMock.mockResolvedValue({
      probe_version: 'patrol-readiness/v1',
      success: true,
      status: 'pass',
      provider: 'ollama',
      model: 'qwen3:8b',
      duration_ms: 9_100,
      max_verified_mode: 'monitor',
      summary: 'Verified for Watch only on this install.',
      recommendation: 'Keep higher autonomy disabled until an extended canary passes.',
      dimensions: {
        connectivity: { status: 'pass', summary: 'Connected.' },
        tool_protocol: { status: 'pass', summary: 'Passed.', attempts: 3, passed: 3 },
        context_quality: { status: 'pass', summary: 'Passed.', attempts: 2, passed: 2 },
        latency: { status: 'pass', summary: 'Passed.', warm_p50_ms: 1_000 },
      },
      modes: {
        monitor: { status: 'verified', summary: 'Verified.' },
        approval: { status: 'not_suitable', summary: 'Continuation failed.' },
        assisted: { status: 'not_assessed', summary: 'Extended canary required.' },
        full: { status: 'not_assessed', summary: 'Governed canary required.' },
      },
      metadata: { context_window: 32768, quantization_level: 'Q4_K_M' },
      recorded_at: '2026-07-13T08:00:00Z',
      recorded_at_unix: 1783929600,
    });

    renderComponent('patrol');
    const button = await screen.findByRole('button', { name: 'Check Patrol model' });
    fireEvent.click(button);

    expect(await screen.findByText('Verified for Watch only')).toBeInTheDocument();
    expect(screen.getByText('Tool protocol')).toBeInTheDocument();
    expect(screen.getByText('Context quality')).toBeInTheDocument();
    expect(screen.getByText('Autonomy suitability')).toBeInTheDocument();
    expect(screen.getByText('Safe auto-fix')).toBeInTheDocument();
    expect(screen.getByText('Autopilot')).toBeInTheDocument();
    expect(screen.getAllByText('Not assessed')).toHaveLength(2);
    expect(screen.getByText(/Context 32,768 tokens/)).toBeInTheDocument();
    expect(runPatrolModelReadinessMock).toHaveBeenCalledWith(
      { model: 'ollama:qwen3:8b' },
      expect.anything(),
    );
  });

  it('surfaces per-scenario evaluation detail for a failed readiness run (issues #1624/#1614)', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: 'ollama:qwen3:8b',
      patrol_model: 'ollama:qwen3:8b',
      ollama_configured: true,
      configured_providers: ['ollama'],
    });
    runPatrolModelReadinessMock.mockResolvedValue({
      probe_version: 'patrol-readiness/v1',
      success: false,
      transport_healthy: true,
      patrol_capable: false,
      status: 'warning',
      provider: 'ollama',
      model: 'qwen3:8b',
      duration_ms: 20_500,
      summary:
        'The provider is healthy for ordinary chat, but the selected model did not demonstrate Patrol’s streaming tool protocol.',
      details: [
        'Scenario "typed-tool" tool protocol: expected exactly one tool call, got 0 (the model hit the generation cap before finishing: done_reason=length)',
        'Scenario "backup-failure" tool protocol: nonce did not match',
      ],
      dimensions: {
        connectivity: { status: 'pass', summary: 'Connected.' },
        tool_protocol: { status: 'fail', summary: 'Failed.', attempts: 3, passed: 0 },
        context_quality: { status: 'fail', summary: 'Failed.', attempts: 2, passed: 0 },
        latency: { status: 'pass', summary: 'Passed.', warm_p50_ms: 1_000 },
      },
      modes: {
        monitor: { status: 'not_suitable', summary: 'Protocol failed.' },
        approval: { status: 'not_suitable', summary: 'Protocol failed.' },
        assisted: { status: 'not_assessed', summary: 'Extended canary required.' },
        full: { status: 'not_assessed', summary: 'Governed canary required.' },
      },
      recorded_at: '2026-07-13T08:00:00Z',
      recorded_at_unix: 1783929600,
    });

    renderComponent('patrol');
    fireEvent.click(await screen.findByRole('button', { name: 'Check Patrol model' }));

    expect(
      await screen.findByText('Provider connected. Patrol capability not verified'),
    ).toBeInTheDocument();
    expect(screen.getByText('Evaluation detail')).toBeInTheDocument();
    expect(screen.getByText(/expected exactly one tool call, got 0/)).toBeInTheDocument();
    expect(screen.getByText(/done_reason=length/)).toBeInTheDocument();
    expect(screen.getByText(/nonce did not match/)).toBeInTheDocument();
  });

  it('cancels an in-flight local model readiness evaluation', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: 'ollama:qwen3:8b',
      patrol_model: 'ollama:qwen3:8b',
      ollama_configured: true,
      configured_providers: ['ollama'],
    });
    runPatrolModelReadinessMock.mockImplementation(
      (_body: unknown, signal: AbortSignal) =>
        new Promise((_resolve, reject) => {
          signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')));
        }),
    );

    renderComponent('patrol');
    fireEvent.click(await screen.findByRole('button', { name: 'Check Patrol model' }));
    const cancelButton = await screen.findByRole('button', { name: 'Cancel check' });
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(notificationInfoMock).toHaveBeenCalledWith('Patrol model evaluation cancelled.');
    });
    expect(await screen.findByRole('button', { name: 'Check Patrol model' })).toBeInTheDocument();
  });
});

describe('AISettings provider setup flow', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
    aiChatStore.close();
  });

  it('warns from setup when the saved provider leaves Patrol not ready', async () => {
    updateSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: 'openrouter:deepseek/deepseek-r1',
      patrol_model: 'openrouter:deepseek/deepseek-r1',
      openrouter_configured: true,
      configured_providers: ['openrouter'],
      patrol_readiness: {
        status: 'not_ready',
        ready: false,
        cause: 'model_unsupported_tools',
        summary:
          'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
        provider: 'openrouter',
        model: 'openrouter:deepseek/deepseek-r1',
        checks: [],
      },
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable pulse intelligence/i }));
    const setupDialog = await screen.findByRole('dialog', {
      name: 'Set up Pulse Intelligence',
    });
    expect(within(setupDialog).getByText('Set Up Pulse Intelligence')).toBeInTheDocument();
    fireEvent.click(within(setupDialog).getByRole('button', { name: /OpenRouter/i }));
    fireEvent.input(screen.getByPlaceholderText('sk-or-...'), {
      target: { value: 'sk-or-test' },
    });
    fireEvent.click(within(setupDialog).getByRole('button', { name: 'Enable Pulse Intelligence' }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith({
        enabled: true,
        openrouter_api_key: 'sk-or-test',
      });
    });
    await waitFor(() => {
      expect(notificationWarningMock).toHaveBeenCalledWith(
        expect.stringContaining('Pulse Intelligence enabled, but Patrol is not ready'),
      );
    });
    const message = String(notificationWarningMock.mock.calls.at(-1)?.[0] ?? '');
    expect(message).toContain('Provider: OpenRouter');
    expect(message).toContain('Model: openrouter:deepseek/deepseek-r1');
    expect(message).toContain('reasoning-only model family');
    expect(notificationSuccessMock).not.toHaveBeenCalledWith(
      expect.stringContaining('Pulse Intelligence enabled. This is the Assistant'),
    );
  });

  it('opens the Assistant and points at it after a successful first-time setup', async () => {
    updateSettingsMock.mockResolvedValue({
      ...baseSettings(),
      enabled: true,
      configured: true,
      anthropic_configured: true,
      configured_providers: ['anthropic'],
      patrol_readiness: {
        status: 'ready',
        ready: true,
        cause: 'none',
        summary: 'Patrol is ready to run tool-backed verification.',
        provider: 'anthropic',
        model: 'anthropic:claude-sonnet-5',
        checks: [],
      },
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable pulse intelligence/i }));
    const setupDialog = await screen.findByRole('dialog', {
      name: 'Set up Pulse Intelligence',
    });
    fireEvent.click(within(setupDialog).getByRole('button', { name: /Anthropic/i }));
    fireEvent.input(within(setupDialog).getByPlaceholderText('sk-ant-...'), {
      target: { value: 'sk-ant-test' },
    });
    fireEvent.click(within(setupDialog).getByRole('button', { name: 'Enable Pulse Intelligence' }));

    await waitFor(() => {
      expect(notificationSuccessMock).toHaveBeenCalledWith(
        'Pulse Intelligence enabled. This is the Assistant — ask it anything about your infrastructure.',
      );
    });
    expect(aiChatStore.isOpen).toBe(true);
  });

  it('names the setup provider when provider setup save fails generically', async () => {
    updateSettingsMock.mockRejectedValue(new Error('Unable to save Provider & Models settings.'));

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable pulse intelligence/i }));
    const setupDialog = await screen.findByRole('dialog', {
      name: 'Set up Pulse Intelligence',
    });
    expect(within(setupDialog).getByText('Set Up Pulse Intelligence')).toBeInTheDocument();
    fireEvent.click(within(setupDialog).getByRole('button', { name: /OpenRouter/i }));
    fireEvent.input(screen.getByPlaceholderText('sk-or-...'), {
      target: { value: 'sk-or-test' },
    });
    fireEvent.click(within(setupDialog).getByRole('button', { name: 'Enable Pulse Intelligence' }));

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        expect.stringContaining('OpenRouter provider'),
      );
    });
    expect(String(notificationErrorMock.mock.calls.at(-1)?.[0] ?? '')).toContain(
      'Unable to save Provider & Models settings.',
    );
  });

  it('keeps legacy quickstart-only installs on the provider setup path', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: false,
      model: 'quickstart:pulse-hosted',
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable pulse intelligence/i }));

    expect(getModelsMock).not.toHaveBeenCalled();
    expect(updateSettingsMock).not.toHaveBeenCalled();
    expect(await screen.findByText('Set Up Pulse Intelligence')).toBeInTheDocument();
    expect(
      screen.getByText('Connect a provider to power Patrol, Assistant, and service context.'),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Patrol quickstart ready • 25\/25 runs left • no API key needed yet/i),
    ).not.toBeInTheDocument();
  });

  it('keeps retired hosted quickstart guidance out of default provider setup', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: false,
      enabled: false,
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable pulse intelligence/i }));

    expect(updateSettingsMock).not.toHaveBeenCalled();
    expect(await screen.findByText('Set Up Pulse Intelligence')).toBeInTheDocument();
    expect(
      screen.getByText('Connect a provider to power Patrol, Assistant, and service context.'),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Hosted quickstart requires an activated entitlement/i),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /open hosted handoff/i })).not.toBeInTheDocument();
  });

  it('keeps retired hosted quickstart guidance hidden when upgrade prompts are disabled', async () => {
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: false,
      enabled: false,
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable pulse intelligence/i }));

    expect(updateSettingsMock).not.toHaveBeenCalled();
    expect(await screen.findByText('Set Up Pulse Intelligence')).toBeInTheDocument();
    expect(
      screen.getByText('Connect a provider to power Patrol, Assistant, and service context.'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /open hosted handoff/i })).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Hosted quickstart requires an activated entitlement/i),
    ).not.toBeInTheDocument();
  });
});

describe('AISettings Patrol cost preview and model guidance', () => {
  const flashProjection = (overrides: Record<string, unknown> = {}) => ({
    provider: 'gemini',
    model: 'gemini-2.5-flash',
    model_route: 'gemini:gemini-2.5-flash',
    billed_per_token: true,
    pricing_known: true,
    pricing_as_of: '2026-06-04',
    input_usd_per_mtok: 0.3,
    output_usd_per_mtok: 2.5,
    per_run_input_tokens: 104_528,
    per_run_output_tokens: 4_491,
    per_run_source: 'default',
    history_run_count: 0,
    per_run_usd: 0.0426,
    interval_minutes: 360,
    scheduled_runs_per_day: 4,
    triggered_runs_per_day: 0,
    triggered_per_run_usd: 0,
    scheduled_projected_30d_usd: 5.11,
    projected_30d_usd: 5.11,
    interval_estimates: [
      { interval_minutes: 60, scheduled_runs_per_day: 24, projected_30d_usd: 30.67 },
      { interval_minutes: 180, scheduled_runs_per_day: 8, projected_30d_usd: 10.22 },
      { interval_minutes: 360, scheduled_runs_per_day: 4, projected_30d_usd: 5.11 },
      { interval_minutes: 720, scheduled_runs_per_day: 2, projected_30d_usd: 2.56 },
      { interval_minutes: 1440, scheduled_runs_per_day: 1, projected_30d_usd: 1.28 },
    ],
    budget_usd_30d: 20,
    spend_30d_usd: 3.2,
    spend_30d_known: true,
    patrol_spend_30d_usd: 2.5,
    budget_reached: false,
    recommended_interval_minutes: 360,
    recommendation_reason: 'fits_budget_share',
    recommendation_target_usd: 10,
    ...overrides,
  });

  const sonnetProjection = (intervalMinutes: number) =>
    flashProjection({
      provider: 'anthropic',
      model: 'claude-sonnet-5',
      model_route: 'anthropic:claude-sonnet-5',
      input_usd_per_mtok: 2,
      output_usd_per_mtok: 10,
      per_run_usd: 0.254,
      interval_minutes: intervalMinutes,
      scheduled_runs_per_day: 1440 / intervalMinutes,
      scheduled_projected_30d_usd: intervalMinutes === 360 ? 30.5 : 7.62,
      projected_30d_usd: intervalMinutes === 360 ? 30.5 : 7.62,
      interval_estimates: [
        { interval_minutes: 60, scheduled_runs_per_day: 24, projected_30d_usd: 183 },
        { interval_minutes: 180, scheduled_runs_per_day: 8, projected_30d_usd: 61 },
        { interval_minutes: 360, scheduled_runs_per_day: 4, projected_30d_usd: 30.5 },
        { interval_minutes: 720, scheduled_runs_per_day: 2, projected_30d_usd: 15.25 },
        { interval_minutes: 1440, scheduled_runs_per_day: 1, projected_30d_usd: 7.62 },
      ],
      recommended_interval_minutes: 1440,
    });

  const cloudSettings = (overrides: Partial<AISettingsType> = {}): AISettingsType => ({
    ...baseSettings(),
    enabled: true,
    configured: true,
    model: 'gemini:gemini-2.5-flash',
    gemini_configured: true,
    anthropic_configured: true,
    ollama_configured: true,
    configured_providers: ['gemini', 'anthropic', 'ollama'],
    patrol_interval_minutes: 360,
    cost_budget_usd_30d: 20,
    ...overrides,
  });

  const cloudModels = () => ({
    models: [
      { id: 'gemini:gemini-2.5-flash', name: 'Gemini 2.5 Flash', provider: 'gemini', notable: true },
      {
        id: 'gemini:gemini-2.5-flash-lite',
        name: 'Gemini 2.5 Flash-Lite',
        provider: 'gemini',
        notable: true,
      },
      { id: 'anthropic:claude-sonnet-5', name: 'Claude Sonnet 5', provider: 'anthropic', notable: true },
      { id: 'ollama:qwen3:8b', name: 'qwen3:8b', provider: 'ollama', notable: true },
    ],
  });

  const guidance = () => ({
    rules: [
      {
        provider: 'gemini',
        model_prefix: 'gemini-',
        exclude: ['!flash-lite'],
        level: 'caution',
        reason: 'Flash-Lite could not file Patrol verdicts on a Pro install.',
      },
      {
        provider: 'gemini',
        model_prefix: 'gemini-2.5-flash',
        exclude: ['lite'],
        level: 'suggested',
        reason: 'Lowest-cost standard tier for this provider.',
      },
      {
        provider: 'ollama',
        model_prefix: 'qwen3:8b',
        model_exact: true,
        level: 'recommended',
        reason: 'Passed the Patrol tool-call check.',
      },
    ],
  });

  it('shows the monthly estimate, assumptions, and spend against budget next to the Patrol model', async () => {
    getSettingsMock.mockResolvedValue(cloudSettings());
    getModelsMock.mockResolvedValue(cloudModels());
    getPatrolCostPreviewMock.mockResolvedValue(flashProjection());
    getPatrolModelGuidanceMock.mockResolvedValue(guidance());

    renderComponent('patrol');

    const preview = await screen.findByTestId('patrol-cost-preview');
    expect(preview).toHaveTextContent('About $5.11 a month for Patrol');
    expect(preview).toHaveTextContent('4 scheduled runs a day × about 105k tokens in and 4k out per run');
    expect(preview).toHaveTextContent('roughly three quarters of a word');
    expect(preview).toHaveTextContent(
      'Spent so far: about $3.20 of your $20 30-day budget ($2.50 of that was Patrol).',
    );
    expect(getPatrolCostPreviewMock).toHaveBeenCalledWith(
      { model: 'gemini:gemini-2.5-flash', intervalMinutes: 360 },
      expect.anything(),
    );
    // The schedule options carry the same estimate so the trade-off is visible where it is made.
    const schedule = screen.getByLabelText('Schedule') as HTMLSelectElement;
    const daily = Array.from(schedule.options).find((option) => option.value === '1440');
    expect(daily?.textContent).toContain('≈ $1.28/month');
    expect(screen.getByTestId('patrol-schedule-cost')).toHaveTextContent(
      'About $5.11 a month for Patrol with gemini:gemini-2.5-flash. Estimate.',
    );
  });

  it('pins guided models at the top of the Patrol picker and warns on known failures', async () => {
    getSettingsMock.mockResolvedValue(cloudSettings({ patrol_model: 'gemini:gemini-2.5-flash-lite' }));
    getModelsMock.mockResolvedValue(cloudModels());
    getPatrolCostPreviewMock.mockResolvedValue(flashProjection());
    getPatrolModelGuidanceMock.mockResolvedValue(guidance());

    renderComponent('patrol');

    // The selection itself carries the warning once the picker is closed.
    expect(
      await screen.findByText('Flash-Lite could not file Patrol verdicts on a Pro install.'),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Select Patrol model'));
    expect(await screen.findByText('Suggested for Patrol')).toBeInTheDocument();
    expect(screen.getByText('Recommended for Patrol')).toBeInTheDocument();
    expect(screen.getByText('Suggested starting point')).toBeInTheDocument();
    expect(screen.getAllByText('Caution').length).toBeGreaterThan(0);
    const listbox = screen.getByRole('listbox', { name: 'Select Patrol model' });
    const optionNames = within(listbox)
      .getAllByRole('option')
      .map((option) => option.getAttribute('aria-label') || '');
    const pinnedOllama = optionNames.findIndex((name) => name.startsWith('qwen3:8b'));
    const pinnedFlash = optionNames.findIndex((name) => name.startsWith('Gemini 2.5 Flash.'));
    const groupedLite = optionNames.findIndex((name) => name.startsWith('Gemini 2.5 Flash-Lite'));
    expect(pinnedOllama).toBeGreaterThanOrEqual(0);
    expect(pinnedFlash).toBeGreaterThan(pinnedOllama);
    expect(groupedLite).toBeGreaterThan(pinnedFlash);
  });

  it('slows the default schedule when a per-token model is picked and explains why', async () => {
    getSettingsMock.mockResolvedValue(cloudSettings({ model: 'ollama:qwen3:8b' }));
    getModelsMock.mockResolvedValue(cloudModels());
    getPatrolCostPreviewMock.mockImplementation(async (query: { model?: string; intervalMinutes?: number }) =>
      query.model === 'anthropic:claude-sonnet-5'
        ? sonnetProjection(query.intervalMinutes ?? 360)
        : flashProjection({
            provider: 'ollama',
            model: 'qwen3:8b',
            model_route: 'ollama:qwen3:8b',
            billed_per_token: false,
            projected_30d_usd: 0,
            scheduled_projected_30d_usd: 0,
            recommended_interval_minutes: 0,
            recommendation_reason: 'not_billed_per_token',
          }),
    );

    renderComponent('patrol');

    expect(await screen.findByTestId('patrol-cost-preview')).toHaveTextContent(
      'No per-token bill for Patrol',
    );
    const schedule = screen.getByLabelText('Schedule') as HTMLSelectElement;
    expect(schedule.value).toBe('360');

    fireEvent.click(screen.getByTitle('Select Patrol model'));
    fireEvent.click(await screen.findByRole('option', { name: /^Claude Sonnet 5/ }));

    await waitFor(() => {
      expect(schedule.value).toBe('1440');
    });
    expect(await screen.findByTestId('patrol-schedule-auto-adjust')).toHaveTextContent(
      'Schedule set to once a day because claude-sonnet-5 bills per token: about $7.62 a month instead of $30.50 at every 6 hours',
    );
    expect(screen.getByTestId('patrol-cost-preview')).toHaveTextContent(
      'Change it under Patrol › Schedule.',
    );
    expect(screen.getByTestId('patrol-schedule-cost').parentElement).toHaveTextContent(
      'Changed from every 6 hours to keep the per-token bill down.',
    );
  });

  it('leaves a schedule the install already chose untouched when the model changes', async () => {
    getSettingsMock.mockResolvedValue(
      cloudSettings({ model: 'ollama:qwen3:8b', patrol_interval_minutes: 720 }),
    );
    getModelsMock.mockResolvedValue(cloudModels());
    getPatrolCostPreviewMock.mockImplementation(async (query: { model?: string; intervalMinutes?: number }) =>
      query.model === 'anthropic:claude-sonnet-5'
        ? sonnetProjection(query.intervalMinutes ?? 720)
        : flashProjection({ billed_per_token: false, recommended_interval_minutes: 0 }),
    );

    renderComponent('patrol');
    await screen.findByTestId('patrol-cost-preview');
    const schedule = screen.getByLabelText('Schedule') as HTMLSelectElement;
    expect(schedule.value).toBe('720');

    fireEvent.click(screen.getByTitle('Select Patrol model'));
    fireEvent.click(await screen.findByRole('option', { name: /^Claude Sonnet 5/ }));

    await waitFor(() => {
      expect(getPatrolCostPreviewMock).toHaveBeenCalledWith(
        { model: 'anthropic:claude-sonnet-5', intervalMinutes: 720 },
        expect.anything(),
      );
    });
    expect(schedule.value).toBe('720');
    expect(screen.queryByTestId('patrol-schedule-auto-adjust')).not.toBeInTheDocument();
  });

  it('shows a reached budget as a pause, not a healthy estimate', async () => {
    getSettingsMock.mockResolvedValue(cloudSettings());
    getModelsMock.mockResolvedValue(cloudModels());
    getPatrolCostPreviewMock.mockResolvedValue(
      flashProjection({ spend_30d_usd: 20.4, budget_reached: true }),
    );

    renderComponent('patrol');

    const preview = await screen.findByTestId('patrol-cost-preview');
    expect(preview).toHaveTextContent(
      'Budget reached: about $20.40 of your $20 30-day budget is spent, so Patrol is paused',
    );
  });
});
