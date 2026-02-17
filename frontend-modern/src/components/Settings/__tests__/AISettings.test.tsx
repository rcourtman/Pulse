import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

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
const notifySettingsChangedMock = vi.fn();
const loggerDebugMock = vi.fn();
const loggerErrorMock = vi.fn();
const hasFeatureMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const trackPaywallViewedMock = vi.fn();
const trackUpgradeClickedMock = vi.fn();

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

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: {
    notifySettingsChanged: (...args: unknown[]) => notifySettingsChangedMock(...args),
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
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  getUpgradeActionUrlOrFallback: () => '/upgrade',
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
}));

const baseSettings = (): AISettingsType => ({
  enabled: false,
  provider: 'anthropic',
  api_key_set: false,
  model: '',
  configured: false,
  autonomous_mode: false,
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

describe('AISettings OpenRouter flow', () => {
  beforeEach(() => {
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
    notifySettingsChangedMock.mockReset();
    loggerDebugMock.mockReset();
    loggerErrorMock.mockReset();
    hasFeatureMock.mockReset();
    loadLicenseStatusMock.mockReset();
    trackPaywallViewedMock.mockReset();
    trackUpgradeClickedMock.mockReset();

    hasFeatureMock.mockReturnValue(true);
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
        expect.objectContaining({ openrouter_api_key: 'sk-or-configured' }),
      );
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
