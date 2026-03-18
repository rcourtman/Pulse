import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { SecurityAuthPanel } from '../SecurityAuthPanel';
import { RelaySettingsPanel } from '../RelaySettingsPanel';
import { AuditWebhookPanel } from '../AuditWebhookPanel';

const loadLicenseStatusMock = vi.fn();
const hasFeatureMock = vi.fn();
const getRelayConfigMock = vi.fn();
const getRelayStatusMock = vi.fn();
const apiFetchJSONMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();
const showWarningMock = vi.fn();
const loggerErrorMock = vi.fn();

vi.mock('../QuickSecuritySetup', () => ({
  QuickSecuritySetup: () => <div data-testid="quick-security-setup">Quick Security Setup</div>,
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  licenseLoaded: () => true,
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  getUpgradeActionUrlOrFallback: () => '/upgrade',
  startProTrial: vi.fn(),
  entitlements: () => ({ trial_eligible: false }),
}));

vi.mock('@/api/relay', () => ({
  RelayAPI: {
    getConfig: (...args: unknown[]) => getRelayConfigMock(...args),
    getStatus: (...args: unknown[]) => getRelayStatusMock(...args),
    updateConfig: vi.fn(),
  },
}));

vi.mock('@/api/onboarding', () => ({
  OnboardingAPI: {
    getQRPayload: vi.fn(),
  },
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showError: (...args: unknown[]) => showErrorMock(...args),
  showWarning: (...args: unknown[]) => showWarningMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
    debug: vi.fn(),
    warn: vi.fn(),
  },
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: vi.fn(),
  trackUpgradeClicked: vi.fn(),
}));

describe('settings read-only panel states', () => {
  beforeEach(() => {
    hasFeatureMock.mockReset();
    loadLicenseStatusMock.mockReset();
    getRelayConfigMock.mockReset();
    getRelayStatusMock.mockReset();
    apiFetchJSONMock.mockReset();
    showSuccessMock.mockReset();
    showErrorMock.mockReset();
    showWarningMock.mockReset();
    loggerErrorMock.mockReset();

    hasFeatureMock.mockReturnValue(true);
    loadLicenseStatusMock.mockResolvedValue(undefined);
    getRelayConfigMock.mockResolvedValue({
      enabled: true,
      server_url: 'wss://relay.example.test/ws/instance',
      identity_fingerprint: 'relay-fingerprint',
    });
    getRelayStatusMock.mockResolvedValue({
      connected: true,
      instance_id: 'relay-instance',
      active_channels: 1,
      last_error: '',
    });
    apiFetchJSONMock.mockResolvedValue({
      urls: ['https://audit.example.test/webhook'],
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps authentication controls disabled in read-only mode', async () => {
    const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
    const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(true);
    const [showPasswordModal, setShowPasswordModal] = createSignal(false);
    const [hideLocalLogin] = createSignal(false);

    render(() => (
      <SecurityAuthPanel
        securityStatus={() => ({ hasAuthentication: true, apiTokenConfigured: true })}
        securityStatusLoading={() => false}
        versionInfo={() => null}
        showQuickSecuritySetup={showQuickSecuritySetup}
        setShowQuickSecuritySetup={setShowQuickSecuritySetup}
        showQuickSecurityWizard={showQuickSecurityWizard}
        setShowQuickSecurityWizard={setShowQuickSecurityWizard}
        showPasswordModal={showPasswordModal}
        setShowPasswordModal={setShowPasswordModal}
        hideLocalLogin={hideLocalLogin}
        hideLocalLoginLocked={() => false}
        savingHideLocalLogin={() => false}
        handleHideLocalLoginChange={vi.fn().mockResolvedValue(undefined)}
        loadSecurityStatus={vi.fn().mockResolvedValue(undefined)}
        canManage={false}
      />
    ));

    expect(screen.getByText(/Authentication settings are read-only/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /change password/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /rotate credentials/i })).toBeDisabled();
    expect(screen.queryByTestId('quick-security-setup')).not.toBeInTheDocument();
  });

  it('renders relay settings as read-only when writes are not allowed', async () => {
    render(() => <RelaySettingsPanel canManage={false} />);

    await waitFor(() => {
      expect(screen.getByText(/Remote access settings are read-only/i)).toBeInTheDocument();
    });

    const serverUrlInput = screen.getByDisplayValue('wss://relay.example.test/ws/instance');
    expect(serverUrlInput).toBeDisabled();
    expect(screen.getByRole('button', { name: /pair new device/i })).toBeDisabled();
  });

  it('renders audit webhooks as read-only when writes are not allowed', async () => {
    render(() => <AuditWebhookPanel canManage={false} />);

    await waitFor(() => {
      expect(screen.getByText(/Audit webhook configuration is read-only/i)).toBeInTheDocument();
    });

    expect(screen.getByDisplayValue('')).toBeDisabled();
    expect(screen.getByRole('button', { name: /add endpoint/i })).toBeDisabled();
  });
});
