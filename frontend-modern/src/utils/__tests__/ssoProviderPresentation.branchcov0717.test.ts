import { describe, expect, it } from 'vitest';

import {
  getSSOCertificatePresentation,
  getSSOCopySuccessMessage,
  getSSOConnectionTestErrorMessage,
  getSSOConnectionTestFailureMessage,
  getSSOConnectionTestSuccessMessage,
  getSSOProviderAddButtonLabel,
  getSSOProviderCardClass,
  getSSOProviderDeleteErrorMessage,
  getSSOProviderDeleteSuccessMessage,
  getSSOProviderDetailsLoadErrorMessage,
  getSSOProviderEmptyStateDescription,
  getSSOProviderEmptyStateTitle,
  getSSOMetadataFetchErrorMessage,
  getSSOMetadataUrlRequiredMessage,
  getSSOProviderModalTitle,
  getSSOProviderSaveErrorMessage,
  getSSOProviderSaveSuccessMessage,
  getSSOProviderSummary,
  getSSOProvidersLoadingState,
  getSSOProvidersLoadErrorMessage,
  getSSOProviderToggleErrorMessage,
  getSSOProviderToggleSuccessMessage,
  getSSOProviderTypeBadgeClass,
  getSSOProviderTypeLabel,
  getSSOSamlFeatureGateCopy,
  getSSOTestResultPresentation,
  type SSOProviderSummaryLike,
  type SSOProviderType,
} from '@/utils/ssoProviderPresentation';

// This supplemental suite targets the branch arms the canonical
// `ssoProviderPresentation.test.ts` leaves open: the `editing === true` arm of
// `getSSOProviderModalTitle`, the falsy `oidcIssuerUrl || ''` arm, and both
// falsy arms of the nested `samlIdpEntityId || samlMetadataUrl || ''` chain in
// `getSSOProviderSummary`. It also pins the full concrete return values and
// `String(error)` coercion behaviour that the sibling suite only spot-checks.

