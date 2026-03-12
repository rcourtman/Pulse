export function getAPITokensLoadErrorMessage(): string {
  return 'Failed to load API tokens';
}

import { API_SCOPE_LABELS } from '@/constants/apiScopes';

type APITokenErrorShape = {
  requiredScope?: string;
  status?: number;
  message?: string;
};

export function getAPITokenGenerateErrorMessage(error?: unknown): string {
  if (
    error &&
    typeof error === 'object' &&
    (error as APITokenErrorShape).status === 403 &&
    typeof (error as APITokenErrorShape).message === 'string'
  ) {
    const message = (error as APITokenErrorShape).message.trim();
    if (message.startsWith('Cannot grant scope')) {
      return message;
    }
    if (message === 'missing_scope') {
      const requiredScope = (error as APITokenErrorShape).requiredScope?.trim();
      if (requiredScope) {
        const label = API_SCOPE_LABELS[requiredScope];
        return label
          ? `This token is missing the required scope: ${label} (${requiredScope}).`
          : `This token is missing the required scope: ${requiredScope}.`;
      }
    }
  }

  return 'Failed to generate API token';
}

export function getAPITokenRevokeErrorMessage(): string {
  return 'Failed to revoke API token';
}
