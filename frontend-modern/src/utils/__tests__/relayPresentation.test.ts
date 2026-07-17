import { describe, expect, it } from 'vitest';
import {
  getRelayStatusErrorMessage,
  getRelayConnectionPresentation,
  getRelayDiagnosticClass,
  RELAY_ACTIVATION_REQUIRED_LABEL,
  RELAY_ACTIVATION_REQUIRED_MESSAGE,
  RELAY_CODE_BLOCK_CLASS,
  RELAY_ENABLE_HELP_TEXT,
  RELAY_INFO_MESSAGE_CLASS,
  RELAY_INFO_TITLE_CLASS,
  RELAY_INLINE_ACTION_CLASS,
  RELAY_LAST_ERROR_CLASS,
  RELAY_LICENSE_REQUIRED_MESSAGE,
  RELAY_PAIRING_AVAILABILITY_MESSAGE,
  RELAY_PAIRING_AVAILABILITY_TITLE,
  RELAY_PRIMARY_BUTTON_CLASS,
  RELAY_PRIMARY_LINK_CLASS,
  RELAY_QR_IMAGE_CLASS,
  RELAY_READONLY_NOTICE_CLASS,
  RELAY_SECONDARY_BUTTON_CLASS,
  RELAY_SETTINGS_DESCRIPTION,
} from '@/utils/relayPresentation';
import relayPresentationSource from '@/utils/relayPresentation.ts?raw';

describe('relayPresentation', () => {
  it('returns muted not-enabled presentation when relay is disabled', () => {
    expect(getRelayConnectionPresentation({ enabled: false } as never, null)).toEqual({
      variant: 'muted',
      label: 'Not enabled',
      pulse: false,
    });
  });

  it('returns success presentation when relay is connected', () => {
    expect(
      getRelayConnectionPresentation({ enabled: true } as never, { connected: true } as never),
    ).toEqual({
      variant: 'success',
      label: 'Connected',
      pulse: true,
    });
  });

  it('returns danger presentation when relay is enabled but disconnected', () => {
    expect(
      getRelayConnectionPresentation({ enabled: true } as never, { connected: false } as never),
    ).toEqual({
      variant: 'danger',
      label: 'Disconnected',
      pulse: false,
    });
  });

  it('returns activation-required presentation for missing Relay token errors', () => {
    expect(
      getRelayConnectionPresentation(
        { enabled: true } as never,
        {
          connected: false,
          active_channels: 0,
          last_error: 'register: no license token available',
        },
      ),
    ).toEqual({
      variant: 'danger',
      label: RELAY_ACTIVATION_REQUIRED_LABEL,
      pulse: false,
    });
  });

  it('translates missing Relay token errors while preserving other diagnostics', () => {
    expect(
      getRelayStatusErrorMessage({
        connected: false,
        active_channels: 0,
        last_error: 'register: no license token available',
      }),
    ).toBe(RELAY_ACTIVATION_REQUIRED_MESSAGE);

    expect(
      getRelayStatusErrorMessage({
        connected: false,
        active_channels: 0,
        last_error: 'relay handshake failed',
      }),
    ).toBe('relay handshake failed');
    expect(getRelayStatusErrorMessage(null)).toBeNull();
  });

  it('centralizes relay action and diagnostics styling', () => {
    expect(RELAY_READONLY_NOTICE_CLASS).toContain('border-blue-200');
    expect(RELAY_PRIMARY_BUTTON_CLASS).toContain('bg-blue-600');
    expect(RELAY_PRIMARY_LINK_CLASS).toContain('text-center');
    expect(RELAY_SECONDARY_BUTTON_CLASS).toContain('bg-surface-hover');
    expect(RELAY_INLINE_ACTION_CLASS).toContain('hover:underline');
    expect(RELAY_INFO_TITLE_CLASS).toContain('font-medium');
    expect(RELAY_INFO_MESSAGE_CLASS).toContain('text-muted');
    expect(RELAY_LAST_ERROR_CLASS).toContain('text-red-600');
    expect(RELAY_CODE_BLOCK_CLASS).toContain('font-mono');
    expect(RELAY_QR_IMAGE_CLASS).toContain('border-border');
    expect(getRelayDiagnosticClass('error')).toContain('bg-red-50');
    expect(getRelayDiagnosticClass('warning')).toContain('bg-amber-50');
  });

  it('centralizes relay availability copy', () => {
    // Value-first copy: every surface states the user job (push notifications
    // through the Pulse Mobile app, no port forwarding/VPN), not transport
    // mechanics.
    expect(RELAY_SETTINGS_DESCRIPTION).toContain('Pulse Mobile app');
    expect(RELAY_SETTINGS_DESCRIPTION).toContain('push notifications');
    expect(RELAY_SETTINGS_DESCRIPTION).toContain('no port forwarding or VPN');
    expect(RELAY_LICENSE_REQUIRED_MESSAGE).toContain('Pulse Mobile app');
    expect(RELAY_LICENSE_REQUIRED_MESSAGE).toContain('push notifications');
    expect(RELAY_LICENSE_REQUIRED_MESSAGE).toContain('Available with Relay and Pro plans');
    expect(RELAY_LICENSE_REQUIRED_MESSAGE).not.toContain('Relay or Pro');
    expect(RELAY_PAIRING_AVAILABILITY_TITLE).toBe('Pair Pulse Mobile through Relay');
    expect(RELAY_PAIRING_AVAILABILITY_MESSAGE).toContain('QR code');
    expect(RELAY_PAIRING_AVAILABILITY_MESSAGE).toContain('deep link');
    expect(RELAY_PAIRING_AVAILABILITY_MESSAGE).toContain('push notifications');
    expect(RELAY_ENABLE_HELP_TEXT).toContain('Pulse Mobile devices');
    expect(RELAY_ENABLE_HELP_TEXT).toContain('No inbound ports');
    expect(RELAY_ACTIVATION_REQUIRED_LABEL).toBe('Activation required');
    expect(RELAY_ACTIVATION_REQUIRED_MESSAGE).toContain('active Relay token');
    expect(RELAY_ACTIVATION_REQUIRED_MESSAGE).toContain('Relay-capable plan');
  });

  it('does not retain retired Relay price or trial-era onboarding copy', () => {
    expect(relayPresentationSource).not.toContain('RELAY_ONBOARDING_UPGRADE_LABEL');
    expect(relayPresentationSource).not.toContain('Get Relay');
    expect(relayPresentationSource).not.toContain('$39');
    expect(relayPresentationSource).not.toContain('$49');
    expect(relayPresentationSource).not.toContain('$99');
    expect(relayPresentationSource).not.toContain('Start free trial');
    expect(relayPresentationSource).not.toContain('Pro feature gate');
  });
});
