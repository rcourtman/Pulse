import { describe, expect, it } from 'vitest';
import {
  getSecurityHardeningActions,
  getSecurityNetworkAccessSubtitle,
  getSecurityPostureItems,
  getSecurityScorePresentation,
  getSecurityWarningPresentation,
} from '@/utils/securityScorePresentation';

// Branch-coverage companion to securityScorePresentation.test.ts.
//
// The sibling suite already exercises one sample per score band (92 / 65 / 25),
// the public+unauthenticated red warning, the auth-disabled message, the
// 2-item missing-controls list, the "off" arm of shouldShowGlobalSecurityWarning,
// both critical/recommended configure-https severities, and a fully-secured
// posture items snapshot.
//
// This file targets the RESIDUAL arms:
//   1. Score-threshold boundaries (exactly 80, exactly 50, just-below 80, 49, 0).
//   2. formatList branches: 1-item, 3-item oxford-comma, and 0-item (default
//      "Review the remaining..." message retained).
//   3. The publicAccess+authenticated arm of the warning message
//      ("reachable from a public network and is still missing ...").
//   4. The non-Moderate posture arm in the warning background/border ternary
//      (Strong or Weak score still yields a RED background, not yellow).
//   5. The `unprotectedExportAllowed` second operand of the
//      `!exportProtected || unprotectedExportAllowed` guard in
//      getSecurityHardeningActions, plus the empty-actions case.
//   6. The "Not configured" / "HTTP only" / "Not enabled" / "Unprotected"
//      negative description arms of every getSecurityPostureItems entry.
//   7. The publicAccess=true && isPrivateNetwork=true arm of
//      getSecurityNetworkAccessSubtitle (returns "Private" because
//      !isPrivateNetwork is false).

