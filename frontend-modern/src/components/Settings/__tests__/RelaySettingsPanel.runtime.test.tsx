import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

const loadLicenseStatusMock = vi.fn();
const hasFeatureMock = vi.fn();
const getRelayConfigMock = vi.fn();
const getRelayStatusMock = vi.fn();
const updateRelayConfigMock = vi.fn();
const getQRPayloadMock = vi.fn();
const createTokenMock = vi.fn();
const deleteTokenMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();
const loggerErrorMock = vi.fn();
const loggerWarnMock = vi.fn();
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

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    createToken: (...args: unknown[]) => createTokenMock(...args),
    deleteToken: (...args: unknown[]) => deleteTokenMock(...args),
  },
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showError: (...args: unknown[]) => showErrorMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
    warn: (...args: unknown[]) => loggerWarnMock(...args),
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
    createTokenMock.mockReset();
    deleteTokenMock.mockReset();
    showSuccessMock.mockReset();
    showErrorMock.mockReset();
    loggerErrorMock.mockReset();
    loggerWarnMock.mockReset();
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
    createTokenMock.mockResolvedValue({
      token: 'token-123',
      record: {
        id: 'relay-token-1',
        name: 'Relay mobile device 2026-03-12T00:00:00Z',
        prefix: 'pmp_',
        suffix: '1234',
        createdAt: '',
      },
    });
    deleteTokenMock.mockResolvedValue(undefined);
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
      expect(createTokenMock).toHaveBeenCalledTimes(1);
      expect(getQRPayloadMock).toHaveBeenCalledTimes(1);
      expect(getQRPayloadMock).toHaveBeenCalledWith('token-123');
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
    expect(deleteTokenMock).not.toHaveBeenCalled();
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

  it('deletes the minted pairing token when onboarding payload generation fails', async () => {
    getQRPayloadMock.mockRejectedValueOnce(new Error('missing auth token'));

    render(() => <RelaySettingsPanel canManage />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('wss://relay.example.test/ws/instance')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Pair New Device' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledTimes(1);
      expect(deleteTokenMock).toHaveBeenCalledWith('relay-token-1');
    });

    expect(showErrorMock).toHaveBeenCalledWith('Failed to generate pairing QR code');
    expect(screen.queryByAltText('Pulse mobile pairing QR code')).not.toBeInTheDocument();
  });

  it('replaces the previous pairing token when refreshing the QR code', async () => {
    createTokenMock
      .mockResolvedValueOnce({
        token: 'token-123',
        record: {
          id: 'relay-token-1',
          name: 'Relay mobile device first',
          prefix: 'pmp_',
          suffix: '1234',
          createdAt: '',
        },
      })
      .mockResolvedValueOnce({
        token: 'token-456',
        record: {
          id: 'relay-token-2',
          name: 'Relay mobile device second',
          prefix: 'pmp_',
          suffix: '5678',
          createdAt: '',
        },
      });
    getQRPayloadMock
      .mockResolvedValueOnce({
        schema: 'pulse-onboarding/v1',
        instance_url: 'https://pulse.example.test',
        relay: {
          enabled: true,
          url: 'wss://relay.example.test/ws/instance',
        },
        auth_token: 'token-123',
        deep_link: 'pulse://connect?instance_id=instance-local&auth_token=token-123',
      })
      .mockResolvedValueOnce({
        schema: 'pulse-onboarding/v1',
        instance_url: 'https://pulse.example.test',
        relay: {
          enabled: true,
          url: 'wss://relay.example.test/ws/instance',
        },
        auth_token: 'token-456',
        deep_link: 'pulse://connect?instance_id=instance-local&auth_token=token-456',
      });

    render(() => <RelaySettingsPanel canManage />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('wss://relay.example.test/ws/instance')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Pair New Device' }));

    await waitFor(() => {
      expect(screen.getByText('pulse://connect?instance_id=instance-local&auth_token=token-123')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Refresh QR Code' }));

    await waitFor(() => {
      expect(screen.getByText('pulse://connect?instance_id=instance-local&auth_token=token-456')).toBeInTheDocument();
      expect(deleteTokenMock).toHaveBeenCalledWith('relay-token-1');
    });
  });

  it('keeps the previous QR code when refresh fails', async () => {
    createTokenMock
      .mockResolvedValueOnce({
        token: 'token-123',
        record: {
          id: 'relay-token-1',
          name: 'Relay mobile device first',
          prefix: 'pmp_',
          suffix: '1234',
          createdAt: '',
        },
      })
      .mockResolvedValueOnce({
        token: 'token-456',
        record: {
          id: 'relay-token-2',
          name: 'Relay mobile device second',
          prefix: 'pmp_',
          suffix: '5678',
          createdAt: '',
        },
      });
    getQRPayloadMock
      .mockResolvedValueOnce({
        schema: 'pulse-onboarding/v1',
        instance_url: 'https://pulse.example.test',
        relay: {
          enabled: true,
          url: 'wss://relay.example.test/ws/instance',
        },
        auth_token: 'token-123',
        deep_link: 'pulse://connect?instance_id=instance-local&auth_token=token-123',
      })
      .mockRejectedValueOnce(new Error('refresh failed'));

    render(() => <RelaySettingsPanel canManage />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('wss://relay.example.test/ws/instance')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Pair New Device' }));

    await waitFor(() => {
      expect(screen.getByText('pulse://connect?instance_id=instance-local&auth_token=token-123')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Refresh QR Code' }));

    await waitFor(() => {
      expect(deleteTokenMock).toHaveBeenCalledWith('relay-token-2');
    });

    expect(screen.getByText('pulse://connect?instance_id=instance-local&auth_token=token-123')).toBeInTheDocument();
    expect(showErrorMock).toHaveBeenCalledWith('Failed to generate pairing QR code');
  });
});
