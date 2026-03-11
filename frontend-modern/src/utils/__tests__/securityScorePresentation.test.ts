import { describe, expect, it } from 'vitest';
import {
  getSecurityFeatureCardPresentation,
  getSecurityFeatureStatePresentation,
  getSecurityNetworkAccessSubtitle,
  getSecurityPostureItems,
  getSecurityScoreIconComponent,
  getSecurityScorePresentation,
  getSecurityScoreSymbol,
  getSecurityScoreTextClass,
  getSecurityWarningPresentation,
} from '@/utils/securityScorePresentation';

describe('securityScorePresentation', () => {
  it('returns strong posture presentation for high scores', () => {
    expect(getSecurityScorePresentation(92)).toMatchObject({
      label: 'Strong',
      icon: 'shield-check',
    });
    expect(getSecurityScorePresentation(92).tone.badge).toContain('emerald');
  });

  it('returns moderate posture presentation for medium scores', () => {
    expect(getSecurityScorePresentation(65)).toMatchObject({
      label: 'Moderate',
      icon: 'shield',
    });
    expect(getSecurityScorePresentation(65).tone.badge).toContain('amber');
  });

  it('returns weak posture presentation for low scores', () => {
    expect(getSecurityScorePresentation(25)).toMatchObject({
      label: 'Weak',
      icon: 'shield-alert',
    });
    expect(getSecurityScorePresentation(25).tone.badge).toContain('rose');
  });

  it('returns matching text classes and symbols for score tiers', () => {
    expect(getSecurityScoreTextClass(90)).toContain('emerald');
    expect(getSecurityScoreSymbol(90)).toBe('✓');
    expect(getSecurityScoreTextClass(60)).toContain('amber');
    expect(getSecurityScoreSymbol(60)).toBe('!');
    expect(getSecurityScoreTextClass(20)).toContain('rose');
    expect(getSecurityScoreSymbol(20)).toBe('!!');
  });

  it('returns a critical public-access warning presentation', () => {
    expect(
      getSecurityWarningPresentation({
        score: 40,
        publicAccess: true,
        hasAuthentication: false,
      }),
    ).toMatchObject({
      background: 'bg-red-50 dark:bg-red-900',
      border: 'border-red-200 dark:border-red-800',
      messageClass: 'font-semibold text-red-700 dark:text-red-300',
    });
  });

  it('returns a shared low-score warning presentation for non-public setups', () => {
    expect(
      getSecurityWarningPresentation({
        score: 35,
        publicAccess: false,
        hasAuthentication: false,
      }),
    ).toMatchObject({
      background: 'bg-red-50 dark:bg-red-900',
      border: 'border-red-200 dark:border-red-800',
      messageClass: 'text-base-content',
    });
  });

  it('returns a shared moderate warning presentation when score is only moderately low', () => {
    expect(
      getSecurityWarningPresentation({
        score: 60,
        publicAccess: false,
        hasAuthentication: false,
      }),
    ).toMatchObject({
      background: 'bg-yellow-50 dark:bg-yellow-900',
      border: 'border-yellow-200 dark:border-yellow-800',
      messageClass: 'text-base-content',
    });
  });

  it('returns canonical yes/no feature-state presentation', () => {
    expect(getSecurityFeatureStatePresentation(true)).toEqual({
      label: 'Yes',
      className: 'text-green-600',
    });
    expect(getSecurityFeatureStatePresentation(false)).toEqual({
      label: 'No',
      className: 'text-red-600',
    });
  });

  it('returns canonical score icon components for posture tiers', () => {
    expect(getSecurityScoreIconComponent(90).name).toBe('ShieldCheck');
    expect(getSecurityScoreIconComponent(60).name).toBe('Shield');
    expect(getSecurityScoreIconComponent(20).name).toBe('ShieldAlert');
  });

  it('returns canonical feature-card presentation for enabled, critical, and optional states', () => {
    expect(
      getSecurityFeatureCardPresentation({
        enabled: true,
        critical: true,
      }),
    ).toMatchObject({
      cardClassName:
        'border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950',
      iconClassName: 'text-emerald-500 dark:text-emerald-400',
      statusLabel: 'Enabled',
    });

    expect(
      getSecurityFeatureCardPresentation({
        enabled: false,
        critical: true,
      }),
    ).toMatchObject({
      cardClassName: 'border-rose-200 dark:border-rose-800 bg-rose-50 dark:bg-rose-950',
      iconClassName: 'text-rose-500 dark:text-rose-400',
      statusLabel: 'Disabled',
    });

    expect(
      getSecurityFeatureCardPresentation({
        enabled: false,
        critical: false,
      }),
    ).toMatchObject({
      cardClassName: 'border-border bg-surface-alt',
      iconClassName: 'text-muted',
      statusLabel: 'Disabled',
    });
  });

  it('returns canonical security posture items and network subtitle', () => {
    expect(
      getSecurityPostureItems({
        hasAuthentication: true,
        ssoEnabled: false,
        hasProxyAuth: false,
        apiTokenConfigured: true,
        exportProtected: true,
        unprotectedExportAllowed: false,
        hasHTTPS: true,
        hasAuditLogging: false,
        requiresAuth: true,
        publicAccess: true,
        isPrivateNetwork: false,
      }),
    ).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: 'password',
          label: 'Password login',
          description: 'Active',
          critical: true,
        }),
        expect.objectContaining({
          key: 'export',
          description: 'Token + passphrase required',
        }),
      ]),
    );

    expect(
      getSecurityNetworkAccessSubtitle({
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        hasAuditLogging: true,
        requiresAuth: true,
        publicAccess: true,
        isPrivateNetwork: false,
      }),
    ).toBe('Public network access detected');
    expect(
      getSecurityNetworkAccessSubtitle({
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        hasAuditLogging: true,
        requiresAuth: true,
        publicAccess: false,
        isPrivateNetwork: true,
      }),
    ).toBe('Private network access');
  });
});