describe('securityScorePresentation — branch coverage (batch 0718)', () => {
  describe('getSecurityScorePresentation — boundary values for the 80/50 thresholds', () => {
    it('treats score === 80 as the start of the Strong band (>=80 boundary)', () => {
      const out = getSecurityScorePresentation(80);
      expect(out).toMatchObject({ label: 'Strong', icon: 'shield-check' });
      // Pin every tone field so a silent class rename fails loudly.
      expect(out.tone).toEqual({
        headerBg: 'bg-emerald-50 dark:bg-emerald-950',
        headerBorder: 'border-b border-emerald-200 dark:border-emerald-800',
        iconWrap: 'bg-emerald-100 dark:bg-emerald-900',
        icon: 'text-emerald-700 dark:text-emerald-300',
        subtitle: 'text-emerald-700 dark:text-emerald-300',
        score: 'text-emerald-800 dark:text-emerald-200',
        badge: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
      });
    });

    it('treats score === 79 as Moderate (one below the Strong boundary)', () => {
      expect(getSecurityScorePresentation(79)).toMatchObject({
        label: 'Moderate',
        icon: 'shield',
      });
      expect(getSecurityScorePresentation(79).tone.badge).toBe(
        'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      );
    });

    it('treats score === 50 as Moderate (>=50 boundary, after >=80 fails)', () => {
      expect(getSecurityScorePresentation(50)).toMatchObject({
        label: 'Moderate',
        icon: 'shield',
      });
      expect(getSecurityScorePresentation(50).tone.headerBg).toBe('bg-amber-50 dark:bg-amber-950');
    });

    it('treats score === 49 as Weak (one below the Moderate boundary)', () => {
      expect(getSecurityScorePresentation(49)).toMatchObject({
        label: 'Weak',
        icon: 'shield-alert',
      });
      expect(getSecurityScorePresentation(49).tone.headerBorder).toBe(
        'border-b border-rose-200 dark:border-rose-800',
      );
    });

    it('treats score === 0 (zero input) as Weak', () => {
      const out = getSecurityScorePresentation(0);
      expect(out).toMatchObject({ label: 'Weak', icon: 'shield-alert' });
      expect(out.tone.score).toBe('text-rose-800 dark:text-rose-200');
      expect(out.tone.iconWrap).toBe('bg-rose-100 dark:bg-rose-900');
    });
  });

  describe('getSecurityWarningPresentation — formatList + message arms', () => {
    it('formats a 3-item missing-controls list with an Oxford comma', () => {
      // auth=true, public=false, all three controls missing.
      const out = getSecurityWarningPresentation({
        score: 60,
        publicAccess: false,
        hasAuthentication: true,
        apiTokenConfigured: false,
        exportProtected: false,
        hasHTTPS: false,
      });
      expect(out.message).toBe(
        'Authentication is enabled, but this Pulse instance is still missing HTTPS, an API token, and protected exports.',
      );
      // Score 60 -> Moderate posture -> yellow tone.
      expect(out.background).toBe('bg-yellow-50 dark:bg-yellow-900');
      expect(out.border).toBe('border-yellow-200 dark:border-yellow-800');
      expect(out.messageClass).toBe('text-base-content');
    });

    it('formats a single missing control without a trailing conjunction (items.length <= 1 arm)', () => {
      // Only HTTPS missing; token + export present.
      const out = getSecurityWarningPresentation({
        score: 60,
        publicAccess: false,
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        hasHTTPS: false,
      });
      expect(out.message).toBe(
        'Authentication is enabled, but this Pulse instance is still missing HTTPS.',
      );
    });

    it('keeps the default review message when authentication is enabled and no controls are missing', () => {
      // The `else if (missingControls.length > 0)` arm is FALSE, so the
      // initial default message is preserved.
      const out = getSecurityWarningPresentation({
        score: 40,
        publicAccess: false,
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        hasHTTPS: true,
      });
      expect(out.message).toBe(
        'Review the remaining security settings before using this instance for live infrastructure.',
      );
      // Weak score (40 < 50) -> non-Moderate arm -> red tone despite auth ok.
      expect(out.background).toBe('bg-red-50 dark:bg-red-900');
      expect(out.border).toBe('border-red-200 dark:border-red-800');
    });

    it('uses the public-reachability message arm when publicAccess && hasAuthentication', () => {
      const out = getSecurityWarningPresentation({
        score: 60,
        publicAccess: true,
        hasAuthentication: true,
        apiTokenConfigured: false,
        exportProtected: true,
        hasHTTPS: false,
      });
      expect(out.message).toBe(
        'This Pulse instance is reachable from a public network and is still missing HTTPS and an API token.',
      );
    });

    it('returns a red (non-Moderate) tone even for a Strong score in the missing-controls path', () => {
      // Subtle: a Strong score (>=80) with auth enabled but one missing control
      // still lands on the red ternary arm because posture.label !== 'Moderate'.
      const out = getSecurityWarningPresentation({
        score: 85,
        publicAccess: false,
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        hasHTTPS: false,
      });
      expect(out.message).toBe(
        'Authentication is enabled, but this Pulse instance is still missing HTTPS.',
      );
      expect(out.background).toBe('bg-red-50 dark:bg-red-900');
      expect(out.border).toBe('border-red-200 dark:border-red-800');
    });
  });

  describe('getSecurityHardeningActions — residual guard and empty-result arms', () => {
    it('returns an empty array when the posture is fully hardened (every guard false)', () => {
      expect(
        getSecurityHardeningActions({
          hasAuthentication: true,
          apiTokenConfigured: true,
          exportProtected: true,
          unprotectedExportAllowed: false,
          hasHTTPS: true,
          hasAuditLogging: true,
          requiresAuth: true,
          publicAccess: false,
          isPrivateNetwork: true,
        }),
      ).toEqual([]);
    });

    it('fires protect-exports via the unprotectedExportAllowed operand even when exportProtected is true', () => {
      // The `!status.exportProtected || status.unprotectedExportAllowed`
      // guard's second operand is the only one that can be true here.
      const actions = getSecurityHardeningActions({
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        unprotectedExportAllowed: true,
        hasHTTPS: true,
        hasAuditLogging: true,
        requiresAuth: true,
        publicAccess: false,
        isPrivateNetwork: true,
      });
      expect(actions.map((a) => a.key)).toEqual(['protect-exports']);
      expect(actions[0]).toMatchObject({ key: 'protect-exports', severity: 'critical' });
      expect(actions[0].title).toBe('Protect exports');
      expect(actions[0].description).toContain('Backup and export flows');
    });
  });

  describe('getSecurityPostureItems — negative description and enabled=false arms', () => {
    it('renders every "Not configured" / "HTTP only" / "Not enabled" description for an all-false posture', () => {
      const items = getSecurityPostureItems({
        hasAuthentication: false,
        ssoEnabled: false,
        hasProxyAuth: false,
        apiTokenConfigured: false,
        exportProtected: false,
        unprotectedExportAllowed: false,
        hasHTTPS: false,
        hasAuditLogging: false,
        requiresAuth: false,
        publicAccess: false,
        isPrivateNetwork: true,
      });
      // Seven canonical items in declared order.
      expect(items.map((i) => i.key)).toEqual([
        'password',
        'oidc',
        'proxy',
        'token',
        'export',
        'https',
        'audit',
      ]);
      const byKey = Object.fromEntries(items.map((i) => [i.key, i]));
      expect(byKey.password).toMatchObject({
        label: 'Password login',
        enabled: false,
        description: 'Not configured',
        critical: true,
      });
      expect(byKey.oidc).toMatchObject({
        label: 'Single sign-on',
        enabled: false,
        description: 'Not configured',
        critical: false,
      });
      expect(byKey.proxy).toMatchObject({
        label: 'Proxy auth',
        enabled: false,
        description: 'Not configured',
        critical: false,
      });
      expect(byKey.token).toMatchObject({
        label: 'API token',
        enabled: false,
        description: 'Not configured',
        critical: false,
      });
      // exportProtected=false AND unprotectedExportAllowed=false: enabled flag is
      // `exportProtected && !unprotectedExportAllowed` = false, and the
      // description falls through to the else ('Token + passphrase required').
      expect(byKey.export).toMatchObject({
        label: 'Export protection',
        enabled: false,
        description: 'Token + passphrase required',
        critical: true,
      });
      expect(byKey.https).toMatchObject({
        label: 'HTTPS',
        enabled: false,
        description: 'HTTP only',
        critical: true,
      });
      expect(byKey.audit).toMatchObject({
        label: 'Audit log',
        enabled: false,
        description: 'Not enabled',
        critical: false,
      });
    });

    it('marks export as Unprotected (enabled=false) when unprotectedExportAllowed is true', () => {
      const items = getSecurityPostureItems({
        hasAuthentication: true,
        apiTokenConfigured: true,
        exportProtected: true,
        unprotectedExportAllowed: true,
        hasHTTPS: true,
        hasAuditLogging: true,
        requiresAuth: true,
      });
      const exportItem = items.find((i) => i.key === 'export');
      expect(exportItem).toBeDefined();
      // `exportProtected && !unprotectedExportAllowed` = true && !true = false.
      expect(exportItem).toMatchObject({
        enabled: false,
        description: 'Unprotected',
        critical: true,
      });
    });
  });

  describe('getSecurityNetworkAccessSubtitle — public+private overlap arm', () => {
    it('returns "Private network access" when publicAccess is true but isPrivateNetwork is also true', () => {
      // The `!isPrivateNetwork` operand flips the AND to false.
      expect(
        getSecurityNetworkAccessSubtitle({
          hasAuthentication: true,
          apiTokenConfigured: true,
          exportProtected: true,
          hasAuditLogging: true,
          requiresAuth: true,
          publicAccess: true,
          isPrivateNetwork: true,
        }),
      ).toBe('Private network access');
    });

    it('returns "Private network access" when publicAccess is false (isPrivateNetwork operand never evaluated)', () => {
      expect(
        getSecurityNetworkAccessSubtitle({
          hasAuthentication: true,
          apiTokenConfigured: true,
          exportProtected: true,
          hasAuditLogging: true,
          requiresAuth: true,
          publicAccess: false,
          isPrivateNetwork: false,
        }),
      ).toBe('Private network access');
    });
  });
});
