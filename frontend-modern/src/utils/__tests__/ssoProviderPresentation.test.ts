import { describe, expect, it } from 'vitest';
import {
  getSSOCopySuccessMessage,
  getSSOProviderAddButtonLabel,
  getSSOCertificatePresentation,
  getSSOConnectionTestErrorMessage,
  getSSOConnectionTestFailureMessage,
  getSSOConnectionTestSuccessMessage,
  getSSOProviderCardClass,
  getSSOProviderDeleteErrorMessage,
  getSSOProviderDeleteSuccessMessage,
  getSSOProviderDetailsLoadErrorMessage,
  getSSOProviderEmptyStateDescription,
  getSSOProviderEmptyStateTitle,
  getSSOMetadataFetchErrorMessage,
  getSSOMetadataUrlRequiredMessage,
  getSSOProvidersLoadingState,
  getSSOProvidersLoadErrorMessage,
  getSSOProviderModalTitle,
  getSSOProviderSaveErrorMessage,
  getSSOProviderSaveSuccessMessage,
  getSSOProviderSummary,
  getSSOProviderToggleErrorMessage,
  getSSOProviderToggleSuccessMessage,
  getSSOProviderTypeBadgeClass,
  getSSOProviderTypeLabel,
  getSSOTestResultPresentation,
} from '../ssoProviderPresentation';

describe('ssoProviderPresentation', () => {
  it('formats provider labels and summaries canonically', () => {
    expect(getSSOProviderTypeLabel('oidc')).toBe('OIDC');
    expect(getSSOProviderTypeLabel('saml')).toBe('SAML');
    expect(getSSOProviderAddButtonLabel('oidc')).toBe('Add OIDC');
    expect(getSSOProviderAddButtonLabel('saml', true)).toBe('Add SAML (Pro)');
    expect(getSSOProviderModalTitle(false, 'saml')).toBe('Add SAML Provider');
    expect(getSSOProviderEmptyStateTitle()).toBe('No SSO providers configured');
    expect(getSSOProviderEmptyStateDescription()).toBe(
      'Add an OIDC or SAML provider to get started.',
    );
    expect(getSSOProvidersLoadingState()).toEqual({ text: 'Loading SSO providers…' });
    expect(getSSOProviderTypeBadgeClass()).toContain('bg-surface-hover');
    expect(
      getSSOProviderSummary({
        type: 'oidc',
        oidcIssuerUrl: 'https://login.example.com/realms/pulse',
      }),
    ).toBe('https://login.example.com/realms/pulse');
    expect(
      getSSOProviderSummary({
        type: 'saml',
        samlIdpEntityId: 'https://idp.example.com/entity',
        samlMetadataUrl: 'https://idp.example.com/metadata',
      }),
    ).toBe('https://idp.example.com/entity');
  });

  it('formats provider card and test result tones canonically', () => {
    expect(getSSOProviderCardClass(true)).toContain('bg-surface');
    expect(getSSOProviderCardClass(false)).toContain('opacity-60');
    expect(getSSOTestResultPresentation(true).panelClass).toContain('bg-green-50');
    expect(getSSOTestResultPresentation(false).panelClass).toContain('bg-red-50');
  });

  it('formats provider operational messages canonically', () => {
    expect(getSSOProvidersLoadErrorMessage()).toBe('Unable to load SSO providers.');
    expect(getSSOProviderDetailsLoadErrorMessage()).toBe(
      'Unable to load SSO provider details.',
    );
    expect(getSSOProviderSaveSuccessMessage(true)).toBe('SSO provider has been updated.');
    expect(getSSOProviderSaveSuccessMessage(false)).toBe('SSO provider has been created.');
    expect(getSSOProviderSaveErrorMessage('boom')).toBe('Unable to save the SSO provider: boom');
    expect(getSSOProviderDeleteSuccessMessage()).toBe('SSO provider has been removed.');
    expect(getSSOProviderDeleteErrorMessage()).toBe('Unable to remove the SSO provider.');
    expect(getSSOProviderToggleSuccessMessage(true)).toBe('SSO provider has been enabled.');
    expect(getSSOProviderToggleSuccessMessage(false)).toBe('SSO provider has been disabled.');
    expect(getSSOProviderToggleErrorMessage()).toBe('Unable to update the SSO provider.');
    expect(getSSOCopySuccessMessage('XML')).toBe('XML has been copied to the clipboard.');
    expect(getSSOConnectionTestSuccessMessage()).toBe('Connection test completed successfully.');
    expect(getSSOConnectionTestFailureMessage('Bad certificate')).toBe(
      'Connection test failed: Bad certificate',
    );
    expect(getSSOConnectionTestErrorMessage()).toBe('Unable to run the connection test.');
    expect(getSSOMetadataUrlRequiredMessage()).toBe('Enter an IdP metadata URL.');
    expect(getSSOMetadataFetchErrorMessage('timeout')).toBe(
      'Unable to fetch metadata: timeout',
    );
  });

  it('formats certificate tone and expired label canonically', () => {
    expect(getSSOCertificatePresentation(true).containerClass).toContain('bg-red-100');
    expect(getSSOCertificatePresentation(false).containerClass).toContain('bg-surface-hover');
    expect(getSSOCertificatePresentation(true).expiredLabel).toBe('(Expired!)');
  });
});