describe('ssoProviderPresentation branch coverage (supplemental)', () => {
  describe('getSSOProviderTypeLabel', () => {
    it('returns "OIDC" on the oidc arm of the ternary', () => {
      expect(getSSOProviderTypeLabel('oidc')).toBe('OIDC');
    });

    it('returns "SAML" on the non-oidc arm of the ternary', () => {
      expect(getSSOProviderTypeLabel('saml')).toBe('SAML');
    });

    it('routes a deliberately-invalid type through the else arm onto "SAML"', () => {
      // The ternary is `type === 'oidc' ? 'OIDC' : 'SAML'`; anything that is not
      // strictly the string 'oidc' falls to the else arm regardless of the
      // declared union type.
      expect(getSSOProviderTypeLabel('unknown' as unknown as SSOProviderType)).toBe('SAML');
    });
  });

  describe('getSSOProviderAddButtonLabel', () => {
    it('composes "Add OIDC" from the oidc label arm', () => {
      expect(getSSOProviderAddButtonLabel('oidc')).toBe('Add OIDC');
    });

    it('composes "Add SAML" from the saml label arm', () => {
      expect(getSSOProviderAddButtonLabel('saml')).toBe('Add SAML');
    });
  });

  describe('getSSOProviderModalTitle', () => {
    it('uses the "Edit" arm when editing is true (oidc label arm)', () => {
      // Covers the previously-open `editing ? 'Edit' : 'Add'` true arm, and the
      // oidc arm of the nested getSSOProviderTypeLabel call.
      expect(getSSOProviderModalTitle(true, 'oidc')).toBe('Edit OIDC Provider');
    });

    it('uses the "Edit" arm with the saml label', () => {
      expect(getSSOProviderModalTitle(true, 'saml')).toBe('Edit SAML Provider');
    });

    it('uses the "Add" arm with the oidc label', () => {
      // The sibling suite only pins (false, 'saml'); this pins the (false,
      // 'oidc') combination.
      expect(getSSOProviderModalTitle(false, 'oidc')).toBe('Add OIDC Provider');
    });
  });

  describe('getSSOSamlFeatureGateCopy', () => {
    it('returns the full feature-gate copy verbatim', () => {
      expect(getSSOSamlFeatureGateCopy()).toStrictEqual({
        title: 'Single Sign-On',
        subtitle: 'Single Sign-On',
        body: 'OIDC, SAML, and multi-provider SSO are included with Community and higher tiers.',
      });
    });
  });

  describe('getSSOProviderEmptyStateTitle / Description / LoadingState', () => {
    it('returns the canonical empty-state copy and loading object', () => {
      expect(getSSOProviderEmptyStateTitle()).toBe('No SSO providers configured');
      expect(getSSOProviderEmptyStateDescription()).toBe(
        'Add an OIDC or SAML provider to get started.',
      );
      expect(getSSOProvidersLoadingState()).toStrictEqual({ text: 'Loading SSO providers…' });
    });
  });

  describe('getSSOProvidersLoadErrorMessage / DetailsLoadErrorMessage', () => {
    it('returns the canonical load-error strings', () => {
      expect(getSSOProvidersLoadErrorMessage()).toBe('Unable to load SSO providers.');
      expect(getSSOProviderDetailsLoadErrorMessage()).toBe('Unable to load SSO provider details.');
    });
  });

  describe('getSSOProviderSaveSuccessMessage', () => {
    it('returns the "updated" copy on the isEdit true arm', () => {
      expect(getSSOProviderSaveSuccessMessage(true)).toBe('SSO provider has been updated.');
    });

    it('returns the "created" copy on the isEdit false arm', () => {
      expect(getSSOProviderSaveSuccessMessage(false)).toBe('SSO provider has been created.');
    });
  });

  describe('getSSOProviderSaveErrorMessage', () => {
    it('coerces an undefined error to the literal string "undefined" via String()', () => {
      // `String(undefined)` => 'undefined' — documents the runtime behaviour of
      // the optional `error` parameter when callers omit it.
      expect(getSSOProviderSaveErrorMessage()).toBe('Unable to save the SSO provider: undefined');
    });

    it('coerces null to "null"', () => {
      expect(getSSOProviderSaveErrorMessage(null)).toBe('Unable to save the SSO provider: null');
    });

    it('coerces a plain object to "[object Object]"', () => {
      expect(getSSOProviderSaveErrorMessage({ status: 500 })).toBe(
        'Unable to save the SSO provider: [object Object]',
      );
    });

    it('coerces an Error instance to "Error: <message>"', () => {
      expect(getSSOProviderSaveErrorMessage(new Error('boom'))).toBe(
        'Unable to save the SSO provider: Error: boom',
      );
    });

    it('coerces a number to its decimal string form', () => {
      expect(getSSOProviderSaveErrorMessage(42)).toBe('Unable to save the SSO provider: 42');
    });
  });

  describe('getSSOProviderDeleteSuccessMessage / DeleteErrorMessage / ToggleErrorMessage', () => {
    it('returns the canonical delete/toggle strings verbatim', () => {
      expect(getSSOProviderDeleteSuccessMessage()).toBe('SSO provider has been removed.');
      expect(getSSOProviderDeleteErrorMessage()).toBe('Unable to remove the SSO provider.');
      expect(getSSOProviderToggleErrorMessage()).toBe('Unable to update the SSO provider.');
    });
  });

  describe('getSSOProviderToggleSuccessMessage', () => {
    it('returns the "enabled" copy on the true arm', () => {
      expect(getSSOProviderToggleSuccessMessage(true)).toBe('SSO provider has been enabled.');
    });

    it('returns the "disabled" copy on the false arm', () => {
      expect(getSSOProviderToggleSuccessMessage(false)).toBe('SSO provider has been disabled.');
    });
  });

  describe('getSSOCopySuccessMessage', () => {
    it('interpolates the label into the clipboard confirmation', () => {
      expect(getSSOCopySuccessMessage('XML')).toBe('XML has been copied to the clipboard.');
    });

    it('renders an empty label verbatim (leading space kept by the template literal)', () => {
      // Edge case: template literal does not trim, so an empty label yields a
      // leading space then the fixed copy.
      expect(getSSOCopySuccessMessage('')).toBe(' has been copied to the clipboard.');
    });
  });

  describe('getSSOConnectionTestSuccessMessage / FailureMessage / ErrorMessage', () => {
    it('returns the canonical connection-test strings', () => {
      expect(getSSOConnectionTestSuccessMessage()).toBe('Connection test completed successfully.');
      expect(getSSOConnectionTestFailureMessage('Bad certificate')).toBe(
        'Connection test failed: Bad certificate',
      );
      expect(getSSOConnectionTestErrorMessage()).toBe('Unable to run the connection test.');
    });

    it('renders an empty failure message after the colon', () => {
      expect(getSSOConnectionTestFailureMessage('')).toBe('Connection test failed: ');
    });
  });

  describe('getSSOMetadataUrlRequiredMessage / MetadataFetchErrorMessage', () => {
    it('returns the canonical metadata-required string', () => {
      expect(getSSOMetadataUrlRequiredMessage()).toBe('Enter an IdP metadata URL.');
    });

    it('coerces an undefined error to "undefined" via String()', () => {
      expect(getSSOMetadataFetchErrorMessage()).toBe('Unable to fetch metadata: undefined');
    });

    it('coerces null to "null"', () => {
      expect(getSSOMetadataFetchErrorMessage(null)).toBe('Unable to fetch metadata: null');
    });

    it('coerces a plain object to "[object Object]"', () => {
      expect(getSSOMetadataFetchErrorMessage({ code: 'ECONNRESET' })).toBe(
        'Unable to fetch metadata: [object Object]',
      );
    });
  });

  describe('getSSOProviderSummary', () => {
    it('returns the oidcIssuerUrl on the oidc truthy arm', () => {
      expect(
        getSSOProviderSummary({
          type: 'oidc',
          oidcIssuerUrl: 'https://login.example.com/realms/pulse',
        }),
      ).toBe('https://login.example.com/realms/pulse');
    });

    it('falls back to "" when an oidc provider has no issuer url (|| falsy arm)', () => {
      // Covers the previously-open falsy arm of `provider.oidcIssuerUrl || ''`.
      expect(getSSOProviderSummary({ type: 'oidc' })).toBe('');
    });

    it('falls back to "" when the oidc issuer url is the empty string', () => {
      // Same `|| ''` falsy arm, exercised with a present-but-falsy value.
      expect(getSSOProviderSummary({ type: 'oidc', oidcIssuerUrl: '' })).toBe('');
    });

    it('prefers samlIdpEntityId on the saml arm when it is set', () => {
      expect(
        getSSOProviderSummary({
          type: 'saml',
          samlIdpEntityId: 'https://idp.example.com/entity',
          samlMetadataUrl: 'https://idp.example.com/metadata',
        }),
      ).toBe('https://idp.example.com/entity');
    });

    it('falls back to samlMetadataUrl when samlIdpEntityId is absent (first || falsy arm)', () => {
      // Covers the previously-open falsy arm of the first `||` and the truthy
      // arm of the second `||`.
      expect(
        getSSOProviderSummary({
          type: 'saml',
          samlMetadataUrl: 'https://idp.example.com/metadata',
        }),
      ).toBe('https://idp.example.com/metadata');
    });

    it('falls back to samlMetadataUrl when samlIdpEntityId is the empty string', () => {
      // Same first-|| falsy arm, exercised with a present-but-falsy entity id.
      expect(
        getSSOProviderSummary({
          type: 'saml',
          samlIdpEntityId: '',
          samlMetadataUrl: 'https://idp.example.com/metadata',
        }),
      ).toBe('https://idp.example.com/metadata');
    });

    it('falls back to "" when a saml provider has neither id nor metadata url (final || falsy arm)', () => {
      // Covers the previously-open final `|| ''` falsy arm of the nested chain.
      expect(getSSOProviderSummary({ type: 'saml' })).toBe('');
    });

    it('falls back to "" when both saml fields are the empty string', () => {
      // Final `|| ''` falsy arm, exercised with present-but-falsy values.
      expect(
        getSSOProviderSummary({
          type: 'saml',
          samlIdpEntityId: '',
          samlMetadataUrl: '',
        }),
      ).toBe('');
    });

    it('treats a non-oidc/non-saml type as the saml summary path', () => {
      // The `if (provider.type === 'oidc')` guard only special-cases oidc;
      // anything else (including an invalid type) flows to the saml fallback
      // chain. With nothing set, that resolves to "".
      expect(
        getSSOProviderSummary({
          type: 'unknown' as unknown as SSOProviderType,
          samlMetadataUrl: 'https://only.example.com/metadata',
        } as unknown as SSOProviderSummaryLike),
      ).toBe('https://only.example.com/metadata');
    });
  });

  describe('getSSOProviderCardClass', () => {
    it('returns the full enabled-card class string', () => {
      expect(getSSOProviderCardClass(true)).toBe('p-4 rounded-md border bg-surface border-border');
    });

    it('returns the full disabled-card class string', () => {
      expect(getSSOProviderCardClass(false)).toBe(
        'p-4 rounded-md border bg-surface-alt border-border opacity-60',
      );
    });
  });

  describe('getSSOProviderTypeBadgeClass', () => {
    it('returns the canonical badge class string verbatim', () => {
      expect(getSSOProviderTypeBadgeClass()).toBe(
        'px-1.5 py-0.5 text-xs font-medium rounded bg-surface-hover',
      );
    });
  });

  describe('getSSOTestResultPresentation', () => {
    it('returns the full success presentation object', () => {
      expect(getSSOTestResultPresentation(true)).toStrictEqual({
        panelClass:
          'p-4 rounded-md border bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800',
        iconClass: 'w-5 h-5 text-emerald-500 dark:text-emerald-400 flex-shrink-0 mt-0.5',
        titleClass: 'text-sm font-medium text-green-800 dark:text-green-200',
        errorClass: 'text-xs text-red-600 dark:text-red-400 mt-1',
      });
    });

    it('returns the full failure presentation object', () => {
      expect(getSSOTestResultPresentation(false)).toStrictEqual({
        panelClass:
          'p-4 rounded-md border bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800',
        iconClass: 'w-5 h-5 text-rose-500 dark:text-rose-400 flex-shrink-0 mt-0.5',
        titleClass: 'text-sm font-medium text-red-800 dark:text-red-200',
        errorClass: 'text-xs text-red-600 dark:text-red-400 mt-1',
      });
    });
  });

  describe('getSSOCertificatePresentation', () => {
    it('returns the full expired presentation object', () => {
      expect(getSSOCertificatePresentation(true)).toStrictEqual({
        containerClass:
          'text-xs px-2 py-1 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300',
        expiredLabelClass: 'ml-1 text-red-600 dark:text-red-400 font-medium',
        expiredLabel: '(Expired!)',
      });
    });

    it('returns the full valid presentation object', () => {
      expect(getSSOCertificatePresentation(false)).toStrictEqual({
        containerClass: 'text-xs px-2 py-1 rounded bg-surface-hover text-base-content',
        expiredLabelClass: 'ml-1 text-red-600 dark:text-red-400 font-medium',
        expiredLabel: '(Expired!)',
      });
    });
  });
});
