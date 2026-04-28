import { describe, expect, it } from 'vitest';
import {
  getRelayConnectionPresentation,
  getRelayDiagnosticClass,
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
    expect(RELAY_SETTINGS_DESCRIPTION).toContain('Pulse Mobile pairing');
    expect(RELAY_LICENSE_REQUIRED_MESSAGE).toContain('supported Pulse Mobile clients');
    expect(RELAY_LICENSE_REQUIRED_MESSAGE).toContain('available with Relay or Pro');
    expect(RELAY_PAIRING_AVAILABILITY_TITLE).toBe('Pair Pulse Mobile through Relay');
    expect(RELAY_PAIRING_AVAILABILITY_MESSAGE).toContain('QR code or deep link');
    expect(RELAY_ENABLE_HELP_TEXT).toContain('Pulse Mobile pairing');
  });

  it('does not retain retired Relay price or trial-era onboarding copy', () => {
    expect(relayPresentationSource).not.toContain('RELAY_ONBOARDING_UPGRADE_LABEL');
    expect(relayPresentationSource).not.toContain('Get Relay');
    expect(relayPresentationSource).not.toContain('$39');
    expect(relayPresentationSource).not.toContain('$49');
    expect(relayPresentationSource).not.toContain('$99');
    expect(relayPresentationSource).not.toContain('Start free trial');
  });
});
