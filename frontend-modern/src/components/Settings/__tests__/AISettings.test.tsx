import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import type { AISettings as AISettingsType } from '@/types/ai';
import { AISettings } from '../AISettings';

const getSettingsMock = vi.fn();
const updateSettingsMock = vi.fn();
const getModelsMock = vi.fn();
const testProviderMock = vi.fn();
const testConnectionMock = vi.fn();
const listSessionsMock = vi.fn();
const summarizeSessionMock = vi.fn();
const getSessionDiffMock = vi.fn();
const revertSessionMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const notificationInfoMock = vi.fn();
const loggerDebugMock = vi.fn();
const loggerErrorMock = vi.fn();
const hasFeatureMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const loadCommercialPostureMock = vi.fn();
const commercialPostureMock = vi.fn();
const entitlementsMock = vi.fn();
const trackPaywallViewedMock = vi.fn();
const trackUpgradeClickedMock = vi.fn();
const runStartProTrialActionMock = vi.fn();

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
    getSessionDiff: (...args: unknown[]) => getSessionDiffMock(...args),
    revertSession: (...args: unknown[]) => revertSessionMock(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
    info: (...args: unknown[]) => notificationInfoMock(...args),
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
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
}));

vi.mock('@/stores/licenseCommercial', () => ({
  canOfferCommercialTrial: () => commercialPostureMock()?.trial_eligible !== false,
  commercialPosture: (...args: unknown[]) => commercialPostureMock(...args),
  entitlements: (...args: unknown[]) => entitlementsMock(...args),
  getUpgradeActionDestination: () => ({ href: 'https://example.com/upgrade', external: true }),
  loadCommercialPosture: (...args: unknown[]) => loadCommercialPostureMock(...args),
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
}));

vi.mock('@/utils/trialStartAction', () => ({
  runStartProTrialAction: (...args: unknown[]) => runStartProTrialActionMock(...args),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
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
  configured_providers: [],
});

const renderComponent = () =>
  render(() => (
    <Router>
      <Route path="/" component={() => <AISettings />} />
    </Router>
  ));

const resetAllMocks = () => {
  getSettingsMock.mockReset();
  updateSettingsMock.mockReset();
  getModelsMock.mockReset();
  testProviderMock.mockReset();
  testConnectionMock.mockReset();
  listSessionsMock.mockReset();
  summarizeSessionMock.mockReset();
  getSessionDiffMock.mockReset();
  revertSessionMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  notificationInfoMock.mockReset();
  loggerDebugMock.mockReset();
  loggerErrorMock.mockReset();
  hasFeatureMock.mockReset();
  loadLicenseStatusMock.mockReset();
  loadCommercialPostureMock.mockReset();
  commercialPostureMock.mockReset();
  entitlementsMock.mockReset();
  trackPaywallViewedMock.mockReset();
  trackUpgradeClickedMock.mockReset();
  runStartProTrialActionMock.mockReset();
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
  listSessionsMock.mockResolvedValue([]);
  summarizeSessionMock.mockResolvedValue(undefined);
  getSessionDiffMock.mockResolvedValue({ files: [], summary: '' });
  revertSessionMock.mockResolvedValue(undefined);
  runStartProTrialActionMock.mockResolvedValue('activated');
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
      expect(screen.getByText('Claude Sonnet')).toBeInTheDocument();
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
      expect(screen.getByText(/Unable to load Assistant & Patrol settings/)).toBeInTheDocument();
    });

    // Retry button should be present
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();

    // Save button should NOT be present (form is hidden)
    expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument();
  });

  it('clears error and shows form after successful retry', async () => {
    getSettingsMock.mockRejectedValueOnce(new Error('Network error'));

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Unable to load Assistant & Patrol settings/)).toBeInTheDocument();
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
      expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
    });

    // Error banner should be gone
    expect(screen.queryByText(/Unable to load Assistant & Patrol settings/)).not.toBeInTheDocument();

    // Verify retry actually called getSettings again
    expect(getSettingsMock).toHaveBeenCalledTimes(2);
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
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }));

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
      expect(testProviderMock).toHaveBeenCalledWith('openrouter');
    });
  });
});

describe('AISettings quickstart enablement flow', () => {
  beforeEach(() => {
    resetAllMocks();
    setupDefaultMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('does not load provider model catalog for quickstart-only installs', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: false,
      model: 'quickstart:pulse-hosted',
      quickstart_credits_total: 25,
      quickstart_credits_remaining: 25,
      quickstart_credits_available: true,
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    expect(getModelsMock).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Patrol quickstart ready • 25\/25 runs left • no API key needed yet/i),
    ).toBeInTheDocument();
  });

  it('enables directly when quickstart is available on an activated install', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: false,
      model: 'quickstart:pulse-hosted',
      quickstart_credits_total: 25,
      quickstart_credits_remaining: 25,
      quickstart_credits_available: true,
    });
    updateSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: true,
      enabled: true,
      model: 'quickstart:pulse-hosted',
      chat_model: 'quickstart:pulse-hosted',
      patrol_model: 'quickstart:pulse-hosted',
      quickstart_credits_total: 25,
      quickstart_credits_remaining: 25,
      quickstart_credits_available: true,
      using_quickstart: true,
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable assistant and patrol/i }));

    await waitFor(() => {
      expect(updateSettingsMock).toHaveBeenCalledWith({ enabled: true });
    });

    expect(screen.queryByText('Choose a provider to get started')).not.toBeInTheDocument();
    expect(notificationSuccessMock).toHaveBeenCalledWith('Pulse Assistant enabled');
  });

  it('shows activation-aware setup guidance instead of generic provider setup when quickstart is blocked', async () => {
    getSettingsMock.mockResolvedValue({
      ...baseSettings(),
      configured: false,
      enabled: false,
      quickstart_credits_total: 0,
      quickstart_credits_remaining: 0,
      quickstart_credits_available: false,
      quickstart_blocked_reason:
        'Activate this install or start a trial to use Patrol quickstart. Otherwise connect your API key.',
    });

    renderComponent();

    await waitFor(() => {
      expect(getSettingsMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: /enable assistant and patrol/i }));

    expect(updateSettingsMock).not.toHaveBeenCalled();
    expect(
      await screen.findByText('Activate quickstart or connect a provider'),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        'Activate this install or start a trial to use Patrol quickstart. Otherwise connect your API key.',
      ),
    ).toHaveLength(2);
    expect(screen.getByRole('button', { name: /start trial/i })).toBeInTheDocument();
  });
});
