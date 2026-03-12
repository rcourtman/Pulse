import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

const loadLicenseStatusMock = vi.fn();
const hasFeatureMock = vi.fn();
const getRelayConfigMock = vi.fn();
const getRelayStatusMock = vi.fn();
const updateRelayConfigMock = vi.fn();
const getQRPayloadMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();
const loggerErrorMock = vi.fn();
const qrToDataUrlMock = vi.fn();

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
    updateConfig: (...args: unknown[]) => updateRelayConfigMock(...args),
  },
}));

vi.mock('@/api/onboarding', () => ({
  OnboardingAPI: {
    getQRPayload: (...args: unknown[]) => getQRPayloadMock(...args),
  },
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showError: (...args: unknown[]) => showErrorMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('qrcode', () => ({
  default: {
    toDataURL: (...args: unknown[]) => qrToDataUrlMock(...args),
  },
}));

import { RelaySettingsPanel } from '../RelaySettingsPanel';

describe('RelaySettingsPanel runtime', () => {
  beforeEach(() => {
    loadLicenseStatusMock.mockReset();
    hasFeatureMock.mockReset();
    getRelayConfigMock.mockReset();
    getRelayStatusMock.mockReset();
    updateRelayConfigMock.mockReset();
    getQRPayloadMock.mockReset();
    showSuccessMock.mockReset();
    showErrorMock.mockReset();
    loggerErrorMock.mockReset();
    qrToDataUrlMock.mockReset();

    hasFeatureMock.mockReturnValue(true);
    loadLicenseStatusMock.mockResolvedValue(undefined);
    getRelayConfigMock.mockResolvedValue({
      enabled: true,
      server_url: 'wss://relay.example.test/ws/instance',
      identity_fingerprint: 'relay-fingerprint',
    });
    getRelayStatusMock.mockResolvedValue({
      connected: true,
      instance_id: 'instance-local',
      active_channels: 1,
      last_error: '',
    });
    getQRPayloadMock.mockResolvedValue({
      schema: 'pulse-onboarding/v1',
      instance_url: 'https://pulse.example.test',
      relay: {
        enabled: true,
        url: 'wss://relay.example.test/ws/instance',
      },
      auth_token: 'token-123',
      deep_link: 'pulse://connect?instance_id=instance-local',
      diagnostics: [
        {
          code: 'relay_beta',
          severity: 'warning',
          message: 'Beta-only pairing flow.',
        },
      ],
    });
    qrToDataUrlMock.mockResolvedValue('data:image/png;base64,qr');
  });

  afterEach(() => {
    cleanup();
  });

  it('loads connected relay state and generates a pairing QR payload', async () => {
    render(() => <RelaySettingsPanel canManage />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('wss://relay.example.test/ws/instance')).toBeInTheDocument();
    });

    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(screen.getByText('Instance: instance-local')).toBeInTheDocument();
    expect(screen.getByText('1 active channel')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Pair New Device' }));

    await waitFor(() => {
      expect(getQRPayloadMock).toHaveBeenCalledTimes(1);
      expect(qrToDataUrlMock).toHaveBeenCalledWith('pulse://connect?instance_id=instance-local', {
        width: 256,
        margin: 2,
      });
    });

    expect(screen.getByAltText('Pulse mobile pairing QR code')).toHaveAttribute(
      'src',
      'data:image/png;base64,qr',
    );
    expect(screen.getByText('pulse://connect?instance_id=instance-local')).toBeInTheDocument();
    expect(screen.getByText('Diagnostics')).toBeInTheDocument();
    expect(screen.getByText('Beta-only pairing flow.')).toBeInTheDocument();
    expect(screen.getByText('relay_beta')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Copy Payload' })).toBeInTheDocument();
  });

  it('shows disconnected state and withholds pairing until relay is connected', async () => {
    getRelayStatusMock.mockResolvedValueOnce({
      connected: false,
      instance_id: 'instance-local',
      active_channels: 0,
      last_error: 'relay handshake failed',
    });

    render(() => <RelaySettingsPanel canManage />);

    await waitFor(() => {
      expect(screen.getByText('Disconnected')).toBeInTheDocument();
    });

    expect(screen.getByText('relay handshake failed')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Pair New Device' })).not.toBeInTheDocument();
    expect(getQRPayloadMock).not.toHaveBeenCalled();
  });
});
