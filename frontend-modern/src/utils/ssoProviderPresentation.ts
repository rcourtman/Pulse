export type SSOProviderType = 'oidc' | 'saml';

export interface SSOProviderSummaryLike {
  type: SSOProviderType;
  oidcIssuerUrl?: string;
  samlIdpEntityId?: string;
  samlMetadataUrl?: string;
}

export function getSSOProviderTypeLabel(type: SSOProviderType): string {
  return type === 'oidc' ? 'OIDC' : 'SAML';
}

export function getSSOProviderAddButtonLabel(type: SSOProviderType, gated = false): string {
  return gated
    ? `Add ${getSSOProviderTypeLabel(type)} (Pro)`
    : `Add ${getSSOProviderTypeLabel(type)}`;
}

export function getSSOProviderModalTitle(editing: boolean, type: SSOProviderType): string {
  return `${editing ? 'Edit' : 'Add'} ${getSSOProviderTypeLabel(type)} Provider`;
}

export function getSSOProviderEmptyStateDescription(): string {
  return 'Add an OIDC or SAML provider to get started.';
}

export function getSSOProviderEmptyStateTitle(): string {
  return 'No SSO providers configured';
}

export function getSSOProvidersLoadingState() {
  return {
    text: 'Loading SSO providers…',
  } as const;
}

export function getSSOProvidersLoadErrorMessage(): string {
  return 'Unable to load SSO providers.';
}

export function getSSOProviderDetailsLoadErrorMessage(): string {
  return 'Unable to load SSO provider details.';
}

export function getSSOProviderSaveSuccessMessage(isEdit: boolean): string {
  return isEdit ? 'SSO provider has been updated.' : 'SSO provider has been created.';
}

export function getSSOProviderSaveErrorMessage(error?: unknown): string {
  return `Unable to save the SSO provider: ${String(error)}`;
}

export function getSSOProviderDeleteSuccessMessage(): string {
  return 'SSO provider has been removed.';
}

export function getSSOProviderDeleteErrorMessage(): string {
  return 'Unable to remove the SSO provider.';
}

export function getSSOProviderToggleSuccessMessage(enabled: boolean): string {
  return enabled ? 'SSO provider has been enabled.' : 'SSO provider has been disabled.';
}

export function getSSOProviderToggleErrorMessage(): string {
  return 'Unable to update the SSO provider.';
}

export function getSSOCopySuccessMessage(label: string): string {
  return `${label} has been copied to the clipboard.`;
}

export function getSSOConnectionTestSuccessMessage(): string {
  return 'Connection test completed successfully.';
}

export function getSSOConnectionTestFailureMessage(message: string): string {
  return `Connection test failed: ${message}`;
}

export function getSSOConnectionTestErrorMessage(): string {
  return 'Unable to run the connection test.';
}

export function getSSOMetadataUrlRequiredMessage(): string {
  return 'Enter an IdP metadata URL.';
}

export function getSSOMetadataFetchErrorMessage(error?: unknown): string {
  return `Unable to fetch metadata: ${String(error)}`;
}

export function getSSOProviderSummary(provider: SSOProviderSummaryLike): string {
  if (provider.type === 'oidc') {
    return provider.oidcIssuerUrl || '';
  }

  return provider.samlIdpEntityId || provider.samlMetadataUrl || '';
}

export function getSSOProviderCardClass(enabled: boolean): string {
  return enabled
    ? 'p-4 rounded-md border bg-surface border-border'
    : 'p-4 rounded-md border bg-surface-alt border-border opacity-60';
}

export function getSSOProviderTypeBadgeClass(): string {
  return 'px-1.5 py-0.5 text-xs font-medium rounded bg-surface-hover';
}

export function getSSOTestResultPresentation(success: boolean) {
  if (success) {
    return {
      panelClass: 'p-4 rounded-md border bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800',
      iconClass: 'w-5 h-5 text-emerald-500 dark:text-emerald-400 flex-shrink-0 mt-0.5',
      titleClass: 'text-sm font-medium text-green-800 dark:text-green-200',
      errorClass: 'text-xs text-red-600 dark:text-red-400 mt-1',
    };
  }

  return {
    panelClass: 'p-4 rounded-md border bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800',
    iconClass: 'w-5 h-5 text-rose-500 dark:text-rose-400 flex-shrink-0 mt-0.5',
    titleClass: 'text-sm font-medium text-red-800 dark:text-red-200',
    errorClass: 'text-xs text-red-600 dark:text-red-400 mt-1',
  };
}

export function getSSOCertificatePresentation(isExpired: boolean) {
  return {
    containerClass: isExpired
      ? 'text-xs px-2 py-1 rounded bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300'
      : 'text-xs px-2 py-1 rounded bg-surface-hover text-base-content',
    expiredLabelClass: 'ml-1 text-red-600 dark:text-red-400 font-medium',
    expiredLabel: '(Expired!)',
  };
}
