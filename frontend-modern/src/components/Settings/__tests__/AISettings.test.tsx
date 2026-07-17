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
  ollama_keep_alive: '30s',
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
    expect(screen.getByLabelText('Chat action mode')).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /Ask first/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Save Assistant settings/i })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'External agents' })).toBeInTheDocument();
    expect(
      screen.getByText('Connect external tools to read Pulse context and request Patrol work.', {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show connector setup' })).toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: 'Connector setup' }),
    ).not.toBeInTheDocument();
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
    expect(
      screen.queryByText(/Choose a Patrol mode on the Patrol page/i),
    ).not.toBeInTheDocument();
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

    expect(screen.queryByRole('option', { name: /Allow chat-only actions/i })).not.toBeInTheDocument();
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
      expect(
        screen.getByRole('button', { name: /Service Context Auto 24h/i }),
      ).toBeInTheDocument();
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
      expect(testProviderMock).toHaveBeenCalledWith(
        'openrouter',
        'openrouter:openai/gpt-4o-mini',
      );
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
