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
  if (error && typeof error === 'object') {
    const typedError = error as APITokenErrorShape;
    if (typedError.status !== 403 || typeof typedError.message !== 'string') {
      return 'Failed to generate API token';
    }

    const message = typedError.message.trim();
    if (message.startsWith('Cannot grant scope')) {
      return message;
    }
    if (message === 'missing_scope') {
      const requiredScope = typedError.requiredScope?.trim();
      if (requiredScope) {
        const label = API_SCOPE_LABELS[requiredScope as keyof typeof API_SCOPE_LABELS];
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
