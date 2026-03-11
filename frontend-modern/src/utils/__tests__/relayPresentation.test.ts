import { describe, expect, it } from 'vitest';
import {
  getRelayConnectionPresentation,
  getRelayDiagnosticClass,
  RELAY_BETA_MESSAGE_CLASS,
  RELAY_BETA_TITLE_CLASS,
  RELAY_CODE_BLOCK_CLASS,
  RELAY_INLINE_ACTION_CLASS,
  RELAY_LAST_ERROR_CLASS,
  RELAY_ONBOARDING_DESCRIPTION,
  RELAY_ONBOARDING_DISCONNECTED_LABEL,
  RELAY_ONBOARDING_SETUP_LABEL,
  RELAY_ONBOARDING_TITLE,
  RELAY_ONBOARDING_TRIAL_LABEL,
  RELAY_ONBOARDING_TRIAL_STARTING_LABEL,
  RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL,
  RELAY_ONBOARDING_TRIAL_HINT,
  RELAY_ONBOARDING_UPGRADE_LABEL,
  RELAY_PRIMARY_BUTTON_CLASS,
  RELAY_PRIMARY_LINK_CLASS,
  RELAY_QR_IMAGE_CLASS,
  RELAY_READONLY_NOTICE_CLASS,
  RELAY_SECONDARY_BUTTON_CLASS,
} from '@/utils/relayPresentation';

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
    expect(RELAY_BETA_TITLE_CLASS).toContain('font-medium');
    expect(RELAY_BETA_MESSAGE_CLASS).toContain('text-muted');
    expect(RELAY_LAST_ERROR_CLASS).toContain('text-red-600');
    expect(RELAY_CODE_BLOCK_CLASS).toContain('font-mono');
    expect(RELAY_QR_IMAGE_CLASS).toContain('border-border');
    expect(getRelayDiagnosticClass('error')).toContain('bg-red-50');
    expect(getRelayDiagnosticClass('warning')).toContain('bg-amber-50');
  });

  it('centralizes relay onboarding copy', () => {
    expect(RELAY_ONBOARDING_TITLE).toBe('Pair Your Mobile Device');
    expect(RELAY_ONBOARDING_DESCRIPTION).toContain('remote monitoring');
    expect(RELAY_ONBOARDING_UPGRADE_LABEL).toBe('Get Relay — $49/yr');
    expect(RELAY_ONBOARDING_TRIAL_LABEL).toBe('or start a Pro trial');
    expect(RELAY_ONBOARDING_TRIAL_STARTING_LABEL).toBe('Starting trial...');
    expect(RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL).toBe('Start Free Trial & Set Up Mobile');
    expect(RELAY_ONBOARDING_TRIAL_HINT).toBe('14-DAY PRO TRIAL · NO CREDIT CARD REQUIRED');
    expect(RELAY_ONBOARDING_SETUP_LABEL).toBe('Set Up Relay');
    expect(RELAY_ONBOARDING_DISCONNECTED_LABEL).toBe('Relay is currently disconnected.');
  });
});
