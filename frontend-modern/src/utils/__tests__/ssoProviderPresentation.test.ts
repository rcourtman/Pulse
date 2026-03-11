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
    expect(getSSOProviderEmptyStateDescription()).toContain('Add OIDC');
    expect(getSSOProvidersLoadingState()).toEqual({ text: 'Loading SSO providers...' });
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
    expect(getSSOProvidersLoadErrorMessage()).toBe('Failed to load SSO providers');
    expect(getSSOProviderDetailsLoadErrorMessage()).toBe('Failed to load provider details');
    expect(getSSOProviderSaveSuccessMessage(true)).toBe('Provider updated');
    expect(getSSOProviderSaveSuccessMessage(false)).toBe('Provider created');
    expect(getSSOProviderSaveErrorMessage('boom')).toBe('Failed to save provider: boom');
    expect(getSSOProviderDeleteSuccessMessage()).toBe('Provider deleted');
    expect(getSSOProviderDeleteErrorMessage()).toBe('Failed to delete provider');
    expect(getSSOProviderToggleSuccessMessage(true)).toBe('Provider enabled');
    expect(getSSOProviderToggleSuccessMessage(false)).toBe('Provider disabled');
    expect(getSSOProviderToggleErrorMessage()).toBe('Failed to update provider');
    expect(getSSOCopySuccessMessage('XML')).toBe('XML copied to clipboard');
    expect(getSSOConnectionTestSuccessMessage()).toBe('Connection test successful');
    expect(getSSOConnectionTestFailureMessage('Bad certificate')).toBe(
      'Connection test failed: Bad certificate',
    );
    expect(getSSOConnectionTestErrorMessage()).toBe('Failed to test connection');
    expect(getSSOMetadataUrlRequiredMessage()).toBe('Please enter an IdP Metadata URL');
    expect(getSSOMetadataFetchErrorMessage('timeout')).toBe('Failed to fetch metadata: timeout');
  });

  it('formats certificate tone and expired label canonically', () => {
    expect(getSSOCertificatePresentation(true).containerClass).toContain('bg-red-100');
    expect(getSSOCertificatePresentation(false).containerClass).toContain('bg-surface-hover');
    expect(getSSOCertificatePresentation(true).expiredLabel).toBe('(Expired!)');
  });
});
