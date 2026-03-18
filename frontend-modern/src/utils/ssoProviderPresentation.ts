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
  return 'Click "Add OIDC" or "Add SAML" to get started';
}

export function getSSOProviderEmptyStateTitle(): string {
  return 'No SSO providers configured';
}

export function getSSOProvidersLoadingState() {
  return {
    text: 'Loading SSO providers...',
  } as const;
}

export function getSSOProvidersLoadErrorMessage(): string {
  return 'Failed to load SSO providers';
}

export function getSSOProviderDetailsLoadErrorMessage(): string {
  return 'Failed to load provider details';
}

export function getSSOProviderSaveSuccessMessage(isEdit: boolean): string {
  return isEdit ? 'Provider updated' : 'Provider created';
}

export function getSSOProviderSaveErrorMessage(error?: unknown): string {
  return `Failed to save provider: ${String(error)}`;
}

export function getSSOProviderDeleteSuccessMessage(): string {
  return 'Provider deleted';
}

export function getSSOProviderDeleteErrorMessage(): string {
  return 'Failed to delete provider';
}

export function getSSOProviderToggleSuccessMessage(enabled: boolean): string {
  return enabled ? 'Provider enabled' : 'Provider disabled';
}

export function getSSOProviderToggleErrorMessage(): string {
  return 'Failed to update provider';
}

export function getSSOCopySuccessMessage(label: string): string {
  return `${label} copied to clipboard`;
}

export function getSSOConnectionTestSuccessMessage(): string {
  return 'Connection test successful';
}

export function getSSOConnectionTestFailureMessage(message: string): string {
  return `Connection test failed: ${message}`;
}

export function getSSOConnectionTestErrorMessage(): string {
  return 'Failed to test connection';
}

export function getSSOMetadataUrlRequiredMessage(): string {
  return 'Please enter an IdP Metadata URL';
}

export function getSSOMetadataFetchErrorMessage(error?: unknown): string {
  return `Failed to fetch metadata: ${String(error)}`;
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
